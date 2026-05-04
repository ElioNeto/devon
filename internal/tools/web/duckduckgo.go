package web

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// DuckDuckGoBackend implements Backend using DuckDuckGo's lite search and
// direct HTTP fetches with html-to-markdown conversion.
type DuckDuckGoBackend struct{}

const (
	duckDuckGoLiteURL = "https://lite.duckduckgo.com/lite/"
	httpTimeout       = 15 * time.Second
	userAgent         = "Mozilla/5.0 (compatible; Devon/1.0)"
)

// Name returns "duckduckgo".
func (d *DuckDuckGoBackend) Name() string { return "duckduckgo" }

// Search performs a web search via DuckDuckGo lite and parses results.
func (d *DuckDuckGoBackend) Search(ctx context.Context, query string) ([]SearchResult, error) {
	client := &http.Client{Timeout: httpTimeout}

	form := url.Values{"q": {query}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, duckDuckGoLiteURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("duckduckgo: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("duckduckgo: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("duckduckgo: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("duckduckgo: read body: %w", err)
	}

	return parseDuckDuckGoResults(string(body))
}

// Fetch retrieves a URL's content and converts it to markdown.
func (d *DuckDuckGoBackend) Fetch(ctx context.Context, targetURL string) (string, error) {
	client := &http.Client{Timeout: httpTimeout}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return "", fmt.Errorf("fetch: create request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("fetch: read body: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/html") {
		return HTMLToMarkdown(string(body)), nil
	}
	return string(body), nil
}

// parseDuckDuckGoResults parses the HTML from DuckDuckGo lite search results.
func parseDuckDuckGoResults(htmlContent string) ([]SearchResult, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, fmt.Errorf("duckduckgo: parse html: %w", err)
	}

	var results []SearchResult
	seen := make(map[string]bool)

	var extractResults func(*html.Node)
	extractResults = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			var href string
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					href = attr.Val
					break
				}
			}
			// DuckDuckGo lite results have links with class="result-link"
			if href != "" && !strings.HasPrefix(href, "/") && !strings.HasPrefix(href, "#") {
				title := extractText(n)
				title = strings.TrimSpace(title)
				if title != "" && !seen[href] {
					seen[href] = true
					// Get snippet from next sibling or child
					snippet := extractSnippet(n.Parent)
					results = append(results, SearchResult{
						Title:   title,
						URL:     href,
						Snippet: snippet,
					})
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extractResults(c)
		}
	}

	extractResults(doc)

	// Limit to top 10 results
	if len(results) > 10 {
		results = results[:10]
	}

	return results, nil
}

// extractText extracts all text content from a node.
func extractText(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			sb.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return sb.String()
}

// extractSnippet looks for a sibling <span> with result snippet.
func extractSnippet(n *html.Node) string {
	if n == nil {
		return ""
	}
	// Look for a sibling with class containing "snippet"
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode {
			text := extractText(c)
			text = strings.TrimSpace(text)
			if text != "" {
				return text
			}
		}
	}
	// Try next sibling
	for s := n.NextSibling; s != nil; s = s.NextSibling {
		if s.Type == html.ElementNode {
			text := extractText(s)
			text = strings.TrimSpace(text)
			if text != "" {
				return text
			}
		}
	}
	return ""
}
