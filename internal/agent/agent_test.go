package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/ElioNeto/devon/internal/config"
	"github.com/ElioNeto/devon/internal/llm"
	"github.com/ElioNeto/devon/internal/tools"
)

func sseHandler(text string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"" + text + "\"}}]}\n"))
		w.Write([]byte("data: {\"choices\":[],\"usage\":{\"prompt_tokens\":5,\"completion_tokens\":5,\"total_tokens\":10}}\n\n"))
	}
}

func sseHandlerEmpty() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("data: [DONE]\n"))
	}
}

func newTestConfig() *config.Config {
	// Ensure WorkDir exists for bash tool
	workDir := "/tmp/test-workdir-" + os.Getenv("USER")
	os.MkdirAll(workDir, 0755)
	return &config.Config{
		WorkDir:  workDir,
		Model:    "test-model",
		BaseURL:  "http://localhost:11434/v1",
		MaxTurns: 5,
		Timeout:  5 * time.Second,
		Mode:     config.ModeAuto,
	}
}

func TestAgent_New(t *testing.T) {
	cfg := newTestConfig()
	r := tools.NewRegistry()
	c := llm.New("", "http://localhost:11434/v1", "test", 5*time.Second)
	agent := New(cfg, c, r)
	if agent == nil {
		t.Fatal("New() returned nil")
	}
	if agent.registry == nil {
		t.Error("registry is nil")
	}
	if len(agent.history) != 1 {
		t.Errorf("expected 1 system message, got %d", len(agent.history))
	}
}

func TestAgent_BuildSystemMessages_WithContextDoc(t *testing.T) {
	cfg := newTestConfig()
	cfg.ContextDoc = "This is my project."
	r := tools.NewRegistry()
	c := llm.New("", cfg.BaseURL, "test", 5*time.Second)
	agent := New(cfg, c, r)
	msgs := agent.history
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Content == "" {
		t.Error("system message is empty")
	}
	if msgs[0].Role != llm.RoleSystem {
		t.Errorf("role = %q, want %q", msgs[0].Role, llm.RoleSystem)
	}
}

func TestAgent_Run_SimpleResponse(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	srv := httptest.NewServer(sseHandler("I can help!"))
	defer srv.Close()

	cfg := newTestConfig()
	r := tools.NewRegistry()
	c := llm.New("", srv.URL, "test", 5*time.Second)
	agent := New(cfg, c, r)

	var mu sync.Mutex
	var events []Event
	ch := agent.Run(context.Background(), "hello")
	for e := range ch {
		mu.Lock()
		events = append(events, e)
		mu.Unlock()
	}

	mu.Lock()
	defer mu.Unlock()

	foundText := false
	foundDone := false
	for _, e := range events {
		if e.Type == "text" {
			foundText = true
		}
		if e.Type == "turn_done" {
			foundDone = true
		}
	}
	if !foundText {
		t.Error("expected text event")
	}
	if !foundDone {
		t.Error("expected turn_done event")
	}
}

func TestAgent_Run_Error(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	cfg := newTestConfig()
	r := tools.NewRegistry()
	c := llm.New("", "http://127.0.0.1:19999", "test", 500*time.Millisecond)
	agent := New(cfg, c, r)

	ch := agent.Run(context.Background(), "test")
	for e := range ch {
		if e.Type == "error" {
			return
		}
	}
	t.Fatal("expected error event")
}

func TestAgent_Run_ContextCancelled(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	ctx, cancel := context.WithCancel(context.Background())
	srv := httptest.NewServer(sseHandler("reply"))
	defer srv.Close()
	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cancel()
		time.Sleep(100 * time.Millisecond)
		w.Write([]byte("data: [DONE]\n"))
	})

	cfg := newTestConfig()
	r := tools.NewRegistry()
	c := llm.New("", srv.URL, "test", 5*time.Second)
	agent := New(cfg, c, r)

	ch := agent.Run(ctx, "test")
	for range ch {
		// drain
	}
}

func TestAgent_ExecuteTool_UnknownTool(t *testing.T) {
	cfg := newTestConfig()
	r := tools.NewRegistry()
	c := llm.New("", "http://localhost:11434/v1", "test", 5*time.Second)
	agent := New(cfg, c, r)

	tc := llm.ToolCall{
		ID:   "t1",
		Type: "function",
		Function: llm.ToolCallFunction{
			Name:      "nonexistent_tool",
			Arguments: "{}",
		},
	}

	_, err := agent.executeTool(context.Background(), tc)
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
}

func TestAgent_Run_NoToolCalls(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	srv := httptest.NewServer(sseHandlerEmpty())
	defer srv.Close()

	cfg := newTestConfig()
	r := tools.NewRegistry()
	c := llm.New("", srv.URL, "test", 5*time.Second)
	agent := New(cfg, c, r)

	foundDone := false
	ch := agent.Run(context.Background(), "test")
	for e := range ch {
		if e.Type == "turn_done" {
			foundDone = true
		}
	}

	if !foundDone {
		t.Error("expected turn_done when no tool calls")
	}
}

func TestAgent_Run_ContextCancelledBeforeCall(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before calling Stream

	srv := httptest.NewServer(sseHandler("reply"))
	defer srv.Close()

	cfg := newTestConfig()
	r := tools.NewRegistry()
	c := llm.New("", srv.URL, "test", 5*time.Second)
	agent := New(cfg, c, r)

	ch := agent.Run(ctx, "test")
	for range ch {
		// drain
	}
}

func TestAgent_Run_ToolCalls(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	dir := t.TempDir()

	// First call returns tool_calls, then tool results are processed
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.Write([]byte("data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"t1\",\"function\":{\"name\":\"bash\",\"arguments\":\"{\\\"command\\\":\\\"echo hi\\\"}\"}}]}}]}\n\n"))
			w.Write([]byte("data: [DONE]\n\n"))
		} else {
			w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"done!\"}}]}\n\n"))
			w.Write([]byte("data: [DONE]\n\n"))
		}
	}))
	defer srv.Close()

	cfg := newTestConfig()
	cfg.WorkDir = dir
	cfg.MaxTurns = 5
	r := tools.NewRegistry()
	c := llm.New("", srv.URL, "test", 5*time.Second)
	agent := New(cfg, c, r)

	var events []string
	ch := agent.Run(context.Background(), "test")
	for e := range ch {
		events = append(events, e.Type)
		switch e.Type {
		case "tool_start":
			events = append(events, "tool="+e.Tool)
		case "tool_done":
			events = append(events, "tool="+e.Tool)
		case "tool_error":
			events = append(events, "tool="+e.Tool+" err="+e.Err.Error())
		case "turn_done":
		case "error":
			events = append(events, "err="+e.Err.Error())
		}
	}

	has := func(s string) bool {
		for _, e := range events {
			if e == s {
				return true
			}
		}
		return false
	}

	if !has("tool_start") {
		t.Errorf("expected tool_start event, got: %v", events)
	}
	if !has("tool_done") {
		t.Errorf("expected tool_done event, got: %v", events)
	}
	if !has("turn_done") {
		t.Errorf("expected turn_done, got: %v", events)
	}
}

func TestAgent_Run_LLM_ErrorEvent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Send an error event via parseSSE
		w.Write([]byte("data: {\"bad_json\":}\n"))
		w.Write([]byte("data: [DONE]\n"))
	}))
	defer srv.Close()

	cfg := newTestConfig()
	r := tools.NewRegistry()
	c := llm.New("", srv.URL, "test", 5*time.Second)
	agent := New(cfg, c, r)

	ch := agent.Run(context.Background(), "test")
	for range ch {
		// Drain - no panic
	}
}

func TestAgent_Run_ToolError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	dir := t.TempDir()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"t1\",\"function\":{\"name\":\"bash\",\"arguments\":\"{\\\"command\\\":\\\"false\\\"}\"}}]}}]}\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()

	cfg := newTestConfig()
	cfg.WorkDir = dir
	cfg.MaxTurns = 5
	r := tools.NewRegistry()
	c := llm.New("", srv.URL, "test", 5*time.Second)
	agent := New(cfg, c, r)

	var toolError bool
	ch := agent.Run(context.Background(), "test")
	for e := range ch {
		if e.Type == "tool_error" {
			toolError = true
		}
	}

	if !toolError {
		t.Error("expected tool_error event for failed command")
	}
}
