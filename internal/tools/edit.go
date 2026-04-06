package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EditTool faz edição cirúrgica: substitui um trecho exato por outro.
type EditTool struct {
	Dir string
}

type editParams struct {
	Path      string `json:"path"`
	OldString string `json:"old_string"`
	NewString string `json:"new_string"`
}

func (t *EditTool) Name() string { return "edit" }
func (t *EditTool) Description() string {
	return "Edição cirurgica em um arquivo: substitui um trecho exato (old_string) por outro (new_string). Falha se old_string não for encontrado ou aparecer mais de uma vez."
}
func (t *EditTool) Schema() json.RawMessage {
	return json.RawMessage(`{
        "type": "object",
        "properties": {
            "path": {
                "type": "string",
                "description": "Caminho do arquivo, relativo ou absoluto"
            },
            "old_string": {
                "type": "string",
                "description": "Trecho exato a ser substituído (deve aparecer exatamente uma vez)"
            },
            "new_string": {
                "type": "string",
                "description": "Trecho que substituíra o old_string"
            }
        },
        "required": ["path", "old_string", "new_string"]
    }`)
}

func (t *EditTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p editParams
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("edit: parametros invalidos: %w", err)
	}
	if p.Path == "" {
		return "", fmt.Errorf("edit: caminho nao pode estar vazio")
	}
	if p.OldString == "" {
		return "", fmt.Errorf("edit: old_string nao pode estar vazio")
	}

	path, err := ensurePath(p.Path, t.Dir)
	if err != nil {
		return "", fmt.Errorf("edit: %w", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("edit: nao foi possivel ler %q: %w", path, err)
	}

	text := string(content)

	// Conta ocorrências de old_string
	count := strings.Count(text, p.OldString)
	if count == 0 {
		return "", fmt.Errorf("edit: old_string nao encontrado em %s", t.relativePath(path))
	}
	if count > 1 {
		return "", fmt.Errorf("edit: old_string encontrado %d vezes em %s — deve ser unico, torne a busca mais especifica", count, t.relativePath(path))
	}

	// Aplica substituição
	result := strings.Replace(text, p.OldString, p.NewString, 1)

	if err := os.WriteFile(path, []byte(result), 0o644); err != nil {
		return "", fmt.Errorf("edit: nao foi possivel escrever em %q: %w", path, err)
	}

	return fmt.Sprintf("Edicao aplicada com sucesso em %s (%d ocorrencias substituidas)", t.relativePath(path), 1), nil
}

func (t *EditTool) resolvePath(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	dir := t.Dir
	if dir == "" {
		dir = "."
	}
	return filepath.Join(dir, p)
}

func (t *EditTool) relativePath(p string) string {
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
