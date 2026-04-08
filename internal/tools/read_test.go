package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadTool_NameDescSchema(t *testing.T) {
	tool := &ReadTool{}
	if tool.Name() != "read" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "read")
	}
	if tool.Description() == "" {
		t.Error("Description() is empty")
	}
	if len(tool.Schema()) == 0 {
		t.Error("Schema() is empty")
	}
}

func TestReadTool_Execute_Success(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello\nworld"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := &ReadTool{Dir: dir}
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"file":"test.txt"}`))
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if result != "   1\thello\n   2\tworld" {
		t.Errorf("Execute() output unexpected: %q", result)
	}
}

func TestReadTool_Execute_FileNotFound(t *testing.T) {
	dir := t.TempDir()
	tool := &ReadTool{Dir: dir}
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"file":"missing.txt"}`))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestReadTool_Execute_EmptyPath(t *testing.T) {
	tool := &ReadTool{}
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"file":""}`))
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestReadTool_Execute_InvalidParams(t *testing.T) {
	tool := &ReadTool{}
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"data":"bad"}`))
	if err == nil {
		t.Fatal("expected error for invalid params")
	}
}

func TestReadTool_Execute_AbsolutePath(t *testing.T) {
	dir := t.TempDir()
	absPath := filepath.Join(dir, "abs.txt")
	if err := os.WriteFile(absPath, []byte("abs content"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := &ReadTool{Dir: dir}
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"file":"`+absPath+`"}`))
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty output for absolute path")
	}
}

func TestReadTool_Execute_PathOutsideDir(t *testing.T) {
	dir := t.TempDir()
	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "secret.txt")
	_ = os.WriteFile(outsideFile, []byte("secret"), 0o644)
	tool := &ReadTool{Dir: dir}
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"file":"`+outsideFile+`"}`))
	if err == nil {
		t.Fatal("expected error for path outside WorkDir")
	}
}

func TestReadTool_resolvePath(t *testing.T) {
	tool := &ReadTool{Dir: "/work"}
	tests := []struct {
		input string
		want  string
	}{
		{"relative.txt", "/work/relative.txt"},
		{"/absolute.txt", "/absolute.txt"},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			if got := tool.resolvePath(tc.input); got != tc.want {
				t.Errorf("resolvePath(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestReadTool_resolvePath_EmptyDir(t *testing.T) {
	tool := &ReadTool{}
	if got := tool.resolvePath("foo.txt"); got != "foo.txt" {
		t.Errorf("resolvePath(%q) with empty Dir = %q, want 'foo.txt'", "foo.txt", got)
	}
}

func TestReadTool_Execute_LargeFile(t *testing.T) {
	dir := t.TempDir()
	large := make([]byte, 1024*1024+1) // > 1MB
	for i := range large {
		large[i] = 'x'
	}
	if err := os.WriteFile(filepath.Join(dir, "large.txt"), large, 0o644); err != nil {
		t.Fatal(err)
	}
	tool := &ReadTool{Dir: dir}
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"file":"large.txt"}`))
	if err == nil {
		t.Fatal("expected error for large file")
	}
}

func TestReadTool_Execute_BinaryFile(t *testing.T) {
	dir := t.TempDir()
	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	_ = os.WriteFile(filepath.Join(dir, "image.png"), pngHeader, 0o644)
	tool := &ReadTool{Dir: dir}
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"file":"image.png"}`))
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if result != "[arquivo binario: image.png, 8 bytes]" {
		t.Errorf("binary result = %q, expected binary message", result)
	}
}

func TestReadTool_Execute_Offset(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "test.txt"), []byte("one\ntwo\nthree\nfour\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := &ReadTool{Dir: dir}
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"file":"test.txt","offset":3}`))
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !strings.Contains(result, "three") {
		t.Errorf("expected line 3 (three) in output with offset 3, got: %q", result)
	}
}

func TestReadTool_Execute_Limit(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "test.txt"), []byte("one\ntwo\nthree\nfour\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := &ReadTool{Dir: dir}
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"file":"test.txt","limit":2}`))
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if strings.Contains(result, "three") {
		t.Errorf("expected only 2 lines (limit=2), but three was found in: %q", result)
	}
	if !strings.Contains(result, "one") {
		t.Errorf("expected line one in output")
	}
	if !strings.Contains(result, "two") {
		t.Errorf("expected line two in output")
	}
}

func TestReadTool_Execute_OffsetAndLimit(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "test.txt"), []byte("one\ntwo\nthree\nfour\nfive\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := &ReadTool{Dir: dir}
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"file":"test.txt","offset":2,"limit":2}`))
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !strings.Contains(result, "two") {
		t.Errorf("expected line two in output, got: %q", result)
	}
	if !strings.Contains(result, "three") {
		t.Errorf("expected line three in output, got: %q", result)
	}
	if strings.Contains(result, "four") {
		t.Errorf("expected only 2 lines but found four in: %q", result)
	}
}

func TestReadTool_Execute_OffsetOutOfRange(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "test.txt"), []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := &ReadTool{Dir: dir}
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"file":"test.txt","offset":100}`))
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !strings.Contains(result, "fora do intervalo") {
		t.Errorf("expected out of range message, got: %q", result)
	}
}
