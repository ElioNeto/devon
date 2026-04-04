package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// WriteTool escreve conteúdo em um arquivo, criando diretórios intermediários.
type WriteTool struct {
	Dir string
}

type writeParams struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func (t *WriteTool) Name() string        { return "write" }
func (t *WriteTool) Description() string { return "Write content to a file. Creates parent directories if they don't existing files will be overwritten." }
func (t *WriteTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {
				"type": "string",
				"description": "Path to the file, relative or absolute"
			},
			"content": {
				"type": "string",
				"description": "The content to write to the file"
			}
		},
		"required": ["path", "content"]
	}`)
}

func (t *WriteTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p writeParams
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("write: invalid params: %w", err)
	}
	if p.Path == "" {
		return "", fmt.Errorf("write: path cannot be empty")
	}

	path := t.resolvePath(p.Path)

	// Cria diretórios intermediários
	if dir := filepath.Dir(path); dir != "." && dir != "/" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", fmt.Errorf("write: cannot create directory %q: %w", dir, err)
		}
	}

	if err := os.WriteFile(path, []byte(p.Content), 0o644); err != nil {
		return "", fmt.Errorf("write: cannot write %q: %w", path, err)
	}

	return fmt.Sprintf("Successfully wrote to %s (%d bytes)", t.relativePath(path), len(p.Content)), nil
}

func (t *WriteTool) resolvePath(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	dir := t.Dir
	if dir == "" {
		dir = "."
	}
	return filepath.Join(dir, p)
}

func (t *WriteTool) relativePath(p string) string {
	dir := t.Dir
	if dir == "" || !filepath.IsAbs(p) {
		return p
	}
	rel, err := filepath.Rel(dir, p)
	if err != nil {
		return p
	}
	return rel
}
