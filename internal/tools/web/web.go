// Package web implements web search and fetch tools with pluggable backends.
package web

import (
	"context"
	"fmt"
	"os"

	"github.com/ElioNeto/devon/internal/config"
)

// SearchResult represents a single search result entry.
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// Backend defines the interface for web search/fetch backends.
type Backend interface {
	// Search performs a web search and returns a list of results.
	Search(ctx context.Context, query string) ([]SearchResult, error)
	// Fetch retrieves the content of a URL and returns it as plain text/markdown.
	Fetch(ctx context.Context, url string) (string, error)
	// Name returns the backend identifier.
	Name() string
}

// SelectBackend returns the appropriate backend based on config and environment.
// Priority: Firecrawl if DEVON_FIRECRAWL_KEY is set AND backend != "duckduckgo";
// otherwise DuckDuckGo.
func SelectBackend(cfg *config.WebConfig) (Backend, error) {
	if cfg == nil || !cfg.Enabled {
		return nil, fmt.Errorf("web tools are not enabled")
	}

	firecrawlKey := os.Getenv("DEVON_FIRECRAWL_KEY")
	useFirecrawl := firecrawlKey != "" && cfg.Backend != "duckduckgo"

	if useFirecrawl {
		return &FirecrawlBackend{APIKey: firecrawlKey}, nil
	}
	return &DuckDuckGoBackend{}, nil
}
