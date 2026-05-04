package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// FirecrawlBackend implements Backend using the Firecrawl API.
type FirecrawlBackend struct {
	APIKey string
}

const (
	firecrawlBaseURL  = "https://api.firecrawl.dev/v1"
	firecrawlTimeout  = 30 * time.Second
)

// Name returns "firecrawl".
func (f *FirecrawlBackend) Name() string { return "firecrawl" }

// firecrawlSearchRequest is the JSON body for /v1/search.
type firecrawlSearchRequest struct {
	Query string `json:"query"`
	Limit int    `json:"limit,omitempty"`
}

// firecrawlSearchResponse is the JSON response from /v1/search.
type firecrawlSearchResponse struct {
	Success bool                     `json:"success"`
	Data    []firecrawlSearchResult `json:"data"`
}

type firecrawlSearchResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
}

// firecrawlScrapeRequest is the JSON body for /v1/scrape.
type firecrawlScrapeRequest struct {
	URL string `json:"url"`
}

// firecrawlScrapeResponse is the JSON response from /v1/scrape.
type firecrawlScrapeResponse struct {
	Success bool                   `json:"success"`
	Data    firecrawlScrapeData    `json:"data"`
}

type firecrawlScrapeData struct {
	Markdown string `json:"markdown"`
	Content  string `json:"content"`
}

// Search performs a web search via Firecrawl API.
func (f *FirecrawlBackend) Search(ctx context.Context, query string) ([]SearchResult, error) {
	body := firecrawlSearchRequest{
		Query: query,
		Limit: 10,
	}
	reqBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("firecrawl: marshal search request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, firecrawlBaseURL+"/search", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("firecrawl: create search request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+f.APIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: firecrawlTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("firecrawl: search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("firecrawl: search HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var searchResp firecrawlSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("firecrawl: decode search response: %w", err)
	}

	if !searchResp.Success {
		return nil, fmt.Errorf("firecrawl: search returned success=false")
	}

	results := make([]SearchResult, 0, len(searchResp.Data))
	for _, r := range searchResp.Data {
		results = append(results, SearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: r.Description,
		})
	}
	return results, nil
}

// Fetch retrieves a URL's content as markdown via Firecrawl API.
func (f *FirecrawlBackend) Fetch(ctx context.Context, targetURL string) (string, error) {
	body := firecrawlScrapeRequest{URL: targetURL}
	reqBody, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("firecrawl: marshal scrape request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, firecrawlBaseURL+"/scrape", bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("firecrawl: create scrape request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+f.APIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: firecrawlTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("firecrawl: scrape request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("firecrawl: scrape HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var scrapeResp firecrawlScrapeResponse
	if err := json.NewDecoder(resp.Body).Decode(&scrapeResp); err != nil {
		return "", fmt.Errorf("firecrawl: decode scrape response: %w", err)
	}

	if !scrapeResp.Success {
		return "", fmt.Errorf("firecrawl: scrape returned success=false")
	}

	// Prefer markdown, fall back to raw content
	if scrapeResp.Data.Markdown != "" {
		return scrapeResp.Data.Markdown, nil
	}
	if scrapeResp.Data.Content != "" {
		return HTMLToMarkdown(scrapeResp.Data.Content), nil
	}
	return "", fmt.Errorf("firecrawl: scrape returned empty content")
}
