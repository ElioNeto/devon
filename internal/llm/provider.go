package llm

import (
	"context"
	"time"
)

// Delta represents a chunk of streamed response from a provider.
// Exactly one field (other than Type) should be non-nil/set for a given event.
type Delta struct {
	Type string // "text" | "tool_call" | "done" | "error"

	text  string
	Tool  *ToolCall
	Err   error
	Usage *Usage
}

// Text creates a text delta.
func Text(content string) Delta {
	return Delta{Type: "text", text: content}
}

// ToolCallDelta creates a tool_call delta.
func ToolCallDelta(tc ToolCall) Delta {
	return Delta{Type: "tool_call", Tool: &tc}
}

// DoneDelta creates a done delta, optionally with usage.
func DoneDelta(usage ...*Usage) Delta {
	d := Delta{Type: "done"}
	if len(usage) > 0 && usage[0] != nil {
		d.Usage = usage[0]
	}
	return d
}

// ErrorDelta creates a Delta carrying an error.
func ErrorDelta(err error) Delta {
	return Delta{Type: "error", Err: err}
}

// Text returns the text content of a text delta.
func (d Delta) Text() string { return d.text }

// Provider is the interface that each LLM provider must implement.
type Provider interface {
	// Name returns the canonical provider name (e.g. "openai", "anthropic", "ollama").
	Name() string

	// ModelInfo returns the model info for this provider.
	Info() ModelInfo

	// Stream sends a chat request and returns a channel of Deltas.
	Stream(ctx context.Context, messages []Message, tools []ToolDef) (<-chan Delta, error)
}

// ModelInfo holds metadata about a model.
type ModelInfo struct {
	Name        string
	Provider    string
	InputCost   float64 // cost per 1M input tokens (USD)
	OutputCost  float64 // cost per 1M output tokens (USD)
	MaxTokens   int
	SupportsTools bool
	SupportsVision bool
}

// ProviderConfig is the unified config used to create a Provider.
type ProviderConfig struct {
	Name    string
	APIKey  string
	BaseURL string
	Model   string
	Timeout time.Duration
}
