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

// Resultado de cada request
type SiteResult struct {
	Input        string `json:"input"`         // entrada original
	ResolvedURL  string `json:"resolved_url"`  // URL final após redirects
	HTTPStatus   int    `json:"http_status"`   // status code
	Title        string `json:"title"`         // conteúdo do <title>
	ContentLen   int64  `json:"content_length"`// tamanho do body (se conhecido)
	Error        string `json:"error,omitempty"`
	CheckedAt    string `json:"checked_at"`
	ElapsedMS    int64  `json:"elapsed_ms"`
}

// helper: adiciona https:// se faltar esquema
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

// parse <title> com tokenizer (sem dependências extras)
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
				// next token should be the title text
				tt2 := z.Next()
				if tt2 == html.TextToken {
					return strings.TrimSpace(string(z.Text())), nil
				}
				return "", nil
			}
		}
	}
}

func worker(id int, jobs <-chan string, results chan<- SiteResult, client *http.Client, path string, delay time.Duration, wg *sync.WaitGroup) {
	defer wg.Done()
	for raw := range jobs {
		start := time.Now()
		input := raw
		raw = ensureScheme(raw)
		target := raw
		if path != "" {
			// garante que path comece com '/'
			if !strings.HasPrefix(path, "/") {
				path = "/" + path
			}
			u, err := url.Parse(raw)
			if err == nil {
				u.Path = strings.TrimSuffix(u.Path, "/") + path
				target = u.String()
			} else {
				// fallback: concat
				target = raw + path
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), client.Timeout)
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; bugsites-check/1.0; +https://example.local)")

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
			// delay before next job to be polite
			time.Sleep(delay)
			continue
		}

		// fill fields
		res.HTTPStatus = resp.StatusCode
		res.ResolvedURL = resp.Request.URL.String()
		if resp.ContentLength >= 0 {
			res.ContentLen = resp.ContentLength
		} else {
			// if unknown, try to read up to a limit to compute length (but don't consume full body)
			// here we won't read body fully to avoid memory bloat; prefer using ContentLength header.
		}

		// parse title (use limited reader to avoid huge pages)
		limited := io.LimitReader(resp.Body, 200*1024) // 200 KB
		title, perr := parseTitle(limited)
		if perr != nil {
			// non-fatal
			res.Error = perr.Error()
		} else {
			res.Title = title
		}
		resp.Body.Close()
		cancel()
		res.CheckedAt = time.Now().UTC().Format(time.RFC3339)
		results <- res

		// politeness
		time.Sleep(delay)
	}
}

func main() {
	// flags
	targetsFlag := flag.String("targets", "", "Lista de sites separados por vírgula (ex: hackerone.com,bugcrowd.com)")
	fileFlag := flag.String("file", "", "Arquivo com uma URL por linha")
	concurrency := flag.Int("concurrency", 5, "Número de workers concorrentes")
	timeoutSec := flag.Int("timeout", 15, "Timeout por request (segundos)")
	delayMS := flag.Int("delay", 300, "Delay entre requests por worker (ms) — para não sobrecarregar")
	path := flag.String("path", "", "Caminho a anexar a cada host (ex: /programs) — opcional")
	outFile := flag.String("out", "results.json", "Arquivo de saída JSON")
	flag.Parse()

	// monta lista de targets
	var targets []string
	if *targetsFlag != "" {
		for _, t := range strings.Split(*targetsFlag, ",") {
			if s := strings.TrimSpace(t); s != "" {
				targets = append(targets, s)
			}
		}
	}
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

	if len(targets) == 0 {
		// exemplo padrão com plataformas de bug-bounty (públicas)
		targets = []string{
			"hackerone.com",
			"bugcrowd.com",
			"yeswehack.com",
			"intigriti.com",
			"gitlab.com", // alguns programas listam scopes aqui
			"bountyfactory.io",
			"facebook.com", // exemplo — REMEMBER: apenas públicos/autorizados
		}
		fmt.Println("[info] nenhum target passado; usando lista de exemplo (substitua por seus alvos)")
	}

	// client http com timeout
	client := &http.Client{
		Timeout: time.Duration(*timeoutSec) * time.Second,
		// transport default is fine; you may tune TLS/timeouts if needed
	}

	jobs := make(chan string, len(targets))
	resultsCh := make(chan SiteResult, len(targets))

	var wg sync.WaitGroup
	delay := time.Duration(*delayMS) * time.Millisecond

	// start workers
	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go worker(i+1, jobs, resultsCh, client, *path, delay, &wg)
	}

	// send jobs
	for _, t := range targets {
		jobs <- t
	}
	close(jobs)

	// wait finish in goroutine
	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	// collect results
	var all []SiteResult
	for r := range resultsCh {
		// print resumo rápido
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

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}
