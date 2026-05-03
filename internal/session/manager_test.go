package session_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ElioNeto/devon/internal/db"
	"github.com/ElioNeto/devon/internal/session"
)

func TestManagerCRUD(t *testing.T) {
	dir, err := os.MkdirTemp("", "session-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	dbPath := filepath.Join(dir, "test.db")
	store, err := db.New(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	mgr := session.NewManager(store)
	ctx := context.Background()

	// Create sessions
	sid1 := "test-session-1"
	sid2 := "test-session-2"

	if err := mgr.Create(ctx, sid1, "test task 1", "gpt-4"); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if err := mgr.Create(ctx, sid2, "test task 2", "claude-3"); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// List sessions
	sessions, err := mgr.List(ctx, 10)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}

	// Get session detail
	s, err := mgr.Get(ctx, sid1)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if s == nil {
		t.Fatal("expected session, got nil")
	}
	if s.ID != sid1 {
		t.Fatalf("expected id %q, got %q", sid1, s.ID)
	}

	// Update session
	if err := mgr.Update(ctx, sid1, "updated task", "gpt-4-turbo", "active"); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	s, err = mgr.Get(ctx, sid1)
	if err != nil {
		t.Fatalf("Get after update failed: %v", err)
	}
	if s.Task != "updated task" {
		t.Fatalf("expected task 'updated task', got %q", s.Task)
	}
	if s.Model != "gpt-4-turbo" {
		t.Fatalf("expected model 'gpt-4-turbo', got %q", s.Model)
	}

	// Delete session
	if err := mgr.Delete(ctx, sid1); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	s, err = mgr.Get(ctx, sid1)
	if err != nil {
		t.Fatalf("Get after delete failed: %v", err)
	}
	if s != nil {
		t.Fatal("expected nil after delete")
	}

	// Stats
	stats, err := mgr.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}
	if stats.TotalSessions != 1 {
		t.Fatalf("expected 1 session, got %d", stats.TotalSessions)
	}
}

func TestManagerEmpty(t *testing.T) {
	dir, err := os.MkdirTemp("", "session-empty-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	dbPath := filepath.Join(dir, "empty.db")
	store, err := db.New(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	mgr := session.NewManager(store)
	ctx := context.Background()

	// List when empty
	sessions, err := mgr.List(ctx, 10)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions, got %d", len(sessions))
	}

	// Get non-existent
	s, err := mgr.Get(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if s != nil {
		t.Fatal("expected nil for nonexistent session")
	}

	// Stats when empty
	stats, err := mgr.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}
	if stats.TotalSessions != 0 {
		t.Fatalf("expected 0 sessions in stats, got %d", stats.TotalSessions)
	}
}

func TestManagerListLimit(t *testing.T) {
	dir, err := os.MkdirTemp("", "session-limit-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	dbPath := filepath.Join(dir, "limit.db")
	store, err := db.New(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	mgr := session.NewManager(store)
	ctx := context.Background()

	// Create 5 sessions
	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("session-%d", i)
		if err := mgr.Create(ctx, id, fmt.Sprintf("task %d", i), "model-x"); err != nil {
			t.Fatal(err)
		}
	}

	// List with limit 3
	sessions, err := mgr.List(ctx, 3)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(sessions) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(sessions))
	}

	// List with limit 0 (should use default 50)
	sessions, err = mgr.List(ctx, 0)
	if err != nil {
		t.Fatalf("List with 0 failed: %v", err)
	}
	if len(sessions) != 5 {
		t.Fatalf("expected 5 sessions with limit 0, got %d", len(sessions))
	}
}
