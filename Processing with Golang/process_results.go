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

type CrawlResult struct {
	URL         string   `json:"url"`
	Title       string   `json:"title"`
	Status      *int     `json:"status"`
	Links       []string `json:"links"`
	Timestamp   string   `json:"timestamp"`
	HtmlSnippet string   `json:"html_snippet"`
	Error       string   `json:"error,omitempty"`
}

func readResults(path string) ([]CrawlResult, error) {
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

func domainOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Host
}

type LinkStatus struct {
	Link   string
	Status int
	Error  string
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: process_results results.json")
		os.Exit(1)
	}
	path := os.Args[1]
	data, err := readResults(path)
	if err != nil {
		fmt.Printf("Error reading %s: %v\n", path, err)
		os.Exit(1)
	}

	// Gather all unique links
	linkSet := make(map[string]struct{})
	for _, r := range data {
		for _, l := range r.Links {
			linkSet[l] = struct{}{}
		}
	}

	links := make([]string, 0, len(linkSet))
	for l := range linkSet {
		links = append(links, l)
	}

	// Count by domain
	domainCount := make(map[string]int)
	for _, l := range links {
		d := domainOf(l)
		if d != "" {
			domainCount[d]++
		}
	}

	fmt.Println("=== Domain counts ===")
	for d, c := range domainCount {
		fmt.Printf("%s -> %d links\n", d, c)
	}
	fmt.Println("=====================")

	// Concurrently fetch HEAD for each link with worker pool
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
				req2.Header.Set("User-Agent", "Mozilla/5.0 (compatible; crawler/1.0)")
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
			fmt.Printf("FAIL: %s -> status=%d err=%s\n", res.Link, res.Status, res.Error)
		} else {
			okCount++
			fmt.Printf("OK:   %s -> %d\n", res.Link, res.Status)
		}
	}

	fmt.Printf("\nSummary: tested=%d ok=%d fail=%d unique_links=%d\n", len(links), okCount, failCount, len(links))
}
