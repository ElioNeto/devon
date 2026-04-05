package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// GlobTool busca arquivos por padrao glob (inclui ** para recursivo).
type GlobTool struct {
	Dir string
}

type globParams struct {
	Pattern string `json:"pattern"`
}

func (t *GlobTool) Name() string        { return "glob" }
func (t *GlobTool) Description() string { return "Busca arquivos por padrao glob. Suporta ** para busca recursiva em diretorios." }
func (t *GlobTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"pattern": {
				"type": "string",
				"description": "Padrao glob para buscar arquivos (ex: **/*.go, src/**/*.ts)"
			}
		},
		"required": ["pattern"]
	}`)
}

func (t *GlobTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p globParams
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("glob: parametros invalidos: %w", err)
	}
	if p.Pattern == "" {
		return "", fmt.Errorf("glob: padrao nao pode estar vazio")
	}

	dir := t.Dir
	if dir == "" {
		dir = "."
	}

	matches, err := doublestar.Glob(os.DirFS(dir), p.Pattern)
	if err != nil {
		return "", fmt.Errorf("glob: erro no padrao: %w", err)
	}

	if len(matches) == 0 {
		return "Nenhum arquivo encontrado para o padrao.", nil
	}

	// Normaliza paths relativos ao workdir
	results := make([]string, len(matches))
	for i, m := range matches {
		rel, err := filepath.Rel(dir, m)
		if err != nil {
			results[i] = m
		} else {
			results[i] = rel
		}
	}

	return strings.Join(results, "\n"), nil
}
