package agent

import (
	"fmt"
	"strings"

	"github.com/ElioNeto/devon/internal/llm"
)

// truncationStats holds counters for display by /usage.
type truncationStats struct {
	TurnsRemoved       int
	ToolCharsSaved     int
	ToolTruncatedCount int
	CacheHits          int
}

// applySlidingWindow limits the history to the most recent N conversation turns.
// A "turn" consists of a user message followed by an optional assistant message
// and associated tool messages. The system message(s) at index 0 are always preserved.
// Returns the filtered messages and the number of turns removed.
func applySlidingWindow(messages []llm.Message, maxTurns int) ([]llm.Message, int) {
	if maxTurns <= 0 || len(messages) <= 1 {
		return messages, 0
	}

	// Find all system messages at the start
	var systemMsgs []llm.Message
	var rest []llm.Message
	for _, msg := range messages {
		if msg.Role == llm.RoleSystem && len(rest) == 0 {
			systemMsgs = append(systemMsgs, msg)
		} else {
			rest = append(rest, msg)
		}
	}

	if len(rest) == 0 {
		return messages, 0
	}

	// Split remaining messages into turns. A turn starts with a RoleUser message.
	type turn struct {
		messages []llm.Message
	}
	var turns []turn
	var current turn
	for _, msg := range rest {
		if msg.Role == llm.RoleUser && len(current.messages) > 0 {
			turns = append(turns, current)
			current = turn{messages: []llm.Message{msg}}
		} else {
			current.messages = append(current.messages, msg)
		}
	}
	if len(current.messages) > 0 {
		turns = append(turns, current)
	}

	if len(turns) <= maxTurns {
		return messages, 0
	}

	removed := len(turns) - maxTurns

	// Build result: system messages + placeholder + last maxTurns turns
	result := make([]llm.Message, 0, len(systemMsgs)+1+maxTurns*3)
	result = append(result, systemMsgs...)

	// Add placeholder between system and first kept turn
	placeholder := fmt.Sprintf("[histórico anterior omitido — %d turno(s)]", removed)
	result = append(result, llm.Message{
		Role:    llm.RoleSystem,
		Content: llm.TextContent(placeholder),
	})

	// Add the last maxTurns turns
	start := len(turns) - maxTurns
	for _, t := range turns[start:] {
		result = append(result, t.messages...)
	}

	return result, removed
}

// truncateToolResult truncates a tool result string if it exceeds maxChars.
// Appends "[... N linhas omitidas]" when truncated.
func truncateToolResult(result string, maxChars int) string {
	if maxChars <= 0 || len(result) <= maxChars {
		return result
	}

	truncated := result[:maxChars]
	removed := result[maxChars:]

	newlinesInRemoved := strings.Count(removed, "\n")
	if newlinesInRemoved > 0 {
		return truncated + fmt.Sprintf("[... %d linhas omitidas]", newlinesInRemoved)
	}

	charsRemoved := len(removed)
	return truncated + fmt.Sprintf("[... %d caracteres omitidos]", charsRemoved)
}
