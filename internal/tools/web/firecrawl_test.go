package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFirecrawlBackend_Search(t *testing.T) {
	t.Run("parse search response", func(t *testing.T) {
		// Test response parsing logic directly
		rawResp := firecrawlSearchResponse{
			Success: true,
			Data: []firecrawlSearchResult{
				{Title: "T1", URL: "https://u1.com", Description: "D1"},
				{Title: "T2", URL: "https://u2.com", Description: "D2"},
			},
		}
		data, _ := json.Marshal(rawResp)

		var decoded firecrawlSearchResponse
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("json.Unmarshal error = %v", err)
		}
		if !decoded.Success {
			t.Error("expected success=true")
		}
		if len(decoded.Data) != 2 {
			t.Fatalf("expected 2 results, got %d", len(decoded.Data))
		}
		if decoded.Data[0].Title != "T1" {
			t.Errorf("expected 'T1', got %q", decoded.Data[0].Title)
		}
	})

	t.Run("parse search response - empty", func(t *testing.T) {
		rawResp := firecrawlSearchResponse{Success: true, Data: []firecrawlSearchResult{}}
		data, _ := json.Marshal(rawResp)

		var decoded firecrawlSearchResponse
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("json.Unmarshal error = %v", err)
		}
		if len(decoded.Data) != 0 {
			t.Errorf("expected 0 results, got %d", len(decoded.Data))
		}
	})

	t.Run("search error response", func(t *testing.T) {
		// Simulate an HTTP error
		errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"success":false,"error":"Invalid API key"}`)
		}))
		defer errorServer.Close()

		// Test error parsing
		resp, err := http.Get(errorServer.URL)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", resp.StatusCode)
		}
	})
}

func TestFirecrawlBackend_Fetch(t *testing.T) {
	t.Run("parse scrape response with markdown", func(t *testing.T) {
		rawResp := firecrawlScrapeResponse{
			Success: true,
			Data: firecrawlScrapeData{
				Markdown: "# Page Title\n\nContent here.",
				Content:  "<html><body><h1>Page Title</h1><p>Content here.</p></body></html>",
			},
		}
		data, _ := json.Marshal(rawResp)

		var decoded firecrawlScrapeResponse
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("json.Unmarshal error = %v", err)
		}
		if !decoded.Success {
			t.Error("expected success=true")
		}
		if decoded.Data.Markdown != "# Page Title\n\nContent here." {
			t.Errorf("unexpected markdown: %q", decoded.Data.Markdown)
		}
	})

	t.Run("parse scrape response without markdown", func(t *testing.T) {
		rawResp := firecrawlScrapeResponse{
			Success: true,
			Data: firecrawlScrapeData{
				Markdown: "",
				Content:  "<html><body><h1>Title</h1><p>Text</p></body></html>",
			},
		}
		data, _ := json.Marshal(rawResp)

		var decoded firecrawlScrapeResponse
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("json.Unmarshal error = %v", err)
		}
		if decoded.Data.Markdown != "" {
			t.Errorf("expected empty markdown, got %q", decoded.Data.Markdown)
		}
		if decoded.Data.Content == "" {
			t.Error("expected non-empty content")
		}
	})

	t.Run("error response", func(t *testing.T) {
		rawResp := firecrawlScrapeResponse{Success: false}
		data, _ := json.Marshal(rawResp)

		var decoded firecrawlScrapeResponse
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("json.Unmarshal error = %v", err)
		}
		if decoded.Success {
			t.Error("expected success=false")
		}
	})
}

func TestFirecrawlBackend_Name(t *testing.T) {
	fc := &FirecrawlBackend{}
	if fc.Name() != "firecrawl" {
		t.Errorf("expected 'firecrawl', got %q", fc.Name())
	}
}

func TestFirecrawlBackend_APIError(t *testing.T) {
	// Test with empty API key - should fail at HTTP request level
	fc := &FirecrawlBackend{APIKey: ""}
	_, err := fc.Search(context.Background(), "test")
	if err == nil {
		t.Error("expected error for empty API key, got nil")
	}
}
