package web

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// mockDuckDuckGoLite returns a minimal HTML page resembling DuckDuckGo lite results.
func mockDuckDuckGoLite() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify it's a POST with form data
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		query := r.Form.Get("q")
		if query == "" {
			http.Error(w, "missing query", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>DuckDuckGo Lite</title></head>
<body>
<div class="results">
	<div class="result">
		<a href="https://example.com/result1" class="result-link">Result One</a>
		<span class="snippet">This is the first result snippet.</span>
	</div>
	<div class="result">
		<a href="https://example.com/result2" class="result-link">Result Two</a>
		<span class="snippet">This is the second result snippet.</span>
	</div>
</div>
</body>
</html>`)
	})
}

func TestDuckDuckGoBackend_Search(t *testing.T) {
	t.Run("parse results", func(t *testing.T) {
		html := `<html><body>
		<div class="results">
			<div class="result">
				<a href="https://example.com/1" class="result-link">Title 1</a>
				<span class="snippet">Snippet 1</span>
			</div>
			<div class="result">
				<a href="https://example.com/2" class="result-link">Title 2</a>
				<span class="snippet">Snippet 2</span>
			</div>
		</div>
		</body></html>`

		results, err := parseDuckDuckGoResults(html)
		if err != nil {
			t.Fatalf("parseDuckDuckGoResults() error = %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}
		if results[0].Title != "Title 1" {
			t.Errorf("expected 'Title 1', got %q", results[0].Title)
		}
		if results[0].URL != "https://example.com/1" {
			t.Errorf("expected 'https://example.com/1', got %q", results[0].URL)
		}
		if results[1].Title != "Title 2" {
			t.Errorf("expected 'Title 2', got %q", results[1].Title)
		}
	})

	t.Run("empty results", func(t *testing.T) {
		html := `<html><body><p>No results</p></body></html>`
		results, err := parseDuckDuckGoResults(html)
		if err != nil {
			t.Fatalf("parseDuckDuckGoResults() error = %v", err)
		}
		if len(results) != 0 {
			t.Errorf("expected 0 results, got %d", len(results))
		}
	})

	t.Run("max 10 results", func(t *testing.T) {
		var links strings.Builder
		links.WriteString("<html><body><div class=\"results\">")
		for i := 0; i < 15; i++ {
			fmt.Fprintf(&links, `<div class="result"><a href="https://example.com/%d" class="result-link">Result %d</a><span>snippet</span></div>`, i, i)
		}
		links.WriteString("</div></body></html>")

		results, err := parseDuckDuckGoResults(links.String())
		if err != nil {
			t.Fatalf("parseDuckDuckGoResults() error = %v", err)
		}
		if len(results) > 10 {
			t.Errorf("expected at most 10 results, got %d", len(results))
		}
	})
}

func TestDuckDuckGoBackend_Name(t *testing.T) {
	ddg := &DuckDuckGoBackend{}
	if ddg.Name() != "duckduckgo" {
		t.Errorf("expected 'duckduckgo', got %q", ddg.Name())
	}
}

func TestDuckDuckGoBackend_Fetch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<html><body><h1>Test Page</h1><p>Hello world.</p></body></html>`)
	}))
	defer server.Close()

	ddg := &DuckDuckGoBackend{}
	result, err := ddg.Fetch(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	if !strings.Contains(result, "Test Page") {
		t.Errorf("expected result to contain 'Test Page', got %q", result)
	}
	if !strings.Contains(result, "Hello world") {
		t.Errorf("expected result to contain 'Hello world', got %q", result)
	}
}

func TestDuckDuckGoBackend_Fetch_Error(t *testing.T) {
	ddg := &DuckDuckGoBackend{}
	_, err := ddg.Fetch(context.Background(), "http://127.0.0.1:1/")
	if err == nil {
		t.Error("expected error for invalid URL, got nil")
	}
}
