package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ElioNeto/devon/internal/permissions"
)

// PatchTool aplica diff unificado ou inserções cirúrgicas em arquivos.
type PatchTool struct {
	Dir string
}

type patchParams struct {
	Path    string   `json:"path"`
	Diff    string   `json:"diff"`
	OldStr  string   `json:"old_str"`
	NewStr  string   `json:"new_str"`
	After   int      `json:"after_line"`
	Content string   `json:"content"`
}

func (t *PatchTool) Name() string                          { return "patch_file" }
func (t *PatchTool) Permission() permissions.PermissionLevel {
	return permissions.PermWrite
}
func (t *PatchTool) Description() string {
	return `Aplica modificacoes cirurgicas em arquivos. Suporta tres modos:

1. **patch_mode**: Aplica diff unificado GNU (passar diff)
2. **replace_mode**: Substitui string exata por outra (passar old_str + new_str)
3. **insert_mode**: Insere conteudo apos linha especifica (passar after_line + content)

Para substituições simples, use replace_mode. Para diffs complexos, use patch_mode.
Todas as operacoes sao atomicas (tempfile + rename).`
}

func (t *PatchTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {
				"type": "string",
				"description": "Caminho do arquivo, relativo ou absoluto"
			},
			"diff": {
				"type": "string",
				"description": "Diff unificado GNU para aplicar (patch_mode)"
			},
			"old_str": {
				"type": "string",
				"description": "Texto exato a ser substituído (replace_mode, deve aparecer exatamente uma vez)"
			},
			"new_str": {
				"type": "string",
				"description": "Texto que substituirá old_str (replace_mode)"
			},
			"after_line": {
				"type": "integer",
				"description": "Número da linha após a qual inserir (insert_mode, 1-based)"
			},
			"content": {
				"type": "string",
				"description": "Conteúdo a ser inserido ou diff unificado"
			}
		}
	}`)
}

func (t *PatchTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p patchParams
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("patch_file: parametros invalidos: %w", err)
	}
	if p.Path == "" {
		return "", fmt.Errorf("patch_file: caminho nao pode estar vazio")
	}

	path, err := ensurePath(p.Path, t.Dir)
	if err != nil {
		return "", fmt.Errorf("patch_file: %w", err)
	}

	// Determina o modo de operação
	mode, err := detectMode(p)
	if err != nil {
		return "", fmt.Errorf("patch_file: %w", err)
	}

	// Lê o conteúdo atual
	oldContent, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("patch_file: nao foi possivel ler %q: %w", path, err)
	}

	var newContent []byte
	var info string

	switch mode {
	case modePatch:
		newContent, info, err = applyPatch(string(oldContent), p.Diff)
	case modeReplace:
		newContent, info, err = applyReplace(string(oldContent), p.OldStr, p.NewStr)
	case modeInsert:
		newContent, info, err = applyInsert(string(oldContent), p.After, p.Content)
	default:
		return "", fmt.Errorf("patch_file: modo invalido - especifique diff (patch), old_str+new_str (replace), ou after_line+content (insert)")
	}

	if err != nil {
		return "", fmt.Errorf("patch_file: %w", err)
	}

	// Escrita atômica usando tempfile + rename
	newPath, err := atomicWrite(path, newContent)
	if err != nil {
		return "", fmt.Errorf("patch_file: %w", err)
	}

	return fmt.Sprintf("%s - %s", info, t.relativePath(newPath)), nil
}

const (
	modePatch   = "patch"
	modeReplace = "replace"
	modeInsert  = "insert"
)

func detectMode(p patchParams) (string, error) {
	hasPatch := p.Diff != ""
	hasReplace := p.OldStr != "" && p.NewStr != ""
	hasInsert := p.After > 0 && p.Content != ""

	count := 0
	if hasPatch {
		count++
	}
	if hasReplace {
		count++
	}
	if hasInsert {
		count++
	}

	if count == 0 {
		return "", fmt.Errorf("modo indeterminado: especifique diff (patch), old_str+new_str (replace), ou after_line+content (insert)")
	}
	if count > 1 {
		return "", fmt.Errorf("modo ambiguo: so especifique um dos modos por vez")
	}

	if hasPatch {
		return modePatch, nil
	}
	if hasReplace {
		return modeReplace, nil
	}
	return modeInsert, nil
}

// applyApplyUnifiedDiff aplica um diff unificado usando o comando patch via subprocesso
// para maior confiabilidade com a formatação GNU.
func applyPatch(content, diff string) ([]byte, string, error) {
	// Usa patch via subprocesso para máxima compatibilidade com unidiff
	tmpIn, err := os.CreateTemp("", "patch-*.unified")
	if err != nil {
		return nil, "", fmt.Errorf("falha ao criar tempfile para diff: %w", err)
	}
	tmpInPath := tmpIn.Name()
	defer func() {
		tmpIn.Close()
		os.Remove(tmpInPath)
	}()

	if _, err := io.WriteString(tmpIn, diff); err != nil {
		tmpIn.Close()
		os.Remove(tmpInPath)
		return nil, "", fmt.Errorf("falha ao escrever diff: %w", err)
	}
	if err := tmpIn.Close(); err != nil {
		os.Remove(tmpInPath)
		return nil, "", fmt.Errorf("falha ao fechar tempfile: %w", err)
	}

	tmpFile, err := os.CreateTemp("", "patch-*.target")
	if err != nil {
		return nil, "", fmt.Errorf("falha ao criar tempfile alvo: %w", err)
	}
	tmpFilePath := tmpFile.Name()
	defer func() {
		tmpFile.Close()
		os.Remove(tmpFilePath)
	}()

	if _, err := io.WriteString(tmpFile, content); err != nil {
		tmpFile.Close()
		os.Remove(tmpFilePath)
		return nil, "", fmt.Errorf("falha ao escrever conteudo alvo: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpFilePath)
		return nil, "", fmt.Errorf("falha ao fechar tempfile: %w", err)
	}

	output, err := exec.Command("patch", "-s", "-o", tmpFilePath, tmpFilePath, tmpInPath).CombinedOutput()
	if err != nil {
		return nil, "", fmt.Errorf("falha ao aplicar diff: %w\n%s", err, string(output))
	}

	newContent, err := os.ReadFile(tmpFilePath)
	if err != nil {
		return nil, "", fmt.Errorf("falha ao ler resultado: %w", err)
	}

	oldLines := strings.Split(content, "\n")
	newLines := strings.Split(string(newContent), "\n")
	linesAdded := len(newLines) - len(oldLines)

	info := fmt.Sprintf("diff aplicado: +%d -%d linhas", linesAdded, 0)
	if linesAdded < 0 {
		info = fmt.Sprintf("diff aplicado: +%d -%d linhas", 0, -linesAdded)
	}

	return newContent, info, nil
}

// applyReplace aplica substituição exata de string com validação de unicidade.
func applyReplace(content, oldStr, newStr string) ([]byte, string, error) {
	if oldStr == "" {
		return nil, "", fmt.Errorf("old_str nao pode estar vazio")
	}

	count := strings.Count(content, oldStr)
	if count == 0 {
		return nil, "", fmt.Errorf("old_string '%s' nao encontrada no arquivo", truncateString(oldStr, 50))
	}
	if count > 1 {
		return nil, "", fmt.Errorf("old_string '%s' encontrada %d vezes - ambigua, torne a busca mais especifica", truncateString(oldStr, 50), count)
	}

	newContent := strings.Replace(content, oldStr, newStr, 1)

	oldLines := strings.Split(content, "\n")
	newLines := strings.Split(newContent, "\n")
	linesAdded := len(newLines) - len(oldLines)

	var info string
	if linesAdded > 0 {
		info = fmt.Sprintf("substituicao aplicada: +%d linhas", linesAdded)
	} else if linesAdded < 0 {
		info = fmt.Sprintf("substituicao aplicada: -%d linhas", -linesAdded)
	} else {
		info = "substituicao aplicada"
	}

	return []byte(newContent), info, nil
}

// applyInsert insere linhas após uma linha específica (1-based).
// after_line=1 insere após a linha 1
// after_line=0 insere antes da primeira linha (topo)
// after_line > numero de linhas insere no final
func applyInsert(content string, afterLine int, insertContent string) ([]byte, string, error) {
	if afterLine < 0 {
		return nil, "", fmt.Errorf("after_line deve ser >= 0")
	}

	lines := strings.Split(content, "\n")

	// after_line=0 -> index 0 (antes de tudo)
	// after_line=1 -> index 1 (após linha 1)
	// after_line > len(lines) -> end
	insertIdx := afterLine
	if afterLine == 0 {
		insertIdx = 0
	} else if afterLine >= len(lines) {
		// Append to end
		insertIdx = len(lines)
	}

	insertLines := strings.Split(insertContent, "\n")
	newLines := append([]string{}, lines[:insertIdx]...)
	newLines = append(newLines, insertLines...)
	newLines = append(newLines, lines[insertIdx:]...)

	newContent := strings.Join(newLines, "\n")

	// Garante newline no fim se conteudo original tinha
	if strings.HasSuffix(content, "\n") && !strings.HasSuffix(newContent, "\n") {
		newContent += "\n"
	}

	linesAdded := len(insertLines)
	infoLine := afterLine
	if afterLine == 0 {
		infoLine = 0
	} else {
		infoLine = afterLine
	}
	info := fmt.Sprintf("insercao aplicada: +%d linhas apos linha %d", linesAdded, infoLine)

	return []byte(newContent), info, nil
}

// atomicWrite escreve conteudo atomicamente usando tempfile + rename.
func atomicWrite(path string, content []byte) (string, error) {
	dir := filepath.Dir(path)
	if dir == "" || dir == "/" {
		dir = "."
	}

	tmpFile, err := os.CreateTemp(dir, ".patch-*.tmp")
	if err != nil {
		return "", fmt.Errorf("falha ao criar tempfile: %w", err)
	}
	tmpPath := tmpFile.Name()

	_, err = tmpFile.Write(content)
	closeErr := tmpFile.Close()
	if err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("falha ao escrever tempfile: %w", err)
	}
	if closeErr != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("falha ao fechar tempfile: %w", closeErr)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("falha ao finalizar escreva (rename): %w", err)
	}

	return path, nil
}

func (t *PatchTool) relativePath(p string) string {
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

func truncateString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// Regex para detectar inicio de hunk
var hunkHeaderRegex = regexp.MustCompile(`^@@ -(\d+),?(\d*) \+(\d+),?(\d*) @@`)
