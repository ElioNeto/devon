package orchestrator

import (
	"context"
	"errors"
	"testing"

	"github.com/ElioNeto/devon/internal/llm"
)

// mockLLM implements llm.Streamer for testing.
type mockLLM struct {
	streamFunc func(ctx context.Context, messages []llm.Message, tools []llm.ToolDef) (<-chan llm.StreamEvent, error)
	modelName  string
}

func (m *mockLLM) Stream(ctx context.Context, messages []llm.Message, tools []llm.ToolDef) (<-chan llm.StreamEvent, error) {
	if m.streamFunc != nil {
		return m.streamFunc(ctx, messages, tools)
	}
	ch := make(chan llm.StreamEvent, 1)
	ch <- llm.StreamEvent{Type: "done"}
	close(ch)
	return ch, nil
}

func (m *mockLLM) Info() llm.ModelInfo {
	return llm.ModelInfo{Name: m.modelName}
}

// sendEvents sends events to a channel in a goroutine and returns the channel.
// This avoids deadlocks from insufficient buffer capacity.
func sendEvents(events []llm.StreamEvent) <-chan llm.StreamEvent {
	ch := make(chan llm.StreamEvent, len(events))
	go func() {
		for _, e := range events {
			ch <- e
		}
		close(ch)
	}()
	return ch
}

func TestPlanner_Plan(t *testing.T) {
	mock := &mockLLM{
		modelName: "test-model",
		streamFunc: func(ctx context.Context, messages []llm.Message, tools []llm.ToolDef) (<-chan llm.StreamEvent, error) {
			return sendEvents([]llm.StreamEvent{
				{Type: "text", Text: `{"tasks":[`},
				{Type: "text", Text: `{"id":"1","description":"Research","priority":1},`},
				{Type: "text", Text: `{"id":"2","description":"Implement","depends_on":["1"],"priority":2}]`},
				{Type: "text", Text: `,"root_tasks":[{"id":"1","description":"Research","priority":1}]}`},
				{Type: "done"},
			}), nil
		},
	}

	p := NewPlanner(mock)
	plan, err := p.Plan(context.Background(), "Build a feature", 5)
	if err != nil {
		t.Fatalf("Plan() unexpected error: %v", err)
	}

	if plan == nil {
		t.Fatal("Plan() returned nil plan")
	}

	if len(plan.Tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(plan.Tasks))
	}

	if len(plan.RootTasks) != 1 {
		t.Errorf("expected 1 root task, got %d", len(plan.RootTasks))
	}

	if plan.RootTasks[0].ID != "1" {
		t.Errorf("expected root task ID '1', got %s", plan.RootTasks[0].ID)
	}
}

func TestPlanner_Plan_ErrorStream(t *testing.T) {
	mock := &mockLLM{
		streamFunc: func(ctx context.Context, messages []llm.Message, tools []llm.ToolDef) (<-chan llm.StreamEvent, error) {
			return nil, errors.New("stream error")
		},
	}

	p := NewPlanner(mock)
	_, err := p.Plan(context.Background(), "test", 3)
	if err == nil {
		t.Fatal("expected error from Plan() when stream fails")
	}
}

func TestPlanner_Plan_ErrorEvent(t *testing.T) {
	mock := &mockLLM{
		streamFunc: func(ctx context.Context, messages []llm.Message, tools []llm.ToolDef) (<-chan llm.StreamEvent, error) {
			return sendEvents([]llm.StreamEvent{
				{Type: "error", Err: errors.New("LLM error")},
			}), nil
		},
	}

	p := NewPlanner(mock)
	_, err := p.Plan(context.Background(), "test", 3)
	if err == nil {
		t.Fatal("expected error from Plan() when LLM returns error event")
	}
}

func TestPlanner_Plan_EmptyResponse(t *testing.T) {
	mock := &mockLLM{
		streamFunc: func(ctx context.Context, messages []llm.Message, tools []llm.ToolDef) (<-chan llm.StreamEvent, error) {
			return sendEvents([]llm.StreamEvent{
				{Type: "text", Text: ""},
				{Type: "done"},
			}), nil
		},
	}

	p := NewPlanner(mock)
	plan, err := p.Plan(context.Background(), "test", 3)
	if err != nil {
		t.Logf("Plan with empty text returned error: %v", err)
	}
	if plan == nil {
		t.Log("Plan returned nil (expected for empty response)")
	}
}

func TestPlanner_PlanSimple(t *testing.T) {
	p := NewPlanner(nil) // client not used by PlanSimple
	plan := p.PlanSimple("Do something")

	if plan == nil {
		t.Fatal("PlanSimple() returned nil")
	}

	if len(plan.Tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(plan.Tasks))
	}

	if plan.Tasks[0].Description != "Do something" {
		t.Errorf("expected description 'Do something', got %s", plan.Tasks[0].Description)
	}

	if len(plan.RootTasks) != 1 {
		t.Errorf("expected 1 root task, got %d", len(plan.RootTasks))
	}
}

func TestParsePlan_EmptyInput(t *testing.T) {
	plan, err := parsePlan("", "test")
	if err == nil {
		t.Error("expected error for empty input")
	}
	if plan == nil {
		t.Log("plan is nil as expected")
	}
}

func TestParsePlan_WithLabel(t *testing.T) {
	input := `some text PLANNED_TASKS:{"tasks":[{"id":"1","description":"task1","priority":1}],"root_tasks":[{"id":"1","description":"task1","priority":1}]}`
	plan, err := parsePlan(input, "test")
	if err != nil {
		t.Fatalf("parsePlan() unexpected error: %v", err)
	}
	if plan == nil {
		t.Fatal("parsePlan() returned nil")
	}
	if len(plan.Tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(plan.Tasks))
	}
}

func TestParsePlan_NoRootTasks(t *testing.T) {
	input := `{"tasks":[{"id":"1","description":"root","priority":1},{"id":"2","description":"child","depends_on":["1"],"priority":2}]}`
	plan, err := parsePlan(input, "test")
	if err != nil {
		t.Fatalf("parsePlan() unexpected error: %v", err)
	}
	// Should auto-calculate root tasks
	if len(plan.RootTasks) == 0 {
		t.Error("expected auto-calculated root tasks")
	}
	if len(plan.RootTasks) != 1 || plan.RootTasks[0].ID != "1" {
		t.Errorf("expected root task '1', got %v", plan.RootTasks)
	}
}

func TestParsePlan_InvalidJSON(t *testing.T) {
	input := `{invalid json}`
	plan, err := parsePlan(input, "test")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
	if plan == nil {
		t.Log("plan is nil as expected")
	}
}

func TestFallback(t *testing.T) {
	plan := pFallback("urgent task")
	if plan == nil {
		t.Fatal("pFallback() returned nil")
	}
	if len(plan.Tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(plan.Tasks))
	}
	if plan.Tasks[0].Description != "urgent task" {
		t.Errorf("expected 'urgent task', got '%s'", plan.Tasks[0].Description)
	}
	if plan.Tasks[0].Priority != 1 {
		t.Errorf("expected priority 1, got %d", plan.Tasks[0].Priority)
	}
}
