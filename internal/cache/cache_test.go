package cache

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ElioNeto/devon/internal/config"
	"github.com/ElioNeto/devon/internal/llm"
	_ "modernc.org/sqlite"
)

// setupTestDB creates an in-memory SQLite database for testing.
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// setupTestCache creates a Cache with in-memory SQLite for testing.
func setupTestCache(t *testing.T, cfg config.CacheConfig) *Cache {
	t.Helper()
	db := setupTestDB(t)
	c, err := New(db, cfg)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	// Migrate schema manually since store.NewCacheStore does it
	return c
}

func TestCache_Hit(t *testing.T) {
	c := setupTestCache(t, config.CacheConfig{Enabled: true})
	ctx := context.Background()

	model := "test-model"
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: llm.TextContent("hello")},
	}

	// Set entry
	err := c.Set(ctx, model, msgs, "cached response", 10, nil)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Get entry - should hit
	result, err := c.Get(ctx, model, msgs, nil)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !result.Hit {
		t.Fatal("expected cache HIT")
	}
	if result.Response != "cached response" {
		t.Errorf("expected 'cached response', got %q", result.Response)
	}
	if result.TokensSaved != 10 {
		t.Errorf("expected 10 tokens saved, got %d", result.TokensSaved)
	}
}

func TestCache_Miss(t *testing.T) {
	c := setupTestCache(t, config.CacheConfig{Enabled: true})
	ctx := context.Background()

	model := "test-model"
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: llm.TextContent("nonexistent")},
	}

	result, err := c.Get(ctx, model, msgs, nil)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if result.Hit {
		t.Fatal("expected cache MISS for nonexistent key")
	}
}

func TestCache_TTLExpiry(t *testing.T) {
	c := setupTestCache(t, config.CacheConfig{
		Enabled: true,
		TTL:     "50ms",
	})
	ctx := context.Background()

	model := "test-model"
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: llm.TextContent("will expire")},
	}

	err := c.Set(ctx, model, msgs, "expiring response", 5, nil)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Immediate read should hit
	result, err := c.Get(ctx, model, msgs, nil)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !result.Hit {
		t.Fatal("expected HIT immediately after Set")
	}

	// Wait for TTL expiry
	time.Sleep(100 * time.Millisecond)

	// Should now miss because TTL expired
	result, err = c.Get(ctx, model, msgs, nil)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if result.Hit {
		t.Fatal("expected MISS after TTL expiry")
	}
}

func TestCache_FileInvalidation(t *testing.T) {
	c := setupTestCache(t, config.CacheConfig{Enabled: true})
	ctx := context.Background()

	// Create a temp file to track
	dir := t.TempDir()
	filePath := filepath.Join(dir, "tracked.go")
	if err := os.WriteFile(filePath, []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}

	model := "test-model"
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: llm.TextContent("with file")},
	}

	// Set entry tracking the file
	err := c.Set(ctx, model, msgs, "file-dependent response", 7, []string{filePath})
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Immediate read should hit
	result, err := c.Get(ctx, model, msgs, []string{filePath})
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !result.Hit {
		t.Fatal("expected HIT before file modification")
	}

	// Modify the file
	if err := os.WriteFile(filePath, []byte("package main\nfunc main() {}"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Should now miss due to file modification
	result, err = c.Get(ctx, model, msgs, []string{filePath})
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if result.Hit {
		t.Fatal("expected MISS after file modification")
	}
}

func TestCache_Stats(t *testing.T) {
	c := setupTestCache(t, config.CacheConfig{Enabled: true})
	ctx := context.Background()

	// Initially empty
	stats, err := c.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}
	if stats.TotalEntries != 0 {
		t.Errorf("expected 0 entries, got %d", stats.TotalEntries)
	}
	if stats.TotalTokens != 0 {
		t.Errorf("expected 0 total tokens, got %d", stats.TotalTokens)
	}

	model := "stats-model"
	msgs1 := []llm.Message{{Role: llm.RoleUser, Content: llm.TextContent("query1")}}
	msgs2 := []llm.Message{{Role: llm.RoleUser, Content: llm.TextContent("query2")}}

	if err := c.Set(ctx, model, msgs1, "response1", 100, nil); err != nil {
		t.Fatal(err)
	}
	if err := c.Set(ctx, model, msgs2, "response2", 200, nil); err != nil {
		t.Fatal(err)
	}

	stats, err = c.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}
	if stats.TotalEntries != 2 {
		t.Errorf("expected 2 entries, got %d", stats.TotalEntries)
	}
	if stats.TotalTokens != 300 {
		t.Errorf("expected 300 total tokens, got %d", stats.TotalTokens)
	}
}

func TestCache_Clear(t *testing.T) {
	c := setupTestCache(t, config.CacheConfig{Enabled: true})
	ctx := context.Background()

	model := "clear-model"
	msgs := []llm.Message{{Role: llm.RoleUser, Content: llm.TextContent("to-clear")}}

	if err := c.Set(ctx, model, msgs, "response", 50, nil); err != nil {
		t.Fatal(err)
	}

	// Verify it was stored
	result, err := c.Get(ctx, model, msgs, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Hit {
		t.Fatal("expected HIT before clear")
	}

	// Clear
	if err := c.Clear(ctx); err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	// Should miss after clear
	result, err = c.Get(ctx, model, msgs, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Hit {
		t.Fatal("expected MISS after clear")
	}

	// Stats should be zero
	stats, err := c.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stats.TotalEntries != 0 {
		t.Errorf("expected 0 entries after clear, got %d", stats.TotalEntries)
	}
}

func TestCache_NoTTL(t *testing.T) {
	c := setupTestCache(t, config.CacheConfig{Enabled: true, TTL: ""})
	ctx := context.Background()

	model := "no-ttl-model"
	msgs := []llm.Message{{Role: llm.RoleUser, Content: llm.TextContent("no-ttl")}}

	if err := c.Set(ctx, model, msgs, "persistent", 3, nil); err != nil {
		t.Fatal(err)
	}

	// Even after time, should still hit since no TTL
	result, err := c.Get(ctx, model, msgs, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Hit {
		t.Fatal("expected HIT with no TTL set")
	}
}

func TestCache_DifferentModelMiss(t *testing.T) {
	c := setupTestCache(t, config.CacheConfig{Enabled: true})
	ctx := context.Background()

	msgs := []llm.Message{{Role: llm.RoleUser, Content: llm.TextContent("same content")}}

	// Set for model A
	if err := c.Set(ctx, "model-a", msgs, "response-a", 5, nil); err != nil {
		t.Fatal(err)
	}

	// Get for model B - should miss
	result, err := c.Get(ctx, "model-b", msgs, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Hit {
		t.Fatal("expected MISS for different model")
	}
}
