package orchestrator

import (
	"fmt"
	"sync"
)


// Aggregator coleta resultados de múltiplos AgentWorkers.
type Aggregator struct {
	results   []WorkerResult
	mu        sync.RWMutex
	done      chan struct{}
	wg        sync.WaitGroup
}

// NewAggregator cria nova instância.
func NewAggregator() *Aggregator {
	return &Aggregator{
		results: make([]WorkerResult, 0),
		done:    make(chan struct{}),
	}
}

// Start inicia o listener de resultados.
func (a *Aggregator) Start(resultsCh <-chan WorkerResult, numWorkers int) {
	a.wg.Add(numWorkers)
	go func() {
		defer a.wg.Done()
		for res := range resultsCh {
			a.mu.Lock()
			a.results = append(a.results, res)
			a.mu.Unlock()
		}
	}()
}

// AddResult adiciona um resultado manualmente.
func (a *Aggregator) AddResult(res WorkerResult) {
	a.mu.Lock()
	a.results = append(a.results, res)
	a.mu.Unlock()
}

// Wait aguarda todos os workers concluírem.
func (a *Aggregator) Wait() {
	a.wg.Wait()
	close(a.done)
}

// Results retorna todos os resultados.
func (a *Aggregator) Results() []WorkerResult {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.results
}

// Aggregate agrega os resultados de forma inteligente.
func (a *Aggregator) Aggregate() string {
	a.wg.Wait()

	a.mu.RLock()
	defer a.mu.RUnlock()

	if len(a.results) == 0 {
		// TODO(#74): PT-BR string — unify language to EN for consistency
		return "Nenhum resultado."
	}

	// TODO(#74): PT-BR string "Resultados consolidados" — unify to EN
	var output []string
	for _, res := range a.results {
		if res.Error != nil {
			output = append(output, fmt.Sprintf("[ERROR] Task %s: %v", res.TaskID, res.Error))
		} else {
			output = append(output, fmt.Sprintf("[OK] Task %s: %s", res.TaskID, res.Output))
		}
	}

	return fmt.Sprintf("Resultados consolidados (%d workers):\n\n%s", len(a.results), joinNonEmpty(output, "\n"))
}

// Summarize retorna um resumo conciso.
func (a *Aggregator) Summarize() string {
	total := len(a.results)
	errors := 0
	successes := 0

	for _, r := range a.results {
		if r.Error != nil {
			errors++
		} else {
			successes++
		}
	}

	return fmt.Sprintf("Total：%d tasks — %d OK, %d errors", total, successes, errors)
}

func joinNonEmpty(items []string, sep string) string {
	var result string
	for i, item := range items {
		if item == "" {
			continue
		}
		if i > 0 && result != "" {
			result += sep
		}
		result += item
	}
	return result
}

// Event aggregator para pub/sub
type EventAggregator struct {
	events map[string][]interface{}
	mu     sync.RWMutex
}

func NewEventAggregator() *EventAggregator {
	return &EventAggregator{
		events: make(map[string][]interface{}),
	}
}

func (e *EventAggregator) Add(topic string, event interface{}) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.events[topic] = append(e.events[topic], event)
}

func (e *EventAggregator) Get(topic string) []interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.events[topic]
}

func (e *EventAggregator) Len(topic string) int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.events[topic])
}
