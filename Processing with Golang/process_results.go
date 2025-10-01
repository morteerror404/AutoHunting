// process_results.go
package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"
)

// Config define a estrutura esperada para o arquivo env.json, 
// incluindo os caminhos das pastas de resultados.
type Config struct {
	// Caminho para o arquivo JSON de resultados brutos da API de BB (Request_API.go)
	APIRawResultsPath string `json:"api_raw_results_path"`
	// Caminho para o arquivo JSON de escopos e vetores processados pela IA (AI_scope_interpreter.go)
	AIProcessedScopesPath string `json:"ai_processed_scopes_path"`
}

// =====================================================================
// Estruturas de Dados
// =====================================================================

type CrawlResult struct {
	URL         string   `json:"url"`
	Title       string   `json:"title"`
	Status      *int     `json:"status"`
	Links       []string `json:"links"`
	Timestamp   string   `json:"timestamp"`
	HtmlSnippet string   `json:"html_snippet"`
	Error       string   `json:"error,omitempty"`
}

// Estrutura Simples para simular os dados de escopo da IA
type ScopeVector struct {
	AssetIdentifier string   `json:"asset_identifier"`
	HighVectors     []string `json:"high_vectors"`
}

type LinkStatus struct {
	Link   string
	Status int
	Error  string
}

// =====================================================================
// Funções de Leitura e Suporte
// =====================================================================

// readConfig lê o arquivo env.json e retorna a configuração.
func readConfig(path string) (Config, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// readAPIResults lê os resultados de crawlers ou API, usando a estrutura CrawlResult
func readAPIResults(path string) ([]CrawlResult, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var data []CrawlResult
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, err
	}
	return data, nil
}

// readAIFSResults simula a leitura dos resultados da IA, usando a estrutura ScopeVector
func readAIFSResults(path string) ([]ScopeVector, error) {
	// Na arquitetura real, o Process_results.go unificaria ScopeVector e CrawlResult, 
    // mas aqui simulamons apenas a leitura da IA.
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var data []ScopeVector
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func domainOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Host
}

// =====================================================================
// Main
// =====================================================================

func main() {
	// 1. LEITURA DA CONFIGURAÇÃO env.json
	cfg, err := readConfig("env.json")
	if err != nil {
		fmt.Printf("Erro lendo env.json. Verifique se o arquivo está presente e formatado corretamente: %v\n", err)
		os.Exit(1)
	}

	// 2. LEITURA DOS RESULTADOS DA API (API Raw Results Path)
	apiResults, err := readAPIResults(cfg.APIRawResultsPath)
	if err != nil {
		fmt.Printf("Erro lendo resultados da API em %s: %v\n", cfg.APIRawResultsPath, err)
		// Continua se o arquivo da IA for crítico, mas falha aqui para fins de exemplo
		os.Exit(1)
	}

	// 3. LEITURA DOS RESULTADOS DA IA (AI Processed Scopes Path)
	aiScopes, err := readAIFSResults(cfg.AIProcessedScopesPath)
	if err != nil {
		fmt.Printf("Erro lendo resultados da IA em %s: %v\n", cfg.AIProcessedScopesPath, err)
		os.Exit(1)
	}

	fmt.Printf("Sucesso na Leitura: %d resultados da API e %d escopos da IA.\n", len(apiResults), len(aiScopes))
	fmt.Println("--- INICIANDO LÓGICA DE UNIFICAÇÃO E VALIDAÇÃO ---")
	
	// A LÓGICA DE UNIFICAÇÃO DO Process_results.go ENTRARIA AQUI:
	// A) Unificar todos os links de apiResults com os AssetIdentifiers de aiScopes.
	// B) Criar o array JSON final para o DB_manager.go, contendo tanto o alvo
	//    quanto os HighVectors extraídos pela IA.
	
	// O código abaixo mantém a lógica original de teste de links para demonstração
	// e para cumprir o que o código GO original fazia.
	
	// Gather all unique links from API results
	linkSet := make(map[string]struct{})
	for _, r := range apiResults {
		for _, l := range r.Links {
			linkSet[l] = struct{}{}
		}
	}

	links := make([]string, 0, len(linkSet))
	for l := range linkSet {
		links = append(links, l)
	}

	// Count by domain (Lógica Original Mantida)
	domainCount := make(map[string]int)
	for _, l := range links {
		d := domainOf(l)
		if d != "" {
			domainCount[d]++
		}
	}

	fmt.Println("\n=== Domain counts ===")
	for d, c := range domainCount {
		fmt.Printf("%s -> %d links\n", d, c)
	}
	fmt.Println("=====================")

	// Lógica de Concorrência Original (Testando Links)
	concurrency := 20
	jobs := make(chan string, len(links))
	resultsCh := make(chan LinkStatus, len(links))
	var wg sync.WaitGroup

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	worker := func() {
		for l := range jobs {
			// perform HEAD; if not allowed, try GET
			req, _ := http.NewRequest("HEAD", l, nil)
			req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; crawler/1.0)")
			resp, err := client.Do(req)
			status := 0
			errStr := ""
			if err != nil {
				// fallback to GET
				req2, _ := http.NewRequest("GET", l, nil)
				req2.Header.Header.Set("User-Agent", "Mozilla/5.0 (compatible; crawler/1.0)")
				resp2, err2 := client.Do(req2)
				if err2 != nil {
					errStr = err2.Error()
				} else {
					status = resp2.StatusCode
					resp2.Body.Close()
				}
			} else {
				status = resp.StatusCode
				resp.Body.Close()
			}
			resultsCh <- LinkStatus{Link: l, Status: status, Error: errStr}
		}
		wg.Done()
	}

	// start workers
	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go worker()
	}

	// feed jobs
	for _, l := range links {
		jobs <- l
	}
	close(jobs)

	// wait for workers to finish in goroutine and then close resultsCh
	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	// collect results
	var okCount, failCount int
	for res := range resultsCh {
		if res.Error != "" || res.Status == 0 {
			failCount++
			// Remover o print de FAIL/OK para focar no propósito do Process_results.go na arquitetura
			// fmt.Printf("FAIL: %s -> status=%d err=%s\n", res.Link, res.Status, res.Error)
		} else {
			okCount++
			// fmt.Printf("OK:   %s -> %d\n", res.Link, res.Status)
		}
	}

	fmt.Printf("\nResumo da Verificacao HTTP: testados=%d ok=%d falhas=%d links_unicos=%d\n", len(links), okCount, failCount, len(links))
}