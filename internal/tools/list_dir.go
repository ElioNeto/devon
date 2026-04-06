package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ListDirTool lista arquivos e diretórios com metadados.
type ListDirTool struct {
	Dir string
}

type listDirParams struct {
	Path string `json:"path"`
}

func (t *ListDirTool) Name() string { return "list_dir" }
func (t *ListDirTool) Description() string {
	return "Lista arquivos e diretorios em um caminho, com metadados (tipo, tamanho em bytes, ultima modificacao)."
}
func (t *ListDirTool) Schema() json.RawMessage {
	return json.RawMessage(`{
        "type": "object",
        "properties": {
            "path": {
                "type": "string",
                "description": "Caminho do diretorio para listar (padrao: raiz do projeto)"
            }
        }
    }`)
}

func (t *ListDirTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p listDirParams
	_ = json.Unmarshal(params, &p) // path é opcional

	path := t.resolvePath(p.Path)

	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("list_dir: nao foi possivel acessar %q: %w", path, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("list_dir: %q nao e um diretorio", path)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return "", fmt.Errorf("list_dir: erro ao ler %q: %w", path, err)
	}

	var sb strings.Builder
	for _, entry := range entries {
		entryInfo, err := entry.Info()
		if err != nil {
			continue
		}
		eType := "file"
		if entry.IsDir() {
			eType = "dir"
		} else if entry.Type()&os.ModeSymlink != 0 {
			eType = "symlink"
		}

		size := entryInfo.Size()
		modTime := entryInfo.ModTime().Format(time.DateTime)
		name := entry.Name()

		if entry.IsDir() {
			sb.WriteString(fmt.Sprintf("%s\t%s\t-\t%s\n", eType, modTime, name))
		} else {
			sb.WriteString(fmt.Sprintf("%s\t%s\t%d\t%s\n", eType, modTime, size, name))
		}
	}

	result := strings.TrimSuffix(sb.String(), "\n")
	if result == "" {
		return "(diretorio vazio)", nil
	}
	return result, nil
}

func (t *ListDirTool) resolvePath(p string) string {
	if p == "" {
		dir := t.Dir
		if dir == "" {
			dir = "."
		}
		return dir
	}
	if filepath.IsAbs(p) {
		return p
	}
	dir := t.Dir
	if dir == "" {
		dir = "."
	}
	return filepath.Join(dir, p)
}
