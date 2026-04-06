package agent

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ElioNeto/devon/internal/config"
	"github.com/ElioNeto/devon/internal/llm"
	"github.com/ElioNeto/devon/internal/tools"
)

func newTestConfig() *config.Config {
	workDir := "/tmp/test-workdir-" + os.Getenv("USER")
	os.MkdirAll(workDir, 0755)
	return &config.Config{
		WorkDir:  workDir,
		Model:    "test-model",
		BaseURL:  "http://localhost:11434/v1",
		MaxTurns: 5,
		Timeout:  5 * time.Second,
		Mode:     config.ModeYolo, // skip permission prompts in tests
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

// ------------------------------------------------------------------
//  Agent construction & system messages
// ------------------------------------------------------------------

func TestAgent_New(t *testing.T) {
	cfg := newTestConfig()
	r := tools.NewRegistry()
	mc := &llm.MockClient{}
	a := New(cfg, mc, r)
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
	mc := &llm.MockClient{}
	a := New(cfg, mc, r)
	msgs := a.history
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Content == "" {
		t.Error("system message is empty")
	}
	if msgs[0].Role != llm.RoleSystem {
		t.Errorf("role = %q, want %q", msgs[0].Role, llm.RoleSystem)
	}
	if msgs[0].Content == "" || msgs[0].Content[0] == 0 {
		t.Error("system message content missing")
	}
}

// ------------------------------------------------------------------
//  Simple response (no tool calls)
// ------------------------------------------------------------------

func TestAgent_Run_SimpleResponse(t *testing.T) {
	cfg := newTestConfig()
	r := tools.NewRegistry()
	mc := &llm.MockClient{
		Responses: []llm.MockResponse{
			{Text: "I can help!"},
		},
	}
	a := New(cfg, mc, r)
	events := collectEvents(a.Run(context.Background(), "hello"))

	if !hasEventType(events, "text") {
		t.Error("expected text event")
	}
	if !hasEventType(events, "turn_done") {
		t.Error("expected turn_done event")
	}
}

// ------------------------------------------------------------------
//  Single tool call with result
// ------------------------------------------------------------------

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
	a := New(cfg, mc, r)
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

// ------------------------------------------------------------------
//  Multiple sequential tool calls
// ------------------------------------------------------------------

func TestAgent_Run_MultipleToolCalls(t *testing.T) {
	dir := t.TempDir()

	cfg := newTestConfig()
	cfg.WorkDir = dir
	cfg.MaxTurns = 5
	cfg.Mode = config.ModeYolo

	r := tools.NewRegistry()
	mc := &llm.MockClient{
		Responses: []llm.MockResponse{
			// First turn: write file
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
			// Second turn: read file
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
			// Third turn: text response
			{Text: "Done reading the file."},
		},
	}
	a := New(cfg, mc, r)
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

// ------------------------------------------------------------------
//  Tool call error
// ------------------------------------------------------------------

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
	a := New(cfg, mc, r)
	events := collectEvents(a.Run(context.Background(), "read a nonexistent file"))

	if !hasEventType(events, "tool_error") {
		t.Errorf("expected tool_error event, got: %v", eventTypeSlice(events))
	}
}

// ------------------------------------------------------------------
//  Unknown tool (LLM calls a tool not in registry)
// ------------------------------------------------------------------

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
	// MaxTurns=1 so it returns after the unknown tool call + text response
	cfg := newTestConfig()
	cfg.MaxTurns = 1
	// Second response for after the tool error
	mc.Responses = append(mc.Responses, llm.MockResponse{Text: "I don't know that tool."})
	a := New(cfg, mc, r)
	events := collectEvents(a.Run(context.Background(), "do something"))

	if !hasEventType(events, "tool_error") {
		t.Errorf("expected tool_error for unknown tool, got: %v", eventTypeSlice(events))
	}
}

// ------------------------------------------------------------------
//  Context cancellation
// ------------------------------------------------------------------

func TestAgent_Run_ContextCancelled(t *testing.T) {
	cfg := newTestConfig()
	r := tools.NewRegistry()

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel before the agent loop can make a second call
	mc := &llm.MockClient{
		Responses: []llm.MockResponse{
			{Text: "thinking..."},
		},
	}
	a := New(cfg, mc, r)

	// Cancel after the first response is consumed
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	events := collectEvents(a.Run(ctx, "hello"))
	// should not hang
	if len(events) == 0 {
		t.Error("expected at least one event before cancellation")
	}
}

func TestAgent_Run_ContextCancelled_BeforeCall(t *testing.T) {
	cfg := newTestConfig()
	r := tools.NewRegistry()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately — before any work

	mc := &llm.MockClient{
		Responses: []llm.MockResponse{
			{Text: "should not be reached"},
		},
	}
	a := New(cfg, mc, r)

	events := collectEvents(a.Run(ctx, "test"))
	// Context is already cancelled, run() checks ctx.Err() at loop start
	// so it should return without events
	if len(events) > 0 {
		t.Errorf("expected no events with pre-cancelled context, got: %v", eventTypeSlice(events))
	}
}

// ------------------------------------------------------------------
//  MaxTurns limit reached
// ------------------------------------------------------------------

func TestAgent_Run_MaxTurnsLimit(t *testing.T) {
	dir := t.TempDir()

	cfg := newTestConfig()
	cfg.WorkDir = dir
	cfg.MaxTurns = 2 // Only allow 2 turns
	cfg.Mode = config.ModeYolo

	r := tools.NewRegistry()
	// Every response asks for another tool call — will exceed MaxTurns
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
			toolCallResp("t1"), // turn 1: asks for tool
			toolCallResp("t2"), // turn 2: asks for another tool
			toolCallResp("t3"), // exceeded max turns
		},
	}
	a := New(cfg, mc, r)
	events := collectEvents(a.Run(context.Background(), "keep doing things"))

	if !hasEventType(events, "error") {
		t.Errorf("expected error event when MaxTurns reached, got: %v", eventTypeSlice(events))
	} else {
		for _, e := range events {
			if e.Type == "error" && e.Err != nil {
				if e.Err.Error() == "" {
					t.Error("error event has empty message")
				}
			}
		}
	}
}

// ------------------------------------------------------------------
//  LLM Stream error
// ------------------------------------------------------------------

func TestAgent_Run_LLMStreamError(t *testing.T) {
	cfg := newTestConfig()
	r := tools.NewRegistry()
	mc := &llm.MockClient{
		Responses: []llm.MockResponse{
			{Err: &mockError{msg: "connection timeout"}},
		},
	}
	a := New(cfg, mc, r)
	events := collectEvents(a.Run(context.Background(), "test"))

	if !hasEventType(events, "error") {
		t.Errorf("expected error event on LLM failure, got: %v", eventTypeSlice(events))
	}
}

// ------------------------------------------------------------------
//  User message is added to history
// ------------------------------------------------------------------

func TestAgent_Run_UserMessageInHistory(t *testing.T) {
	cfg := newTestConfig()
	r := tools.NewRegistry()
	mc := &llm.MockClient{
		Responses: []llm.MockResponse{
			{Text: "Got it"},
		},
	}
	a := New(cfg, mc, r)
	events := collectEvents(a.Run(context.Background(), "my question"))

	_ = events // just checking no error occurred
	// Verify history has system + user messages
	if len(a.history) < 2 {
		t.Errorf("expected at least 2 history entries (system+user), got %d", len(a.history))
	}
	if a.history[len(a.history)-2].Role != llm.RoleUser {
		t.Errorf("expected user role, got %q", a.history[len(a.history)-2].Role)
	}
	if a.history[len(a.history)-2].Content != "my question" {
		t.Errorf("history user content = %q, want %q", a.history[len(a.history)-2].Content, "my question")
	}
}

// ------------------------------------------------------------------
//  executeTool with unknown tool (unit test)
// ------------------------------------------------------------------

func TestAgent_ExecuteTool_UnknownTool(t *testing.T) {
	cfg := newTestConfig()
	r := tools.NewRegistry()
	mc := &llm.MockClient{}
	a := New(cfg, mc, r)

	tc := llm.ToolCall{
		ID:   "t1",
		Type: "function",
		Function: llm.ToolCallFunction{
			Name:      "nonexistent_tool",
			Arguments: "{}",
		},
	}

	_, err := a.executeTool(context.Background(), tc)
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
}

// ------------------------------------------------------------------
//  No tool calls (empty LLM response)
// ------------------------------------------------------------------

func TestAgent_Run_NoToolCalls(t *testing.T) {
	r := tools.NewRegistry()
	mc := &llm.MockClient{
		Responses: []llm.MockResponse{
			{Text: "Just a plain answer"},
		},
	}
	cfg := newTestConfig()
	a := New(cfg, mc, r)
	events := collectEvents(a.Run(context.Background(), "test"))

	if !hasEventType(events, "text") {
		t.Error("expected text event")
	}
	if !hasEventType(events, "turn_done") {
		t.Error("expected turn_done event")
	}
}

// ------------------------------------------------------------------
//  Permission checker — blocked tool
// ------------------------------------------------------------------

func TestAgent_Checker_Blocklist(t *testing.T) {
	cfg := newTestConfig()
	cfg.Mode = config.ModeYolo
	r := tools.NewRegistry()
	mc := &llm.MockClient{}
	a := New(cfg, mc, r)

	// Verify the checker has a blocklist
	if a.checker == nil {
		t.Fatal("checker is nil")
	}
	if len(a.checker.Blocklist) == 0 {
		t.Error("blocklist should not be empty")
	}
}

// ------------------------------------------------------------------
//  Multiple tool calls in a single turn (parallel)
// ------------------------------------------------------------------

func TestAgent_Run_ParallelToolCalls(t *testing.T) {
	dir := t.TempDir()

	cfg := newTestConfig()
	cfg.WorkDir = dir
	cfg.Mode = config.ModeYolo

	r := tools.NewRegistry()
	mc := &llm.MockClient{
		Responses: []llm.MockResponse{
			// Two tool calls in one turn
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
	a := New(cfg, mc, r)
	events := collectEvents(a.Run(context.Background(), "create two files"))

	// Count tool execution events
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

// ------------------------------------------------------------------
//  Event ordering: tool_start before tool_done
// ------------------------------------------------------------------

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
	a := New(cfg, mc, r)
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

// ------------------------------------------------------------------
//  Helper
// ------------------------------------------------------------------

func eventTypeSlice(events []Event) []string {
	types := make([]string, len(events))
	for i, e := range events {
		types[i] = e.Type
	}
	return types
}

// mockError for TestAgent_Run_LLMStreamError
type mockError struct{ msg string }

func (e *mockError) Error() string { return e.msg }
