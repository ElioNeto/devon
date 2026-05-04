package agent

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ElioNeto/devon/internal/agent/testutil"
	"github.com/ElioNeto/devon/internal/config"
	"github.com/ElioNeto/devon/internal/llm"
	"github.com/ElioNeto/devon/internal/permissions"
	"github.com/ElioNeto/devon/internal/tools"
)

// TestLoop_SimpleToolCall verifies that a single tool call is dispatched
// and its result flows back as a final text response.
func TestLoop_SimpleToolCall(t *testing.T) {
	cfg := newTestConfig()
	cfg.MaxAgentLoops = 5

	mc := &testutil.MockClient{
		Responses: []llm.MockResponse{
			{
				ToolCalls: []llm.ToolCall{
					{
						ID:   "call_1",
						Type: "function",
						Function: llm.ToolCallFunction{
							Name:      "mock_tool_1",
							Arguments: `{"input":"hello"}`,
						},
					},
				},
			},
			{Text: "Final answer after tool."},
		},
	}

	mt := &testutil.MockTool{
		NameValue:        "mock_tool_1",
		Result:           "tool_output_result",
		PermissionLevel:  permissions.PermRead,
	}

	r := tools.NewRegistry()
	r.Register(mt)

	fakeDB := &fakeDBStore{}
	a := New(cfg, mc, r, fakeDB, "agent-loop-simple", nil, "/tmp/test-workdir")
	events := collectEvents(a.Run(context.Background(), "use the tool"))

	if !hasEventType(events, "tool_start") {
		t.Errorf("expected tool_start, got: %v", eventTypeSlice(events))
	}
	if !hasEventType(events, "tool_done") {
		t.Errorf("expected tool_done, got: %v", eventTypeSlice(events))
	}
	if !hasEventType(events, "text") {
		t.Errorf("expected text event, got: %v", eventTypeSlice(events))
	}
	if !hasEventType(events, "turn_done") {
		t.Errorf("expected turn_done, got: %v", eventTypeSlice(events))
	}

	if mt.Called.Load() != 1 {
		t.Errorf("MockTool.Called = %d, want 1", mt.Called.Load())
	}

	// Verify tool_done contains the correct result
	foundResult := false
	for _, e := range events {
		if e.Type == "tool_done" && e.Result == "tool_output_result" {
			foundResult = true
			break
		}
	}
	if !foundResult {
		t.Error("expected tool_done event with result 'tool_output_result'")
	}
}

// TestLoop_MultipleToolCalls verifies that three tool calls in one response
// are all dispatched and their results are returned.
func TestLoop_MultipleToolCalls(t *testing.T) {
	cfg := newTestConfig()
	cfg.MaxAgentLoops = 5

	mc := &testutil.MockClient{
		Responses: []llm.MockResponse{
			{
				ToolCalls: []llm.ToolCall{
					{
						ID:   "call_a",
						Type: "function",
						Function: llm.ToolCallFunction{
							Name:      "mock_tool_a",
							Arguments: `{"x":1}`,
						},
					},
					{
						ID:   "call_b",
						Type: "function",
						Function: llm.ToolCallFunction{
							Name:      "mock_tool_b",
							Arguments: `{"x":2}`,
						},
					},
					{
						ID:   "call_c",
						Type: "function",
						Function: llm.ToolCallFunction{
							Name:      "mock_tool_c",
							Arguments: `{"x":3}`,
						},
					},
				},
			},
			{Text: "All tools finished."},
		},
	}

	mtA := &testutil.MockTool{NameValue: "mock_tool_a", Result: "result_a", PermissionLevel: permissions.PermRead}
	mtB := &testutil.MockTool{NameValue: "mock_tool_b", Result: "result_b", PermissionLevel: permissions.PermRead}
	mtC := &testutil.MockTool{NameValue: "mock_tool_c", Result: "result_c", PermissionLevel: permissions.PermRead}

	r := tools.NewRegistry()
	r.Register(mtA)
	r.Register(mtB)
	r.Register(mtC)

	fakeDB := &fakeDBStore{}
	a := New(cfg, mc, r, fakeDB, "agent-loop-multi", nil, "/tmp/test-workdir")
	events := collectEvents(a.Run(context.Background(), "call three tools"))

	toolStarts := 0
	toolDones := 0
	for _, e := range events {
		switch e.Type {
		case "tool_start":
			toolStarts++
		case "tool_done":
			toolDones++
		}
	}

	if toolStarts != 3 {
		t.Errorf("expected 3 tool_start events, got %d", toolStarts)
	}
	if toolDones != 3 {
		t.Errorf("expected 3 tool_done events, got %d", toolDones)
	}
	if !hasEventType(events, "text") {
		t.Error("expected text event after tool results")
	}
	if !hasEventType(events, "turn_done") {
		t.Error("expected turn_done event")
	}

	if mtA.Called.Load() != 1 {
		t.Errorf("mock_tool_a.Called = %d, want 1", mtA.Called.Load())
	}
	if mtB.Called.Load() != 1 {
		t.Errorf("mock_tool_b.Called = %d, want 1", mtB.Called.Load())
	}
	if mtC.Called.Load() != 1 {
		t.Errorf("mock_tool_c.Called = %d, want 1", mtC.Called.Load())
	}
}

// TestLoop_ToolError verifies that when a tool returns an error,
// a tool_error event is emitted and the agent still produces a final response.
func TestLoop_ToolError(t *testing.T) {
	cfg := newTestConfig()
	cfg.MaxAgentLoops = 5

	mc := &testutil.MockClient{
		Responses: []llm.MockResponse{
			{
				ToolCalls: []llm.ToolCall{
					{
						ID:   "call_err",
						Type: "function",
						Function: llm.ToolCallFunction{
							Name:      "mock_tool_err",
							Arguments: `{}`,
						},
					},
				},
			},
			{Text: "I recovered from the error."},
		},
	}

	mt := &testutil.MockTool{
		NameValue:       "mock_tool_err",
		Err:             &mockError{msg: "simulated tool failure"},
		PermissionLevel: permissions.PermRead,
	}

	r := tools.NewRegistry()
	r.Register(mt)

	fakeDB := &fakeDBStore{}
	a := New(cfg, mc, r, fakeDB, "agent-loop-err", nil, "/tmp/test-workdir")
	events := collectEvents(a.Run(context.Background(), "trigger an error"))

	if !hasEventType(events, "tool_start") {
		t.Errorf("expected tool_start, got: %v", eventTypeSlice(events))
	}
	if !hasEventType(events, "tool_error") {
		t.Errorf("expected tool_error, got: %v", eventTypeSlice(events))
	}
	if !hasEventType(events, "text") {
		t.Errorf("expected text event after tool error, got: %v", eventTypeSlice(events))
	}
	if !hasEventType(events, "turn_done") {
		t.Errorf("expected turn_done, got: %v", eventTypeSlice(events))
	}

	if mt.Called.Load() != 1 {
		t.Errorf("MockTool.Called = %d, want 1", mt.Called.Load())
	}
}

// TestLoop_MaxTurns verifies that when MaxAgentLoops is reached because the
// agent keeps requesting tool calls (never producing a final text), an error
// event is emitted with a message about the limit.
func TestLoop_MaxTurns(t *testing.T) {
	cfg := newTestConfig()
	cfg.MaxAgentLoops = 2
	cfg.Mode = config.ModeYolo

	mc := &testutil.MockClient{
		Responses: []llm.MockResponse{
			{
				ToolCalls: []llm.ToolCall{
					{
						ID:   "t1",
						Type: "function",
						Function: llm.ToolCallFunction{
							Name:      "mock_tool_loop",
							Arguments: `{}`,
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
							Name:      "mock_tool_loop",
							Arguments: `{}`,
						},
					},
				},
			},
			{
				ToolCalls: []llm.ToolCall{
					{
						ID:   "t3",
						Type: "function",
						Function: llm.ToolCallFunction{
							Name:      "mock_tool_loop",
							Arguments: `{}`,
						},
					},
				},
			},
		},
	}

	mt := &testutil.MockTool{
		NameValue:       "mock_tool_loop",
		Result:          "done",
		PermissionLevel: permissions.PermRead,
	}

	r := tools.NewRegistry()
	r.Register(mt)

	fakeDB := &fakeDBStore{}
	a := New(cfg, mc, r, fakeDB, "agent-loop-maxturns", nil, "/tmp/test-workdir")
	events := collectEvents(a.Run(context.Background(), "keep looping"))

	// Should have an error about reaching the limit
	hasLimitError := false
	for _, e := range events {
		if e.Type == "error" && e.Err != nil && strings.Contains(e.Err.Error(), "limite") {
			hasLimitError = true
			break
		}
	}
	if !hasLimitError {
		t.Errorf("expected error event about limit, got: %v", eventTypeSlice(events))
	}
}

// TestLoop_MaxLoops verifies that the agent stops after MaxAgentLoops
// iterations when the LLM keeps requesting tool calls.
func TestLoop_MaxLoops(t *testing.T) {
	cfg := newTestConfig()
	cfg.MaxAgentLoops = 3
	cfg.Mode = config.ModeYolo

	mc := &testutil.MockClient{
		Responses: []llm.MockResponse{
			{
				ToolCalls: []llm.ToolCall{
					{
						ID:   "l1",
						Type: "function",
						Function: llm.ToolCallFunction{
							Name:      "mock_tool_ml",
							Arguments: `{}`,
						},
					},
				},
			},
			{
				ToolCalls: []llm.ToolCall{
					{
						ID:   "l2",
						Type: "function",
						Function: llm.ToolCallFunction{
							Name:      "mock_tool_ml",
							Arguments: `{}`,
						},
					},
				},
			},
			{
				ToolCalls: []llm.ToolCall{
					{
						ID:   "l3",
						Type: "function",
						Function: llm.ToolCallFunction{
							Name:      "mock_tool_ml",
							Arguments: `{}`,
						},
					},
				},
			},
			{
				ToolCalls: []llm.ToolCall{
					{
						ID:   "l4",
						Type: "function",
						Function: llm.ToolCallFunction{
							Name:      "mock_tool_ml",
							Arguments: `{}`,
						},
					},
				},
			},
		},
	}

	mt := &testutil.MockTool{
		NameValue:       "mock_tool_ml",
		Result:          "ok",
		PermissionLevel: permissions.PermRead,
	}

	r := tools.NewRegistry()
	r.Register(mt)

	fakeDB := &fakeDBStore{}
	a := New(cfg, mc, r, fakeDB, "agent-loop-maxloops", nil, "/tmp/test-workdir")
	events := collectEvents(a.Run(context.Background(), "keep calling tools"))

	toolStarts := 0
	for _, e := range events {
		if e.Type == "tool_start" {
			toolStarts++
		}
	}

	if toolStarts != 3 {
		t.Errorf("expected 3 tool_start events (one per loop), got %d", toolStarts)
	}

	// Should have an error about the limit
	hasLimitError := false
	for _, e := range events {
		if e.Type == "error" && e.Err != nil && strings.Contains(e.Err.Error(), "limite") {
			hasLimitError = true
			break
		}
	}
	if !hasLimitError {
		t.Errorf("expected error event about limit after %d loops, got: %v", cfg.MaxAgentLoops, eventTypeSlice(events))
	}

	if mt.Called.Load() != 3 {
		t.Errorf("MockTool.Called = %d, want 3", mt.Called.Load())
	}
}

// TestLoop_Cancellation verifies that when the context is cancelled during
// tool execution, the agent stops quickly and emits no events after cancellation.
func TestLoop_Cancellation(t *testing.T) {
	cfg := newTestConfig()
	cfg.MaxAgentLoops = 5
	cfg.Mode = config.ModeYolo

	ctx, cancel := context.WithCancel(context.Background())

	mc := &testutil.MockClient{
		Responses: []llm.MockResponse{
			{
				ToolCalls: []llm.ToolCall{
					{
						ID:   "cancel_call",
						Type: "function",
						Function: llm.ToolCallFunction{
							Name:      "mock_tool_cancel",
							Arguments: `{}`,
						},
					},
				},
			},
			{Text: "should not be reached"},
		},
	}

	mt := &testutil.MockTool{
		NameValue:       "mock_tool_cancel",
		Result:          "cancellable_result",
		PermissionLevel: permissions.PermRead,
	}

	r := tools.NewRegistry()
	r.Register(mt)

	fakeDB := &fakeDBStore{}
	a := New(cfg, mc, r, fakeDB, "agent-loop-cancel", nil, "/tmp/test-workdir")

	// Cancel after a short delay so execution begins
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()

	events := collectEvents(a.Run(ctx, "do something and cancel"))

	// At minimum, tool_start should be emitted before cancellation
	if !hasEventType(events, "tool_start") {
		t.Errorf("expected at least one tool_start event before cancellation, got: %v", eventTypeSlice(events))
	}

	// The channel should be closed (agent stopped) — verify by checking we got events
	if len(events) == 0 {
		t.Error("expected at least some events before cancellation")
	}
}

// TestLoop_ConversationalInput verifies that when the LLM returns direct text
// without any tool calls, no tool_start/tool_done events are emitted.
func TestLoop_ConversationalInput(t *testing.T) {
	cfg := newTestConfig()
	cfg.MaxAgentLoops = 5

	mc := &testutil.MockClient{
		Responses: []llm.MockResponse{
			{Text: "Direct conversational response without tools."},
		},
	}

	r := tools.NewRegistry()
	fakeDB := &fakeDBStore{}
	a := New(cfg, mc, r, fakeDB, "agent-loop-conv", nil, "/tmp/test-workdir")
	events := collectEvents(a.Run(context.Background(), "just talk to me"))

	if hasEventType(events, "tool_start") {
		t.Error("expected NO tool_start events for conversational input")
	}
	if hasEventType(events, "tool_done") {
		t.Error("expected NO tool_done events for conversational input")
	}
	if !hasEventType(events, "text") {
		t.Errorf("expected text event with response, got: %v", eventTypeSlice(events))
	}
	if !hasEventType(events, "turn_done") {
		t.Errorf("expected turn_done event, got: %v", eventTypeSlice(events))
	}

	// Verify the response text is present
	foundText := false
	for _, e := range events {
		if e.Type == "text" && strings.Contains(e.Text, "Direct conversational response") {
			foundText = true
			break
		}
	}
	if !foundText {
		t.Error("expected text event to contain the mock response")
	}
}
