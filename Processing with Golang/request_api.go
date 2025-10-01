package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"golang.org/x/net/html"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
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
type APIConfig struct {
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
// APIs de Bug Bounty
// -----------------------------

// HackerOne
func fetchHackerOneScopes(username, apiKey string) ([]string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
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
			} `json:"attributes"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var targets []string
	for _, prog := range data.Data {
		if prog.Attributes.Handle != "" {
			targets = append(targets, fmt.Sprintf("%s.hackerone.com", prog.Attributes.Handle))
		}
	}
	return targets, nil
}

// Bugcrowd
func fetchBugcrowdScopes(token string) ([]string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest("GET", "https://api.bugcrowd.com/v2/programs", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Bugcrowd status code %d", resp.StatusCode)
	}

	var data struct {
		Programs []struct {
			Name   string   `json:"name"`
			Scopes []string `json:"targets"` // depende da API real
		} `json:"programs"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var targets []string
	for _, p := range data.Programs {
		targets = append(targets, p.Scopes...)
	}
	return targets, nil
}

// Intigriti
func fetchIntigritiScopes(token string) ([]string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest("GET", "https://api.intigriti.com/v1/programs", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 { return nil, fmt.Errorf("Intigriti status code %d", resp.StatusCode) }

	var data struct {
		Programs []struct {
			Name   string   `json:"name"`
			Scopes []string `json:"targets"` // ajuste conforme API real
		} `json:"programs"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil { return nil, err }

	var targets []string
	for _, p := range data.Programs {
		targets = append(targets, p.Scopes...)
	}
	return targets, nil
}

// YesWeHack
func fetchYesWeHackScopes(token string) ([]string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest("GET", "https://api.yeswehack.com/api/v1/programs", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil { return nil, err }
	defer resp.Body.Close()
	if resp.StatusCode != 200 { return nil, fmt.Errorf("YesWeHack status code %d", resp.StatusCode) }

	var data struct {
		Programs []struct {
			Name   string   `json:"name"`
			Scopes []string `json:"targets"` // ajuste conforme API real
		} `json:"programs"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil { return nil, err }

	var targets []string
	for _, p := range data.Programs {
		targets = append(targets, p.Scopes...)
	}
	return targets, nil
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
				Identifier              string `json:"identifier"`
				EligibleForSubmission   bool   `json:"eligible_for_submission"`
				EligibleForBounty       bool   `json:"eligible_for_bounty"`
				AssetType               string `json:"asset_type"`
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
		if strings.Contains(strings.ToLower(attr.AssetType), "domain") || strings.Contains(strings.ToLower(attr.AssetType), "url") || strings.Contains(strings.ToLower(attr.AssetType), "cidr") {
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
		if strings.TrimSpace(l) == "" { continue }
		if _, err := w.WriteString(l + "
		"); err != nil { return err }
	}
	return w.Flush()
}

// writeLinesRotate renomeia o arquivo existente para <basename>-old.txt (removendo qualquer antigo -old primeiro)
// e depois cria o novo arquivo com as linhas fornecidas.
func writeLinesRotate(path string, lines []string) error {
	// se não existir, apenas criar
	if _, err := os.Stat(path); err == nil {
		// arquivo existe
		// construir nome do arquivo antigo: inserir "-old" antes da extensão, se houver
		ext := path
		old := ""
		if idx := strings.LastIndex(ext, "."); idx != -1 {
			old = ext[:idx] + "-old" + ext[idx:]
		} else {
			old = ext + "-old"
		}

		// remover old se já existir
		if _, err := os.Stat(old); err == nil {
			if err := os.Remove(old); err != nil {
				return fmt.Errorf("erro removendo %s: %v", old, err)
			}
		}

		// renomear
		if err := os.Rename(path, old); err != nil {
			return fmt.Errorf("erro renomeando %s -> %s: %v", path, old, err)
		}
	}

	// criar novo arquivo e escrever
	return writeLinesToFile(path, lines)
}
	defer f.Close()
	w := bufio.NewWriter(f)
	for _, l := range lines {
		if strings.TrimSpace(l) == "" { continue }
		if _, err := w.WriteString(l + "
"); err != nil { return err }
	}
	return w.Flush()
}

// -----------------------------
// getScopes: coleta escopos por plataforma, escreve em arquivos separados e imprime status sucinto
// -----------------------------
func getScopes(h1User, h1Key, bcToken, intToken, ywhToken string) {
	// armazenar comandos/endpoints/headers em variáveis (maior parte das informações do "comando")
	h1Endpoint := "https://api.hackerone.com/v1/hackers/programs"
	h1Structured := "https://api.hackerone.com/v1/hackers/programs/%s/structured_scopes"
	h1Auth := fmt.Sprintf("%s:%s", h1User, h1Key)

	bcEndpoint := "https://api.bugcrowd.com/v2/programs"
	bcAuth := fmt.Sprintf("Bearer %s", bcToken)

	intEndpoint := "https://api.intigriti.com/v1/programs"
	intAuth := fmt.Sprintf("Bearer %s", intToken)

	ywhEndpoint := "https://api.yeswehack.com/api/v1/programs"
	ywhAuth := fmt.Sprintf("Bearer %s", ywhToken)

	// HackerOne
	if h1User != "" && h1Key != "" {
		fmt.Fprintf(os.Stderr, "[hackerone] buscando handles...
")
		handles, err := fetchHackerOneProgramHandles(h1User, h1Key)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[hackerone] erro: %v
", err)
		} else {
			// salvar handles
			if err := writeLinesRotate("hackerone_programs.txt", handles); err != nil { fmt.Fprintf(os.Stderr, "[hackerone] erro salvando handles: %v
", err) }
			fmt.Printf("[hackerone] handles: %d
", len(handles))
			// para cada handle, buscar structured scopes e agregar
			var allScopes []string
			for _, h := range handles {
				scopes, err := fetchHackerOneStructuredScopes(h, h1User, h1Key)
				if err != nil {
					// log sucinto
					fmt.Fprintf(os.Stderr, "[hackerone] %s -> err
", h)
					continue
				}
				allScopes = append(allScopes, scopes...)
			}
			if err := writeLinesRotate("hackerone_scopes.txt", allScopes); err != nil { fmt.Fprintf(os.Stderr, "[hackerone] erro salvando scopes: %v
", err) }
			fmt.Printf("[hackerone] scopes colhidos: %d (arquivo: hackerone_scopes.txt)
", len(allScopes))
			// (opcional) imprimir endpoints usados de forma sucinta
			_ = h1Endpoint
			_ = h1Structured
			_ = h1Auth
		}
	}

	// Bugcrowd
	if bcToken != "" {
		fmt.Fprintf(os.Stderr, "[bugcrowd] consultando API...
")
		s, err := fetchBugcrowdScopes(bcToken)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[bugcrowd] erro: %v
", err)
		} else {
			if err := writeLinesRotate("bugcrowd_scopes.txt", s); err != nil { fmt.Fprintf(os.Stderr, "[bugcrowd] erro salvando scopes: %v
", err) }
			fmt.Printf("[bugcrowd] scopes colhidos: %d (arquivo: bugcrowd_scopes.txt)
", len(s))
			_ = bcEndpoint
			_ = bcAuth
		}
	}

	// Intigriti
	if intToken != "" {
		fmt.Fprintf(os.Stderr, "[intigriti] consultando API...
")
		s, err := fetchIntigritiScopes(intToken)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[intigriti] erro: %v
", err)
		} else {
			if err := writeLinesRotate("intigriti_scopes.txt", s); err != nil { fmt.Fprintf(os.Stderr, "[intigriti] erro salvando scopes: %v
", err) }
			fmt.Printf("[intigriti] scopes colhidos: %d (arquivo: intigriti_scopes.txt)
", len(s))
			_ = intEndpoint
			_ = intAuth
		}
	}

	// YesWeHack
	if ywhToken != "" {
		fmt.Fprintf(os.Stderr, "[yeswehack] consultando API...
")
		s, err := fetchYesWeHackScopes(ywhToken)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[yeswehack] erro: %v
", err)
		} else {
			if err := writeLinesRotate("yeswehack_scopes.txt", s); err != nil { fmt.Fprintf(os.Stderr, "[yeswehack] erro salvando scopes: %v
", err) }
			fmt.Printf("[yeswehack] scopes colhidos: %d (arquivo: yeswehack_scopes.txt)
", len(s))
			_ = ywhEndpoint
			_ = ywhAuth
		}
	}

	// breve resumo final
	fmt.Fprintln(os.Stderr, "[getScopes] coleta finalizada")
}

// -----------------------------
// Main
// -----------------------------
func main() {
	// flags
	h1User := flag.String("h1-user", "", "HackerOne username")
	h1Key := flag.String("h1-key", "", "HackerOne API key")
	bcToken := flag.String("bc-token", "", "Bugcrowd API token")
	intToken := flag.String("int-token", "", "Intigriti API token")
	ywhToken := flag.String("ywh-token", "", "YesWeHack API token")

	fileFlag := flag.String("file", "", "Arquivo com uma URL por linha")
	concurrency := flag.Int("concurrency", 5, "Número de workers concorrentes")
	timeoutSec := flag.Int("timeout", 15, "Timeout por request (segundos)")
	delayMS := flag.Int("delay", 300, "Delay entre requests por worker (ms)")
	path := flag.String("path", "", "Caminho a anexar a cada host")
	outFile := flag.String("out", "results.json", "Arquivo de saída JSON")
	flag.Parse()

	// tentar carregar tokens.json se existir
	var apiCfg APIConfig
	if data, err := os.ReadFile("tokens.json"); err == nil {
		if err := json.Unmarshal(data, &apiCfg); err != nil {
			fmt.Fprintf(os.Stderr, "erro parseando tokens.json: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "[info] tokens.json carregado\n")
		}
	} else {
		// arquivo não existe; não é um erro
		fmt.Fprintf(os.Stderr, "[info] tokens.json não encontrado, usando flags\n")
	}

	var targets []string

	// Targets via arquivo
	if *fileFlag != "" {
		f, err := os.Open(*fileFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "erro abrindo arquivo %s: %v\n", *fileFlag, err)
			os.Exit(1)
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" && !strings.HasPrefix(line, "#") {
				targets = append(targets, line)
			}
		}
		f.Close()
	}

	// decidir credenciais: flags > tokens.json
	h1UserVal := *h1User
	h1KeyVal := *h1Key
	if h1UserVal == "" && apiCfg.HackerOne.Username != "" {
		h1UserVal = apiCfg.HackerOne.Username
	}
	if h1KeyVal == "" && apiCfg.HackerOne.ApiKey != "" {
		h1KeyVal = apiCfg.HackerOne.ApiKey
	}

	bcTokenVal := *bcToken
	if bcTokenVal == "" && apiCfg.Bugcrowd.Token != "" {
		bcTokenVal = apiCfg.Bugcrowd.Token
	}

	intTokenVal := *intToken
	if intTokenVal == "" && apiCfg.Intigriti.Token != "" {
		intTokenVal = apiCfg.Intigriti.Token
	}

	ywhTokenVal := *ywhToken
	if ywhTokenVal == "" && apiCfg.YesWeHack.Token != "" {
		ywhTokenVal = apiCfg.YesWeHack.Token
	}

	// Targets via APIs
	if h1UserVal != "" && h1KeyVal != "" {
		if t, err := fetchHackerOneScopes(h1UserVal, h1KeyVal); err == nil {
			targets = append(targets, t...)
		} else {
			fmt.Fprintf(os.Stderr, "HackerOne: %v\n", err)
		}
	}
	if bcTokenVal != "" {
		if t, err := fetchBugcrowdScopes(bcTokenVal); err == nil { targets = append(targets, t...) } else { fmt.Fprintf(os.Stderr, "Bugcrowd: %v\n", err) }
	}
	if intTokenVal != "" {
		if t, err := fetchIntigritiScopes(intTokenVal); err == nil { targets = append(targets, t...) } else { fmt.Fprintf(os.Stderr, "Intigriti: %v\n", err) }
	}
	if ywhTokenVal != "" {
		if t, err := fetchYesWeHackScopes(ywhTokenVal); err == nil { targets = append(targets, t...) } else { fmt.Fprintf(os.Stderr, "YesWeHack: %v\n", err) }
	}

	if len(targets) == 0 {
		fmt.Println("[warn] nenhum target definido; finalize a execução passando tokens ou arquivo com URLs")
		os.Exit(0)
	}

	// Client HTTP
	client := &http.Client{Timeout: time.Duration(*timeoutSec) * time.Second}

	jobs := make(chan string, len(targets))
	resultsCh := make(chan SiteResult, len(targets))
	var wg sync.WaitGroup
	delay := time.Duration(*delayMS) * time.Millisecond

	// start workers
	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go worker(i+1, jobs, resultsCh, client, *path, delay, &wg)
	}

	for _, t := range targets { jobs <- t }
	close(jobs)

	go func() { wg.Wait(); close(resultsCh) }()

	var all []SiteResult
	for r := range resultsCh {
		if r.Error != "" {
			fmt.Printf("ERR  %-30s -> %s\n", r.Input, r.Error)
		} else {
			fmt.Printf("OK   %-30s -> %d  title=\"%s\"\n", r.Input, r.HTTPStatus, truncate(r.Title, 60))
		}
		all = append(all, r)
	}

	// salvar JSON
	fout, err := os.Create(*outFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "erro criando %s: %v\n", *outFile, err)
		os.Exit(1)
	}
	enc := json.NewEncoder(fout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(all); err != nil {
		fmt.Fprintf(os.Stderr, "erro escrevendo json: %v\n", err)
	}
	fout.Close()
	fmt.Printf("[done] resultados salvos em %s\n", *outFile)
}