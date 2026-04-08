package orchestrator

import (
	"context"
	"fmt"
	"sync"

	"github.com/ElioNeto/devon/internal/db"
	"github.com/ElioNeto/devon/internal/llm"
)

// Orchestrator coordena múltiplos AgentWorkers.
type Orchestrator struct {
	client         llm.Streamer
	db             db.Store
	planer         *Planner
	scheduler      *Scheduler
	aggregator     *Aggregator
	workers        []*AgentWorker
	tasks          []Task
	sessionID      string
	ctx            context.Context
	cancel         context.CancelFunc
	mu             sync.RWMutex
}

// New cria novo Orchestrator.
func New(ctx context.Context, client llm.Streamer, mode ExecutionMode, db db.Store) *Orchestrator {
	childCtx, cancel := context.WithCancel(ctx)

	o := &Orchestrator{
		client:     client,
		db:         db,
		planer:     NewPlanner(client),
		scheduler:  NewScheduler(mode),
		aggregator: NewAggregator(),
		ctx:        childCtx,
		cancel:     cancel,
		workers:    make([]*AgentWorker, 0),
	}

	return o
}

// SessionID retorna o ID da sessão atual.
func (o *Orchestrator) SessionID() string {
	return o.sessionID
}

// ProcessTask processa uma task do usuário.
func (o *Orchestrator) ProcessTask(userTask string, numWorkers int) (string, error) {
	// Cria sessão
	o.sessionID = generateSessionID()
	if o.db != nil {
		o.db.CreateSession(o.ctx, o.sessionID)
	}

	// Gera plano
	plan, err := o.planer.Plan(o.ctx, userTask, numWorkers*2)
	if err != nil {
		return "", fmt.Errorf("planning: %w", err)
	}

	o.tasks = plan.Tasks

	// Spawna workers
	resultsCh := make(chan WorkerResult, numWorkers*3)
	for i := 0; i < numWorkers; i++ {
		workerConfig := WorkerConfig{
			ID:      fmt.Sprintf("agent-%d", i+1),
			AgentRole: o.roleForTask(i, plan),
			DB:      o.db,
		}

		worker := NewAgentWorker(o.ctx, workerConfig, o.client, make(chan Task), resultsCh)
		o.workers = append(o.workers, worker)
	}

	// Start aggregator
	o.aggregator.Start(resultsCh, numWorkers)

	// Dispatch tasks via scheduler
	o.scheduler.Execute(o.tasks, o.dispatchTask)

	// Wait completion
	o.aggregator.Wait()

	// Cleanup
	for _, w := range o.workers {
		if w.Cancel != nil {
			w.Cancel()
		}
	}

	aggregateResult := o.aggregator.Aggregate()
	return aggregateResult, nil
}

// dispatchTask envia task para um worker.
func (o *Orchestrator) dispatchTask(task Task) {
	o.mu.RLock()
	workers := o.workers
	o.mu.RUnlock()

	if len(workers) == 0 {
		return
	}

	// Round-robin distribution
	worker := workers[task.Priority%len(workers)]
	worker.tasksCh <- task
}

func (o *Orchestrator) roleForTask(index int, plan *Plan) string {
	roles := []string{
		"Engineer - Implementação",
		"Tester - Validação",
		"Reviewer - Revisão de código",
		"Analyst - Análise e documentação",
	}

	if index < len(roles) {
		return roles[index]
	}
	return "Agent"
}

// Cancel aborta toda a execução.
func (o *Orchestrator) Cancel() {
	o.cancel()
	o.aggregator = nil
}

// Close limpa recursos.
func (o *Orchestrator) Close() error {
	o.Cancel()
	if o.db != nil {
		return o.db.Close()
	}
	return nil
}

func generateSessionID() string {
	// Simplificado - na prática usaria UUID
	return "sess_"
}
