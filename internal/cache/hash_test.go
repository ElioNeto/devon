package cache

import (
	"testing"

	"github.com/ElioNeto/devon/internal/llm"
)

func TestHashKey_Deterministic(t *testing.T) {
	model := "gpt-4"
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: llm.TextContent("hello world")},
	}

	h1 := HashKey(model, msgs)
	h2 := HashKey(model, msgs)

	if h1 != h2 {
		t.Errorf("HashKey should be deterministic, got %q vs %q", h1, h2)
	}

	if len(h1) != 64 {
		t.Errorf("HashKey should be 64 hex chars (SHA-256), got %d", len(h1))
	}
}

func TestHashKey_DifferentModel_DifferentHash(t *testing.T) {
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: llm.TextContent("hello")},
	}

	h1 := HashKey("model-a", msgs)
	h2 := HashKey("model-b", msgs)

	if h1 == h2 {
		t.Error("HashKey should differ for different models")
	}
}

func TestHashKey_DifferentMessages_DifferentHash(t *testing.T) {
	model := "gpt-4"

	h1 := HashKey(model, []llm.Message{
		{Role: llm.RoleUser, Content: llm.TextContent("message one")},
	})
	h2 := HashKey(model, []llm.Message{
		{Role: llm.RoleUser, Content: llm.TextContent("message two")},
	})

	if h1 == h2 {
		t.Error("HashKey should differ for different messages")
	}
}

func TestHashKey_EmptyMessages(t *testing.T) {
	model := "gpt-4"
	h := HashKey(model, []llm.Message{})

	if h == "" {
		t.Error("HashKey should not be empty even with empty messages")
	}
	if len(h) != 64 {
		t.Errorf("HashKey should be 64 hex chars, got %d", len(h))
	}
}

func TestHashKey_MultipleMessages(t *testing.T) {
	model := "claude-3"
	msgs := []llm.Message{
		{Role: llm.RoleSystem, Content: llm.TextContent("You are a helpful assistant.")},
		{Role: llm.RoleUser, Content: llm.TextContent("What is Go?")},
		{Role: llm.RoleAssistant, Content: llm.TextContent("Go is a programming language.")},
	}

	h1 := HashKey(model, msgs)
	h2 := HashKey(model, msgs)

	if h1 != h2 {
		t.Errorf("HashKey should be deterministic with multiple messages, got %q vs %q", h1, h2)
	}
}

func TestHashKey_SameContentSameHash(t *testing.T) {
	model := "test-model"
	msgs1 := []llm.Message{
		{Role: llm.RoleUser, Content: llm.TextContent("some content")},
	}
	msgs2 := []llm.Message{
		{Role: llm.RoleUser, Content: llm.TextContent("some content")},
	}

	h1 := HashKey(model, msgs1)
	h2 := HashKey(model, msgs2)

	if h1 != h2 {
		t.Error("HashKey should produce same hash for same content")
	}
}

func TestMessageForTask(t *testing.T) {
	msgs := MessageForTask("my task")
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Role != llm.RoleUser {
		t.Errorf("expected role user, got %s", msgs[0].Role)
	}
	if msgs[0].Content == nil || *msgs[0].Content != "my task" {
		t.Errorf("expected content 'my task', got %v", msgs[0].Content)
	}
}
