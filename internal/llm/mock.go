//go:build !production

// Package llm provides an OpenAI-compatible HTTP client.
// This file contains test mocks used for the agent loop tests.
package llm

import (
	"context"
)

// MockResponse defines what a MockClient should emit for a single Stream call.
type MockResponse struct {
	// Text is emitted as a single "text" event (can contain the full response).
	Text string
	// ToolCalls is emitted as "tool_call" events (one per entry).
	ToolCalls []ToolCall
	// Err, if set, causes the Stream call to return an error instead of a channel.
	Err error
	// Usage is emitted as part of the "done" event.
	Usage *Usage
}

// MockClient implements Streamer for tests — it emits pre-configured
// StreamEvent channels without any HTTP call.
type MockClient struct {
	// Responses is consumed in FIFO order — one per Stream() call.
	Responses []MockResponse
	// callCount tracks how many times Stream() was called (for assertions).
	callCount int
}

// Stream returns a channel that emits the next MockResponse in sequence.
func (m *MockClient) Stream(ctx context.Context, messages []Message, tools []ToolDef) (<-chan StreamEvent, error) {
	if m.callCount >= len(m.Responses) {
		return nil, ErrNoMoreResponses
	}
	resp := m.Responses[m.callCount]
	m.callCount++

	if resp.Err != nil {
		return nil, resp.Err
	}

	ch := make(chan StreamEvent, 16)
	go func() {
		defer close(ch)

		if resp.Text != "" {
			sendOrDone(ctx, ch, StreamEvent{Type: "text", Text: resp.Text})
		}
		for i := range resp.ToolCalls {
			tc := resp.ToolCalls[i] // capture
			sendOrDone(ctx, ch, StreamEvent{Type: "tool_call", Tool: &tc})
		}
		if resp.Usage != nil {
			sendOrDone(ctx, ch, StreamEvent{Type: "done", Usage: resp.Usage})
		} else {
			sendOrDone(ctx, ch, StreamEvent{Type: "done"})
		}
	}()
	return ch, nil
}

// Info returns model info for the mock client.
func (m *MockClient) Info() ModelInfo {
	return ModelInfo{
		Name:           "mock",
		Provider:       "mock",
		SupportsTools:  true,
		SupportsVision: true,
	}
}

// CallCount returns the number of Stream() calls made so far.
func (m *MockClient) CallCount() int {
	return m.callCount
}

func sendOrDone(ctx context.Context, ch chan<- StreamEvent, ev StreamEvent) {
	select {
	case ch <- ev:
	case <-ctx.Done():
	}
}

// ErrNoMoreResponses is returned when all configured responses are consumed.
var ErrNoMoreResponses = &mockError{"no more configured responses"}

type mockError struct{ msg string }

func (e *mockError) Error() string { return "llm: " + e.msg }
