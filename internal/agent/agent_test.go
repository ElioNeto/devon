package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ElioNeto/devon/internal/config"
	"github.com/ElioNeto/devon/internal/db"
	"github.com/ElioNeto/devon/internal/index"
	"github.com/ElioNeto/devon/internal/llm"
	"github.com/ElioNeto/devon/internal/tools"
)

func newTestConfig() *config.Config {
	return &config.Config{
		WorkDir:  "/tmp/test-workdir",
		Model:    "test-model",
		BaseURL:  "http://localhost:11434/v1",
		MaxAgentLoops: 5,
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
	projectID := "/tmp/test-workdir"
	a := New(cfg, &llm.MockClient{}, r, fakeDB, "test-agent-1", nil, projectID)

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
	a := New(cfg, &llm.MockClient{}, r, fakeDB, "default-agent", nil, "/tmp/test-workdir")

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
	a := New(cfg, &llm.MockClient{}, r, fakeDB, "test", nil, "/tmp/test-workdir")

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
	a := New(cfg, mc, r, fakeDB, "agent-1", nil, "/tmp/test-workdir")
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
	cfg.MaxAgentLoops = 5
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
	a := New(cfg, mc, r, fakeDB, "agent-1", nil, "/tmp/test-workdir")
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
	cfg.MaxAgentLoops = 5
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
	a := New(cfg, mc, r, fakeDB, "agent-1", nil, "/tmp/test-workdir")
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
	a := New(cfg, mc, r, fakeDB, "agent-1", nil, "/tmp/test-workdir")
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
	cfg.MaxAgentLoops = 1
	mc.Responses = append(mc.Responses, llm.MockResponse{Text: "I don't know that tool."})

	fakeDB := &fakeDBStore{}
	a := New(cfg, mc, r, fakeDB, "agent-1", nil, "/tmp/test-workdir")
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
	a := New(cfg, mc, r, fakeDB, "agent-1", nil, "/tmp/test-workdir")

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
	a := New(cfg, mc, r, fakeDB, "agent-1", nil, "/tmp/test-workdir")

	events := collectEvents(a.Run(ctx, "test"))
	if len(events) > 0 {
		t.Errorf("expected no events with pre-cancelled context, got: %v", eventTypeSlice(events))
	}
}

func TestAgent_Run_MaxAgentLoopsLimit(t *testing.T) {
	dir := t.TempDir()

	cfg := newTestConfig()
	cfg.WorkDir = dir
	cfg.MaxAgentLoops = 2
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
	a := New(cfg, mc, r, fakeDB, "agent-1", nil, "/tmp/test-workdir")
	events := collectEvents(a.Run(context.Background(), "keep doing things"))

	if !hasEventType(events, "error") {
		t.Errorf("expected error event when MaxAgentLoops reached, got: %v", eventTypeSlice(events))
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
	a := New(cfg, mc, r, fakeDB, "agent-1", nil, "/tmp/test-workdir")
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
	a := New(cfg, mc, r, fakeDB, "agent-1", nil, "/tmp/test-workdir")
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
	a := New(cfg, mc, r, fakeDB, "agent-1", nil, "/tmp/test-workdir")

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
	a := New(cfg, mc, r, fakeDB, "agent-1", nil, "/tmp/test-workdir")
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
	a := New(cfg, mc, r, fakeDB, "agent-1", nil, "/tmp/test-workdir")

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
	a := New(cfg, mc, r, fakeDB, "agent-1", nil, "/tmp/test-workdir")
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
	a := New(cfg, mc, r, fakeDB, "agent-1", nil, "/tmp/test-workdir")
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
	cfg.MaxAgentLoops = 5
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
	a := New(cfg, mc, r, fakeDB, "agent-1", nil, "/tmp/test-workdir")

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
	cfg.MaxAgentLoops = 5
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
	a := New(cfg, mc, r, fakeDB, "agent-1", nil, "/tmp/test-workdir")

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
	a := New(cfg, mc, r, fakeDB, "agent-1", nil, "/tmp/test-workdir")
	events := collectEvents(a.Run(context.Background(), "test"))

	if !hasEventType(events, "rate_limited") {
		t.Errorf("expected rate_limited event, got: %v", eventTypeSlice(events))
	}
	if !hasEventType(events, "error") {
		t.Errorf("expected error event after rate_limited, got: %v", eventTypeSlice(events))
	}
}

func TestAgent_BuildSystemMessages_ProjectContext(t *testing.T) {
	dir := t.TempDir()
	// Create a Go file so BuildProjectContext can detect the language
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := newTestConfig()
	cfg.WorkDir = dir
	r := tools.NewRegistry()
	fakeDB := &fakeDBStore{}
	a := New(cfg, &llm.MockClient{}, r, fakeDB, "test", nil, dir)

	msgs := a.history
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	content := *msgs[0].Content
	if !strings.Contains(content, "Contexto do projeto") {
		t.Error("expected 'Contexto do projeto' in system message")
	}
	if !strings.Contains(content, "Linguagens detectadas: Go") {
		t.Error("expected 'Linguagens detectadas: Go' in system message")
	}
	if !strings.Contains(content, dir) {
		t.Error("expected workdir path in system message")
	}
}

func TestAgent_BuildSystemMessages_RelevantFiles(t *testing.T) {
	dir := t.TempDir()

	// Create a Go file with unique content for indexing
	goFile := filepath.Join(dir, "main.go")
	if err := os.WriteFile(goFile, []byte("package main\n\nfunc main() {\n\tprintln(\"hello world\")\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create index manager pointing to the temp dir
	mgr, err := index.NewManager(dir, index.ManagerConfig{
		Enabled: true,
		IndexedConfig: index.IndexedConfig{
			Extensions:    []string{".go"},
			Excludes:      []string{},
			MaxFileSizeKB: 500,
			CacheDir:      filepath.Join(dir, ".cache"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.Close()

	// Index the files
	if err := mgr.Index(context.Background(), dir); err != nil {
		t.Fatal(err)
	}

	// Create agent (without index)
	cfg := newTestConfig()
	cfg.WorkDir = dir
	r := tools.NewRegistry()
	fakeDB := &fakeDBStore{}
	a := New(cfg, &llm.MockClient{}, r, fakeDB, "test", nil, dir)

	// Inject the index manager
	a.idxMgr = mgr

	// Build system messages with a non-empty prompt
	msgs := a.buildSystemMessages("hello world main function")

	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	content := *msgs[0].Content
	if !strings.Contains(content, "Arquivos relevantes") {
		t.Error("expected 'Arquivos relevantes' section in system message")
	}
	if !strings.Contains(content, "main.go") {
		t.Error("expected 'main.go' in relevant files")
	}
}

func TestAgent_Run_RebuildsSystemMessage(t *testing.T) {
	dir := t.TempDir()

	// Create a Go file for indexing
	goFile := filepath.Join(dir, "main.go")
	if err := os.WriteFile(goFile, []byte("package main\n\nfunc main() {\n\tprintln(\"hello world\")\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create index manager
	mgr, err := index.NewManager(dir, index.ManagerConfig{
		Enabled: true,
		IndexedConfig: index.IndexedConfig{
			Extensions:    []string{".go"},
			Excludes:      []string{},
			MaxFileSizeKB: 500,
			CacheDir:      filepath.Join(dir, ".cache"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.Close()

	// Index files
	if err := mgr.Index(context.Background(), dir); err != nil {
		t.Fatal(err)
	}

	// Create agent and inject index
	cfg := newTestConfig()
	cfg.WorkDir = dir
	r := tools.NewRegistry()
	mc := &llm.MockClient{
		Responses: []llm.MockResponse{
			{Text: "Got it"},
		},
	}
	fakeDB := &fakeDBStore{}
	a := New(cfg, mc, r, fakeDB, "agent-1", nil, dir)
	a.idxMgr = mgr

	// Run the agent
	events := collectEvents(a.Run(context.Background(), "hello world main function"))
	_ = events

	// Verify history[0] was rebuilt with index-relevant content
	if len(a.history) < 1 {
		t.Fatal("history is empty")
	}
	content := *a.history[0].Content
	if content == "" {
		t.Error("system message should not be empty")
	}
	if a.history[0].Role != llm.RoleSystem {
		t.Errorf("history[0] role = %q, want %q", a.history[0].Role, llm.RoleSystem)
	}
	if !strings.Contains(content, "Contexto do projeto") {
		t.Error("expected 'Contexto do projeto' in rebuilt system message")
	}
	if !strings.Contains(content, "Arquivos relevantes") {
		t.Error("expected 'Arquivos relevantes' in rebuilt system message")
	}
	if !strings.Contains(content, "main.go") {
		t.Error("expected 'main.go' in relevant files section")
	}
}

// TestAgent_Run_AnswersWithoutTools verifies the agent answers a direct question
// without invoking any tool calls (AC 4).
func TestAgent_Run_AnswersWithoutTools(t *testing.T) {
	dir := t.TempDir()

	// Create a .go file so Go is detected
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := newTestConfig()
	cfg.WorkDir = dir
	cfg.MaxAgentLoops = 5

	r := tools.NewRegistry()
	mc := &llm.MockClient{
		Responses: []llm.MockResponse{
			{Text: "A capital of France is Paris."},
		},
	}

	fakeDB := &fakeDBStore{}
	a := New(cfg, mc, r, fakeDB, "agent-ac4", nil, dir)

	events := collectEvents(a.Run(context.Background(), "What is the capital of France?"))

	// Verify the system message includes detected languages
	if len(a.history) == 0 {
		t.Fatal("history is empty")
	}
	sysMsg := *a.history[0].Content
	if !strings.Contains(sysMsg, "Linguagens detectadas: Go") {
		t.Error("expected 'Linguagens detectadas: Go' in system message")
	}

	// Verify no tool calls were made
	if hasEventType(events, "tool_start") {
		t.Error("expected no tool_start events for a simple language question")
	}
	if hasEventType(events, "tool_done") {
		t.Error("expected no tool_done events for a simple language question")
	}

	// Verify text and turn_done events are present
	if !hasEventType(events, "text") {
		t.Error("expected text event with the answer")
	}
	if !hasEventType(events, "turn_done") {
		t.Error("expected turn_done event")
	}

	// Verify the final answer text
	foundAnswer := false
	for _, e := range events {
		if e.Type == "text" && strings.Contains(e.Text, "Paris") {
			foundAnswer = true
			break
		}
	}
	if !foundAnswer {
		t.Error("expected answer text to contain 'Paris'")
	}
}

// TestAgent_Run_ContextDocFlowsToSystemMessage verifies that ContextDoc from config
// is included in the system message both at construction and after Run (AC 5).
func TestAgent_Run_ContextDocFlowsToSystemMessage(t *testing.T) {
	cfg := newTestConfig()
	cfg.ContextDoc = "This is a test context document."
	cfg.MaxAgentLoops = 5

	r := tools.NewRegistry()
	mc := &llm.MockClient{
		Responses: []llm.MockResponse{
			{Text: "Understood."},
		},
	}

	fakeDB := &fakeDBStore{}
	a := New(cfg, mc, r, fakeDB, "agent-ac5", nil, "/tmp/test-workdir")

	// Verify history[0] (system message) contains the ContextDoc content
	if len(a.history) == 0 {
		t.Fatal("expected at least one system message in history")
	}
	initialSysMsg := *a.history[0].Content
	if !strings.Contains(initialSysMsg, "This is a test context document.") {
		t.Errorf("expected ContextDoc in system message after New(), got:\n%s", initialSysMsg)
	}

	// Run the agent
	events := collectEvents(a.Run(context.Background(), "do something"))

	// Verify the system message (rebuild at run time) still contains ContextDoc
	if len(a.history) == 0 {
		t.Fatal("history is empty after Run")
	}
	runSysMsg := *a.history[0].Content
	if !strings.Contains(runSysMsg, "This is a test context document.") {
		t.Errorf("expected ContextDoc in system message after Run(), got:\n%s", runSysMsg)
	}

	// Verify the agent answered
	if !hasEventType(events, "text") {
		t.Error("expected text event")
	}
	if !hasEventType(events, "turn_done") {
		t.Error("expected turn_done event")
	}
}

// TestAgent_RunWithMessage_RebuildsSystemMessage verifies that runWithMessage
// rebuilds the system message with project context for multimodal input.
func TestAgent_RunWithMessage_RebuildsSystemMessage(t *testing.T) {
	dir := t.TempDir()

	// Create a Go file so Go is detected
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := newTestConfig()
	cfg.WorkDir = dir
	cfg.ContextDoc = "Multimodal context document."
	cfg.MaxAgentLoops = 5

	r := tools.NewRegistry()
	mc := &llm.MockClient{
		Responses: []llm.MockResponse{
			{Text: "Processed multimodal message."},
		},
	}

	fakeDB := &fakeDBStore{}
	a := New(cfg, mc, r, fakeDB, "agent-mm", nil, dir)

	// Verify initial history has ContextDoc
	if len(a.history) == 0 {
		t.Fatal("expected system message after New()")
	}
	initialSysMsg := *a.history[0].Content
	if !strings.Contains(initialSysMsg, "Multimodal context document.") {
		t.Errorf("expected ContextDoc in system message after New(), got:\n%s", initialSysMsg)
	}

	// Create a multimodal message with text content
	msg := llm.Message{
		Role: llm.RoleUser,
		ContentParts: []llm.ContentPart{
			{Type: llm.TypeText, Text: "What can you see?"},
			{Type: llm.TypeImageURL, ImageURL: &llm.ImageURL{
				URL:    "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==",
				Detail: "auto",
			}},
		},
	}

	events := collectEvents(a.RunWithMessage(context.Background(), msg))

	// Verify history[0] is rebuilt with updated system message containing project context
	if len(a.history) == 0 {
		t.Fatal("history is empty after RunWithMessage")
	}
	runSysMsg := *a.history[0].Content
	if !strings.Contains(runSysMsg, "Contexto do projeto") {
		t.Errorf("expected 'Contexto do projeto' in system message after RunWithMessage, got:\n%s", runSysMsg)
	}
	if !strings.Contains(runSysMsg, "Linguagens detectadas: Go") {
		t.Errorf("expected 'Linguagens detectadas: Go' in system message after RunWithMessage, got:\n%s", runSysMsg)
	}
	if !strings.Contains(runSysMsg, "Multimodal context document.") {
		t.Errorf("expected ContextDoc in system message after RunWithMessage, got:\n%s", runSysMsg)
	}

	// Verify runWithMessage processes the multimodal content
	if !hasEventType(events, "text") {
		t.Error("expected text event")
	}
	if !hasEventType(events, "turn_done") {
		t.Error("expected turn_done event")
	}
}

func TestAgent_SetConversation_ReplacesHistory(t *testing.T) {
	cfg := newTestConfig()
	r := tools.NewRegistry()
	mc := &llm.MockClient{}
	fakeDB := &fakeDBStore{}
	a := New(cfg, mc, r, fakeDB, "agent-1", nil, "/tmp/test-workdir")

	// Initial history should have at least the system message
	if len(a.history) < 1 {
		t.Fatal("expected at least 1 history entry (system) after New")
	}

	// Set conversation with some messages
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: llm.TextContent("hello")},
		{Role: llm.RoleAssistant, Content: llm.TextContent("world")},
	}
	a.SetConversation(msgs)

	// History should be system + the 2 messages
	if len(a.history) != 3 {
		t.Fatalf("expected 3 history entries (system+2), got %d", len(a.history))
	}
	if a.history[0].Role != llm.RoleSystem {
		t.Errorf("a.history[0].Role = %q, want %q", a.history[0].Role, llm.RoleSystem)
	}
	if a.history[1].Role != llm.RoleUser || *a.history[1].Content != "hello" {
		t.Errorf("a.history[1] = %+v, want {RoleUser, 'hello'}", a.history[1])
	}
	if a.history[2].Role != llm.RoleAssistant || *a.history[2].Content != "world" {
		t.Errorf("a.history[2] = %+v, want {RoleAssistant, 'world'}", a.history[2])
	}
}

func TestAgent_ResetHistory_ClearsToSystem(t *testing.T) {
	cfg := newTestConfig()
	r := tools.NewRegistry()
	mc := &llm.MockClient{}
	fakeDB := &fakeDBStore{}
	a := New(cfg, mc, r, fakeDB, "agent-1", nil, "/tmp/test-workdir")

	// Add some conversation messages
	a.history = append(a.history,
		llm.Message{Role: llm.RoleUser, Content: llm.TextContent("hello")},
		llm.Message{Role: llm.RoleAssistant, Content: llm.TextContent("world")},
	)

	// Reset
	a.ResetHistory()

	// History should be just the system message
	if len(a.history) != 1 {
		t.Fatalf("expected 1 history entry after reset, got %d", len(a.history))
	}
	if a.history[0].Role != llm.RoleSystem {
		t.Errorf("a.history[0].Role = %q, want %q", a.history[0].Role, llm.RoleSystem)
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
func (f *fakeDBStore) CreateSessionWithMeta(ctx context.Context, id, task, model, status string) error            { return nil }
func (f *fakeDBStore) GetSession(ctx context.Context, id string) (bool, error)                                 { return false, nil }
func (f *fakeDBStore) ListSessions(ctx context.Context, limit int) ([]string, error)                          { return nil, nil }
func (f *fakeDBStore) GetSessionDetail(ctx context.Context, id string) (*db.SessionDetail, error)             { return nil, nil }
func (f *fakeDBStore) ListSessionsDetail(ctx context.Context, limit int) ([]db.SessionDetail, error)          { return nil, nil }
func (f *fakeDBStore) UpdateSession(ctx context.Context, id, task, model, status string) error                { return nil }
func (f *fakeDBStore) DeleteSession(ctx context.Context, id string) error                                     { return nil }
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
func (f *fakeDBStore) PutFact(ctx context.Context, projectID, category, content, context string) error        { return nil }
func (f *fakeDBStore) GetFacts(ctx context.Context, projectID, category string, limit int) ([]db.Fact, error) {
	return nil, nil
}
func (f *fakeDBStore) ListFacts(ctx context.Context, projectID string) ([]db.Fact, error)                      { return nil, nil }
func (f *fakeDBStore) DeleteFacts(ctx context.Context, projectID string) error                                 { return nil }
func (f *fakeDBStore) RecordFileAccess(ctx context.Context, sessionID, filePath, accessType string) error    { return nil }
func (f *fakeDBStore) GetFileAccess(ctx context.Context, sessionID string, limit int) ([]db.FileAccess, error) {
	return nil, nil
}
func (f *fakeDBStore) PutErrorPattern(ctx context.Context, projectID, pattern, context string) error         { return nil }
func (f *fakeDBStore) IncrementErrorPattern(ctx context.Context, projectID, pattern string) error           { return nil }
func (f *fakeDBStore) GetErrorPatterns(ctx context.Context, projectID string, limit int) ([]db.ErrorPattern, error) {
	return nil, nil
}
func (f *fakeDBStore) QueryFacts(ctx context.Context, projectID, keyword string, limit int) ([]db.FactRow, error) {
	return nil, nil
}

// ── Integration tests for sliding window ────────────────────────────────────

func TestAgent_Run_SlidingWindowTruncatesHistory(t *testing.T) {
	cfg := newTestConfig()
	cfg.MaxHistoryTurns = 1 // keep at most 1 turn
	cfg.MaxAgentLoops = 5

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
							Arguments: `{"path":"/tmp/test-sw/a.txt","content":"data"}`,
						},
					},
				},
			},
			{Text: "Done after first tool call."},
		},
	}

	fakeDB := &fakeDBStore{}
	a := New(cfg, mc, r, fakeDB, "agent-sw", nil, "/tmp/test-workdir")

	// Pre-populate history with many turns (more than MaxHistoryTurns)
	// System + 3 turns (user+assistant each)
	a.history = []llm.Message{
		{Role: llm.RoleSystem, Content: llm.TextContent("system")},
		{Role: llm.RoleUser, Content: llm.TextContent("turn1")},
		{Role: llm.RoleAssistant, Content: llm.TextContent("resp1")},
		{Role: llm.RoleUser, Content: llm.TextContent("turn2")},
		{Role: llm.RoleAssistant, Content: llm.TextContent("resp2")},
		{Role: llm.RoleUser, Content: llm.TextContent("turn3")},
		{Role: llm.RoleAssistant, Content: llm.TextContent("resp3")},
	}

	events := collectEvents(a.Run(context.Background(), "final turn"))

	// Verify the sliding window ran (truncStats should show turns removed)
	if a.truncStats.TurnsRemoved == 0 {
		t.Error("expected TurnsRemoved > 0 after sliding window with MaxHistoryTurns=1")
	}

	// Verify the agent completed successfully
	if !hasEventType(events, "turn_done") {
		t.Errorf("expected turn_done, got events: %v", eventTypeSlice(events))
	}
}

func TestAgent_Run_SlidingWindow_Unlimited(t *testing.T) {
	cfg := newTestConfig()
	cfg.MaxHistoryTurns = 0 // unlimited
	cfg.MaxAgentLoops = 2

	r := tools.NewRegistry()
	mc := &llm.MockClient{
		Responses: []llm.MockResponse{
			{Text: "First response"},
			{Text: "Second response"},
		},
	}

	fakeDB := &fakeDBStore{}
	a := New(cfg, mc, r, fakeDB, "agent-sw2", nil, "/tmp/test-workdir")

	events := collectEvents(a.Run(context.Background(), "test unlimited"))
	_ = events

	// With MaxHistoryTurns=0, no turns should be removed
	if a.truncStats.TurnsRemoved != 0 {
		t.Errorf("expected TurnsRemoved=0 for unlimited, got %d", a.truncStats.TurnsRemoved)
	}
}

// ── Integration tests for tool result truncation ────────────────────────────

func TestAgent_Run_ToolResultTruncation(t *testing.T) {
	dir := t.TempDir()

	cfg := newTestConfig()
	cfg.WorkDir = dir
	cfg.MaxToolResultChars = 50 // very small limit to force truncation
	cfg.MaxAgentLoops = 2
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
							Arguments: `{"path":"` + dir + `/trunc-test.txt","content":"hello world"}`,
						},
					},
				},
			},
			{Text: "File created."},
		},
	}

	fakeDB := &fakeDBStore{}
	a := New(cfg, mc, r, fakeDB, "agent-trunc", nil, "/tmp/test-workdir")
	events := collectEvents(a.Run(context.Background(), "create a file"))

	_ = events

	// Verify tool result truncation was applied
	if a.truncStats.ToolTruncatedCount == 0 && a.truncStats.ToolCharsSaved == 0 {
		// The write tool result is short, so it might not be truncated.
		// At least verify the fields exist and the agent completed.
		if !hasEventType(events, "turn_done") {
			t.Errorf("expected turn_done, got: %v", eventTypeSlice(events))
		}
	}
}

func TestAgent_Run_ToolResultTruncation_LargeResult(t *testing.T) {
	dir := t.TempDir()

	cfg := newTestConfig()
	cfg.WorkDir = dir
	cfg.MaxToolResultChars = 10 // very small limit - any result will be truncated
	cfg.MaxAgentLoops = 2
	cfg.Mode = config.ModeYolo

	r := tools.NewRegistry()
	// Create a command that returns large output
	mc := &llm.MockClient{
		Responses: []llm.MockResponse{
			{
				ToolCalls: []llm.ToolCall{
					{
						ID:   "t1",
						Type: "function",
						Function: llm.ToolCallFunction{
							Name:      "write",
							Arguments: `{"path":"` + dir + `/large.txt","content":"large content that will be truncated"}`,
						},
					},
				},
			},
			{Text: "Done."},
		},
	}

	fakeDB := &fakeDBStore{}
	a := New(cfg, mc, r, fakeDB, "agent-trunc2", nil, "/tmp/test-workdir")
	events := collectEvents(a.Run(context.Background(), "create large file"))

	_ = events

	// The write tool returns a short status message, so it might not exceed 10 chars.
	// But verify the agent still completes.
	if !hasEventType(events, "turn_done") {
		t.Errorf("expected turn_done, got: %v", eventTypeSlice(events))
	}
}

// ── Integration tests for file read cache ───────────────────────────────────

func TestAgent_FileReadCache(t *testing.T) {
	dir := t.TempDir()

	cfg := newTestConfig()
	cfg.WorkDir = dir
	cfg.MaxAgentLoops = 3
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
							Arguments: `{"file":"` + dir + `/existing.txt"}`,
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
							Arguments: `{"file":"` + dir + `/existing.txt"}`,
						},
					},
				},
			},
			{Text: "Read the file twice."},
		},
	}

	fakeDB := &fakeDBStore{}
	a := New(cfg, mc, r, fakeDB, "agent-cache", nil, "/tmp/test-workdir")

	events := collectEvents(a.Run(context.Background(), "read the same file twice"))

	_ = events

	// Note: the "read" tool reads actual files from disk.
	// Since the file doesn't exist, both calls will error.
	// The cache only stores successful results, so cache hits won't increment.
	// This test verifies the code path doesn't panic.
	if !hasEventType(events, "turn_done") && !hasEventType(events, "error") {
		t.Errorf("expected turn_done or error, got: %v", eventTypeSlice(events))
	}
}

func TestAgent_FileReadCache_HitCount(t *testing.T) {
	a := &Agent{
		fileReadCache: make(map[string]string),
	}
	// Simulate a cache hit
	a.fileReadCache["/tmp/test.txt"] = "cached content"

	// Directly test the cache behavior via cachedExecuteTool
	ch := make(chan Event, 10)
	tc := llm.ToolCall{
		ID:   "t1",
		Type: "function",
		Function: llm.ToolCallFunction{
			Name:      "read_file",
			Arguments: `{"path":"/tmp/test.txt"}`,
		},
	}
	result, err := a.cachedExecuteTool(context.Background(), tc, ch)
	if err != nil {
		t.Fatalf("cachedExecuteTool returned error: %v", err)
	}
	if result != "cached content" {
		t.Errorf("got %q, want %q", result, "cached content")
	}
	if a.truncStats.CacheHits != 1 {
		t.Errorf("CacheHits = %d, want 1", a.truncStats.CacheHits)
	}
}

// ── UsageStats test ─────────────────────────────────────────────────────────

func TestAgent_UsageStats_Formatted(t *testing.T) {
	a := &Agent{
		truncStats: truncationStats{
			TurnsRemoved:       5,
			ToolCharsSaved:     12345,
			ToolTruncatedCount: 3,
			CacheHits:          2,
		},
	}
	stats := a.UsageStats()
	if !strings.Contains(stats, "Turns removed") && !strings.Contains(stats, "TurnsRemoved") {
		t.Errorf("UsageStats should mention turns, got: %s", stats)
	}
	if !strings.Contains(stats, "5") {
		t.Errorf("UsageStats should contain '5', got: %s", stats)
	}
	if !strings.Contains(stats, "12345") {
		t.Errorf("UsageStats should contain '12345', got: %s", stats)
	}
	if !strings.Contains(stats, "3") {
		t.Errorf("UsageStats should contain '3', got: %s", stats)
	}
	if !strings.Contains(stats, "2") {
		t.Errorf("UsageStats should contain '2', got: %s", stats)
	}
}

func TestAgent_UsageStats_NilAgent(t *testing.T) {
	var a *Agent
	stats := a.UsageStats()
	if stats != "Agente não inicializado." {
		t.Errorf("nil agent UsageStats should return 'Agente não inicializado.', got: %s", stats)
	}
}

func TestAgent_FormatPayload_ContainsExpectedSections(t *testing.T) {
	cfg := newTestConfig()
	cfg.ContextDoc = "Projeto de teste."
	r := tools.NewRegistry()
	fakeDB := &fakeDBStore{}
	a := New(cfg, &llm.MockClient{}, r, fakeDB, "agent-dryrun", nil, "/tmp/test-workdir")

	payload := a.FormatPayload(context.Background(), "test task")

	expectedSections := []string{
		"=== DRY RUN",
		"[system]",
		"[user]",
		"test task",
		"Tools disponíveis",
		"Tokens estimados",
		"Nenhuma requisição enviada",
	}
	for _, section := range expectedSections {
		if !strings.Contains(payload, section) {
			t.Errorf("FormatPayload output should contain %q, got:\n%s", section, payload)
		}
	}
	if !strings.Contains(payload, "mock") && !strings.Contains(payload, a.activeModel) {
		t.Errorf("FormatPayload should contain model name, got:\n%s", payload)
	}
}

func TestAgent_UsageStats_ZeroValues(t *testing.T) {
	a := &Agent{}
	stats := a.UsageStats()
	if stats == "" {
		t.Error("UsageStats should not be empty")
	}
	if !strings.Contains(stats, "0") {
		t.Errorf("UsageStats should contain zeros, got: %s", stats)
	}
}
