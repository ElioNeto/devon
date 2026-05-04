// Package testutil provides reusable mocks for agent integration tests.
package testutil

import (
	"context"
	"sync/atomic"

	"github.com/ElioNeto/devon/internal/llm"
)

// MockClient implements llm.Streamer for agent loop tests.
// It emits pre-configured StreamEvent channels without any HTTP call.
type MockClient struct {
	// Responses is consumed in FIFO order — one per Stream() call.
	Responses []llm.MockResponse
	// CallCount tracks how many times Stream() was called (for assertions).
	CallCount atomic.Int32
	// InfoFn, if set, overrides the default model info returned by Info().
	InfoFn func() llm.ModelInfo
}

// Info returns model metadata for the mock client.
func (m *MockClient) Info() llm.ModelInfo {
	if m.InfoFn != nil {
		return m.InfoFn()
	}
	return llm.ModelInfo{
		Name:           "testutil-mock",
		Provider:       "mock",
		SupportsTools:  true,
		SupportsVision: true,
	}
}

// Stream returns a channel that emits the next MockResponse in sequence.
func (m *MockClient) Stream(ctx context.Context, messages []llm.Message, tools []llm.ToolDef) (<-chan llm.StreamEvent, error) {
	idx := int(m.CallCount.Add(1) - 1)
	if idx >= len(m.Responses) {
		return nil, llm.ErrNoMoreResponses
	}
	resp := m.Responses[idx]

	if resp.Err != nil {
		return nil, resp.Err
	}

	ch := make(chan llm.StreamEvent, 16)
	go func() {
		defer close(ch)

		if resp.Text != "" {
			sendOrDone(ctx, ch, llm.StreamEvent{Type: "text", Text: resp.Text})
		}
		for i := range resp.ToolCalls {
			tc := resp.ToolCalls[i] // capture
			sendOrDone(ctx, ch, llm.StreamEvent{Type: "tool_call", Tool: &tc})
		}
		if resp.Usage != nil {
			sendOrDone(ctx, ch, llm.StreamEvent{Type: "done", Usage: resp.Usage})
		} else {
			sendOrDone(ctx, ch, llm.StreamEvent{Type: "done"})
		}
	}()
	return ch, nil
}

func sendOrDone(ctx context.Context, ch chan<- llm.StreamEvent, ev llm.StreamEvent) {
	select {
	case ch <- ev:
	case <-ctx.Done():
	}
}
