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

// GlobTool busca arquivos por padrão glob (inclui ** para recursivo).
type GlobTool struct {
	Dir string
}

type globParams struct {
	Pattern string `json:"pattern"`
}

func (t *GlobTool) Name() string        { return "glob" }
func (t *GlobTool) Description() string { return "Search for files matching a glob pattern. Supports ** for recursive directory matching." }
func (t *GlobTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"pattern": {
				"type": "string",
				"description": "Glob pattern to match files (e.g. **/*.go, src/**/*.ts)"
			}
		},
		"required": ["pattern"]
	}`)
}

func (t *GlobTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p globParams
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("glob: invalid params: %w", err)
	}
	if p.Pattern == "" {
		return "", fmt.Errorf("glob: pattern cannot be empty")
	}

	dir := t.Dir
	if dir == "" {
		dir = "."
	}

	matches, err := doublestar.Glob(os.DirFS(dir), p.Pattern)
	if err != nil {
		return "", fmt.Errorf("glob: pattern error: %w", err)
	}

	if len(matches) == 0 {
		return "No files matched the pattern.", nil
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
