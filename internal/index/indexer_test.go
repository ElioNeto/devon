package index_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ElioNeto/devon/internal/index"
)

func TestIndexer_NewIndexer(t *testing.T) {
	t.Run("creates indexer with default config", func(t *testing.T) {
		idx, err := index.NewIndexer("/tmp/test", index.Config{})
		if err != nil {
			t.Fatalf("NewIndexer failed: %v", err)
		}
		defer idx.Close()

		stats := idx.GetStats()
		if stats.TotalDocs != 0 {
			t.Errorf("expected 0 docs, got %d", stats.TotalDocs)
		}
	})

	t.Run("applies default config values", func(t *testing.T) {
		cfg := index.Config{}
		cfg.WithDefaults()

		if len(cfg.Extensions) == 0 {
			t.Error("extensions should have defaults")
		}
		if len(cfg.Excludes) == 0 {
			t.Error("excludes should have defaults")
		}
		if cfg.MaxFileSizeKB <= 0 {
			t.Error("MaxFileSizeKB should be positive")
		}
	})

	t.Run("rebuilds index", func(t *testing.T) {
		idx, err := index.NewIndexer("/tmp/test", index.Config{})
		if err != nil {
			t.Fatalf("NewIndexer failed: %v", err)
		}
		defer idx.Close()

		// Create test files
		tmpDir := t.TempDir()
		testFile1 := tmpDir + "/test1.go"
		testFile2 := tmpDir + "/test2.go"

	content1 := `package main

func main() {
	println("Hello, World!")
}

type User struct {
	Name string
	Age  int
}`

content2 := `package index

import "context"

func Search(ctx context.Context) ([]string, error) {
	return []string{"hello", "world"}, nil
}`

if err := writeTestFile(testFile1, content1); err != nil {
t.Fatalf("writeTestFile failed: %v", err)
}
if err := writeTestFile(testFile2, content2); err != nil {
t.Fatalf("writeTestFile failed: %v", err)
}

		// Index files
		if err := idx.Rebuild(context.Background(), tmpDir); err != nil {
			t.Fatalf("Rebuild failed: %v", err)
		}

		stats := idx.GetStats()
		if stats.TotalDocs < 2 {
			t.Errorf("expected at least 2 docs, got %d", stats.TotalDocs)
		}
	})
}

func TestIndexer_Search(t *testing.T) {
	ctx := context.Background()

	idx, err := index.NewIndexer("/tmp/test_search", index.Config{})
	if err != nil {
		t.Fatalf("NewIndexer failed: %v", err)
	}
	defer idx.Close()

// Create test directory and files
tmpDir := t.TempDir()
writeTestFileT(t, tmpDir+"/auth.go", `package auth

import "context"

func Authenticate(ctx context.Context, username, password string) (bool, error) {
	return checkCredentials(username, password)
}

func checkCredentials(user, pass string) bool {
	return len(user) > 0 && len(pass) > 0
}`)

writeTestFileT(t, tmpDir+"/handler.go", `package main

import "net/http"

func HandleRequest(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}`)

	// Index
	if err := idx.Rebuild(ctx, tmpDir); err != nil {
		t.Fatalf("Rebuild failed: %v", err)
	}

	t.Run("search finds relevant files", func(t *testing.T) {
		results := idx.Search("authenticate", 5)

		if len(results) == 0 {
			t.Error("expected results for 'authenticate' query")
		}

		found := false
		for _, r := range results {
			if r.Path == "auth.go" {
				found = true
				break
			}
		}
		if !found {
			t.Error("auth.go should be in results for authenticate query")
		}
	})

	t.Run("search is case insensitive", func(t *testing.T) {
		results1 := idx.Search("authenticate", 5)
		results2 := idx.Search("AUTHENTICATE", 5)

		if len(results1) != len(results2) {
			t.Errorf("case insensitive search mismatch: %d vs %d",
				len(results1), len(results2))
		}
	})

	t.Run("search returns scored results", func(t *testing.T) {
		results := idx.Search("http", 5)

		for _, r := range results {
			if r.Score <= 0 {
				t.Errorf("expected positive score, got %.2f", r.Score)
			}
		}
	})
}

func TestIndexer_Exclusions(t *testing.T) {
	ctx := context.Background()

// Custom config with specific exclusions
cfg := index.DefaultConfig()
cfg.Excludes = []string{"vendor/", "node_modules/"}

idx, err := index.NewIndexer("/tmp/test_exclude", cfg)
if err != nil {
t.Fatalf("NewIndexer failed: %v", err)
}
defer idx.Close()

tmpDir := t.TempDir()

// Create files in excluded directories
exclDir := tmpDir + "/vendor/other"
if err := os.MkdirAll(exclDir, 0755); err != nil {
t.Fatalf("MkdirAll failed: %v", err)
}

writeTestFileT(t, tmpDir+"/main.go", `package main

func main() {}`)

writeTestFileT(t, exclDir+"/vendor.go", `package vendor

func VendorFunc() {}`)

	// Index
	if err := idx.Rebuild(ctx, tmpDir); err != nil {
		t.Fatalf("Rebuild failed: %v", err)
	}

	stats := idx.GetStats()

	// vendor files should be excluded
	if stats.TotalDocs > 1 {
		t.Errorf("vendor files should be excluded, got %d docs", stats.TotalDocs)
	}
}

func TestIndexer_AddFile(t *testing.T) {
	ctx := context.Background()

	idx, err := index.NewIndexer("/tmp/test_add", index.Config{})
	if err != nil {
		t.Fatalf("NewIndexer failed: %v", err)
	}
	defer idx.Close()

	tmpDir := t.TempDir()
	testFile := tmpDir + "/test.go"

	content := `package test

func TestFunction() string {
	return "test"
}`

	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Add single file
	if _, err := idx.AddFile(ctx, "test.go", content); err != nil {
		t.Fatalf("AddFile failed: %v", err)
	}

	stats := idx.GetStats()
	if stats.TotalDocs != 1 {
		t.Errorf("expected 1 doc, got %d", stats.TotalDocs)
	}

	// Remove file
	idx.RemoveFile("test.go")
	stats = idx.GetStats()
	if stats.TotalDocs != 0 {
		t.Errorf("expected 0 docs after removal, got %d", stats.TotalDocs)
	}
}

func TestIndexer_FileSizeLimit(t *testing.T) {
	cfg := index.DefaultConfig()
	cfg.MaxFileSizeKB = 1 // 1 KB limit

	idx, err := index.NewIndexer("/tmp/test_size", cfg)
	if err != nil {
		t.Fatalf("NewIndexer failed: %v", err)
	}
	defer idx.Close()

	ctx := context.Background()
	tmpDir := t.TempDir()

	// Create small file (should be indexed)
	smallFile := tmpDir + "/small.txt"
	smallContent := "small"
	if err := os.WriteFile(smallFile, []byte(smallContent), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Create large file (should be skipped)
	largeFile := tmpDir + "/large.txt"
	largeContent := make([]byte, 2000) // 2 KB
	for i := range largeContent {
		largeContent[i] = 'a'
	}
	if err := os.WriteFile(largeFile, largeContent, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	if err := idx.Rebuild(ctx, tmpDir); err != nil {
		t.Fatalf("Rebuild failed: %v", err)
	}

	stats := idx.GetStats()
	if stats.TotalDocs != 1 {
		t.Errorf("expected 1 doc (small file only), got %d", stats.TotalDocs)
	}
}

func TestIndexer_Config(t *testing.T) {
	t.Run("withDefaults sets defaults", func(t *testing.T) {
		cfg := index.Config{}
		cfg.WithDefaults()

		if len(cfg.Extensions) == 0 {
			t.Error("expected extensions to be set")
		}
		if len(cfg.Excludes) == 0 {
			t.Error("expected excludes to be set")
		}
	})
}

func TestManager_CreateEnableDisable(t *testing.T) {
	cfg := index.ManagerConfig{Enabled: false}
	mgr, err := index.NewManager("/tmp/test_mgr", cfg)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	defer mgr.Close()

	if mgr.IsEnabled() {
		t.Error("expected manager to be disabled initially")
	}

	mgr.Enable()
	if !mgr.IsEnabled() {
		t.Error("expected manager to be enabled after Enable()")
	}

	mgr.Disable()
	if mgr.IsEnabled() {
		t.Error("expected manager to be disabled after Disable()")
	}
}

func TestManager_SearchDisabled(t *testing.T) {
	cfg := index.ManagerConfig{Enabled: false}
	mgr, err := index.NewManager("/tmp/test_mgr_disabled", cfg)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	defer mgr.Close()

	results, err := mgr.Search("test", 5)
	if err != nil {
		t.Fatalf("Search on disabled manager should not error: %v", err)
	}
	if results != nil {
		t.Error("expected nil results when disabled")
	}
}

func TestManager_IndexAndSearch(t *testing.T) {
	cfg := index.ManagerConfig{Enabled: true}
	mgr, err := index.NewManager("/tmp/test_mgr_search", cfg)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	defer mgr.Close()

	ctx := context.Background()
	tmpDir := t.TempDir()

	writeTestFileT(t, tmpDir+"/hello.go", `package main
func main() { println("hello world") }`)

	if err := mgr.Index(ctx, tmpDir); err != nil {
		t.Fatalf("Index failed: %v", err)
	}

	results, err := mgr.Search("hello", 5)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected search results for 'hello'")
	}
}

func TestManager_Rebuild(t *testing.T) {
	cfg := index.ManagerConfig{Enabled: true}
	mgr, err := index.NewManager("/tmp/test_mgr_rebuild", cfg)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	defer mgr.Close()

	ctx := context.Background()
	tmpDir := t.TempDir()

	writeTestFileT(t, tmpDir+"/test.go", `package test
func Foo() { return }`)

	if err := mgr.Rebuild(ctx, tmpDir); err != nil {
		t.Fatalf("Rebuild failed: %v", err)
	}

	if mgr.GetStats().TotalDocs != 1 {
		t.Errorf("expected 1 doc after rebuild, got %d", mgr.GetStats().TotalDocs)
	}
}

func TestManager_GetIndex(t *testing.T) {
	cfg := index.ManagerConfig{Enabled: true}
	mgr, err := index.NewManager("/tmp/test_mgr_index", cfg)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	defer mgr.Close()

	idx := mgr.GetIndex()
	if idx == nil {
		t.Error("expected non-nil Index from GetIndex")
	}
}

func TestManager_CreateTool(t *testing.T) {
	cfg := index.ManagerConfig{Enabled: true, IndexedConfig: index.IndexedConfig{TopK: 10}}
	mgr, err := index.NewManager("/tmp/test_mgr_tool", cfg)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	defer mgr.Close()

	tool := mgr.CreateTool()
	if tool == nil {
		t.Fatal("expected non-nil tool from CreateTool")
	}
	if tool.Name() != "search_codebase" {
		t.Errorf("expected tool name 'search_codebase', got %q", tool.Name())
	}
}

func TestSearcher_NilSafe(t *testing.T) {
	searcher := index.NewSearcher(nil)

	if results := searcher.Search("test", 5); results != nil {
		t.Error("expected nil results from nil index")
	}
	if doc := searcher.SearchByPath("/nonexistent"); doc != nil {
		t.Error("expected nil doc from nil index")
	}
	if results := searcher.SearchRegex("test", 5); results != nil {
		t.Error("expected nil results from nil index")
	}
	if results := searcher.SearchPrefix("test", 5); results != nil {
		t.Error("expected nil results from nil index")
	}
}

func TestSearchCodebaseTool_Execute(t *testing.T) {
	ctx := context.Background()
	idx, err := index.NewIndexer("/tmp/test_tool_exec", index.Config{})
	if err != nil {
		t.Fatalf("NewIndexer failed: %v", err)
	}
	defer idx.Close()

	tmpDir := t.TempDir()
	writeTestFileT(t, tmpDir+"/tool.go", `package tool
func SearchTool() string { return "found" }`)

	if err := idx.Rebuild(ctx, tmpDir); err != nil {
		t.Fatalf("Rebuild failed: %v", err)
	}

	tool := index.NewSearchCodebaseTool(idx, 5)

	// Valid execution — search for "found" which appears in the file content
	result, err := tool.Execute(ctx, json.RawMessage(`{"query":"found"}`))
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !strings.Contains(result, "tool.go") {
		t.Errorf("expected result to contain file path, got: %s", result)
	}

	// Empty query
	_, err = tool.Execute(ctx, json.RawMessage(`{"query":""}`))
	if err == nil {
		t.Error("expected error for empty query")
	}

	// Invalid JSON
	_, err = tool.Execute(ctx, json.RawMessage(`{invalid}`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}

	// No match
	result, err = tool.Execute(ctx, json.RawMessage(`{"query":"zzzzz_nonexistent"}`))
	if err != nil {
		t.Fatalf("Execute with no match failed: %v", err)
	}
	if !strings.Contains(result, "No matching files found") {
		t.Errorf("expected 'No matching files found', got: %s", result)
	}

	// With top_k
	result, err = tool.Execute(ctx, json.RawMessage(`{"query":"found","top_k":3}`))
	if err != nil {
		t.Fatalf("Execute with top_k failed: %v", err)
	}
	if !strings.Contains(result, "tool.go") {
		t.Errorf("expected result to contain file path, got: %s", result)
	}
}

func TestNewTokenizer(t *testing.T) {
	tokenizer := index.NewTokenizer()

	tokens := tokenizer.Tokenize("Hello World")
	if len(tokens) != 2 {
		t.Errorf("expected 2 tokens, got %d: %v", len(tokens), tokens)
	}
	if len(tokens) > 0 && tokens[0].Text != "hello" {
		t.Errorf("expected 'hello', got %q", tokens[0].Text)
	}

	// Stopwords are removed
	tokens = tokenizer.Tokenize("the quick brown fox")
	// "the" is a stopword, so only "quick", "brown", "fox" should remain
	if len(tokens) != 3 {
		t.Errorf("expected 3 tokens after stopword removal, got %d: %v", len(tokens), tokens)
	}

	// Empty string
	tokens = tokenizer.Tokenize("")
	if len(tokens) != 0 {
		t.Errorf("expected 0 tokens for empty string, got %d", len(tokens))
	}

	// Short tokens are excluded (minLength=2 means keep tokens with length >= 2)
	tokens = tokenizer.Tokenize("a bc def")
	// "a" (len 1) is too short, "bc" (len 2) and "def" (len 3) are kept
	if len(tokens) != 2 {
		t.Errorf("expected 2 tokens (minLength=2 keeps bc, def), got %d: %v", len(tokens), tokens)
	}

	// WithMinLength
	tokenizer2 := index.NewTokenizer().WithMinLength(3)
	tokens = tokenizer2.Tokenize("a bc def")
	if len(tokens) != 1 || tokens[0].Text != "def" {
		t.Errorf("expected only 'def', got %v", tokens)
	}
}

func TestNewBM25Calculator(t *testing.T) {
	bm25 := index.NewBM25Calculator(1.2, 0.75)

	score := bm25.ScoreTerm(2, 3, 10, 50, 30)
	if score <= 0 {
		t.Errorf("expected positive score, got %f", score)
	}

	// Zero document frequency
	score = bm25.ScoreTerm(2, 0, 10, 50, 30)
	if score != 0 {
		t.Errorf("expected 0 score when docFreq=0, got %f", score)
	}

	// Zero total docs
	score = bm25.ScoreTerm(2, 3, 0, 50, 30)
	if score != 0 {
		t.Errorf("expected 0 score when totalDocs=0, got %f", score)
	}
}

func TestIndexer_ShouldExcludeViaIndexing(t *testing.T) {
	ctx := context.Background()
	cfg := index.DefaultConfig()
	cfg.Excludes = []string{"vendor/", "node_modules/"}

	idx, err := index.NewIndexer("/tmp/test_exclude_via_idx", cfg)
	if err != nil {
		t.Fatalf("NewIndexer failed: %v", err)
	}
	defer idx.Close()

	tmpDir := t.TempDir()
	os.MkdirAll(tmpDir+"/vendor/pkg", 0755)
	os.MkdirAll(tmpDir+"/node_modules/pkg", 0755)
	writeTestFileT(t, tmpDir+"/main.go", `package main
func main() {}`)
	writeTestFileT(t, tmpDir+"/vendor/pkg/lib.go", `package lib
func Lib() {}`)
	writeTestFileT(t, tmpDir+"/node_modules/pkg/index.js", `const x = 1;`)

	if err := idx.Rebuild(ctx, tmpDir); err != nil {
		t.Fatalf("Rebuild failed: %v", err)
	}

	if idx.GetStats().TotalDocs != 1 {
		t.Errorf("expected 1 doc (only main.go), got %d", idx.GetStats().TotalDocs)
	}
}

func TestConfig_WithDefaults(t *testing.T) {
	cfg := index.Config{}
	cfg.WithDefaults()

	if len(cfg.Extensions) == 0 {
		t.Error("expected extensions to have defaults")
	}
	if len(cfg.Excludes) == 0 {
		t.Error("expected excludes to have defaults")
	}
	if cfg.MaxFileSizeKB <= 0 {
		t.Error("expected MaxFileSizeKB to be positive")
	}
	if cfg.TopK <= 0 {
		t.Error("expected TopK to be positive")
	}
}

func TestIndex_EmptyIndex(t *testing.T) {
	idx := index.NewIndex()
	if idx.TotalDocs() != 0 {
		t.Errorf("expected 0 docs in empty index, got %d", idx.TotalDocs())
	}
	if idx.TermCount() != 0 {
		t.Errorf("expected 0 terms in empty index, got %d", idx.TermCount())
	}
	if doc := idx.GetDocument("/nonexistent"); doc != nil {
		t.Error("expected nil for nonexistent document")
	}

	results := idx.Search("test", 5)
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty index, got %d", len(results))
	}
}

func TestIndex_RemoveDocument(t *testing.T) {
	ctx := context.Background()

	idx, err := index.NewIndexer("/tmp/test_remove", index.Config{})
	if err != nil {
		t.Fatalf("NewIndexer failed: %v", err)
	}
	defer idx.Close()

	// Add a test file
	if _, err := idx.AddFile(ctx, "test.go", "package test\nfunc Test() {}"); err != nil {
		t.Fatalf("AddFile failed: %v", err)
	}

	stats := idx.GetStats()
	if stats.TotalDocs != 1 {
		t.Fatalf("expected 1 doc after add, got %d", stats.TotalDocs)
	}

	// Remove it
	idx.RemoveFile("test.go")
	stats = idx.GetStats()
	if stats.TotalDocs != 0 {
		t.Errorf("expected 0 docs after removal, got %d", stats.TotalDocs)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := index.DefaultConfig()
	if len(cfg.Extensions) == 0 {
		t.Error("DefaultConfig should have extensions")
	}
	if cfg.TopK != 5 {
		t.Errorf("expected TopK=5, got %d", cfg.TopK)
	}
}

// Helper functions
func writeTestFile(path, content string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}

func writeTestFileT(t *testing.T, path, content string) {
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
}
