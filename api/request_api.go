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
// Estruturas de Dados
// -----------------------------

// SiteResult define a estrutura de dados para o resultado da verificação de um único site.
// Esta estrutura é usada para serializar os resultados em formato JSON.
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

// Tokens espelha a estrutura do arquivo `tokens.json`, armazenando as credenciais
// de API necessárias para autenticar nas plataformas de Bug Bounty.
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

// ensureScheme garante que uma URL string tenha um esquema (http:// ou https://).
// Se nenhum esquema estiver presente, adiciona "https://" como padrão.
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

// parseTitle extrai o conteúdo da tag <title> de um corpo de resposta HTML.
// Ele lê de um io.Reader e para assim que encontra o título ou o fim do arquivo.
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

// truncate limita uma string a um número `n` de caracteres, adicionando "..." se for cortada.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

// -----------------------------
// Workers HTTP
// -----------------------------

// worker é a função executada por cada goroutine concorrente para processar jobs.
// Ele recebe URLs de um canal `jobs`, realiza a requisição HTTP, processa a resposta,
// e envia o resultado (SiteResult) para o canal `results`.
// Respeita um `delay` entre as requisições para evitar rate limiting.
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

// fetchHackerOneProgramHandles busca na API do HackerOne a lista de "handles" (identificadores únicos)
// de todos os programas públicos. Ele se autentica usando as credenciais fornecidas e filtra
// os programas que estão em "public_mode".
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

// fetchHackerOneStructuredScopes, para um determinado `handle` de programa, consulta o endpoint
// de escopos estruturados da API do HackerOne.
// A função filtra os ativos para retornar apenas aqueles que são:
// 1. Elegíveis para submissão (`eligible_for_submission`).
// 2. Elegíveis para recompensa (`eligible_for_bounty`).
// 3. De tipos relevantes para automação (`URL`, `DOMAIN`, `CIDR`).
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

// writeLinesToFile é uma função auxiliar que escreve um slice de strings em um arquivo,
// com cada string em uma nova linha. Ele cria o arquivo se não existir e sobrescreve o conteúdo.
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

// writeLinesRotate escreve linhas em um arquivo, mas antes renomeia qualquer arquivo
// existente com o mesmo nome para `<basename>-old.txt`. Isso serve como um mecanismo
// simples de backup da execução anterior.
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
// Ele orquestra a coleta de escopos para uma plataforma específica (`platform`)
// usando as credenciais fornecidas (`tokens`) e salva os resultados brutos
// no caminho especificado (`apiDirtResultsPath`).
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

		// TODO: Paralelizar a busca de escopos. Atualmente, é feita sequencialmente para cada handle.
		// Usar goroutines e um WaitGroup para buscar escopos de múltiplos handles simultaneamente pode acelerar muito o processo.
		// É preciso garantir que a adição ao slice 'allScopes' seja segura para concorrência (usando um mutex).
		for _, h := range handles {
			scopes, errS := fetchHackerOneStructuredScopes(h, tokens.HackerOne.Username, tokens.HackerOne.ApiKey)
			if errS != nil {
				fmt.Fprintf(os.Stderr, "[hackerone] AVISO: falha ao buscar escopos para o handle '%s': %v\n", h, errS)
				continue // Continua para o próximo handle
			}
			allScopes = append(allScopes, scopes...)
		}

	// TODO: Implementar a coleta de escopos para as outras plataformas.
	// case "bugcrowd":
	// 	// allScopes, err = fetchBugcrowdScopes(tokens.Bugcrowd.Token)
	// case "intigriti":
	// 	// allScopes, err = fetchIntigritiScopes(tokens.Intigriti.Token)
	// case "yeswehack":
	// 	// allScopes, err = fetchYesWeHackScopes(tokens.YesWeHack.Token)

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
