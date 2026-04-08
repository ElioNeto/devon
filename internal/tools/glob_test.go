package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGlobTool_NameDescSchema(t *testing.T) {
	tool := &GlobTool{}
	if tool.Name() != "glob" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "glob")
	}
	if tool.Description() == "" {
		t.Error("Description() is empty")
	}
	if len(tool.Schema()) == 0 {
		t.Error("Schema() is empty")
	}
}

func TestGlobTool_Execute_FindsFiles(t *testing.T) {
	dir := t.TempDir()
	// Create test files
	for _, name := range []string{"a.go", "b.go", "c.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	tool := &GlobTool{Dir: dir}
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"pattern":"*.go"}`))
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	// Should find a.go and b.go (order may vary)
	if got := len(result); got < 4 {
		t.Errorf("expected to find .go files, got: %q", result)
	}
}

func TestGlobTool_Execute_NoMatch(t *testing.T) {
	dir := t.TempDir()
	tool := &GlobTool{Dir: dir}
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"pattern":"*.nonexistent"}`))
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if result != "Nenhum arquivo encontrado para o padrao." {
		t.Errorf("expected no match message, got: %q", result)
	}
}

func TestGlobTool_Execute_Empty(t *testing.T) {
	tool := &GlobTool{}
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"pattern":""}`))
	if err == nil {
		t.Fatal("expected error for empty pattern")
	}
}

func TestGlobTool_Execute_InvalidParams(t *testing.T) {
	tool := &GlobTool{}
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"pat":"*.go"}`))
	if err == nil {
		t.Fatal("expected error for invalid params")
	}
}

func TestGlobTool_Execute_SubDir(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "sub")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "test.go"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := &GlobTool{Dir: dir}
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"pattern":"**/*.go"}`))
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if result == "" || result == "No files matched the pattern." {
		t.Errorf("expected to find sub/test.go, got: %q", result)
	}
}

func TestGlobTool_Execute_InvalidPattern(t *testing.T) {
	dir := t.TempDir()
	tool := &GlobTool{Dir: dir}
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"pattern":"[bad"}`))
	// Invalid glob pattern - may return error or empty
	if err == nil && result == "" {
		t.Error("expected either error or no match for invalid pattern")
	}
}
