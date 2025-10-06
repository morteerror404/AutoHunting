package api

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html" // Adicionado ao go.mod
)

// -----------------------------
// Estrutura do resultado final
// -----------------------------
type SiteResult struct {
	Input       string `json:"input"`
	ResolvedURL string `json:"resolved_url"`
	HTTPStatus  int    `json:"http_status"`
	Title       string `json:"title"`
	ContentLen  int64  `json:"content_length"`
	Error       string `json:"error,omitempty"`
	CheckedAt   string `json:"checked_at"`
	ElapsedMS   int64  `json:"elapsed_ms"`
}

// -----------------------------
// Estrutura para tokens.json
// -----------------------------
type Tokens struct {
	HackerOne struct {
		Username string `json:"username"`
		ApiKey   string `json:"api_key"`
	} `json:"hackerone"`
	Bugcrowd struct {
		Token string `json:"token"`
	} `json:"bugcrowd"`
	Intigriti struct {
		Token string `json:"token"`
	} `json:"intigriti"`
	YesWeHack struct {
		Token string `json:"token"`
	} `json:"yeswehack"`
}

// -----------------------------
// Helpers
// -----------------------------
func ensureScheme(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return raw
	}
	return "https://" + raw
}

// parse <title> do HTML
func parseTitle(r io.Reader) (string, error) {
	z := html.NewTokenizer(r)
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			if z.Err() == io.EOF {
				return "", nil
			}
			return "", z.Err()
		case html.StartTagToken:
			t := z.Token()
			if t.Data == "title" {
				tt2 := z.Next()
				if tt2 == html.TextToken {
					return strings.TrimSpace(string(z.Text())), nil
				}
				return "", nil
			}
		}
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

// -----------------------------
// Workers HTTP
// -----------------------------
func worker(id int, jobs <-chan string, results chan<- SiteResult, client *http.Client, path string, delay time.Duration, wg *sync.WaitGroup) {
	defer wg.Done()
	for raw := range jobs {
		start := time.Now()
		input := raw
		raw = ensureScheme(raw)
		target := raw
		if path != "" {
			if !strings.HasPrefix(path, "/") {
				path = "/" + path
			}
			u, err := url.Parse(raw)
			if err == nil {
				u.Path = strings.TrimSuffix(u.Path, "/") + path
				target = u.String()
			} else {
				target = raw + path
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), client.Timeout)
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; bugsites-check/1.0)")

		res := SiteResult{
			Input:     input,
			CheckedAt: time.Now().UTC().Format(time.RFC3339),
		}

		startReq := time.Now()
		resp, err := client.Do(req)
		elapsed := time.Since(startReq)
		res.ElapsedMS = elapsed.Milliseconds()
		if err != nil {
			res.Error = err.Error()
			results <- res
			cancel()
			time.Sleep(delay)
			continue
		}

		res.HTTPStatus = resp.StatusCode
		res.ResolvedURL = resp.Request.URL.String()
		if resp.ContentLength >= 0 {
			res.ContentLen = resp.ContentLength
		}

		limited := io.LimitReader(resp.Body, 200*1024) // 200 KB
		title, perr := parseTitle(limited)
		if perr != nil {
			res.Error = perr.Error()
		} else {
			res.Title = title
		}
		resp.Body.Close()
		cancel()
		res.CheckedAt = time.Now().UTC().Format(time.RFC3339)
		results <- res
		time.Sleep(delay)
		_ = start // placeholder if you later want to calculate overall elapsed
	}
}

// -----------------------------
// HackerOne: programas ativos + structured scopes
// -----------------------------

// fetchHackerOneProgramHandles busca handles dos programas acessíveis ao hacker (filtra public_mode)
func fetchHackerOneProgramHandles(username, apiKey string) ([]string, error) {
	client := &http.Client{Timeout: 20 * time.Second}
	req, _ := http.NewRequest("GET", "https://api.hackerone.com/v1/hackers/programs", nil)
	req.SetBasicAuth(username, apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HackerOne status code %d", resp.StatusCode)
	}

	var data struct {
		Data []struct {
			Attributes struct {
				Handle string `json:"handle"`
				State  string `json:"state"`
			} `json:"attributes"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var handles []string
	for _, it := range data.Data {
		if strings.EqualFold(it.Attributes.State, "public_mode") && it.Attributes.Handle != "" {
			handles = append(handles, it.Attributes.Handle)
		}
	}
	return handles, nil
}

// fetchHackerOneStructuredScopes consulta /structured_scopes e retorna identifiers filtrados (eligible_for_submission, eligible_for_bounty e asset types comuns)
func fetchHackerOneStructuredScopes(handle, username, apiKey string) ([]string, error) {
	endpoint := fmt.Sprintf("https://api.hackerone.com/v1/hackers/programs/%s/structured_scopes", url.PathEscape(handle))
	client := &http.Client{Timeout: 20 * time.Second}
	req, _ := http.NewRequest("GET", endpoint, nil)
	req.SetBasicAuth(username, apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HackerOne structured_scopes status code %d for %s", resp.StatusCode, handle)
	}

	var data struct {
		Data []struct {
			Attributes struct {
				Identifier            string `json:"identifier"`
				EligibleForSubmission bool   `json:"eligible_for_submission"`
				EligibleForBounty     bool   `json:"eligible_for_bounty"`
				AssetType             string `json:"asset_type"`
			} `json:"attributes"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var out []string
	for _, item := range data.Data {
		attr := item.Attributes
		if !attr.EligibleForSubmission || !attr.EligibleForBounty {
			continue
		}
		if attr.AssetType == "" {
			// aceitar mesmo se não tiver asset_type, mas normalmente vem
			out = append(out, attr.Identifier)
			continue
		}
		// filtrar por tipos úteis (Domain, Url, Cidr)
		assetTypeLower := strings.ToLower(attr.AssetType)
		if strings.Contains(assetTypeLower, "domain") || strings.Contains(assetTypeLower, "url") || strings.Contains(assetTypeLower, "cidr") {
			out = append(out, attr.Identifier)
		}
	}
	return out, nil
}

// helper: escreve slice de strings em arquivo (uma linha por item)
func writeLinesToFile(path string, lines []string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		if _, err := w.WriteString(l + "\n"); err != nil {
			return err
		}
	}
	return w.Flush()
}

// writeLinesRotate renomeia o arquivo existente para <basename>-old.txt e cria um novo.
func writeLinesRotate(path string, lines []string) error {
	if _, err := os.Stat(path); err == nil {
		ext := filepath.Ext(path)
		oldPath := strings.TrimSuffix(path, ext) + "-old" + ext
		if _, err := os.Stat(oldPath); err == nil {
			if err := os.Remove(oldPath); err != nil {
				return fmt.Errorf("erro removendo arquivo antigo %s: %w", oldPath, err)
			}
		}
		if err := os.Rename(path, oldPath); err != nil {
			return fmt.Errorf("erro renomeando %s para %s: %w", path, oldPath, err)
		}
	}
	return writeLinesToFile(path, lines)
}

// RunRequestAPI é o ponto de entrada para o maestro.
// Ele busca escopos para uma plataforma específica e os salva em um arquivo.
func RunRequestAPI(apiDirtResultsPath string, platform string, tokens Tokens) error {
	var allScopes []string
	var err error

	switch platform {
	case "hackerone":
		if tokens.HackerOne.Username == "" || tokens.HackerOne.ApiKey == "" {
			return fmt.Errorf("credenciais do HackerOne não fornecidas")
		}
		fmt.Fprintf(os.Stderr, "[hackerone] buscando handles de programas...\n")
		handles, errH := fetchHackerOneProgramHandles(tokens.HackerOne.Username, tokens.HackerOne.ApiKey)
		if errH != nil {
			return fmt.Errorf("erro ao buscar handles do HackerOne: %w", errH)
		}
		fmt.Fprintf(os.Stderr, "[hackerone] %d handles encontrados. buscando escopos...\n", len(handles))

		for _, h := range handles {
			scopes, errS := fetchHackerOneStructuredScopes(h, tokens.HackerOne.Username, tokens.HackerOne.ApiKey)
			if errS != nil {
				fmt.Fprintf(os.Stderr, "[hackerone] AVISO: falha ao buscar escopos para o handle '%s': %v\n", h, errS)
				continue // Continua para o próximo handle
			}
			allScopes = append(allScopes, scopes...)
		}

	// Adicione casos para 'bugcrowd', 'intigriti', 'yeswehack' aqui quando as funções de fetch forem implementadas.
	// case "bugcrowd":
	// 	// allScopes, err = fetchBugcrowdScopes(tokens.Bugcrowd.Token)

	default:
		return fmt.Errorf("plataforma '%s' não suportada para coleta de API", platform)
	}

	if err != nil {
		return fmt.Errorf("erro ao buscar escopos para a plataforma '%s': %w", platform, err)
	}

	if len(allScopes) == 0 {
		fmt.Fprintf(os.Stderr, "AVISO: Nenhum escopo encontrado para a plataforma '%s'.\n", platform)
		return nil // Não é um erro fatal, mas nada foi encontrado.
	}

	// Garante que o diretório de saída exista
	if err := os.MkdirAll(filepath.Dir(apiDirtResultsPath), 0755); err != nil {
		return fmt.Errorf("erro ao criar diretório de saída '%s': %w", filepath.Dir(apiDirtResultsPath), err)
	}

	// Salva os escopos no arquivo de resultados brutos especificado pelo maestro
	if err := writeLinesToFile(apiDirtResultsPath, allScopes); err != nil {
		return fmt.Errorf("erro ao salvar escopos em '%s': %w", apiDirtResultsPath, err)
	}

	fmt.Printf("Sucesso! %d escopos da plataforma '%s' salvos em: %s\n", len(allScopes), platform, apiDirtResultsPath)
	return nil
}
