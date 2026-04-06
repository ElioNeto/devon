package agent

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
	msgs := []llm.Message{
		{Role: llm.RoleSystem, Content: "you are devon"},
	}
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
	if result[0].Role != llm.RoleSystem {
		t.Error("system prompt not preserved as first message after compaction")
	}
	if result[0].Content != "you are devon" {
		t.Errorf("system prompt content changed after compaction")
	}
	if len(result) >= len(msgs) {
		t.Errorf("compaction should reduce messages: %d -> %d", len(msgs), len(result))
	}
}

func TestEstimateTokens(t *testing.T) {
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: strings.Repeat("x", 400)},
	}
	tokens := estimateTokens(msgs)
	if tokens != 100 {
		t.Errorf("expected ~100 tokens for 400 chars, got %d", tokens)
	}
}
