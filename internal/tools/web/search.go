package web

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ElioNeto/devon/internal/config"
	"github.com/ElioNeto/devon/internal/permissions"
)

// SearchTool implements the web_search tool.
type SearchTool struct {
	Config *config.WebConfig
}

type searchParams struct {
	Query string `json:"query"`
}

// Name returns the tool name.
func (t *SearchTool) Name() string { return "web_search" }

// Permission returns the permission level.
func (t *SearchTool) Permission() permissions.PermissionLevel { return permissions.PermRead }

// Description returns the tool description.
func (t *SearchTool) Description() string {
	return "Realiza uma busca na web usando o mecanismo configurado (DuckDuckGo ou Firecrawl). Retorna uma lista de resultados com titulo, URL e descricao."
}

// Schema returns the JSON schema for parameters.
func (t *SearchTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {
				"type": "string",
				"description": "Termo de busca a ser pesquisado na web"
			}
		},
		"required": ["query"]
	}`)
}

// Execute performs a web search using the selected backend.
func (t *SearchTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p searchParams
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("web_search: parametros invalidos: %w", err)
	}
	if p.Query == "" {
		return "", fmt.Errorf("web_search: query nao pode estar vazia")
	}

	backend, err := SelectBackend(t.Config)
	if err != nil {
		return "", fmt.Errorf("web_search: %w", err)
	}

	results, err := backend.Search(ctx, p.Query)
	if err != nil {
		return "", fmt.Errorf("web_search (%s): %w", backend.Name(), err)
	}

	if len(results) == 0 {
		return "Nenhum resultado encontrado.", nil
	}

	out := fmt.Sprintf("Resultados da busca via %s para %q:\n\n", backend.Name(), p.Query)
	for i, r := range results {
		out += fmt.Sprintf("%d. %s\n   URL: %s\n   %s\n\n", i+1, r.Title, r.URL, r.Snippet)
	}
	return out, nil
}
