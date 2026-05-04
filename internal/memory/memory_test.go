package memory_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ElioNeto/devon/internal/db"
	"github.com/ElioNeto/devon/internal/memory"
)

const testProjectID = "testproject"

func TestRememberAndRecall(t *testing.T) {
	store, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create in-memory store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	mgr := memory.New(store, testProjectID)

	// Remember a fact
	factContent := "usar fmt.Errorf com %w"
	if err := mgr.Remember(ctx, testProjectID, "convention", factContent); err != nil {
		t.Fatalf("failed to save fact: %v", err)
	}

	// Recall facts
	facts, err := mgr.Recall(ctx, testProjectID, "convention", "")
	if err != nil {
		t.Fatalf("failed to retrieve facts: %v", err)
	}

	if len(facts) != 1 {
		t.Fatalf("expected 1 fact, got %d", len(facts))
	}

	if facts[0].Content != factContent {
		t.Errorf("expected content %q, got %q", factContent, facts[0].Content)
	}

	if facts[0].Category != "convention" {
		t.Errorf("expected category %q, got %q", "convention", facts[0].Category)
	}
}

func TestContextFor(t *testing.T) {
	store, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create in-memory store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	mgr := memory.New(store, testProjectID)

	// Save a fact
	content := "usar interfaces para injeção de dependência"
	if err := mgr.Remember(ctx, testProjectID, "architecture", content); err != nil {
		t.Fatalf("failed to save fact: %v", err)
	}

	// ContextFor with keyword "interfaces"
	result, err := mgr.ContextFor(ctx, testProjectID, "interfaces")
	if err != nil {
		t.Fatalf("failed to get context: %v", err)
	}

	if result == "" {
		t.Fatal("expected non-empty context, got empty string")
	}

	if !strings.Contains(result, "## Memória do projeto") {
		t.Errorf("expected context to contain '## Memória do projeto', got: %s", result)
	}

	if !strings.Contains(result, "interfaces") {
		t.Errorf("expected context to contain 'interfaces', got: %s", result)
	}
}

func TestProjectIDFromWorkDir(t *testing.T) {
	id1 := memory.ProjectIDFromWorkDir("/tmp/project1")
	id2 := memory.ProjectIDFromWorkDir("/tmp/project2")
	id1again := memory.ProjectIDFromWorkDir("/tmp/project1")

	if id1 == "" {
		t.Error("expected non-empty project ID")
	}
	if len(id1) != 8 {
		t.Errorf("expected 8-char hash, got %d chars: %s", len(id1), id1)
	}
	if id1 == id2 {
		t.Error("expected different IDs for different paths")
	}
	if id1 != id1again {
		t.Error("expected same ID for same path")
	}
}

func TestContextFor_EmptyPrompt(t *testing.T) {
	store, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	mgr := memory.New(store, testProjectID)

	result, err := mgr.ContextFor(ctx, testProjectID, "")
	if err != nil {
		t.Fatalf("ContextFor with empty prompt: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty result for empty prompt, got: %s", result)
	}
}

func TestContextFor_NoMatch(t *testing.T) {
	store, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	mgr := memory.New(store, testProjectID)

	// Save a fact
	if err := mgr.Remember(ctx, testProjectID, "convention", "use interfaces"); err != nil {
		t.Fatalf("failed to save fact: %v", err)
	}

	// Query with unrelated keyword
	result, err := mgr.ContextFor(ctx, testProjectID, "zzzzz")
	if err != nil {
		t.Fatalf("ContextFor: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty result for unrelated keyword, got: %s", result)
	}
}

func TestRememberTool_Execute(t *testing.T) {
	store, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	mgr := memory.New(store, testProjectID)
	tool := &memory.RememberTool{Manager: mgr, ProjectID: testProjectID}

	// Valid execution
	result, err := tool.Execute(ctx, json.RawMessage(`{"category":"test","content":"hello world"}`))
	if err != nil {
		t.Fatalf("RememberTool.Execute failed: %v", err)
	}
	if !strings.Contains(result, "hello world") {
		t.Errorf("expected result to contain content, got: %s", result)
	}

	// Missing params
	_, err = tool.Execute(ctx, json.RawMessage(`{}`))
	if err == nil {
		t.Error("expected error for empty params")
	}

	// Invalid JSON
	_, err = tool.Execute(ctx, json.RawMessage(`{invalid}`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestRecallTool_Execute(t *testing.T) {
	store, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	mgr := memory.New(store, testProjectID)

	// Save a fact first
	if err := mgr.Remember(ctx, testProjectID, "arch", "dependency injection"); err != nil {
		t.Fatalf("failed to save fact: %v", err)
	}

	tool := &memory.RecallTool{Manager: mgr, ProjectID: testProjectID}

	// Recall all
	result, err := tool.Execute(ctx, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("RecallTool.Execute failed: %v", err)
	}
	if !strings.Contains(result, "dependency injection") {
		t.Errorf("expected result to contain fact content, got: %s", result)
	}

	// Filter by category
	result, err = tool.Execute(ctx, json.RawMessage(`{"category":"arch"}`))
	if err != nil {
		t.Fatalf("RecallTool.Execute with category failed: %v", err)
	}
	if !strings.Contains(result, "dependency injection") {
		t.Errorf("expected result to contain fact content, got: %s", result)
	}

	// No match category
	result, err = tool.Execute(ctx, json.RawMessage(`{"category":"nonexistent"}`))
	if err != nil {
		t.Fatalf("RecallTool.Execute with no-match category: %v", err)
	}
	if !strings.Contains(result, "No facts found") {
		t.Errorf("expected 'No facts found', got: %s", result)
	}

	// Invalid JSON
	_, err = tool.Execute(ctx, json.RawMessage(`{invalid}`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestRememberAndRecall_MultipleFacts(t *testing.T) {
	store, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	mgr := memory.New(store, testProjectID)

	// Save two facts with different content
	if err := mgr.Remember(ctx, testProjectID, "convention", "use fmt.Errorf with %w"); err != nil {
		t.Fatalf("failed to save fact 1: %v", err)
	}
	if err := mgr.Remember(ctx, testProjectID, "convention", "use interfaces for DI"); err != nil {
		t.Fatalf("failed to save fact 2: %v", err)
	}

	facts, err := mgr.Recall(ctx, testProjectID, "convention", "")
	if err != nil {
		t.Fatalf("failed to retrieve facts: %v", err)
	}
	if len(facts) != 2 {
		t.Errorf("expected 2 facts, got %d", len(facts))
	}
}

func TestRecall_SpecialCharacters(t *testing.T) {
	store, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	mgr := memory.New(store, testProjectID)

	specialContent := "cost > $100 & error == nil || value < 0.5"
	if err := mgr.Remember(ctx, testProjectID, "metrics", specialContent); err != nil {
		t.Fatalf("failed to save fact: %v", err)
	}

	facts, err := mgr.Recall(ctx, testProjectID, "", "$100")
	if err != nil {
		t.Fatalf("failed to retrieve facts: %v", err)
	}
	if len(facts) == 0 {
		t.Error("expected to find fact with special characters")
	}
}

func TestClear(t *testing.T) {
	store, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create in-memory store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	mgr := memory.New(store, testProjectID)

	// Save 2 facts
	if err := mgr.Remember(ctx, testProjectID, "cat1", "content1"); err != nil {
		t.Fatalf("failed to save fact 1: %v", err)
	}
	if err := mgr.Remember(ctx, testProjectID, "cat2", "content2"); err != nil {
		t.Fatalf("failed to save fact 2: %v", err)
	}

	// Verify there are 2 facts
	facts, err := mgr.Recall(ctx, testProjectID, "", "")
	if err != nil {
		t.Fatalf("failed to retrieve facts before clear: %v", err)
	}
	if len(facts) != 2 {
		t.Fatalf("expected 2 facts before clear, got %d", len(facts))
	}

	// Clear all facts
	if err := mgr.Clear(ctx, testProjectID); err != nil {
		t.Fatalf("failed to clear memory: %v", err)
	}

	// Verify no facts remain
	facts, err = mgr.Recall(ctx, testProjectID, "", "")
	if err != nil {
		t.Fatalf("failed to retrieve facts after clear: %v", err)
	}
	if len(facts) != 0 {
		t.Errorf("expected 0 facts after clear, got %d", len(facts))
	}
}


