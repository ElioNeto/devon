package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)


func TestPatchTool_ReplaceSimple(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	content := `func Main() {
    const SECRET = 'hardcoded'
    use SECRET
}`

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := &PatchTool{Dir: dir}
	params := mustJSON(t, patchParams{
		Path:    "test.txt",
		OldStr:  "const SECRET = 'hardcoded'",
		NewStr:  "const SECRET = process.env.JWT_SECRET",
	})

	result, err := tool.Execute(context.Background(), params)
	if err != nil {
		t.Fatalf("Execute falhou: %v", err)
	}

	newContent, _ := os.ReadFile(path)
	expected := `func Main() {
    const SECRET = process.env.JWT_SECRET
    use SECRET
}`

	if string(newContent) != expected {
		t.Errorf("conteudo inesperado\ngot:  %q\nwant: %q", string(newContent), expected)
	}

	if !contains(result, "substituicao aplicada") {
		t.Errorf("resultado esperado mencione subistituicao: %q", result)
	}
}

func TestPatchTool_ReplaceNotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	content := `func Main() {
    const SECRET = 'hardcoded'
}`

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := &PatchTool{Dir: dir}
	params := mustJSON(t, patchParams{
		Path:    "test.txt",
		OldStr:  "const NONEXISTENT = 'value'",
		NewStr:  "const NEW = 'value'",
	})

	_, err := tool.Execute(context.Background(), params)
	if err == nil {
		t.Fatal("esperado erro quando string nao encontrada")
	}

	if !contains(err.Error(), "nao encontrada") {
		t.Errorf("erro esperado mencione nao encontrada: %v", err)
	}
}

func TestPatchTool_ReplaceAmbiguous(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	content := `const SECRET = 'hardcoded'
const SECRET = 'hardcoded'
const SECRET = 'hardcoded'
`

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := &PatchTool{Dir: dir}
	params := mustJSON(t, patchParams{
		Path:    "test.txt",
		OldStr:  "const SECRET = 'hardcoded'",
		NewStr:  "const SECRET = process.env.JWT_SECRET",
	})

	_, err := tool.Execute(context.Background(), params)
	if err == nil {
		t.Fatal("esperado erro quando string ambigua")
	}

	if !contains(err.Error(), "ambigua") && !contains(err.Error(), "3 vezes") {
		t.Errorf("erro esperado mencione ambiguidade: %v", err)
	}
}

func TestPatchTool_InsertSimple(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	content := `func Main() {
    console.log("hello")
}`

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := &PatchTool{Dir: dir}
	params := mustJSON(t, patchParams{
		Path:    "test.txt",
		After:   1,
		Content: "    const MSG = 'world'",
	})

	result, err := tool.Execute(context.Background(), params)
	if err != nil {
		t.Fatalf("Execute falhou: %v", err)
	}

	newContent, _ := os.ReadFile(path)
	// after=1 significa apos a linha 1 (após "func Main() {")
	expected := `func Main() {
    const MSG = 'world'
    console.log("hello")
}`

	if string(newContent) != expected {
		t.Errorf("conteudo inesperado\ngot:  %q\nwant: %q", string(newContent), expected)
	}

	if !contains(result, "insercao aplicada: +1 linhas apos linha 1") {
		t.Errorf("resultado esperado mencione insercao: %q", result)
	}
}

func TestPatchTool_InsertAtStart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	content := `line2
line3
`

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := &PatchTool{Dir: dir}
	params := mustJSON(t, patchParams{
		Path:    "test.txt",
		After:   0,
		Content: "line1",
	})

	result, err := tool.Execute(context.Background(), params)
	if err != nil {
		t.Fatalf("Execute falhou: %v", err)
	}

	newContent, _ := os.ReadFile(path)
	expected := `line1
line2
line3
`

	if string(newContent) != expected {
		t.Errorf("conteudo inesperado\ngot: %q\nwant: %q", string(newContent), expected)
	}

	if !contains(result, "insercao aplicada") {
		t.Errorf("resultado esperado mencione insercao: %q", result)
	}
}

func TestPatchTool_InsertAtEnd(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	content := `line1
line2
`

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := &PatchTool{Dir: dir}
	params := mustJSON(t, patchParams{
		Path:    "test.txt",
		After:   100, // maior que numero de linhas
		Content: "line3",
	})

	result, err := tool.Execute(context.Background(), params)
	if err != nil {
		t.Fatalf("Execute falhou: %v", err)
	}

	newContent, _ := os.ReadFile(path)
	expected := `line1
line2
line3
`

	if string(newContent) != expected {
		t.Errorf("conteudo inesperado\ngot: %q\nwant: %q", string(newContent), expected)
	}

	if !contains(result, "insercao aplicada") {
		t.Errorf("resultado esperado mencione insercao: %q", result)
	}
}

func TestPatchTool_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	content := `original
`

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := &PatchTool{Dir: dir}
	params := mustJSON(t, patchParams{
		Path:    "test.txt",
		OldStr:  "original",
		NewStr:  "replaced",
	})

	_, err := tool.Execute(context.Background(), params)
	if err != nil {
		t.Fatalf("Execute falhou: %v", err)
	}

	// Verifica que arquivo foi substituir atomicamente (não existe .tmp)
	files, _ := filepath.Glob(filepath.Join(dir, "*.tmp"))
	if len(files) > 0 {
		t.Errorf("tempfiles nao foram limpos: %v", files)
	}

	finalContent, _ := os.ReadFile(path)
	if string(finalContent) != "replaced\n" {
		t.Errorf("conteudo invalido: %q", string(finalContent))
	}
}

func TestPatchTool_FileNotFound(t *testing.T) {
	dir := t.TempDir()
	_ = filepath.Join(dir, "nonexistent.txt")

	tool := &PatchTool{Dir: dir}
	params := mustJSON(t, patchParams{
		Path:    "nonexistent.txt",
		OldStr:  "test",
		NewStr:  "value",
	})

	_, err := tool.Execute(context.Background(), params)
	if err == nil {
		t.Fatal("esperado erro quando arquivo nao existe")
	}

	if !contains(err.Error(), "nao foi possivel ler") {
		t.Errorf("erro esperado mencione leitura: %v", err)
	}
}

func TestPatchTool_PatchDiffSimple(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	content := `func Main() {
    const SECRET = 'hardcoded'
}`

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := &PatchTool{Dir: dir}

	diff := ` @@ -1,3 +1,3 @@
 func Main() {
-    const SECRET = 'hardcoded'
+    const SECRET = process.env.JWT_SECRET
 }`

	params := mustJSON(t, patchParams{
		Path: "test.txt",
		Diff: diff,
	})

	result, err := tool.Execute(context.Background(), params)
	if err != nil {
		t.Fatalf("Execute falhou: %v", err)
	}

	newContent, _ := os.ReadFile(path)
	expected := `func Main() {
    const SECRET = process.env.JWT_SECRET
}`

	if string(newContent) != expected {
		t.Fatalf("conteudo inesperado\ngot:  %q\nwant: %q", string(newContent), expected)
	}

	if !contains(result, "diff aplicado") {
		t.Errorf("resultado esperado mencione diff aplicado: %q", result)
	}
}

func TestPatchTool_PatchDiffInvalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	content := `func Main() {
    const SECRET = 'hardcoded'
}`

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := &PatchTool{Dir: dir}

	// Diff invalido
	params := mustJSON(t, patchParams{
		Path: "test.txt",
		Diff: `@@ Invalid header @@`,
	})

	_, err := tool.Execute(context.Background(), params)
	if err == nil {
		t.Fatal("esperado erro para diff invalido")
	}

	t.Logf("Erro esperado para diff invalido: %v", err)
}

func TestPatchTool_Modes(t *testing.T) {
	tests := []struct {
		name        string
		params      patchParams
		expectError bool
		description string
	}{
		{
			name:        "sem modo",
			params:      patchParams{Path: "x.txt"},
			expectError: true,
		},
		{
			name: "modo patch e replace",
			params: patchParams{
				Path:    "x.txt",
				Diff:    "@@ @@\n-test",
				OldStr:  "test",
				NewStr:  "new",
			},
			expectError: true,
		},
		{
			name: "modo apenas replace",
			params: patchParams{
				Path:    "x.txt",
				OldStr:  "test",
				NewStr:  "new",
			},
			expectError: false,
		},
		{
			name: "modo apenas insert",
			params: patchParams{
				Path:    "x.txt",
				After:   0,
				Content: "test",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Cria arquivo temporário
			dir := t.TempDir()
			path := filepath.Join(dir, "test.txt")
			os.WriteFile(path, []byte("original\n"), 0o644)

			tool := &PatchTool{Dir: dir}
			params := mustJSON(t, tt.params)

			_, err := tool.Execute(context.Background(), params)
			if tt.expectError && err == nil {
				t.Errorf("esperado erro mas obteve sucesso")
			}
			if !tt.expectError && err != nil {
				t.Errorf("esperado sucesso mas obteve erro: %v", err)
			}
		})
	}
}

func TestPatchTool_EdgeCases(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	tool := &PatchTool{Dir: dir}

	t.Run("substiçao_multilinea", func(t *testing.T) {
		content := `func(a, b int) {
    return a + b
}`
		os.WriteFile(path, []byte(content), 0o644)

		params := mustJSON(t, patchParams{
			Path:    "test.txt",
			OldStr:  "return a + b",
			NewStr:  "return a + b + 1",
		})

		result, err := tool.Execute(context.Background(), params)
		if err != nil {
			t.Fatalf("Execute falhou: %v", err)
		}
		t.Logf("Resultado: %s", result)
	})

	t.Run("insercao_multilinea", func(t *testing.T) {
		content := `func Main() {
}`
		os.WriteFile(path, []byte(content), 0o644)

		newLines := `    // Import statements
    import "fmt"
    import "log"`

		params := mustJSON(t, patchParams{
			Path:    "test.txt",
			After:   1,
			Content: newLines,
		})

		result, err := tool.Execute(context.Background(), params)
		if err != nil {
			t.Fatalf("Execute falhou: %v", err)
		}
		t.Logf("Resultado: %s", result)
	})
}

func mustJSON(t *testing.T, v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("falha ao marshallar: %v", err)
	}
	return data
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
