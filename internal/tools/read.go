package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ReadTool lê o conteúdo de um arquivo do disco.
type ReadTool struct {
	Dir string
}

type readParams struct {
	Path string `json:"file"`
}

func (t *ReadTool) Name() string        { return "read" }
func (t *ReadTool) Description() string { return "Le o conteudo de um arquivo e retorna como uma string com numeros de linha." }
func (t *ReadTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"file": {
				"type": "string",
				"description": "Caminho do arquivo para ler, relativo ou absoluto"
			}
		},
		"required": ["file"]
	}`)
}

func (t *ReadTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p readParams
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("read: parametros invalidos: %w", err)
	}
	if p.Path == "" {
		return "", fmt.Errorf("read: caminho nao pode estar vazio")
	}

	path := t.resolvePath(p.Path)

	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("read: nao foi possivel acessar %q: %w", path, err)
	}
	if info.Size() > 1024*1024 {
		return "", fmt.Errorf("read: arquivo %q muito grande (maximo 1 MB)", path)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read: nao foi possivel ler %q: %w", path, err)
	}

	text := string(content)
	if strings.HasSuffix(path, ".png") || strings.HasSuffix(path, ".jpg") ||
		strings.HasSuffix(path, ".jpeg") || strings.HasSuffix(path, ".gif") {
		return fmt.Sprintf("[arquivo binario: %s, %d bytes]", filepath.Base(path), len(content)), nil
	}

	lines := strings.Split(text, "\n")
	var sb strings.Builder
	for i, line := range lines {
		sb.WriteString(fmt.Sprintf("%4d\t%s\n", i+1, line))
	}
	result := strings.TrimSuffix(sb.String(), "\n")
	return sanitizeOutput(result), nil
}

func (t *ReadTool) resolvePath(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	dir := t.Dir
	if dir == "" {
		dir = "."
	}
	return filepath.Join(dir, p)
}
