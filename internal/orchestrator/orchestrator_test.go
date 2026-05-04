package orchestrator

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ElioNeto/devon/internal/llm"
)

func TestOrchestrator_New(t *testing.T) {
	ctx := context.Background()
	mock := &mockLLM{modelName: "test"}

	o := New(ctx, mock, Sequential, nil)
	if o == nil {
		t.Fatal("New() returned nil")
	}

	if o.SessionID() != "" {
		t.Errorf("expected empty SessionID initially, got '%s'", o.SessionID())
	}

	if o.client == nil {
		t.Error("client should not be nil")
	}
	if o.planer == nil {
		t.Error("planer should not be nil")
	}
	if o.scheduler == nil {
		t.Error("scheduler should not be nil")
	}
	if o.aggregator == nil {
		t.Error("aggregator should not be nil")
	}
}

func TestOrchestrator_SessionID(t *testing.T) {
	ctx := context.Background()
	mock := &mockLLM{modelName: "test"}
	o := New(ctx, mock, Sequential, nil)

	if o.SessionID() != "" {
		t.Errorf("expected empty session ID, got '%s'", o.SessionID())
	}

	// SessionID is updated after ProcessTask
	_ = o
}

func TestOrchestrator_Cancel(t *testing.T) {
	ctx := context.Background()
	mock := &mockLLM{modelName: "test"}
	o := New(ctx, mock, Sequential, nil)

	// Cancel should not panic
	o.Cancel()

	// After cancel, aggregator should be nil
	if o.aggregator != nil {
		t.Error("expected aggregator to be nil after Cancel")
	}
}

func TestOrchestrator_Close(t *testing.T) {
	ctx := context.Background()
	mock := &mockLLM{modelName: "test"}
	o := New(ctx, mock, Sequential, nil)

	// Close should not panic and return nil (no db)
	err := o.Close()
	if err != nil {
		t.Errorf("Close() unexpected error: %v", err)
	}

	// After Close, aggregator should be nil (Cancel was called)
	if o.aggregator != nil {
		t.Error("expected aggregator to be nil after Close")
	}
}

func TestOrchestrator_ProcessTask_Error(t *testing.T) {
	// Test error path: mock returns stream error so Plan fails
	mock := &mockLLM{
		modelName: "test",
		streamFunc: func(ctx context.Context, messages []llm.Message, tools []llm.ToolDef) (<-chan llm.StreamEvent, error) {
			return nil, errors.New("mock stream error")
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	o := New(ctx, mock, Sequential, nil)

	result, err := o.ProcessTask("test task", 2)
	if err == nil {
		t.Fatal("expected error from ProcessTask when planning fails")
	}

	if !strings.Contains(err.Error(), "planning") {
		t.Errorf("expected error to contain 'planning', got: %v", err)
	}

	if result != "" {
		t.Errorf("expected empty result on error, got '%s'", result)
	}
}

func TestOrchestrator_ProcessTask_PlanErrorEvent(t *testing.T) {
	// Test error path: LLM returns error event during planning
	mock := &mockLLM{
		streamFunc: func(ctx context.Context, messages []llm.Message, tools []llm.ToolDef) (<-chan llm.StreamEvent, error) {
			ch := make(chan llm.StreamEvent, 1)
			ch <- llm.StreamEvent{Type: "error", Err: errors.New("LLM planning error")}
			close(ch)
			return ch, nil
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	o := New(ctx, mock, Sequential, nil)

	result, err := o.ProcessTask("task with plan error", 2)
	if err == nil {
		t.Fatal("expected error from ProcessTask when planning fails with error event")
	}

	if result != "" {
		t.Errorf("expected empty result on error, got '%s'", result)
	}
}

func TestOrchestrator_RoleForTask(t *testing.T) {
	ctx := context.Background()
	mock := &mockLLM{modelName: "test"}
	o := New(ctx, mock, Sequential, nil)

	plan := &Plan{
		Tasks: []Task{
			{ID: "1", Description: "task 1", Priority: 1},
		},
		RootTasks: []Task{
			{ID: "1", Description: "task 1", Priority: 1},
		},
	}

	roles := []string{
		"Engineer",
		"Tester",
		"Reviewer",
		"Analyst",
	}

	for i, expected := range roles {
		role := o.roleForTask(i, plan)
		if !strings.Contains(role, expected) {
			t.Errorf("roleForTask(%d) expected to contain '%s', got '%s'", i, expected, role)
		}
	}

	// Beyond 4 roles, should return "Agent"
	role := o.roleForTask(10, plan)
	if role != "Agent" {
		t.Errorf("roleForTask(10) expected 'Agent', got '%s'", role)
	}
}

func TestOrchestrator_DispatchTask(t *testing.T) {
	ctx := context.Background()
	mock := &mockLLM{modelName: "test"}

	resultsCh := make(chan WorkerResult, 5)
	tasksCh := make(chan Task, 5)

	o := New(ctx, mock, Sequential, nil)

	worker := NewAgentWorker(ctx, WorkerConfig{ID: "test-worker"}, mock, tasksCh, resultsCh)
	worker.Run()

	o.mu.Lock()
	o.workers = append(o.workers, worker)
	o.mu.Unlock()

	// Dispatch a task
	o.dispatchTask(Task{ID: "42", Description: "dispatched task", Priority: 1})

	// Worker should pick it up
	select {
	case res := <-resultsCh:
		if res.TaskID != "42" {
			t.Errorf("expected TaskID '42', got '%s'", res.TaskID)
		}
		if res.Error != nil {
			t.Errorf("unexpected error: %v", res.Error)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for dispatched task result")
	}
}

func TestOrchestrator_DoubleClose(t *testing.T) {
	ctx := context.Background()
	mock := &mockLLM{modelName: "test"}
	o := New(ctx, mock, Sequential, nil)

	// Close twice should not panic
	err1 := o.Close()
	err2 := o.Close()

	if err1 != nil {
		t.Errorf("first Close() error: %v", err1)
	}
	if err2 != nil {
		t.Errorf("second Close() error: %v", err2)
	}
}

func TestGenerateSessionID(t *testing.T) {
	id := generateSessionID()
	if id == "" {
		t.Error("generateSessionID() returned empty string")
	}
	if !strings.HasPrefix(id, "sess_") {
		t.Errorf("generateSessionID() should start with 'sess_', got '%s'", id)
	}
}
