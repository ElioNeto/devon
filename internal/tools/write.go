package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ElioNeto/devon/internal/permissions"
)

// WriteTool escreve conteudo em um arquivo, criando diretorios intermediarios.
type WriteTool struct {
	Dir string
}

type writeParams struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func (t *WriteTool) Name() string        { return "write" }
func (t *WriteTool) Permission() permissions.PermissionLevel { return permissions.PermWrite }
func (t *WriteTool) Description() string { return "Escreve conteudo em um arquivo. Cria diretorios intermediarios se necessario. Arquivos existentes serao sobrescritos." }
func (t *WriteTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {
				"type": "string",
				"description": "Caminho do arquivo, relativo ou absoluto"
			},
			"content": {
				"type": "string",
				"description": "O conteudo a ser escrito no arquivo"
			}
		},
		"required": ["path", "content"]
	}`)
}

func (t *WriteTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p writeParams
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("write: parametros invalidos: %w", err)
	}
	if p.Path == "" {
		return "", fmt.Errorf("write: caminho nao pode estar vazio")
	}

	path, err := ensurePath(p.Path, t.Dir)
	if err != nil {
		return "", fmt.Errorf("write: %w", err)
	}

	// Cria diretorios intermediarios
	if dir := filepath.Dir(path); dir != "." && dir != "/" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", fmt.Errorf("write: nao foi possivel criar diretorio %q: %w", dir, err)
		}
	}

	if err := os.WriteFile(path, []byte(p.Content), 0o644); err != nil {
		return "", fmt.Errorf("write: nao foi possivel escrever em %q: %w", path, err)
	}

	return fmt.Sprintf("Arquivo escrito com sucesso em %s (%d bytes)", t.relativePath(path), len(p.Content)), nil
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
