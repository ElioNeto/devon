package headless

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ElioNeto/devon/internal/config"
	"github.com/ElioNeto/devon/internal/llm"
	"github.com/ElioNeto/devon/internal/tools"
)

// TestAgentRunCancelledContext verifies that agentRun returns immediately when
// the context is already cancelled.
func TestAgentRunCancelledContext(t *testing.T) {
	cfg := &config.Config{
		WorkDir:       "/tmp/headless-test",
		Model:         "test-model",
		BaseURL:       "http://localhost:11434/v1",
		MaxAgentLoops: 1,
		Timeout:       5 * time.Second,
	}
	registry := tools.NewRegistry()
	mockClient := &llm.MockClient{
		Responses: []llm.MockResponse{
			{Text: "test response"},
		},
	}

	// Create a context that is already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	eventsCh := make(chan agentEvent, 10)

	// agentRun should return quickly when context is cancelled
	done := make(chan struct{})
	go func() {
		agentRun(ctx, mockClient, cfg, registry, nil, "test prompt", eventsCh)
		close(done)
	}()

	select {
	case <-done:
		// agentRun returned — verify the channel is closed
		_, ok := <-eventsCh
		if ok {
			t.Error("expected eventsCh to be closed after cancelled context")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("agentRun timed out — it should have returned immediately")
	}
}

// TestSendSSENilPayload verifies that sendSSE handles nil payload without panicking.
func TestSendSSENilPayload(t *testing.T) {
	w := &flushableRecorder{httptest.NewRecorder()}

	// Should not panic with nil payload
	sendSSE(w, w, "test_event", nil)

	body := w.Body.String()
	if !strings.Contains(body, "test_event") {
		t.Error("expected test_event in SSE output")
	}
}

// TestSendSSEEmptyPayload verifies that sendSSE handles an empty struct payload
// without panicking.
func TestSendSSEEmptyPayload(t *testing.T) {
	w := &flushableRecorder{httptest.NewRecorder()}

	// Should not panic with TurnDonePayload (empty struct)
	sendSSE(w, w, EventTypeTurnDone, TurnDonePayload{})

	body := w.Body.String()
	if !strings.Contains(body, EventTypeTurnDone) {
		t.Error("expected turn_done in SSE output")
	}
}
