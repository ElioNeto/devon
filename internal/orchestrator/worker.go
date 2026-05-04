package orchestrator

import (
	"context"
	"fmt"

	"github.com/ElioNeto/devon/internal/db"
	"github.com/ElioNeto/devon/internal/llm"
)

type WorkerConfig struct {
	ID           string
	AgentRole    string
	EnabledTools []string
	DB           db.Store
}

type WorkerResult struct {
	TaskID      string
	Description string
	Output      string
	Error       error
	Cost        float64
	TokensUsed  map[string]int
}

type AgentWorker struct {
	cfg       WorkerConfig
	client    llm.Streamer
	tasksCh   chan Task
	resultsCh chan<- WorkerResult
	cancel    context.CancelFunc
	executed  []Task
}

func NewAgentWorker(
	ctx context.Context,
	cfg WorkerConfig,
	client llm.Streamer,
	tasksCh chan Task,
	resultsCh chan<- WorkerResult,
) *AgentWorker {
	childCtx, cancel := context.WithCancel(ctx)
	// TODO(#74): childCtx is created but never used — should be passed to executeTask for cancellation support
	_ = childCtx

	w := &AgentWorker{
		cfg:       cfg,
		client:    client,
		tasksCh:   tasksCh,
		resultsCh: resultsCh,
		cancel:    cancel,
		executed:  make([]Task, 0),
	}

	return w
}

func (w *AgentWorker) Run() {
	go w.run()
}

func (w *AgentWorker) run() {
	defer close(w.tasksCh)

	for task := range w.tasksCh {
		w.executeTask(task)
	}
}

func (w *AgentWorker) executeTask(task Task) {
	result := WorkerResult{
		TaskID:      task.ID,
		Description: task.Description,
	}

	// Se houver erro, envia sem processar
	if task.Description == "" {
		result.Error = fmt.Errorf("empty task description")
		w.resultsCh <- result
		return
	}

	// TODO(#74): placeholder — replace with real LLM execution like agent.go's runLoop
	result.Output = fmt.Sprintf("Task %s completed by agent %s", task.ID, w.cfg.ID)
	w.executed = append(w.executed, task)

	w.resultsCh <- result
}

// Cancel aborta a execução do worker.
func (w *AgentWorker) Cancel() {
	if w.cancel != nil {
		w.cancel()
	}
}

func (w *AgentWorker) ID() string {
	return w.cfg.ID
}

func (w *AgentWorker) ExecutedTasks() []Task {
	return w.executed
}

const standardSystemPrompt = `You are an AI agent specialized in software engineering tasks.
Help the user complete their tasks efficiently and correctly.`
