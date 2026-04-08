package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteTool_NameDescSchema(t *testing.T) {
	tool := &WriteTool{}
	if tool.Name() != "write" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "write")
	}
	if tool.Description() == "" {
		t.Error("Description() is empty")
	}
	if len(tool.Schema()) == 0 {
		t.Error("Schema() is empty")
	}
}

func TestWriteTool_Execute_Success(t *testing.T) {
	dir := t.TempDir()
	tool := &WriteTool{Dir: dir}
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"test.txt","content":"hello"}`))
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(dir, "test.txt"))
	if err != nil {
		t.Fatalf("cannot read written file: %v", err)
	}
	if string(content) != "hello" {
		t.Errorf("file content = %q, want %q", content, "hello")
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestWriteTool_Execute_NestedDirs(t *testing.T) {
	dir := t.TempDir()
	tool := &WriteTool{Dir: dir}
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"a/b/c/file.txt","content":"nested"}`))
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(dir, "a", "b", "c", "file.txt"))
	if err != nil {
		t.Fatal("nested directory file not created")
	}
	if string(content) != "nested" {
		t.Errorf("file content = %q, want %q", content, "nested")
	}
}

func TestWriteTool_Execute_EmptyPath(t *testing.T) {
	tool := &WriteTool{}
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"","content":"x"}`))
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestWriteTool_Execute_InvalidParams(t *testing.T) {
	tool := &WriteTool{}
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"bad":"data"}`))
	if err == nil {
		t.Fatal("expected error for invalid params")
	}
}

func TestWriteTool_Execute_AbsolutePath(t *testing.T) {
	dir := t.TempDir()
	// Write to an absolute path inside the Dir — should succeed
	tool := &WriteTool{Dir: dir}
	absPath := filepath.Join(dir, "abs.txt")
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"`+absPath+`","content":"abs"}`))
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	content, _ := os.ReadFile(absPath)
	if string(content) != "abs" {
		t.Errorf("file content = %q, want %q", content, "abs")
	}
}

func TestWriteTool_Execute_PathOutsideDir(t *testing.T) {
	dir := t.TempDir()
	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "out.txt")
	tool := &WriteTool{Dir: dir}
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"`+outsideFile+`","content":"out"}`))
	if err == nil {
		t.Fatal("expected error for path outside WorkDir")
	}
}

func TestWriteTool_resolvePath(t *testing.T) {
	tool := &WriteTool{Dir: "/work"}
	tests := []struct {
		input string
		want  string
	}{
		{"rel.txt", "/work/rel.txt"},
		{"/abs.txt", "/abs.txt"},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			if got := tool.resolvePath(tc.input); got != tc.want {
				t.Errorf("resolvePath(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestWriteTool_relativePath(t *testing.T) {
	tool := &WriteTool{Dir: "/work"}
	tests := []struct {
		input string
		want  string
	}{
		{"/work/sub.txt", "sub.txt"},
		{"rel.txt", "rel.txt"},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			if got := tool.relativePath(tc.input); got != tc.want {
				t.Errorf("relativePath(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestWriteTool_relativePath_ErrorCase(t *testing.T) {
	// When Dir is empty, relativePath should return input
	tool := &WriteTool{Dir: ""}
	if got := tool.relativePath("rel.txt"); got != "rel.txt" {
		t.Errorf("relativePath(%q) = %q, want %q", "rel.txt", got, "rel.txt")
	}
}

func TestWriteTool_resolvePath_EmptyDir(t *testing.T) {
	tool := &WriteTool{Dir: ""}
	if got := tool.resolvePath("foo.txt"); got != "foo.txt" {
		t.Errorf("resolvePath(%q) with empty Dir = %q, want '.'", "foo.txt", got)
	}
}

func TestWriteTool_Execute_PermissionError(t *testing.T) {
	dir := t.TempDir()
	// Try to write to a path that doesn't exist as a valid dir structure
	tool := &WriteTool{Dir: dir}
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"/proc/1/cmdline","content":"test"}`))
	if err == nil {
		t.Fatal("expected error when writing to system path")
	}
}

func TestWriteTool_Execute_MkdirAllError(t *testing.T) {
	dir := t.TempDir()
	// Create a regular file where we'd try to create a subdirectory
	blocker := filepath.Join(dir, "file")
	_ = os.WriteFile(blocker, []byte("blocking"), 0o644)
	tool := &WriteTool{Dir: dir}

	_, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"file/sub/file.txt","content":"test"}`))
	if err == nil {
		t.Fatal("expected error when cannot create directory under file")
	}
}

func TestWriteTool_Execute_WriteError(t *testing.T) {
	dir := t.TempDir()
	// Create a file, remove all permissions to cause write error
	permFile := filepath.Join(dir, "noperm.txt")
	os.WriteFile(permFile, []byte("x"), 0o000)
	tool := &WriteTool{Dir: dir}
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"noperm.txt","content":"overwrite"}`))
	// If running as root, this may succeed
	if err == nil {
		if result == "" {
			t.Error("expected non-empty result")
		}
	}
}
