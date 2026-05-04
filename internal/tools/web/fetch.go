package web

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ElioNeto/devon/internal/config"
	"github.com/ElioNeto/devon/internal/permissions"
)

// FetchTool implements the web_fetch tool.
type FetchTool struct {
	Config *config.WebConfig
}

type fetchParams struct {
	URL string `json:"url"`
}

// Name returns the tool name.
func (t *FetchTool) Name() string { return "web_fetch" }

// Permission returns the permission level.
func (t *FetchTool) Permission() permissions.PermissionLevel { return permissions.PermRead }

// Description returns the tool description.
func (t *FetchTool) Description() string {
	return "Faz o download do conteudo de uma URL e retorna como Markdown. Usa o backend configurado (DuckDuckGo para HTML direto, Firecrawl via API)."
}

// Schema returns the JSON schema for parameters.
func (t *FetchTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"url": {
				"type": "string",
				"description": "URL completa (incluindo https://) da pagina a ser baixada"
			}
		},
		"required": ["url"]
	}`)
}

// Execute fetches a URL and converts to markdown using the selected backend.
func (t *FetchTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p fetchParams
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("web_fetch: parametros invalidos: %w", err)
	}
	if p.URL == "" {
		return "", fmt.Errorf("web_fetch: url nao pode estar vazia")
	}

	backend, err := SelectBackend(t.Config)
	if err != nil {
		return "", fmt.Errorf("web_fetch: %w", err)
	}

	content, err := backend.Fetch(ctx, p.URL)
	if err != nil {
		return "", fmt.Errorf("web_fetch (%s): %w", backend.Name(), err)
	}

	if content == "" {
		return "(conteudo vazio)", nil
	}

	// Truncate very long content
	const maxContentLen = 50 * 1024 // 50 KB
	if len(content) > maxContentLen {
		content = content[:maxContentLen] + "\n\n... [conteudo truncado: excedeu 50 KB]"
	}

	return fmt.Sprintf("Conteudo de %s (via %s):\n\n%s", p.URL, backend.Name(), content), nil
}
