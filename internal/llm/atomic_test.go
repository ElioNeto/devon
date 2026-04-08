package llm

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"
)

// --- Test helpers ---

// mockProvider streams configured deltas.
type mockProvider struct {
	name    string
	streams []mockStreamCfg
	idx     int
	mu      sync.Mutex
}

type mockStreamCfg struct {
	text    string
	err     error
	toolCall *ToolCall
	usage   *Usage
}

func (m *mockProvider) addStream(cfg mockStreamCfg) { m.streams = append(m.streams, cfg) }

func (m *mockProvider) Name() string    { return m.name }
func (m *mockProvider) Info() ModelInfo { return ModelInfo{Name: m.name} }

func (m *mockProvider) Stream(ctx context.Context, messages []Message, tools []ToolDef) (<-chan Delta, error) {
	m.mu.Lock()
	if m.idx >= len(m.streams) {
		m.mu.Unlock()
		return nil, errors.New("no more streams")
	}
	cfg := m.streams[m.idx]
	m.idx++
	m.mu.Unlock()

	if cfg.err != nil {
		return nil, cfg.err
	}

	ch := make(chan Delta, 16)
	go func() {
		defer close(ch)
		if cfg.text != "" {
			sendDelta(ctx, ch, Text(cfg.text))
		}
		if cfg.toolCall != nil {
			sendDelta(ctx, ch, ToolCallDelta(*cfg.toolCall))
		}
		if cfg.usage != nil {
			sendDelta(ctx, ch, DoneDelta(cfg.usage))
		} else {
			sendDelta(ctx, ch, DoneDelta())
		}
	}()
	return ch, nil
}

func sendDelta(ctx context.Context, ch chan<- Delta, d Delta) {
	select {
	case ch <- d:
	case <-ctx.Done():
	}
}

func httpStatusErr(statusCode int, msg string) error {
	return &httpStatusError{StatusCode: statusCode, Message: msg}
}

// -- Tests --

func TestAtomicClient_Send_CollectsText(t *testing.T) {
	mp := &mockProvider{name: "test"}
	mp.addStream(mockStreamCfg{
		text:  "Hello, world!",
		usage: &Usage{PromptTokens: 5, CompletionTokens: 10, TotalTokens: 15},
	})

	client := &AtomicClient{
		Provider:   mp,
		MaxRetries: 1,
	}

	resp, err := client.Send(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("Send() error: %v", err)
	}
	if resp.Text != "Hello, world!" {
		t.Errorf("Text = %q", resp.Text)
	}
	if resp.Usage == nil || resp.Usage.TotalTokens != 15 {
		t.Errorf("Usage = %+v", resp.Usage)
	}
}

func TestAtomicClient_Send_CollectsToolCalls(t *testing.T) {
	mp := &mockProvider{name: "test"}
	mp.addStream(mockStreamCfg{
		text: "I'll call the tool",
		toolCall: &ToolCall{
			ID:   "1",
			Type: "function",
			Function: ToolCallFunction{
				Name:      "file_write",
				Arguments: `{"path":"x.go"}`,
			},
		},
	})

	client := &AtomicClient{Provider: mp, MaxRetries: 1}
	resp, err := client.Send(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("Send() error: %v", err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("got %d tool calls", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Function.Name != "file_write" {
		t.Errorf("tool name = %q", resp.ToolCalls[0].Function.Name)
	}
}

func TestAtomicClient_RetryOnTransient(t *testing.T) {
	mp := &mockProvider{name: "test"}
	// First call returns 429, second succeeds
	mp.streams = append(mp.streams,
		mockStreamCfg{err: httpStatusErr(http.StatusTooManyRequests, "rate limited")},
		mockStreamCfg{text: "ok"},
	)

	client := &AtomicClient{Provider: mp, MaxRetries: 3, BaseDelay: time.Millisecond}
	resp, err := client.Send(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("Send() error: %v", err)
	}
	if resp.Text != "ok" {
		t.Errorf("Text = %q", resp.Text)
	}
}

func TestAtomicClient_NoRetryOnAuthError(t *testing.T) {
	mp := &mockProvider{name: "test"}
	err401 := httpStatusErr(http.StatusUnauthorized, "unauthorized")
	mp.streams = append(mp.streams, mockStreamCfg{err: err401})

	client := &AtomicClient{Provider: mp, MaxRetries: 3, BaseDelay: time.Millisecond}
	_, err := client.Send(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	// Should fail immediately without retrying — callCount should be 1
	if mp.idx != 1 {
		t.Errorf("provider called %d times, want 1 (no retry)", mp.idx)
	}
}

func TestAtomicClient_ContextCanceledDuringRetry(t *testing.T) {
	mp := &mockProvider{name: "test"}
	mp.streams = append(mp.streams, mockStreamCfg{err: httpStatusErr(500, "oom")})

	ctx, cancel := context.WithCancel(context.Background())

	client := &AtomicClient{
		Provider:   mp,
		MaxRetries: 10,
		BaseDelay:  500 * time.Millisecond,
	}

	done := make(chan struct{})
	go func() {
		_, _ = client.Send(ctx, nil, nil)
		close(done)
	}()

	// Cancel context to interrupt the retry delay
	cancel()

	select {
	case <-done:
		// Good
	case <-time.After(time.Second):
		t.Fatal("Send did not return after context was canceled")
	}
}

func TestIsRetryable(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		want     bool
	}{
		{"429", httpStatusErr(429, "rl"), true},
		{"500", httpStatusErr(500, "err"), true},
		{"502", httpStatusErr(502, "bg"), true},
		{"503", httpStatusErr(503, "unavail"), true},
		{"504", httpStatusErr(504, "timeout"), true},
		{"400", httpStatusErr(400, "bad"), false},
		{"401", httpStatusErr(401, "unauth"), false},
		{"403", httpStatusErr(403, "forbidden"), false},
		{"404", httpStatusErr(404, "notfound"), false},
		{"nil", nil, false},
		{"context canceled", context.Canceled, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isRetryable(tc.err)
			if got != tc.want {
				t.Errorf("isRetryable(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
