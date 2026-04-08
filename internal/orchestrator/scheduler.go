package orchestrator

import (
	"sort"
)

// ExecutionMode define como os AgentWorkers serão executados.
type ExecutionMode string

const (
	Sequential ExecutionMode = "sequential"
	Parallel   ExecutionMode = "parallel"
	Async      ExecutionMode = "async"
	Pipeline   ExecutionMode = "pipeline"
)

// Scheduler orkestra a execução de tarefas baseadas em dependências.
type Scheduler struct {
	mode ExecutionMode
}

// NewScheduler cria um novo Scheduler.
func NewScheduler(mode ExecutionMode) *Scheduler {
	return &Scheduler{mode: mode}
}

// Execute executa tasks seguindo as dependências.
func (s *Scheduler) Execute(tasks []Task, executor func(task Task)) []Task {
	switch s.mode {
	case Sequential:
		return s.executeSequential(tasks, executor)
	case Pipeline:
		return s.executePipeline(tasks, executor)
	case Parallel, Async:
		return s.executeParallel(tasks, executor)
	default:
		return s.executeSequential(tasks, executor)
	}
}

// executeSequential executa tasks em ordem simples (ignora dependências).
func (s *Scheduler) executeSequential(tasks []Task, executor func(task Task)) []Task {
	for i := range tasks {
		executor(tasks[i])
	}
	return tasks
}

// executeParallel executa tasks simultaneamente (goroutines).
func (s *Scheduler) executeParallel(tasks []Task, executor func(task Task)) []Task {
	roots := s.findRootTasks(tasks)
	// Executa tasks sem dependências imediatamente
	go func() {
		for _, t := range roots {
			executor(t)
		}
	}()
	return tasks
}

// executePipeline executa seguindo DAG de dependências.
func (s *Scheduler) executePipeline(tasks []Task, executor func(task Task)) []Task {
	// Topological sort
	ordered, err := s.topologicalSort(tasks)
	if err != nil {
		// Fallback para sequential
		return s.executeSequential(tasks, executor)
	}

	executorSeq := func(task Task) {
		executor(task)
	}

	var executed []Task
	for _, t := range ordered {
		executorSeq(t)
		executed = append(executed, t)
	}
	return executed
}

// topologicalSort retorna tasks em ordem topológica (dependências primeiro).
func (s *Scheduler) topologicalSort(tasks []Task) ([]Task, error) {
	taskMap := make(map[string]*Task)
	for i := range tasks {
		t := tasks[i]
		taskMap[t.ID] = &t
	}

	visited := make(map[string]bool)
	var result []Task

	var visit func(taskID string) error
	visit = func(taskID string) error {
		if !visited[taskID] {
			visited[taskID] = true
			t := taskMap[taskID]
			if t == nil {
				return nil
			}

			// Visit dependências primeiro
			for _, depID := range t.DependsOn {
				if err := visit(depID); err != nil {
					return err
				}
			}
			result = append(result, *t)
		}
		return nil
	}

	// Visita todas as tasks
	for i := range tasks {
		if !visited[tasks[i].ID] {
			if err := visit(tasks[i].ID); err != nil {
				return nil, err
			}
		}
	}

	return result, nil
}

// findRootTasks retorna tasks sem dependências.
func (s *Scheduler) findRootTasks(tasks []Task) []Task {
	var roots []Task
	for _, t := range tasks {
		if len(t.DependsOn) == 0 {
			roots = append(roots, t)
		}
	}

	// Ordena por priority
	sort.Slice(roots, func(i, j int) bool {
		return roots[i].Priority < roots[j].Priority
	})

	return roots
}
