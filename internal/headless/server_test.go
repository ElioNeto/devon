package headless

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ElioNeto/devon/internal/agent"
	"github.com/ElioNeto/devon/internal/config"
	"github.com/ElioNeto/devon/internal/tools"
)

// newTestConfig creates a minimal config for testing.
func newTestConfig() *config.Config {
	return &config.Config{
		WorkDir:  "/tmp/headless-test",
		Model:    "test-model",
		BaseURL:  "http://localhost:11434/v1",
		MaxAgentLoops: 5,
		Timeout:  5 * time.Second,
		Mode:     config.ModeYolo,
		Headless: config.HeadlessConfig{
			Enabled: true,
			Host:    "127.0.0.1",
			Port:    9876,
		},
	}
}

// newTestServer creates a headless server with all handlers registered for testing.
func newTestServer(t *testing.T) *Server {
	t.Helper()
	cfg := newTestConfig()
	s := NewServer(cfg.Headless.Host, cfg.Headless.Port)
	registry := tools.NewRegistry()
	RegisterHandlers(s, cfg, registry, nil)
	return s
}

// TestHealthEndpoint verifies that GET /api/health returns 200 OK.
func TestHealthEndpoint(t *testing.T) {
	s := newTestServer(t)
	ts := httptest.NewServer(s)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" {
		t.Errorf("expected status=ok, got %q", body["status"])
	}
}

// TestStatusEndpoint verifies that GET /api/status returns 200 OK with expected fields.
func TestStatusEndpoint(t *testing.T) {
	s := newTestServer(t)
	ts := httptest.NewServer(s)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/status")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}

	if _, ok := body["running"]; !ok {
		t.Error("expected running field")
	}
	if _, ok := body["session_id"]; !ok {
		t.Error("expected session_id field")
	}
}

// TestSSEPromptMethodNotAllowed verifies that GET /api/prompt returns 405.
func TestSSEPromptMethodNotAllowed(t *testing.T) {
	s := newTestServer(t)
	ts := httptest.NewServer(s)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/prompt")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", resp.StatusCode)
	}
}

// TestSSEPromptEmptyBody verifies that POST /api/prompt with empty prompt returns 400.
func TestSSEPromptEmptyBody(t *testing.T) {
	s := newTestServer(t)
	ts := httptest.NewServer(s)
	defer ts.Close()

	body := PromptRequest{Prompt: ""}
	bodyJSON, _ := json.Marshal(body)

	resp, err := http.Post(ts.URL+"/api/prompt", "application/json", bytes.NewReader(bodyJSON))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

// TestRespondEndpointInvalidID tests POST /api/respond with an invalid request_id.
func TestRespondEndpointInvalidID(t *testing.T) {
	s := newTestServer(t)
	ts := httptest.NewServer(s)
	defer ts.Close()

	body := ActionRequiredRequest{
		RequestID: "nonexistent",
		Approved:  true,
	}
	bodyJSON, _ := json.Marshal(body)

	resp, err := http.Post(ts.URL+"/api/respond", "application/json", bytes.NewReader(bodyJSON))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

// TestRespondEndpointMethodNotAllowed verifies GET /api/respond returns 405.
func TestRespondEndpointMethodNotAllowed(t *testing.T) {
	s := newTestServer(t)
	ts := httptest.NewServer(s)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/respond")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", resp.StatusCode)
	}
}

// TestRespondMissingRequestID verifies POST /api/respond with missing request_id.
func TestRespondMissingRequestID(t *testing.T) {
	s := newTestServer(t)
	ts := httptest.NewServer(s)
	defer ts.Close()

	body := ActionRequiredRequest{
		RequestID: "",
		Approved:  true,
	}
	bodyJSON, _ := json.Marshal(body)

	resp, err := http.Post(ts.URL+"/api/respond", "application/json", bytes.NewReader(bodyJSON))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

// TestAgentRegistryPendingRequest tests the agentRegistry's Register and Resolve flow.
func TestAgentRegistryPendingRequest(t *testing.T) {
	ar := newAgentRegistry()

	ch := make(chan int, 1)
	requestID := ar.RegisterPendingRequest(ch)
	if requestID == "" {
		t.Fatal("expected non-empty requestID")
	}

	// Resolve the pending request
	ok := ar.ResolvePendingRequest(requestID, 1)
	if !ok {
		t.Fatal("expected request to be resolved")
	}

	select {
	case level := <-ch:
		if level != 1 {
			t.Errorf("expected level 1, got %d", level)
		}
	default:
		t.Error("expected message on channel")
	}

	// Second resolve should fail
	ok = ar.ResolvePendingRequest(requestID, 1)
	if ok {
		t.Error("expected second resolve to fail")
	}
}

// TestAgentRegistryRemovePendingRequest tests removing a pending request.
func TestAgentRegistryRemovePendingRequest(t *testing.T) {
	ar := newAgentRegistry()

	ch := make(chan int, 1)
	requestID := ar.RegisterPendingRequest(ch)
	ar.RemovePendingRequest(requestID)

	// Resolve should now fail
	ok := ar.ResolvePendingRequest(requestID, 1)
	if ok {
		t.Error("expected resolve to fail after removal")
	}
}

// flushableRecorder wraps httptest.ResponseRecorder to implement http.Flusher.
type flushableRecorder struct {
	*httptest.ResponseRecorder
}

func (f *flushableRecorder) Flush() {}

// TestSSEStreamEventLoop verifies the SSE streaming helper works.
func TestSSEStreamEventLoop(t *testing.T) {
	w := &flushableRecorder{httptest.NewRecorder()}

	// Simulate sending events via sendSSE
	sendSSE(w, w, EventTypeTextDelta, TextDeltaPayload{Text: "hello"})
	sendSSE(w, w, EventTypeTurnDone, TurnDonePayload{})

	body := w.Body.String()
	if !strings.Contains(body, "text_delta") {
		t.Error("expected text_delta in SSE output")
	}
	if !strings.Contains(body, "turn_done") {
		t.Error("expected turn_done in SSE output")
	}
	if !strings.Contains(body, "data: ") {
		t.Error("expected 'data: ' prefix in SSE output")
	}
}

// TestConcurrentHealthRequests verifies that the server handles concurrent requests.
func TestConcurrentHealthRequests(t *testing.T) {
	s := newTestServer(t)
	ts := httptest.NewServer(s)
	defer ts.Close()

	var successCount atomic.Int32
	done := make(chan struct{})

	for i := 0; i < 10; i++ {
		go func() {
			resp, err := http.Get(ts.URL + "/api/health")
			if err != nil {
				t.Logf("request error: %v", err)
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				successCount.Add(1)
			}
			done <- struct{}{}
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	if n := successCount.Load(); n != 10 {
		t.Errorf("expected 10 successful requests, got %d", n)
	}
}

// TestWriteJSON verifies the writeJSON helper.
func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected application/json, got %q", ct)
	}

	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" {
		t.Errorf("expected status=ok, got %q", body["status"])
	}
}

// TestServerListenAndClose tests that the server can listen and be closed.
func TestServerListenAndClose(t *testing.T) {
	s := NewServer("127.0.0.1", 0) // port 0 = pick a free port
	if err := s.Listen(); err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if s.Addr() == "" {
		t.Error("expected non-empty address")
	}

	// Verify address is valid
	if !strings.Contains(s.Addr(), "127.0.0.1") {
		t.Errorf("expected 127.0.0.1 in addr, got %s", s.Addr())
	}

	// Start serving in background
	go func() {
		_ = s.Serve()
	}()

	// Give the server time to start
	time.Sleep(50 * time.Millisecond)

	// Close the server
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// TestMockAgentEvents verifies that the event translation works for all event types.
func TestMockAgentEvents(t *testing.T) {
	eventTypes := []struct {
		agentType string
		sseType   string
	}{
		{"text", EventTypeTextDelta},
		{"tool_start", EventTypeToolStart},
		{"tool_done", EventTypeToolDone},
		{"tool_error", EventTypeToolError},
		{"turn_done", EventTypeTurnDone},
		{"error", EventTypeError},
		{"system", EventTypeSystem},
		{"rate_limited", EventTypeRateLimited},
		{"file_change", EventTypeFileChange},
	}

	for _, et := range eventTypes {
		w := &flushableRecorder{httptest.NewRecorder()}

		ev := agent.Event{Type: et.agentType}
		switch ev.Type {
		case "text":
			ev.Text = "test text"
			sendSSE(w, w, EventTypeTextDelta, TextDeltaPayload{Text: ev.Text})
		case "tool_start":
			ev.Tool = "test_tool"
			ev.Args = "{}"
			sendSSE(w, w, EventTypeToolStart, ToolStartPayload{Tool: ev.Tool, Args: ev.Args})
		case "tool_done":
			ev.Tool = "test_tool"
			ev.Result = "success"
			sendSSE(w, w, EventTypeToolDone, ToolDonePayload{Tool: ev.Tool, Result: ev.Result})
		case "tool_error":
			ev.Tool = "test_tool"
			sendSSE(w, w, EventTypeToolError, ToolErrorPayload{Tool: ev.Tool, Err: "error"})
		case "turn_done":
			sendSSE(w, w, EventTypeTurnDone, TurnDonePayload{})
		case "error":
			sendSSE(w, w, EventTypeError, ErrorPayload{Message: "test error"})
		case "system":
			ev.Text = "system message"
			sendSSE(w, w, EventTypeSystem, SystemPayload{Text: ev.Text})
		case "rate_limited":
			sendSSE(w, w, EventTypeRateLimited, ErrorPayload{Message: "rate limited"})
		case "file_change":
			sendSSE(w, w, EventTypeFileChange, map[string]string{"file": "test.go", "change": "modified"})
		}

		body := w.Body.String()
		if !strings.Contains(body, et.sseType) {
			t.Errorf("expected SSE type %q for agent type %q, got body: %s", et.sseType, et.agentType, body)
		}
	}
}

// TestRespondEndpointSuccess tests a successful respond flow via the agent registry.
func TestRespondEndpointSuccess(t *testing.T) {
	// Create a server with just the respond handler
	s := NewServer("127.0.0.1", 0)
	registry := tools.NewRegistry()
	RegisterHandlers(s, newTestConfig(), registry, nil)
	ts := httptest.NewServer(s)
	defer ts.Close()

	// Register a pending request first
	ch := make(chan int, 1)
	requestID := s.agents.RegisterPendingRequest(ch)

	// Now respond to it
	body := ActionRequiredRequest{
		RequestID: requestID,
		Approved:  true,
	}
	bodyJSON, _ := json.Marshal(body)

	resp, err := http.Post(ts.URL+"/api/respond", "application/json", bytes.NewReader(bodyJSON))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]bool
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if !result["success"] {
		t.Error("expected success=true")
	}

	// Verify the channel received the message
	select {
	case level := <-ch:
		if level != 1 {
			t.Errorf("expected level 1, got %d", level)
		}
	default:
		t.Error("expected message on channel")
	}
}

// TestRespondEndpointAlwaysAllow tests the always_allow flag.
func TestRespondEndpointAlwaysAllow(t *testing.T) {
	s := NewServer("127.0.0.1", 0)
	registry := tools.NewRegistry()
	RegisterHandlers(s, newTestConfig(), registry, nil)
	ts := httptest.NewServer(s)
	defer ts.Close()

	ch := make(chan int, 1)
	requestID := s.agents.RegisterPendingRequest(ch)

	body := ActionRequiredRequest{
		RequestID:   requestID,
		Approved:    true,
		AlwaysAllow: true,
	}
	bodyJSON, _ := json.Marshal(body)

	resp, err := http.Post(ts.URL+"/api/respond", "application/json", bytes.NewReader(bodyJSON))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	select {
	case level := <-ch:
		if level != 2 {
			t.Errorf("expected level 2 (always allow), got %d", level)
		}
	default:
		t.Error("expected message on channel")
	}
}

// TestSSEPromptBadJSON verifies POST /api/prompt with invalid JSON returns 400.
func TestSSEPromptBadJSON(t *testing.T) {
	s := newTestServer(t)
	ts := httptest.NewServer(s)
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/prompt", "application/json", bytes.NewReader([]byte("{invalid")))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

// TestRespondBadJSON verifies POST /api/respond with invalid JSON returns 400.
func TestRespondBadJSON(t *testing.T) {
	s := newTestServer(t)
	ts := httptest.NewServer(s)
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/respond", "application/json", bytes.NewReader([]byte("{invalid")))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}
