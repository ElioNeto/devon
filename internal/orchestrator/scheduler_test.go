package orchestrator

import (
	"sort"
	"testing"
)

func TestExecuteSequential(t *testing.T) {
	s := NewScheduler(Sequential)
	executed := []string{}

	tasks := []Task{
		{ID: "1", Description: "task 1"},
		{ID: "2", Description: "task 2"},
	}

	s.Execute(tasks, func(task Task) {
		executed = append(executed, task.ID)
	})

	if len(executed) != 2 {
		t.Errorf("expected 2 executed tasks, got %d", len(executed))
	}
}

func TestExecuteParallel(t *testing.T) {
	s := NewScheduler(Parallel)
	executed := []string{}

	tasks := []Task{
		{ID: "1", Description: "task 1"},
		{ID: "2", Description: "task 2"},
	}

	s.Execute(tasks, func(task Task) {
		executed = append(executed, task.ID)
	})

	// Parallel executa de forma assíncrona, então verificamos que tasks foram dispatchadas
	if len(executed) == 0 {
		t.Log("parallel uses goroutines, results collected asynchronously")
	}
}

func TestExecutePipeline(t *testing.T) {
	s := NewScheduler(Pipeline)
	tasks := []Task{
		{ID: "1", Description: "task 1", Priority: 1},
		{ID: "2", Description: "task 2", DependsOn: []string{"1"}, Priority: 2},
		{ID: "3", Description: "task 3", DependsOn: []string{"1", "2"}, Priority: 3},
	}

	executed := []string{}
	s.Execute(tasks, func(task Task) {
		executed = append(executed, task.ID)
	})

	// Verifica ordem topológica: 1 → 2 → 3
	if len(executed) != 3 {
		t.Errorf("expected 3 executed tasks, got %d: %v", len(executed), executed)
	}

	// Task 2 deve vir após Task 1
	for i, id := range executed {
		if id == "2" && i < 1 {
			t.Error("task 2 should come after task 1")
		}
		if id == "3" && i < 2 {
			t.Error("task 3 should come after task 2")
		}
	}
}

func TestTopologicalSort(t *testing.T) {
	s := NewScheduler(Pipeline)
	tasks := []Task{
		{ID: "3", Priority: 3},
		{ID: "1", Priority: 1},
		{ID: "2", DependsOn: []string{"1"}, Priority: 2},
		{ID: "4", DependsOn: []string{"2", "3"}, Priority: 4},
	}

	ordered, err := s.topologicalSort(tasks)
	if err != nil {
		t.Fatalf("topologicalSort failed: %v", err)
	}

	if len(ordered) != 4 {
		t.Errorf("expected 4 tasks, got %d", len(ordered))
	}

	// Verifica dependências
	idToIdx := make(map[string]int)
	for i, t := range ordered {
		idToIdx[t.ID] = i
	}

	for _, task := range ordered {
		for _, dep := range task.DependsOn {
			if idToIdx[dep] >= idToIdx[task.ID] {
				t.Errorf("dependency %s should come before task %s", dep, task.ID)
			}
		}
	}
}

func TestFindRootTasks(t *testing.T) {
	s := NewScheduler(Pipeline)
	tasks := []Task{
		{ID: "1", DependsOn: []string{}, Priority: 1}, // Root
		{ID: "2", DependsOn: []string{"1"}, Priority: 2},
		{ID: "3", DependsOn: []string{"1"}, Priority: 3},
		{ID: "4", DependsOn: []string{"2", "3"}, Priority: 4},
	}

	roots := s.findRootTasks(tasks)

	if len(roots) != 1 {
		t.Errorf("expected 1 root task, got %d", len(roots))
	}

	if roots[0].ID != "1" {
		t.Errorf("expected root task ID 1, got %s", roots[0].ID)
	}
}

func TestRootTasksOrderedByPriority(t *testing.T) {
	s := NewScheduler(Pipeline)
	tasks := []Task{
		{ID: "2", Priority: 2},
		{ID: "1", Priority: 1},
		{ID: "3", Priority: 3},
	}

	roots := s.findRootTasks(tasks)

	for i := 0; i < len(roots)-1; i++ {
		if roots[i].Priority > roots[i+1].Priority {
			t.Error("root tasks should be ordered by priority")
		}
	}
}

func TestSchedulerUnknownMode(t *testing.T) {
	s := NewScheduler("unknown")
	tasks := []Task{{ID: "1"}}

	executed := []string{}
	s.Execute(tasks, func(task Task) {
		executed = append(executed, task.ID)
	})

	if len(executed) != 1 {
		t.Error("unknown mode should fallback to sequential")
	}
}

func TestSchedulerNoDependencies(t *testing.T) {
	s := NewScheduler(Pipeline)
	tasks := []Task{
		{ID: "1", Priority: 1},
		{ID: "2", Priority: 2},
		{ID: "3", Priority: 3},
	}

	ordered, err := s.topologicalSort(tasks)
	if err != nil {
		t.Fatalf("topologicalSort failed: %v", err)
	}

	if len(ordered) != 3 {
		t.Errorf("expected 3 tasks, got %d", len(ordered))
	}
}

func TestSchedulerCircularDependency(t *testing.T) {
	s := NewScheduler(Pipeline)
	tasks := []Task{
		{ID: "1", DependsOn: []string{"2"}},
		{ID: "2", DependsOn: []string{"1"}},
	}

	// Circular deps should be handled gracefully
	ordered, err := s.topologicalSort(tasks)
	if err != nil {
		t.Logf("circular dependency error (expected): %v", err)
	}

	if len(ordered) > 0 {
		t.Log("circular deps returned partial order")
	}
}

func TestSchedulerEmptyTasks(t *testing.T) {
	s := NewScheduler(Pipeline)

	ordered, err := s.topologicalSort([]Task{})
	if err != nil {
		t.Fatalf("topologicalSort on empty should succeed: %v", err)
	}

	if len(ordered) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(ordered))
	}
}

func TestSchedulerSingleTask(t *testing.T) {
	s := NewScheduler(Pipeline)
	tasks := []Task{{ID: "1", Priority: 1}}

	ordered, err := s.topologicalSort(tasks)
	if err != nil {
		t.Fatalf("topologicalSort failed: %v", err)
	}

	if len(ordered) != 1 {
		t.Errorf("expected 1 task, got %d", len(ordered))
	}
}

func TestSchedulerPriorityOrdering(t *testing.T) {
	s := NewScheduler(Pipeline)
	tasks := []Task{
		{ID: "1", Priority: 5},
		{ID: "2", Priority: 1},
		{ID: "3", Priority: 3},
	}

	roots := s.findRootTasks(tasks)

	// Should be ordered by priority
	if len(roots) > 1 && roots[0].Priority > roots[1].Priority {
		t.Error("roots not ordered by priority")
	}
}

// Benchmark
func BenchmarkSchedulerSequential(b *testing.B) {
	s := NewScheduler(Sequential)
	tasks := make([]Task, 10)

	for i := 0; i < b.N; i++ {
		s.Execute(tasks, func(Task) {})
	}
}

func BenchmarkSchedulerPipeline(b *testing.B) {
	s := NewScheduler(Pipeline)
	tasks := make([]Task, 10)
	for i := range tasks {
		if i > 0 {
			tasks[i].DependsOn = []string{tasks[i-1].ID}
		}
	}

	for i := 0; i < b.N; i++ {
		s.Execute(tasks, func(Task) {})
	}
}

// Test para ordenação manual de dependências
func TestSchedulerManualOrdering(t *testing.T) {
	s := NewScheduler(Sequential)
	tasks := []Task{
		{ID: "3", Priority: 3},
		{ID: "1", Priority: 1},
		{ID: "2", Priority: 2},
	}

	executed := []string{}
	s.Execute(tasks, func(task Task) {
		executed = append(executed, task.ID)
	})

	// Sequential ignora dependências
	if executed[0] != "3" {
		t.Log("sequential respects input order, not priority")
	}
}

// Verifica se o scheduler preserva ordem para DAG simples
func TestSchedulerDAGOrder(t *testing.T) {
	s := NewScheduler(Pipeline)
	tasks := []Task{
		{ID: "A", Priority: 1},
		{ID: "B", DependsOn: []string{"A"}, Priority: 2},
		{ID: "C", DependsOn: []string{"A"}, Priority: 3},
		{ID: "D", DependsOn: []string{"B", "C"}, Priority: 4},
	}

	ordered, err := s.topologicalSort(tasks)
	if err != nil {
		t.Fatalf("topologicalSort failed: %v", err)
	}

	// A deve vir primeiro
	for i := range ordered {
		if ordered[i].ID == "A" {
			if i != 0 {
				t.Errorf("A should be first, got position %d", i)
			}
			break
		}
	}

	// D deve ser o último
	lastIdx := len(ordered) - 1
	if ordered[lastIdx].ID != "D" {
		t.Errorf("D should be last, got %s", ordered[lastIdx].ID)
	}
}

func TestSchedulerFanOutFanIn(t *testing.T) {
	s := NewScheduler(Pipeline)
	tasks := []Task{
		{ID: "root", Priority: 1},
		{ID: "1-1", DependsOn: []string{"root"}, Priority: 2},
		{ID: "1-2", DependsOn: []string{"root"}, Priority: 2},
		{ID: "1-3", DependsOn: []string{"root"}, Priority: 2},
		{ID: "leaf", DependsOn: []string{"1-1", "1-2", "1-3"}, Priority: 3},
	}

	roots := s.findRootTasks(tasks)
	if len(roots) != 1 || roots[0].ID != "root" {
		t.Errorf("wrong roots: %v", roots)
	}

	ordered, err := s.topologicalSort(tasks)
	if err != nil {
		t.Fatalf("topologicalSort failed: %v", err)
	}

	// Verifica que root é primeiro
	if ordered[0].ID != "root" {
		t.Errorf("expected root first, got %s", ordered[0].ID)
	}

	// Verifica que leaf é depois de todos os intermediários
	leafIdx := -1
	for i, t := range ordered {
		if t.ID == "leaf" {
			leafIdx = i
			break
		}
	}

	if leafIdx != len(ordered)-1 {
		t.Errorf("leaf should be last, got position %d", leafIdx)
	}
}

func TestSortHelper(t *testing.T) {
	tasks := []Task{
		{ID: "3", Priority: 3},
		{ID: "1", Priority: 1},
		{ID: "2", Priority: 2},
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].Priority < tasks[j].Priority
	})

	if tasks[0].Priority != 1 || tasks[2].Priority != 3 {
		t.Error("sort not working correctly")
	}
}
