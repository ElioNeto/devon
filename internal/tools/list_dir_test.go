package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListDirTool_NameDescSchema(t *testing.T) {
	tool := &ListDirTool{}
	if tool.Name() != "list_dir" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "list_dir")
	}
	if tool.Description() == "" {
		t.Error("Description() is empty")
	}
	if len(tool.Schema()) == 0 {
		t.Error("Schema() is empty")
	}
}

func TestListDirTool_Execute_Success(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hello"), 0o644)
	_ = os.Mkdir(filepath.Join(dir, "subdir"), 0o755)

	tool := &ListDirTool{Dir: dir}
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"path":""}`))
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestListDirTool_Execute_DefaultPath(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "default.txt"), []byte("data"), 0o644)

	tool := &ListDirTool{Dir: dir}
	result, err := tool.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !strings.Contains(result, "default.txt") {
		t.Errorf("expected file default.txt in output, got: %q", result)
	}
}

func TestListDirTool_Execute_NotADir(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "file.txt"), []byte("data"), 0o644)

	tool := &ListDirTool{Dir: dir}
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"file.txt"}`))
	if err == nil {
		t.Fatal("expected error for non-directory path")
	}
}

func TestListDirTool_Execute_NotFound(t *testing.T) {
	dir := t.TempDir()
	tool := &ListDirTool{Dir: dir}
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"nonexistent"}`))
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}
}

func TestListDirTool_Execute_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	tool := &ListDirTool{Dir: dir}
	result, err := tool.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if result != "(diretorio vazio)" {
		t.Errorf("expected empty dir message, got: %q", result)
	}
}

func TestListDirTool_resolvePath(t *testing.T) {
	tests := []struct {
		name  string
		dir   string
		input string
		want  string
	}{
		{"absolute", "/work", "/abs/path", "/abs/path"},
		{"relative", "/work", "sub", "/work/sub"},
		{"empty input", "/work", "", "/work"},
		{"empty dir", "", "foo", "foo"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tool := &ListDirTool{Dir: tc.dir}
			if got := tool.resolvePath(tc.input); got != tc.want {
				t.Errorf("resolvePath(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
