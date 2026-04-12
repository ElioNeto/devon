package index

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ElioNeto/devon/internal/permissions"
)

// SearchCodebaseTool is a tool that allows the agent to search the codebase
// using semantic search powered by TF-IDF/BM25.
type SearchCodebaseTool struct {
	indexer *Indexer
	topK    int
}

// NewSearchCodebaseTool creates a new search_codebase tool.
func NewSearchCodebaseTool(indexer *Indexer, topK int) *SearchCodebaseTool {
	if topK <= 0 {
		topK = 5
	}
	return &SearchCodebaseTool{
		indexer: indexer,
		topK:    topK,
	}
}

// Name returns the tool name.
func (t *SearchCodebaseTool) Name() string {
	return "search_codebase"
}

// Description returns the tool description for the LLM.
func (t *SearchCodebaseTool) Description() string {
	return `Search the codebase for relevant files using semantic search.
Use this when you need to find code related to a specific concept, function, or error.
The search uses term frequency-inverse document frequency (TF-IDF) scoring to find
the most relevant files for your query.

Parameters:
- query (required): The search query - describe what you're looking for
- top_k (optional): Number of results to return (default: 5, max: 20)

Returns:
A list of matching files with their file paths and relevance scores.
Include the number of lines in each file for context.`
}

// Schema returns the JSON Schema for the tool parameters.
func (t *SearchCodebaseTool) Schema() json.RawMessage {
	schema := `{
		"type": "object",
		"properties": {
			"query": {
				"type": "string",
				"description": "Search query - describe what you're looking for"
			},
			"top_k": {
				"type": "integer",
				"description": "Number of results to return (default: 5, max: 20)",
				"default": 5
			}
		},
		"required": ["query"]
	}`
	return json.RawMessage(schema)
}

// Execute executes the search tool.
func (t *SearchCodebaseTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var searchParams struct {
		Query string `json:"query"`
		TopK  int    `json:"top_k"`
	}

	if err := json.Unmarshal(params, &searchParams); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}

	if searchParams.Query == "" {
		return "", fmt.Errorf("query is required")
	}

	if searchParams.TopK <= 0 {
		searchParams.TopK = t.topK
	}
	if searchParams.TopK > 20 {
		searchParams.TopK = 20
	}

	results := t.indexer.Search(searchParams.Query, searchParams.TopK)

	if len(results) == 0 {
		return "No matching files found.", nil
	}

	var sb fmt.Stringer = &stringsBuilder{}
	for i, result := range results {
		lines := result.Length
		sbStringer := sb.(*stringsBuilder)
		sbStringer.WriteString(fmt.Sprintf("%d. %s (score: %.2f) — %d lines\n",
			i+1, result.Path, result.Score, lines))
	}

	return sb.String(), nil
}

// Permission returns the permission level required for this tool.
func (t *SearchCodebaseTool) Permission() permissions.PermissionLevel {
	return permissions.PermRead
}

// stringsBuilder is a helper for building strings.
type stringsBuilder struct {
	str string
}

func (b *stringsBuilder) WriteString(s string) (int, error) {
	b.str += s
	return len(s), nil
}

func (b *stringsBuilder) String() string {
	return b.str
}
