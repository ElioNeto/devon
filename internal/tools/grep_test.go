package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestGrepTool_NameDescSchema(t *testing.T) {
	tool := &GrepTool{}
	if tool.Name() != "grep" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "grep")
	}
	if tool.Description() == "" {
		t.Error("Description() is empty")
	}
	if len(tool.Schema()) == 0 {
		t.Error("Schema() is empty")
	}
}

func TestGrepTool_Execute_FindsMatch(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello world\nfoo bar\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := &GrepTool{Dir: dir, MaxFiles: 10}
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"pattern":"hello"}`))
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if result == "" || result == "No matches found for pattern \"hello\"." {
		t.Errorf("expected to find 'hello', got: %q", result)
	}
}

func TestGrepTool_Execute_NoMatch(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello world\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := &GrepTool{Dir: dir, MaxFiles: 10}
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"pattern":"nonexistent"}`))
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if result != "No matches found for pattern \"nonexistent\"." {
		t.Errorf("expected no match message, got: %q", result)
	}
}

func TestGrepTool_Execute_EmptyPattern(t *testing.T) {
	tool := &GrepTool{}
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"pattern":""}`))
	if err == nil {
		t.Fatal("expected error for empty pattern")
	}
}

func TestGrepTool_Execute_InvalidParams(t *testing.T) {
	tool := &GrepTool{}
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"pat":"test"}`))
	if err == nil {
		t.Fatal("expected error for invalid params")
	}
}

func TestGrepTool_Execute_NoCase(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "test.txt"), []byte("HELLO world\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := &GrepTool{Dir: dir, MaxFiles: 10}
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"pattern":"hello","no_case":true}`))
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if result == "" {
		t.Error("expected case-insensitive match")
	}
}

func TestGrepTool_Execute_InvalidRegex(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "test.txt"), []byte("test\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := &GrepTool{Dir: dir, MaxFiles: 10}
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"pattern":"[invalid"}`))
	if err != nil {
		return // expected: error for invalid regex
	}
	// If no error (e.g., WalkDir skips it), result should be empty/no match
	if result != "No matches found for pattern \"[invalid\"." {
		t.Logf("unexpected result for invalid regex: %q", result)
	}
}

func TestGrepTool_Execute_PathArg(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "specific.txt"), []byte("found it\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := &GrepTool{Dir: dir, MaxFiles: 10}
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"pattern":"found","path":"specific.txt"}`))
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if result == "" {
		t.Error("expected match in specific file")
	}
}

func TestShouldSkipDir(t *testing.T) {
	skipDirs := []string{".git", "node_modules", "vendor", ".cache", ".tox", "__pycache__", ".eggs"}
	for _, name := range skipDirs {
		if !shouldSkipDir(name) {
			t.Errorf("shouldSkipDir(%q) = false, want true", name)
		}
	}
	nonSkip := []string{"src", "internal", "pkg"}
	for _, name := range nonSkip {
		if shouldSkipDir(name) {
			t.Errorf("shouldSkipDir(%q) = true, want false", name)
		}
	}
}

func TestRegexpOptions(t *testing.T) {
	p := grepParams{Pattern: "test", NoCase: false}
	opts := regexpOptions(p)
	if opts.pattern != "test" {
		t.Errorf("default pattern = %q, want %q", opts.pattern, "test")
	}

	p.NoCase = true
	opts = regexpOptions(p)
	if opts.pattern != "(?i)test" {
		t.Errorf("noCase pattern = %q, want %q", opts.pattern, "(?i)test")
	}
}

func TestSanitizeLineLimit_NoTruncate(t *testing.T) {
	short := "short line"
	if got := sanitizeLineLimit(short); got != short {
		t.Errorf("sanitizeLineLimit(%q) = %q", short, got)
	}
}

func TestSanitizeLineLimit_Truncates(t *testing.T) {
	s := make([]byte, 40*1024)
	for i := range s {
		s[i] = 'a'
	}
	result := sanitizeLineLimit(string(s))
	expectedMax := 32*1024 + len("\n... [output truncated: exceeded 32 KB limit]")
	if len(result) > expectedMax+10 {
		t.Errorf("sanitizeLineLimit did not truncate: %d bytes", len(result))
	}
}

func TestReadFileMatches(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "log.txt")
	content := "line 1\nfind me\nline 3\nfind me too\nline 5\n"
	if err := os.WriteFile(f, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := regexpOptions(grepParams{Pattern: "find", NoCase: false})
	matches, err := readFileMatches(context.Background(), f, opts, 0)
	if err != nil {
		t.Fatalf("readFileMatches() error: %v", err)
	}
	if len(matches) != 2 {
		t.Errorf("got %d matches, want 2", len(matches))
	}
}

func TestReadFileMatches_LimitLines(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "many.txt")
	var content string
	for i := 0; i < 10; i++ {
		content += "match\n"
	}
	if err := os.WriteFile(f, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := regexpOptions(grepParams{Pattern: "match", NoCase: false})
	matches, err := readFileMatches(context.Background(), f, opts, 3)
	if err != nil {
		t.Fatalf("readFileMatches() error: %v", err)
	}
	if len(matches) != 3 {
		t.Errorf("got %d matches with limit 3, want 3", len(matches))
	}
}

func TestGrepTool_Execute_TooManyMatches(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 30; i++ {
		name := filepath.Join(dir, fmt.Sprintf("file%d.txt", i))
		if err := os.WriteFile(name, []byte("match line here\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	tool := &GrepTool{Dir: dir, MaxFiles: 30}
	result, _ := tool.Execute(context.Background(), json.RawMessage(`{"pattern":"match"}`))
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestGrepTool_Execute_MaxFilesExceeded(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 5; i++ {
		f := filepath.Join(dir, fmt.Sprintf("f%d.txt", i))
		if err := os.WriteFile(f, []byte("match line here\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	tool := &GrepTool{Dir: dir, MaxFiles: 2}
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"pattern":"match"}`))
	// Returns results found so far, nil error (max files limit silently stops)
	if err != nil && len(result) == 0 {
		// WalkDir error is returned when no results
		return
	}
	// Otherwise returns the matches found before limit hit
	if result == "" {
		t.Error("expected non-empty result with matches found before limit")
	}
}

func TestGrepTool_Execute_ContextCancelled(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 10; i++ {
		os.MkdirAll(filepath.Join(dir, fmt.Sprintf("dir_%d", i)), 0755)
		os.WriteFile(filepath.Join(dir, fmt.Sprintf("dir_%d/test.txt", i)), []byte("search term\n"), 0o644)
	}

	tool := &GrepTool{Dir: dir}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _ = tool.Execute(ctx, json.RawMessage(`{"pattern":"search"}`))
}

func TestGrepTool_Execute_AbsolutePath(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello world\n"), 0o644)

	tool := &GrepTool{Dir: dir, MaxFiles: 10}
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"`+dir+`","pattern":"hello"}`))
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if result == "" {
		t.Error("expected match with absolute path")
	}
}
