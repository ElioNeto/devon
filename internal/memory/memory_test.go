package memory_test

import (
	"context"
	"testing"

	"github.com/ElioNeto/devon/internal/db"
	"github.com/ElioNeto/devon/internal/memory"
)

const testProjectID = "testproject"

func TestRememberAndRecall(t *testing.T) {
	store, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("falha ao criar store em memoria: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	mgr := memory.New(store, testProjectID)

	// Remember um fato
	factContent := "usar fmt.Errorf com %w"
	if err := mgr.Remember(ctx, testProjectID, "convention", factContent); err != nil {
		t.Fatalf("falha ao salvar fato: %v", err)
	}

	// Recall os fatos
	facts, err := mgr.Recall(ctx, testProjectID, "convention", "")
	if err != nil {
		t.Fatalf("falha ao recuperar fatos: %v", err)
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
		t.Fatalf("falha ao criar store em memoria: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	mgr := memory.New(store, testProjectID)

	// Salva um fato
	content := "usar interfaces para injeção de dependência"
	if err := mgr.Remember(ctx, testProjectID, "architecture", content); err != nil {
		t.Fatalf("falha ao salvar fato: %v", err)
	}

	// ContextFor com keyword "interfaces"
	result, err := mgr.ContextFor(ctx, testProjectID, "interfaces")
	if err != nil {
		t.Fatalf("falha ao obter contexto: %v", err)
	}

	if result == "" {
		t.Fatal("expected non-empty context, got empty string")
	}

	if !contains(result, "## Memória do projeto") {
		t.Errorf("expected context to contain '## Memória do projeto', got: %s", result)
	}

	if !contains(result, "interfaces") {
		t.Errorf("expected context to contain 'interfaces', got: %s", result)
	}
}

func TestClear(t *testing.T) {
	store, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("falha ao criar store em memoria: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	mgr := memory.New(store, testProjectID)

	// Salva 2 fatos
	if err := mgr.Remember(ctx, testProjectID, "cat1", "content1"); err != nil {
		t.Fatalf("falha ao salvar fato 1: %v", err)
	}
	if err := mgr.Remember(ctx, testProjectID, "cat2", "content2"); err != nil {
		t.Fatalf("falha ao salvar fato 2: %v", err)
	}

	// Verifica que existem 2 fatos
	facts, err := mgr.Recall(ctx, testProjectID, "", "")
	if err != nil {
		t.Fatalf("falha ao recuperar fatos antes de clear: %v", err)
	}
	if len(facts) != 2 {
		t.Fatalf("expected 2 facts before clear, got %d", len(facts))
	}

	// Clear todos os fatos
	if err := mgr.Clear(ctx, testProjectID); err != nil {
		t.Fatalf("falha ao limpar memoria: %v", err)
	}

	// Verifica que não restam fatos
	facts, err = mgr.Recall(ctx, testProjectID, "", "")
	if err != nil {
		t.Fatalf("falha ao recuperar fatos apos clear: %v", err)
	}
	if len(facts) != 0 {
		t.Errorf("expected 0 facts after clear, got %d", len(facts))
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
