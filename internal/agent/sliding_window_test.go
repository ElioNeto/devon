package agent

import (
	"strings"
	"testing"

	"github.com/ElioNeto/devon/internal/llm"
)

// ── applySlidingWindow tests ────────────────────────────────────────────────

func TestApplySlidingWindow_ZeroTurns(t *testing.T) {
	// Only system messages, no user turns
	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: llm.TextContent("You are a helpful assistant.")},
	}
	filtered, removed := applySlidingWindow(messages, 5)
	if removed != 0 {
		t.Errorf("removed = %d, want 0", removed)
	}
	if len(filtered) != len(messages) {
		t.Errorf("len(filtered) = %d, want %d", len(filtered), len(messages))
	}
}

func TestApplySlidingWindow_FewerTurnsThanLimit(t *testing.T) {
	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: llm.TextContent("system")},
		{Role: llm.RoleUser, Content: llm.TextContent("hello")},
		{Role: llm.RoleAssistant, Content: llm.TextContent("hi")},
	}
	filtered, removed := applySlidingWindow(messages, 10)
	if removed != 0 {
		t.Errorf("removed = %d, want 0", removed)
	}
	if len(filtered) != len(messages) {
		t.Errorf("len(filtered) = %d, want %d", len(filtered), len(messages))
	}
}

func TestApplySlidingWindow_MoreTurnsThanLimit(t *testing.T) {
	// 3 turns, max=2 → 1 turn removed
	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: llm.TextContent("system")},
		{Role: llm.RoleUser, Content: llm.TextContent("turn1")},
		{Role: llm.RoleAssistant, Content: llm.TextContent("resp1")},
		{Role: llm.RoleUser, Content: llm.TextContent("turn2")},
		{Role: llm.RoleAssistant, Content: llm.TextContent("resp2")},
		{Role: llm.RoleUser, Content: llm.TextContent("turn3")},
		{Role: llm.RoleAssistant, Content: llm.TextContent("resp3")},
	}
	filtered, removed := applySlidingWindow(messages, 2)
	if removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}
	// Expected: system + placeholder + last 2 turns (4 messages) = 6 total
	if len(filtered) != 6 {
		t.Errorf("len(filtered) = %d, want 6", len(filtered))
	}
	// Verify system message preserved
	if filtered[0].Role != llm.RoleSystem {
		t.Errorf("filtered[0].Role = %q, want %q", filtered[0].Role, llm.RoleSystem)
	}
	// Verify placeholder present
	if filtered[1].Role != llm.RoleSystem || filtered[1].Content == nil {
		t.Error("filtered[1] should be a system placeholder message")
	} else if !strings.Contains(*filtered[1].Content, "histórico anterior omitido") {
		t.Errorf("placeholder should mention history omitted, got: %s", *filtered[1].Content)
	}
	// Verify last two turns are preserved
	if filtered[2].Role != llm.RoleUser || *filtered[2].Content != "turn2" {
		t.Errorf("expected turn2 as first kept turn, got: %+v", filtered[2])
	}
	if filtered[4].Role != llm.RoleUser || *filtered[4].Content != "turn3" {
		t.Errorf("expected turn3 as last turn, got: %+v", filtered[4])
	}
}

func TestApplySlidingWindow_PlaceholderMessagePresent(t *testing.T) {
	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: llm.TextContent("system")},
		{Role: llm.RoleUser, Content: llm.TextContent("a")},
		{Role: llm.RoleAssistant, Content: llm.TextContent("b")},
		{Role: llm.RoleUser, Content: llm.TextContent("c")},
		{Role: llm.RoleAssistant, Content: llm.TextContent("d")},
	}
	filtered, removed := applySlidingWindow(messages, 1)
	if removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}
	// system + placeholder + 1 turn = 4
	if len(filtered) != 4 {
		t.Errorf("len(filtered) = %d, want 4", len(filtered))
	}
	if filtered[1].Role != llm.RoleSystem {
		t.Errorf("placeholder should be RoleSystem, got %q", filtered[1].Role)
	}
}

func TestApplySlidingWindow_PlaceholderAbsentWhenUnderLimit(t *testing.T) {
	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: llm.TextContent("system")},
		{Role: llm.RoleUser, Content: llm.TextContent("only turn")},
	}
	filtered, removed := applySlidingWindow(messages, 5)
	if removed != 0 {
		t.Errorf("removed = %d, want 0", removed)
	}
	// Should be unchanged — no placeholder
	if len(filtered) != 2 {
		t.Errorf("len(filtered) = %d, want 2 (unchanged)", len(filtered))
	}
}

func TestApplySlidingWindow_EmptySlice(t *testing.T) {
	filtered, removed := applySlidingWindow([]llm.Message{}, 5)
	if removed != 0 {
		t.Errorf("removed = %d, want 0", removed)
	}
	if len(filtered) != 0 {
		t.Errorf("len(filtered) = %d, want 0", len(filtered))
	}
}

func TestApplySlidingWindow_NilSlice(t *testing.T) {
	filtered, removed := applySlidingWindow(nil, 5)
	if removed != 0 {
		t.Errorf("removed = %d, want 0", removed)
	}
	if filtered != nil {
		t.Errorf("filtered should be nil, got %+v", filtered)
	}
}

func TestApplySlidingWindow_ZeroMaxTurnsNoop(t *testing.T) {
	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: llm.TextContent("system")},
		{Role: llm.RoleUser, Content: llm.TextContent("hello")},
	}
	filtered, removed := applySlidingWindow(messages, 0)
	if removed != 0 {
		t.Errorf("removed = %d, want 0", removed)
	}
	if len(filtered) != len(messages) {
		t.Errorf("len(filtered) = %d, want %d", len(filtered), len(messages))
	}
}

func TestApplySlidingWindow_NegativeMaxTurnsNoop(t *testing.T) {
	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: llm.TextContent("system")},
		{Role: llm.RoleUser, Content: llm.TextContent("hello")},
	}
	filtered, removed := applySlidingWindow(messages, -1)
	if removed != 0 {
		t.Errorf("removed = %d, want 0", removed)
	}
	if len(filtered) != len(messages) {
		t.Errorf("len(filtered) = %d, want %d", len(filtered), len(messages))
	}
}

func TestApplySlidingWindow_WithToolMessages(t *testing.T) {
	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: llm.TextContent("system")},
		{Role: llm.RoleUser, Content: llm.TextContent("do something")},
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "t1"}}},
		{Role: llm.RoleTool, ToolCallID: "t1", Content: llm.TextContent("result")},
		{Role: llm.RoleAssistant, Content: llm.TextContent("done")},
		{Role: llm.RoleUser, Content: llm.TextContent("next")},
		{Role: llm.RoleAssistant, Content: llm.TextContent("ok")},
	}
	filtered, removed := applySlidingWindow(messages, 1)
	if removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}
	// system + placeholder + turn2 (user+assistant) = 4
	if len(filtered) != 4 {
		t.Errorf("len(filtered) = %d, want 4", len(filtered))
	}
	if *filtered[2].Content != "next" {
		t.Errorf("first kept turn should be 'next', got %s", *filtered[2].Content)
	}
}

// ── truncateToolResult tests ────────────────────────────────────────────────

func TestTruncateToolResult_ShortResult(t *testing.T) {
	result := "short result"
	got := truncateToolResult(result, 100)
	if got != result {
		t.Errorf("got %q, want %q", got, result)
	}
}

func TestTruncateToolResult_LongResultWithNewlines(t *testing.T) {
	// Create a result with 10 lines, each ~20 chars, total ~200 chars
	lines := make([]string, 10)
	for i := range lines {
		lines[i] = "line number " + string(rune('0'+i))
	}
	result := strings.Join(lines, "\n")
	// Max 50 chars → truncated, should have some lines omitted
	got := truncateToolResult(result, 50)
	if len(got) > 50+50 { // allow for suffix
		t.Errorf("truncated result too long: %d chars", len(got))
	}
	if !strings.Contains(got, "linhas omitidas") {
		t.Errorf("expected 'linhas omitidas' in truncated result, got: %s", got)
	}
}

func TestTruncateToolResult_LongResultWithoutNewlines(t *testing.T) {
	result := "abcdefghijklmnopqrstuvwxyz" // 26 chars
	got := truncateToolResult(result, 10)
	if len(got) > 10+40 {
		t.Errorf("truncated result too long: %d chars", len(got))
	}
	if !strings.Contains(got, "caracteres omitidos") {
		t.Errorf("expected 'caracteres omitidos' in truncated result, got: %s", got)
	}
	if !strings.HasPrefix(got, "abcdefghij") {
		t.Errorf("expected prefix 'abcdefghij', got: %s", got)
	}
}

func TestTruncateToolResult_ExactlyAtLimit(t *testing.T) {
	result := "exactly 20 chars!!"
	got := truncateToolResult(result, 20)
	if got != result {
		t.Errorf("result at limit should be unchanged, got %q", got)
	}
}

func TestTruncateToolResult_EmptyString(t *testing.T) {
	got := truncateToolResult("", 100)
	if got != "" {
		t.Errorf("empty string should remain empty, got %q", got)
	}
}

func TestTruncateToolResult_ZeroMaxChars(t *testing.T) {
	result := "some result"
	got := truncateToolResult(result, 0)
	if got != result {
		t.Errorf("zero maxChars should return unchanged, got %q", got)
	}
}

func TestTruncateToolResult_NegativeMaxChars(t *testing.T) {
	result := "some result"
	got := truncateToolResult(result, -1)
	if got != result {
		t.Errorf("negative maxChars should return unchanged, got %q", got)
	}
}

// ── truncationStats zero value ──────────────────────────────────────────────

func TestTruncationStats_ZeroValue(t *testing.T) {
	var s truncationStats
	if s.TurnsRemoved != 0 || s.ToolCharsSaved != 0 || s.ToolTruncatedCount != 0 || s.CacheHits != 0 {
		t.Error("truncationStats zero value should have all fields at 0")
	}
}
