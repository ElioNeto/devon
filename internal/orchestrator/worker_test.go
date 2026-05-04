package orchestrator

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ElioNeto/devon/internal/llm"
)

func TestAgentWorker_Run(t *testing.T) {
	resultsCh := make(chan WorkerResult, 5)
	tasksCh := make(chan Task, 5)

	mock := &mockLLM{
		modelName: "test",
		streamFunc: func(ctx context.Context, messages []llm.Message, tools []llm.ToolDef) (<-chan llm.StreamEvent, error) {
			ch := make(chan llm.StreamEvent, 1)
			ch <- llm.StreamEvent{Type: "text", Text: "done"}
			close(ch)
			return ch, nil
		},
	}

	worker := NewAgentWorker(context.Background(), WorkerConfig{
		ID:        "worker-1",
		AgentRole: "Tester",
	}, mock, tasksCh, resultsCh)

	worker.Run()

	// Send a task
	tasksCh <- Task{ID: "1", Description: "test task", Priority: 1}

	// Read result
	select {
	case res := <-resultsCh:
		if res.TaskID != "1" {
			t.Errorf("expected TaskID '1', got '%s'", res.TaskID)
		}
		if res.Error != nil {
			t.Errorf("unexpected error: %v", res.Error)
		}
		if !strings.Contains(res.Output, "Task 1 completed by agent worker-1") {
			t.Errorf("unexpected output: %s", res.Output)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for worker result")
	}

	// Check executed tasks
	executed := worker.ExecutedTasks()
	if len(executed) != 1 {
		t.Errorf("expected 1 executed task, got %d", len(executed))
	}
	if executed[0].ID != "1" {
		t.Errorf("expected executed task ID '1', got '%s'", executed[0].ID)
	}
}

func TestAgentWorker_EmptyTask(t *testing.T) {
	resultsCh := make(chan WorkerResult, 5)
	tasksCh := make(chan Task, 5)

	mock := &mockLLM{modelName: "test"}
	worker := NewAgentWorker(context.Background(), WorkerConfig{ID: "worker-err"}, mock, tasksCh, resultsCh)
	worker.Run()

	// Send empty description task
	tasksCh <- Task{ID: "2", Description: ""}

	select {
	case res := <-resultsCh:
		if res.Error == nil {
			t.Error("expected error for empty task description")
		}
		if !strings.Contains(res.Error.Error(), "empty task description") {
			t.Errorf("unexpected error message: %v", res.Error)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for result from empty task")
	}
}

func TestAgentWorker_ID(t *testing.T) {
	resultsCh := make(chan WorkerResult, 1)
	tasksCh := make(chan Task, 1)

	mock := &mockLLM{modelName: "test"}
	worker := NewAgentWorker(context.Background(), WorkerConfig{ID: "my-worker"}, mock, tasksCh, resultsCh)

	if worker.ID() != "my-worker" {
		t.Errorf("expected ID 'my-worker', got '%s'", worker.ID())
	}
}

func TestAgentWorker_ExecutedTasks(t *testing.T) {
	resultsCh := make(chan WorkerResult, 5)
	tasksCh := make(chan Task, 5)

	mock := &mockLLM{modelName: "test"}
	worker := NewAgentWorker(context.Background(), WorkerConfig{ID: "w1"}, mock, tasksCh, resultsCh)
	worker.Run()

	// No tasks executed yet
	if len(worker.ExecutedTasks()) != 0 {
		t.Errorf("expected 0 executed tasks initially, got %d", len(worker.ExecutedTasks()))
	}

	tasksCh <- Task{ID: "a", Description: "task a"}
	select {
	case <-resultsCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for task a")
	}

	if len(worker.ExecutedTasks()) != 1 {
		t.Errorf("expected 1 executed task, got %d", len(worker.ExecutedTasks()))
	}

	tasksCh <- Task{ID: "b", Description: "task b"}
	select {
	case <-resultsCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for task b")
	}

	if len(worker.ExecutedTasks()) != 2 {
		t.Errorf("expected 2 executed tasks, got %d", len(worker.ExecutedTasks()))
	}

	// Verify order
	executed := worker.ExecutedTasks()
	if executed[0].ID != "a" || executed[1].ID != "b" {
		t.Errorf("unexpected execution order: %+v", executed)
	}
}

func TestAgentWorker_Cancel(t *testing.T) {
	resultsCh := make(chan WorkerResult, 5)
	tasksCh := make(chan Task, 5)

	mock := &mockLLM{modelName: "test"}
	worker := NewAgentWorker(context.Background(), WorkerConfig{ID: "cancel-test"}, mock, tasksCh, resultsCh)
	worker.Run()

	// Cancel before sending tasks — this calls context.CancelFunc but the
	// worker's run() goroutine does not check ctx.Done(), so it keeps running.
	worker.Cancel()

	// Send a task — the worker should still process it since run() doesn't
	// check the context.
	select {
	case tasksCh <- Task{ID: "99", Description: "runs even after cancel"}:
	case <-time.After(time.Second):
		t.Fatal("timeout sending task after cancel")
	}

	// The worker should still execute the task
	select {
	case res := <-resultsCh:
		if res.TaskID != "99" {
			t.Errorf("expected TaskID '99', got '%s'", res.TaskID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for result after cancel")
	}

	// ExecutedTasks should reflect the task
	if len(worker.ExecutedTasks()) != 1 {
		t.Errorf("expected 1 executed task after cancel, got %d", len(worker.ExecutedTasks()))
	}

	// TODO(#74): Cancel should stop the worker's run() goroutine by checking ctx.Done().
	// Currently the context is created but not wired into the task loop.
}
