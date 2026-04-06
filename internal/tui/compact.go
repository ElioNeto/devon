// Package tui — context compaction when approaching model token limits.
package tui

import (
	"math"
	"strings"

	"github.com/ElioNeto/devon/internal/llm"
)

// contextLimits maps model names (or prefixes) to their token limits.
var contextLimits = map[string]int{
	"gpt-4o":       128_000,
	"gpt-4o-mini":  128_000,
	"gpt-4":        8_192,
	"claude":       200_000,
	"gemini-2.5":   1_000_000,
	"qwen":         32_000,
}

// estimateTokens returns a rough token count using 4-chars-per-token heuristic.
func estimateTokens(messages []llm.Message) int {
	total := 0
	for _, msg := range messages {
		if msg.Content != nil {
			total += len(*msg.Content) / 4
		}
	}
	return total
}

// compactIfNeeded checks if the message history exceeds 80% of the model's
// token limit and, if so, truncates old messages while preserving the system prompt.
// Returns the compacted slice and true if compaction occurred.
func compactIfNeeded(messages []llm.Message, model string, used int) ([]llm.Message, bool) {
	limit := resolveLimit(model)

	if used < int(float64(limit)*0.80) {
		return messages, false
	}

	// Find system prompt
	var sysMsg *llm.Message
	var rest []llm.Message
	for i := range messages {
		if messages[i].Role == llm.RoleSystem && sysMsg == nil {
			sysMsg = &messages[i]
			continue
		}
		rest = append(rest, messages[i])
	}

	// Keep last 40% (rounded up) of non-system messages
	keep := int(math.Ceil(float64(len(rest)) * 0.40))
	if keep >= len(rest) {
		return messages, false
	}

	rest = rest[len(rest)-keep:]

	result := make([]llm.Message, 0, 1+len(rest))
	if sysMsg != nil {
		result = append(result, *sysMsg)
	}
	result = append(result, rest...)
	return result, true
}

// resolveLimit returns the token limit for a given model string.
func resolveLimit(model string) int {
	if limit, ok := contextLimits[model]; ok {
		return limit
	}
	modelLower := strings.ToLower(model)
	bestMatch := 0
	bestLimit := 32_000 // default
	for prefix, limit := range contextLimits {
		if strings.HasPrefix(modelLower, strings.ToLower(prefix)) && len(prefix) > bestMatch {
			bestMatch = len(prefix)
			bestLimit = limit
		}
	}
	return bestLimit
}
