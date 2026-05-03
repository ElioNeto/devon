package index_test

import (
	"context"
	"os"
	"path/filepath"
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

// Authentication module for user credentials
func Authenticate(ctx context.Context, username, password string) (bool, error) {
	return checkCredentials(username, password)
}

// checkCredentials validates user credentials
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

	stats := idx.GetStats()
	t.Logf("Index stats: %d docs, %d terms", stats.TotalDocs, stats.TermCount)

	t.Run("search finds relevant files", func(t *testing.T) {
		results := idx.Search("authenticate", 5)

		// Debug: log all paths found and terms
		terms := idx.GetIndex().GetTerms()
		t.Logf("All terms in index: %v", terms)
		t.Logf("Search results for 'authenticate': %d", len(results))
		for _, r := range results {
			t.Logf("  Found: path=%q score=%.4f", r.Path, r.Score)
		}

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
		results1 := idx.Search("auth", 5)
		results2 := idx.Search("AUTH", 5)

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
