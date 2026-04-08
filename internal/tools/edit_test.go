package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditTool_NameDescSchema(t *testing.T) {
	tool := &EditTool{}
	if tool.Name() != "edit" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "edit")
	}
	if tool.Description() == "" {
		t.Error("Description() is empty")
	}
	if len(tool.Schema()) == 0 {
		t.Error("Schema() is empty")
	}
}

func TestEditTool_Execute_Success(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "test.go"), []byte("package main\n\nfunc hello() {\n\tprintln(\"hello\")\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := &EditTool{Dir: dir}
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"test.go","old_string":"println(\"hello\")","new_string":"println(\"world\")"}`))
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}

	// Verify content was changed
	content, err := os.ReadFile(filepath.Join(dir, "test.go"))
	if err != nil {
		t.Fatal(err)
	}
	want := `package main

func hello() {
	println("world")
}
`
	if string(content) != want {
		t.Errorf("file content = %q, want %q", string(content), want)
	}
}

func TestEditTool_Execute_NotFound(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello world\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := &EditTool{Dir: dir}
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"test.txt","old_string":"nonexistent","new_string":"xyz"}`))
	if err == nil {
		t.Fatal("expected error for old_string not found")
	}
	if !strings.Contains(err.Error(), "nao encontrado") {
		t.Errorf("error message should mention not found, got: %v", err)
	}
}

func TestEditTool_Execute_Ambiguous(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "test.txt"), []byte("foo\nfoo\nbar\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := &EditTool{Dir: dir}
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"test.txt","old_string":"foo","new_string":"baz"}`))
	if err == nil {
		t.Fatal("expected error for ambiguous old_string")
	}
	if !strings.Contains(err.Error(), "2 vezes") {
		t.Errorf("error message should mention occurrence count, got: %v", err)
	}
}

func TestEditTool_Execute_EmptyOldString(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "test.txt"), []byte("something\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := &EditTool{Dir: dir}
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"test.txt","old_string":"","new_string":"x"}`))
	if err == nil {
		t.Fatal("expected error for empty old_string")
	}
}

func TestEditTool_Execute_EmptyPath(t *testing.T) {
	tool := &EditTool{}
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"","old_string":"a","new_string":"b"}`))
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestEditTool_Execute_InvalidParams(t *testing.T) {
	tool := &EditTool{}
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"bad":"data"}`))
	if err == nil {
		t.Fatal("expected error for invalid params")
	}
}

func TestEditTool_Execute_FileNotFound(t *testing.T) {
	dir := t.TempDir()
	tool := &EditTool{Dir: dir}
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"missing.txt","old_string":"a","new_string":"b"}`))
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestEditTool_Execute_SingleOccurrenceWithSurrounding(t *testing.T) {
	dir := t.TempDir()
	content := `package main

func hello() {}

func goodbye() {
	hello()
}
`
	if err := os.WriteFile(filepath.Join(dir, "src.go"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := &EditTool{Dir: dir}
	// Replace the full function body — unique occurrence
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"src.go","old_string":"func goodbye() {\n\thello()\n}","new_string":"func goodbye() {\n\tprintln(\"bye\")\n}"}`))
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestEditTool_Execute_PathOutsideDir(t *testing.T) {
	dir := t.TempDir()
	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "out.txt")
	_ = os.WriteFile(outsideFile, []byte("out"), 0o644)
	tool := &EditTool{Dir: dir}
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"`+outsideFile+`","old_string":"out","new_string":"in"}`))
	if err == nil {
		t.Fatal("expected error for path outside WorkDir")
	}
}
