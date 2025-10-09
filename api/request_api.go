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

	"golang.org/x/net/html" // Added to go.mod
)

// -----------------------------
// Data Structures
// -----------------------------

// SiteResult defines the data structure for the result of checking a single site.
// This structure is used to serialize the results in JSON format.
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

// Tokens mirrors the structure of the `tokens.json` file, storing the credentials
// API required to authenticate on Bug Bounty platforms.
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

// ensureScheme ensures that a URL string has a scheme (http:// or https://).
// If no scheme is present, it adds "https://" as default.
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

// parseTitle extracts the content of the <title> tag from an HTML response body.
// It reads from an io.Reader and stops as soon as it finds the title or the end of the file.
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

// truncate limits a string to a number `n` of characters, adding "..." if it is cut.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

// -----------------------------
// HTTP Workers
// -----------------------------

// worker is the function executed by each concurrent goroutine to process jobs.
// It receives URLs from a `jobs` channel, performs the HTTP request, processes the response,
// and sends the result (SiteResult) to the `results` channel.
// Respects a `delay` between requests to avoid rate limiting.
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
// HackerOne: active programs + structured scopes
// -----------------------------

// fetchHackerOneProgramHandles searches the HackerOne API for the list of "handles" (unique identifiers)
// of all public programs. It authenticates using the credentials provided and filters
// the programs that are in "public_mode".
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

// fetchHackerOneStructuredScopes, for a given program `handle`, queries the endpoint
// of structured scopes of the HackerOne API.
// The function filters the assets to return only those that are:
// 1. Eligible for submission (`eligible_for_submission`).
// 2. Eligible for reward (`eligible_for_bounty`).
// 3. Of types relevant to automation (`URL`, `DOMAIN`, `CIDR`).
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
			// accept even if it doesn't have asset_type, but it usually comes
			out = append(out, attr.Identifier)
			continue
		}
		// filter by useful types (Domain, Url, Cidr)
		assetTypeLower := strings.ToLower(attr.AssetType)
		if strings.Contains(assetTypeLower, "domain") || strings.Contains(assetTypeLower, "url") || strings.Contains(assetTypeLower, "cidr") {
			out = append(out, attr.Identifier)
		}
	}
	return out, nil
}

// writeLinesToFile is a helper function that writes a slice of strings to a file,
// with each string on a new line. It creates the file if it does not exist and overwrites the content.
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

// writeLinesRotate writes lines to a file, but first renames any
// existing file with the same name to `<basename>-old.txt`. This serves as a
// simple backup mechanism from the previous execution.
func writeLinesRotate(path string, lines []string) error {
	if _, err := os.Stat(path); err == nil {
		ext := filepath.Ext(path)
		oldPath := strings.TrimSuffix(path, ext) + "-old" + ext
		if _, err := os.Stat(oldPath); err == nil {
			if err := os.Remove(oldPath); err != nil {
				return fmt.Errorf("error removing old file %s: %w", oldPath, err)
			}
		}
		if err := os.Rename(path, oldPath); err != nil {
			return fmt.Errorf("error renaming %s to %s: %w", path, oldPath, err)
		}
	}
	return writeLinesToFile(path, lines)
}

// RunRequestAPI is the entry point for the maestro.
// It orchestrates the collection of scopes for a specific platform (`platform`)
// using the credentials provided (`tokens`) and saves the raw results
// in the specified path (`apiDirtResultsPath`).
func RunRequestAPI(apiDirtResultsPath string, platform string, tokens Tokens) error {
	var allScopes []string
	var err error

	switch platform {
	case "hackerone":
		if tokens.HackerOne.Username == "" || tokens.HackerOne.ApiKey == "" {
			return fmt.Errorf("HackerOne credentials not provided")
		}
		fmt.Fprintf(os.Stderr, "[hackerone] fetching program handles...\n")
		handles, errH := fetchHackerOneProgramHandles(tokens.HackerOne.Username, tokens.HackerOne.ApiKey)
		if errH != nil {
			return fmt.Errorf("error fetching HackerOne handles: %w", errH)
		}
		fmt.Fprintf(os.Stderr, "[hackerone] %d handles found. fetching scopes...\n", len(handles))

		// TODO: Parallelize scope fetching. Currently, it is done sequentially for each handle.
		// Using goroutines and a WaitGroup to fetch scopes from multiple handles simultaneously can greatly speed up the process.
		// It is necessary to ensure that adding to the 'allScopes' slice is concurrency-safe (using a mutex).
		for _, h := range handles {
			scopes, errS := fetchHackerOneStructuredScopes(h, tokens.HackerOne.Username, tokens.HackerOne.ApiKey)
			if errS != nil {
				fmt.Fprintf(os.Stderr, "[hackerone] WARNING: failed to fetch scopes for handle '%s': %v\n", h, errS)
				continue // Continue to the next handle
			}
			allScopes = append(allScopes, scopes...)
		}

	// TODO: Implement scope collection for the other platforms.
	// case "bugcrowd":
	// 	// allScopes, err = fetchBugcrowdScopes(tokens.Bugcrowd.Token)
	// case "intigriti":
	// 	// allScopes, err = fetchIntigritiScopes(tokens.Intigriti.Token)
	// case "yeswehack":
	// 	// allScopes, err = fetchYesWeHackScopes(tokens.YesWeHack.Token)

	default:
		return fmt.Errorf("platform '%s' not supported for API collection", platform)
	}

	if err != nil {
		return fmt.Errorf("error fetching scopes for platform '%s': %w", platform, err)
	}

	if len(allScopes) == 0 {
		fmt.Fprintf(os.Stderr, "WARNING: No scopes found for platform '%s'.\n", platform)
		return nil // Not a fatal error, but nothing was found.
	}

	// Ensures the output directory exists
	if err := os.MkdirAll(filepath.Dir(apiDirtResultsPath), 0755); err != nil {
		return fmt.Errorf("error creating output directory '%s': %w", filepath.Dir(apiDirtResultsPath), err)
	}

	// Saves the scopes in the raw results file specified by the maestro
	if err := writeLinesToFile(apiDirtResultsPath, allScopes); err != nil {
		return fmt.Errorf("error saving scopes to '%s': %w", apiDirtResultsPath, err)
	}

	fmt.Printf("Success! %d scopes from platform '%s' saved to: %s\n", len(allScopes), platform, apiDirtResultsPath)
	return nil
}

// CheckAPIStatus is a placeholder for the function that checks the platform's API status.
func CheckAPIStatus(destiny string) error {
	fmt.Printf("Function 'CheckAPIStatus' called with destiny: %s\n", destiny)
	// TODO: Implement the actual logic for checking API status.
	// This might involve making a simple request to a health check endpoint.
	return nil
}

// FetchApiScopes is a placeholder for the function that fetches scopes from the API.
func FetchApiScopes(destiny string) error {
	fmt.Printf("Function 'FetchApiScopes' called with destiny: %s\n", destiny)
	// TODO: Implement the actual logic for fetching API scopes.
	// This would likely call RunRequestAPI or a similar function.
	return nil
}