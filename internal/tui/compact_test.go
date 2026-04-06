package tui

import (
	"strings"
	"testing"

	"github.com/ElioNeto/devon/internal/llm"
)

func TestCompactIfNeeded_NoCompactionNeeded(t *testing.T) {
	msgs := make([]llm.Message, 11) // 1 system + 10 others
	msgs[0] = llm.Message{Role: llm.RoleSystem, Content: "you are devon"}
	for i := 1; i <= 10; i++ {
		msgs[i] = llm.Message{Role: llm.RoleUser, Content: "msg"}
	}

	result, compacted := compactIfNeeded(msgs, "qwen", 1000)

	if compacted {
		t.Error("expected no compaction for 1000 tokens with 32k limit")
	}
	if len(result) != len(msgs) {
		t.Errorf("expected %d messages, got %d", len(msgs), len(result))
	}
}

func TestCompactIfNeeded_CompactionActivates(t *testing.T) {
	// Create enough content to exceed 80% of 32k limit (25600 tokens)
	msgs := []llm.Message{
		{Role: llm.RoleSystem, Content: "you are devon"},
	}
	// Each message of 800 chars = ~200 tokens. Need ~130+ messages for 25600 tokens
	for i := 0; i < 150; i++ {
		msgs = append(msgs, llm.Message{
			Role:    llm.RoleUser,
			Content: strings.Repeat("a", 800),
		})
	}
	msgs = append(msgs, llm.Message{
		Role:    llm.RoleAssistant,
		Content: "last reply",
	})

	used := estimateTokens(msgs)
	result, compacted := compactIfNeeded(msgs, "qwen", used)

	if !compacted {
		t.Fatalf("expected compaction, used=%d tokens, limit=32k, 80%%=%d", used, int(float64(32000)*0.80))
	}
	// System prompt must be preserved as first message
	if result[0].Role != llm.RoleSystem {
		t.Error("system prompt not preserved as first message after compaction")
	}
	if result[0].Content != "you are devon" {
		t.Errorf("system prompt content changed after compaction")
	}
	// Should have fewer messages than before
	if len(result) >= len(msgs) {
		t.Errorf("compaction should reduce messages: %d -> %d", len(msgs), len(result))
	}
}

func TestEstimateTokens(t *testing.T) {
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: strings.Repeat("x", 400)},
	}

	tokens := estimateTokens(msgs)

	// 400 / 4 = 100 tokens
	if tokens != 100 {
		t.Errorf("expected ~100 tokens for 400 chars, got %d", tokens)
	}
}
