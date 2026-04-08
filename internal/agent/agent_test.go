package agent

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ElioNeto/devon/internal/config"
	"github.com/ElioNeto/devon/internal/db"
	"github.com/ElioNeto/devon/internal/llm"
	"github.com/ElioNeto/devon/internal/tools"
)

func newTestConfig() *config.Config {
	return &config.Config{
		WorkDir:  "/tmp/test-workdir",
		Model:    "test-model",
		BaseURL:  "http://localhost:11434/v1",
		MaxTurns: 5,
		Timeout:  5 * time.Second,
		Mode:     config.ModeYolo,
	}
}

func collectEvents(ch <-chan Event) []Event {
	var events []Event
	for e := range ch {
		events = append(events, e)
	}
	return events
}

func hasEventType(events []Event, t string) bool {
	for _, e := range events {
		if e.Type == t {
			return true
		}
	}
	return false
}

// Test for agent ID
func TestAgent_NewWithAgentID(t *testing.T) {
	cfg := newTestConfig()
	r := tools.NewRegistry()

	fakeDB := &fakeDBStore{}
	a := New(cfg, &llm.MockClient{}, r, fakeDB, "test-agent-1")

	if a == nil {
		t.Fatal("New() returned nil")
	}

	if a.AgentID() != "test-agent-1" {
		t.Errorf("AgentID = %q, want %q", a.AgentID(), "test-agent-1")
	}
}

func TestAgent_New(t *testing.T) {
	cfg := newTestConfig()
	r := tools.NewRegistry()

	fakeDB := &fakeDBStore{}
	a := New(cfg, &llm.MockClient{}, r, fakeDB, "default-agent")

	if a == nil {
		t.Fatal("New() returned nil")
	}
	if a.registry == nil {
		t.Error("registry is nil")
	}
	if len(a.history) != 1 {
		t.Errorf("expected 1 system message, got %d", len(a.history))
	}
}

func TestAgent_BuildSystemMessages_WithContextDoc(t *testing.T) {
	cfg := newTestConfig()
	cfg.ContextDoc = "This is my project."
	r := tools.NewRegistry()

	fakeDB := &fakeDBStore{}
	a := New(cfg, &llm.MockClient{}, r, fakeDB, "test")

	msgs := a.history
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Content == nil {
		t.Error("system message is nil")
	}
	if msgs[0].Role != llm.RoleSystem {
		t.Errorf("role = %q, want %q", msgs[0].Role, llm.RoleSystem)
	}
}

func TestAgent_Run_SimpleResponse(t *testing.T) {
	cfg := newTestConfig()
	r := tools.NewRegistry()
	mc := &llm.MockClient{
		Responses: []llm.MockResponse{
			{Text: "I can help!"},
		},
	}

	fakeDB := &fakeDBStore{}
	a := New(cfg, mc, r, fakeDB, "agent-1")
	events := collectEvents(a.Run(context.Background(), "hello"))

	if !hasEventType(events, "text") {
		t.Error("expected text event")
	}
	if !hasEventType(events, "turn_done") {
		t.Error("expected turn_done event")
	}
}

func TestAgent_Run_SingleToolCall(t *testing.T) {
	dir := t.TempDir()

	cfg := newTestConfig()
	cfg.WorkDir = dir
	cfg.MaxTurns = 5
	cfg.Mode = config.ModeYolo

	r := tools.NewRegistry()
	mc := &llm.MockClient{
		Responses: []llm.MockResponse{
			{
				ToolCalls: []llm.ToolCall{
					{
						ID:   "t1",
						Type: "function",
						Function: llm.ToolCallFunction{
							Name:      "write",
							Arguments: `{"path":"` + dir + `/hello.txt","content":"world"}`,
						},
					},
				},
			},
			{Text: "File created!"},
		},
	}

	fakeDB := &fakeDBStore{}
	a := New(cfg, mc, r, fakeDB, "agent-1")
	events := collectEvents(a.Run(context.Background(), "create a file"))

	if !hasEventType(events, "tool_start") {
		t.Errorf("expected tool_start, got: %v", eventTypeSlice(events))
	}
	if !hasEventType(events, "tool_done") {
		t.Errorf("expected tool_done, got: %v", eventTypeSlice(events))
	}
	if !hasEventType(events, "text") {
		t.Errorf("expected text response after tool, got: %v", eventTypeSlice(events))
	}
	if !hasEventType(events, "turn_done") {
		t.Errorf("expected turn_done, got: %v", eventTypeSlice(events))
	}
}

func TestAgent_Run_MultipleToolCalls(t *testing.T) {
	dir := t.TempDir()

	cfg := newTestConfig()
	cfg.WorkDir = dir
	cfg.MaxTurns = 5
	cfg.Mode = config.ModeYolo

	r := tools.NewRegistry()
	mc := &llm.MockClient{
		Responses: []llm.MockResponse{
			{
				ToolCalls: []llm.ToolCall{
					{
						ID:   "t1",
						Type: "function",
						Function: llm.ToolCallFunction{
							Name:      "write",
							Arguments: `{"path":"` + dir + `/a.txt","content":"hello"}`,
						},
					},
				},
			},
			{
				ToolCalls: []llm.ToolCall{
					{
						ID:   "t2",
						Type: "function",
						Function: llm.ToolCallFunction{
							Name:      "read",
							Arguments: `{"file":"` + dir + `/a.txt"}`,
						},
					},
				},
			},
			{Text: "Done reading the file."},
		},
	}

	fakeDB := &fakeDBStore{}
	a := New(cfg, mc, r, fakeDB, "agent-1")
	events := collectEvents(a.Run(context.Background(), "write and then read file"))

	toolStarts := 0
	toolDones := 0
	texts := 0
	for _, e := range events {
		switch e.Type {
		case "tool_start":
			toolStarts++
		case "tool_done":
			toolDones++
		case "text":
			texts++
		}
	}
	if toolStarts < 2 {
		t.Errorf("expected at least 2 tool_start events, got %d", toolStarts)
	}
	if toolDones < 2 {
		t.Errorf("expected at least 2 tool_done events, got %d", toolDones)
	}
	if texts < 1 {
		t.Errorf("expected at least 1 text event, got %d", texts)
	}
	if !hasEventType(events, "turn_done") {
		t.Error("expected turn_done")
	}
}

func TestAgent_Run_ToolCall_Error(t *testing.T) {
	dir := t.TempDir()

	cfg := newTestConfig()
	cfg.WorkDir = dir
	cfg.Mode = config.ModeYolo

	r := tools.NewRegistry()
	mc := &llm.MockClient{
		Responses: []llm.MockResponse{
			{
				ToolCalls: []llm.ToolCall{
					{
						ID:   "t1",
						Type: "function",
						Function: llm.ToolCallFunction{
							Name:      "read",
							Arguments: `{"file":"/nonexistent/file.xyz"}`,
						},
					},
				},
			},
			{Text: "That file does not exist."},
		},
	}

	fakeDB := &fakeDBStore{}
	a := New(cfg, mc, r, fakeDB, "agent-1")
	events := collectEvents(a.Run(context.Background(), "read a nonexistent file"))

	if !hasEventType(events, "tool_error") {
		t.Errorf("expected tool_error event, got: %v", eventTypeSlice(events))
	}
}

func TestAgent_Run_UnknownTool(t *testing.T) {
	r := tools.NewRegistry()
	mc := &llm.MockClient{
		Responses: []llm.MockResponse{
			{
				ToolCalls: []llm.ToolCall{
					{
						ID:   "t1",
						Type: "function",
						Function: llm.ToolCallFunction{
							Name:      "nonexistent_tool",
							Arguments: "{}",
						},
					},
				},
			},
		},
	}

	cfg := newTestConfig()
	cfg.MaxTurns = 1
	mc.Responses = append(mc.Responses, llm.MockResponse{Text: "I don't know that tool."})

	fakeDB := &fakeDBStore{}
	a := New(cfg, mc, r, fakeDB, "agent-1")
	events := collectEvents(a.Run(context.Background(), "do something"))

	if !hasEventType(events, "tool_error") {
		t.Errorf("expected tool_error for unknown tool, got: %v", eventTypeSlice(events))
	}
}

func TestAgent_Run_ContextCancelled(t *testing.T) {
	cfg := newTestConfig()
	r := tools.NewRegistry()

	ctx, cancel := context.WithCancel(context.Background())
	mc := &llm.MockClient{
		Responses: []llm.MockResponse{
			{Text: "thinking..."},
		},
	}

	fakeDB := &fakeDBStore{}
	a := New(cfg, mc, r, fakeDB, "agent-1")

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	events := collectEvents(a.Run(ctx, "hello"))
	if len(events) == 0 {
		t.Error("expected at least one event before cancellation")
	}
}

func TestAgent_Run_ContextCancelled_BeforeCall(t *testing.T) {
	cfg := newTestConfig()
	r := tools.NewRegistry()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	mc := &llm.MockClient{
		Responses: []llm.MockResponse{
			{Text: "should not be reached"},
		},
	}

	fakeDB := &fakeDBStore{}
	a := New(cfg, mc, r, fakeDB, "agent-1")

	events := collectEvents(a.Run(ctx, "test"))
	if len(events) > 0 {
		t.Errorf("expected no events with pre-cancelled context, got: %v", eventTypeSlice(events))
	}
}

func TestAgent_Run_MaxTurnsLimit(t *testing.T) {
	dir := t.TempDir()

	cfg := newTestConfig()
	cfg.WorkDir = dir
	cfg.MaxTurns = 2
	cfg.Mode = config.ModeYolo

	r := tools.NewRegistry()
	toolCallResp := func(id string) llm.MockResponse {
		return llm.MockResponse{
			ToolCalls: []llm.ToolCall{
				{
					ID:   id,
					Type: "function",
					Function: llm.ToolCallFunction{
						Name:      "write",
						Arguments: `{"path":"` + dir + `/` + id + `.txt","content":"data"}`,
					},
				},
			},
		}
	}

	mc := &llm.MockClient{
		Responses: []llm.MockResponse{
			toolCallResp("t1"),
			toolCallResp("t2"),
			toolCallResp("t3"),
		},
	}

	fakeDB := &fakeDBStore{}
	a := New(cfg, mc, r, fakeDB, "agent-1")
	events := collectEvents(a.Run(context.Background(), "keep doing things"))

	if !hasEventType(events, "error") {
		t.Errorf("expected error event when MaxTurns reached, got: %v", eventTypeSlice(events))
	} else {
		for _, e := range events {
			if e.Type == "error" && e.Err != nil && e.Err.Error() == "" {
				t.Error("error event has empty message")
			}
		}
	}
}

func TestAgent_Run_LLMStreamError(t *testing.T) {
	cfg := newTestConfig()
	r := tools.NewRegistry()
	mc := &llm.MockClient{
		Responses: []llm.MockResponse{
			{Err: &mockError{msg: "connection timeout"}},
		},
	}

	fakeDB := &fakeDBStore{}
	a := New(cfg, mc, r, fakeDB, "agent-1")
	events := collectEvents(a.Run(context.Background(), "test"))

	if !hasEventType(events, "error") {
		t.Errorf("expected error event on LLM failure, got: %v", eventTypeSlice(events))
	}
}

func TestAgent_Run_UserMessageInHistory(t *testing.T) {
	cfg := newTestConfig()
	r := tools.NewRegistry()
	mc := &llm.MockClient{
		Responses: []llm.MockResponse{
			{Text: "Got it"},
		},
	}

	fakeDB := &fakeDBStore{}
	a := New(cfg, mc, r, fakeDB, "agent-1")
	events := collectEvents(a.Run(context.Background(), "my question"))

	_ = events

	if len(a.history) < 2 {
		t.Errorf("expected at least 2 history entries (system+user), got %d", len(a.history))
	}
	if a.history[len(a.history)-2].Role != llm.RoleUser {
		t.Errorf("expected user role, got %q", a.history[len(a.history)-2].Role)
	}
	if a.history[len(a.history)-2].Content == nil || *a.history[len(a.history)-2].Content != "my question" {
		t.Errorf("history user content = %q, want %q", *a.history[len(a.history)-2].Content, "my question")
	}
}

func TestAgent_ExecuteTool_UnknownTool(t *testing.T) {
	cfg := newTestConfig()
	r := tools.NewRegistry()
	mc := &llm.MockClient{}

	fakeDB := &fakeDBStore{}
	a := New(cfg, mc, r, fakeDB, "agent-1")

	tc := llm.ToolCall{
		ID:   "t1",
		Type: "function",
		Function: llm.ToolCallFunction{
			Name:      "nonexistent_tool",
			Arguments: "{}",
		},
	}

	ch := make(chan Event, 1)
	_, err := a.executeToolWithPermission(context.Background(), tc, ch)
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
}

func TestAgent_Run_NoToolCalls(t *testing.T) {
	r := tools.NewRegistry()
	mc := &llm.MockClient{
		Responses: []llm.MockResponse{
			{Text: "Just a plain answer"},
		},
	}

	cfg := newTestConfig()
	fakeDB := &fakeDBStore{}
	a := New(cfg, mc, r, fakeDB, "agent-1")
	events := collectEvents(a.Run(context.Background(), "test"))

	if !hasEventType(events, "text") {
		t.Error("expected text event")
	}
	if !hasEventType(events, "turn_done") {
		t.Error("expected turn_done event")
	}
}

func TestAgent_Checker_Blocklist(t *testing.T) {
	cfg := newTestConfig()
	cfg.Mode = config.ModeYolo
	r := tools.NewRegistry()
	mc := &llm.MockClient{}

	fakeDB := &fakeDBStore{}
	a := New(cfg, mc, r, fakeDB, "agent-1")

	if a.checker == nil {
		t.Fatal("checker is nil")
	}
	if len(a.checker.Blocklist) == 0 {
		t.Error("blocklist should not be empty")
	}
}

func TestAgent_Run_ParallelToolCalls(t *testing.T) {
	dir := t.TempDir()

	cfg := newTestConfig()
	cfg.WorkDir = dir
	cfg.Mode = config.ModeYolo

	r := tools.NewRegistry()
	mc := &llm.MockClient{
		Responses: []llm.MockResponse{
			{
				ToolCalls: []llm.ToolCall{
					{
						ID:   "t1",
						Type: "function",
						Function: llm.ToolCallFunction{
							Name:      "write",
							Arguments: `{"path":"` + dir + `/file1.txt","content":"one"}`,
						},
					},
					{
						ID:   "t2",
						Type: "function",
						Function: llm.ToolCallFunction{
							Name:      "write",
							Arguments: `{"path":"` + dir + `/file2.txt","content":"two"}`,
						},
					},
				},
			},
			{Text: "Both files created."},
		},
	}

	fakeDB := &fakeDBStore{}
	a := New(cfg, mc, r, fakeDB, "agent-1")
	events := collectEvents(a.Run(context.Background(), "create two files"))

	toolStarts := 0
	toolDoneOrErr := 0
	for _, e := range events {
		switch e.Type {
		case "tool_start":
			toolStarts++
		case "tool_done", "tool_error":
			toolDoneOrErr++
		}
	}
	if toolStarts != 2 {
		t.Errorf("expected 2 tool_start events, got %d", toolStarts)
	}
	if toolDoneOrErr != 2 {
		t.Errorf("expected 2 tool_done/error events, got %d", toolDoneOrErr)
	}
}

func TestAgent_Run_EventOrder(t *testing.T) {
	dir := t.TempDir()

	cfg := newTestConfig()
	cfg.WorkDir = dir
	cfg.Mode = config.ModeYolo

	r := tools.NewRegistry()
	mc := &llm.MockClient{
		Responses: []llm.MockResponse{
			{
				ToolCalls: []llm.ToolCall{
					{
						ID:   "t1",
						Type: "function",
						Function: llm.ToolCallFunction{
							Name:      "write",
							Arguments: `{"path":"` + dir + `/order.txt","content":"test"}`,
						},
					},
				},
			},
			{Text: "done"},
		},
	}

	fakeDB := &fakeDBStore{}
	a := New(cfg, mc, r, fakeDB, "agent-1")
	events := collectEvents(a.Run(context.Background(), "test order"))

	toolStartIdx := -1
	toolDoneIdx := -1
	for i, e := range events {
		if e.Type == "tool_start" && toolStartIdx == -1 {
			toolStartIdx = i
		}
		if e.Type == "tool_done" && toolDoneIdx == -1 {
			toolDoneIdx = i
		}
	}
	if toolStartIdx == -1 {
		t.Fatal("no tool_start event found")
	}
	if toolDoneIdx == -1 {
		t.Fatal("no tool_done event found")
	}
	if toolDoneIdx <= toolStartIdx {
		t.Errorf("tool_done (idx %d) should come after tool_start (idx %d)", toolDoneIdx, toolStartIdx)
	}
}

func TestAgent_TurnDelay_Respected(t *testing.T) {
	dir := t.TempDir()

	cfg := newTestConfig()
	cfg.WorkDir = dir
	cfg.MaxTurns = 5
	cfg.Mode = config.ModeYolo
	cfg.TurnDelay = 200 * time.Millisecond

	r := tools.NewRegistry()
	mc := &llm.MockClient{
		Responses: []llm.MockResponse{
			{
				ToolCalls: []llm.ToolCall{
					{
						ID:   "t1",
						Type: "function",
						Function: llm.ToolCallFunction{
							Name:      "write",
							Arguments: `{"path":"` + dir + `/delay.txt","content":"data"}`,
						},
					},
				},
			},
			{
				ToolCalls: []llm.ToolCall{
					{
						ID:   "t2",
						Type: "function",
						Function: llm.ToolCallFunction{
							Name:      "write",
							Arguments: `{"path":"` + dir + `/delay2.txt","content":"data2"}`,
						},
					},
				},
			},
			{Text: "All done!"},
		},
	}

	fakeDB := &fakeDBStore{}
	a := New(cfg, mc, r, fakeDB, "agent-1")

	start := time.Now()
	events := collectEvents(a.Run(context.Background(), "test turn delay"))
	elapsed := time.Since(start)

	if elapsed < 350*time.Millisecond {
		t.Errorf("expected at least 350ms elapsed with turn delay, got %v", elapsed)
	}
	if !hasEventType(events, "turn_done") {
		t.Error("expected turn_done event")
	}
}

func TestAgent_TurnDelay_CancelOnContext(t *testing.T) {
	dir := t.TempDir()

	cfg := newTestConfig()
	cfg.WorkDir = dir
	cfg.MaxTurns = 5
	cfg.Mode = config.ModeYolo
	cfg.TurnDelay = 5 * time.Second

	r := tools.NewRegistry()
	mc := &llm.MockClient{
		Responses: []llm.MockResponse{
			{
				ToolCalls: []llm.ToolCall{
					{
						ID:   "t1",
						Type: "function",
						Function: llm.ToolCallFunction{
							Name:      "write",
							Arguments: `{"path":"` + dir + `/cancel.txt","content":"data"}`,
						},
					},
				},
			},
			{Text: "should not reach"},
		},
	}

	fakeDB := &fakeDBStore{}
	a := New(cfg, mc, r, fakeDB, "agent-1")

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(150 * time.Millisecond)
		cancel()
	}()

	events := collectEvents(a.Run(ctx, "test cancel during turn delay"))

	if len(events) == 0 {
		t.Error("expected at least some events before cancellation")
	}
}

func TestAgent_Run_RateLimited_Event(t *testing.T) {
	cfg := newTestConfig()
	r := tools.NewRegistry()
	mc := &llm.MockClient{
		Responses: []llm.MockResponse{
			{Err: fmt.Errorf("llm: provedor retornou HTTP 429: rate limited")},
		},
	}

	fakeDB := &fakeDBStore{}
	a := New(cfg, mc, r, fakeDB, "agent-1")
	events := collectEvents(a.Run(context.Background(), "test"))

	if !hasEventType(events, "rate_limited") {
		t.Errorf("expected rate_limited event, got: %v", eventTypeSlice(events))
	}
	if !hasEventType(events, "error") {
		t.Errorf("expected error event after rate_limited, got: %v", eventTypeSlice(events))
	}
}

func eventTypeSlice(events []Event) []string {
	types := make([]string, len(events))
	for i, e := range events {
		types[i] = e.Type
	}
	return types
}

type mockError struct{ msg string }

func (e *mockError) Error() string { return e.msg }

// fakeDBStore implements db.Store for tests
type fakeDBStore struct{}

func (f *fakeDBStore) CreateSession(ctx context.Context, id string) error                                         { return nil }
func (f *fakeDBStore) GetSession(ctx context.Context, id string) (bool, error)                                 { return false, nil }
func (f *fakeDBStore) ListSessions(ctx context.Context, limit int) ([]string, error)                          { return nil, nil }
func (f *fakeDBStore) PutMessage(ctx context.Context, agentID, sessionID, role, content string) error          { return nil }
func (f *fakeDBStore) GetMessages(ctx context.Context, agentID, sessionID string, limit int) ([]db.Message, error) {
	return nil, nil
}
func (f *fakeDBStore) SlidingWindow(ctx context.Context, agentID, sessionID string, windowSize int) error     { return nil }
func (f *fakeDBStore) PutAgentState(ctx context.Context, agentID, sessionID, snapshot string) error           { return nil }
func (f *fakeDBStore) GetAgentState(ctx context.Context, agentID string) (*db.AgentState, error)             { return nil, nil }
func (f *fakeDBStore) PutToolCall(ctx context.Context, agentID, sessionID, toolName, arguments, status, result, err string) (int64, error) {
	return 0, nil
}
func (f *fakeDBStore) GetToolCalls(ctx context.Context, sessionID string) ([]db.ToolCall, error)             { return nil, nil }
func (f *fakeDBStore) ArchiveMessages(ctx context.Context, agentID, sessionID string) error                   { return nil }
func (f *fakeDBStore) GetSessionHistory(ctx context.Context, sessionID string, limit int) ([]db.Message, error) {
	return nil, nil
}
func (f *fakeDBStore) PutArtifact(ctx context.Context, key, sessionID string, data []byte) error              { return nil }
func (f *fakeDBStore) GetArtifact(ctx context.Context, key string) ([]byte, error)                            { return nil, nil }
func (f *fakeDBStore) GetCostSummary(ctx context.Context, sessionID string) (*db.CostSummary, error)          { return nil, nil }
func (f *fakeDBStore) UpdateCostSummary(ctx context.Context, sessionID string, cost float64, tokens map[string]int) error {
	return nil
}
func (f *fakeDBStore) Subscribe(ctx context.Context, topic string) (<-chan db.Event, error)                   { return nil, nil }
func (f *fakeDBStore) Publish(ctx context.Context, topic string, payload interface{}) error                     { return nil }
func (f *fakeDBStore) Close() error                                                                           { return nil }
