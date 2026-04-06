package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ElioNeto/devon/internal/permissions"
)

// ReadTool lê o conteúdo de um arquivo do disco.
type ReadTool struct {
	Dir string
}

type readParams struct {
	Path   string `json:"file"`
	Offset int    `json:"offset"`
	Limit  int    `json:"limit"`
}

func (t *ReadTool) Name() string { return "read" }
func (t *ReadTool) Permission() permissions.PermissionLevel { return permissions.PermRead }
func (t *ReadTool) Description() string {
	return "Lee o conteudo de um arquivo e retorna como uma string com numeros de linha. Suporta offset e limit para ler parcialmente arquivos grandes."
}
func (t *ReadTool) Schema() json.RawMessage {
	return json.RawMessage(`{
        "type": "object",
        "properties": {
            "file": {
                "type": "string",
                "description": "Caminho do arquivo para ler, relativo ou absoluto"
            },
            "offset": {
                "type": "integer",
                "description": "Numero da linha inicial (1-based). Padrao: 1"
            },
            "limit": {
                "type": "integer",
                "description": "Numero maximo de linhas a ler. Padrao: sem limite"
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

	path, err := ensurePath(p.Path, t.Dir)
	if err != nil {
		return "", fmt.Errorf("read: %w", err)
	}

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

	// Apply offset (1-based)
	offset := p.Offset
	if offset < 1 {
		offset = 1
	}
	start := offset - 1
	if start >= len(lines) {
		return "Offset fora do intervalo do arquivo.", nil
	}

	sliced := lines[start:]

	// Apply limit
	if p.Limit > 0 && p.Limit < len(sliced) {
		sliced = sliced[:p.Limit]
	}

	var sb strings.Builder
	// Mostra numeros de linha absolutos
	for i, line := range sliced {
		absNum := start + i + 1
		sb.WriteString(fmt.Sprintf("%4d\t%s\n", absNum, line))
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
