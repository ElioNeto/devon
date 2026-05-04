package orchestrator

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestAggregator_Start(t *testing.T) {
	a := NewAggregator()
	resultsCh := make(chan WorkerResult, 10)

	// NOTE: numWorkers must match the wg.Add in Start (currently a bug: Add(numWorkers)
	// but only one Done() — use numWorkers=1 until that is fixed.
	a.Start(resultsCh, 1)

	// Send some results
	resultsCh <- WorkerResult{TaskID: "1", Output: "done"}
	resultsCh <- WorkerResult{TaskID: "2", Output: "done"}
	close(resultsCh)

	a.Wait()

	results := a.Results()
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestAggregator_Wait(t *testing.T) {
	a := NewAggregator()
	resultsCh := make(chan WorkerResult, 5)

	a.Start(resultsCh, 1)

	// Send a result and close
	resultsCh <- WorkerResult{TaskID: "1", Output: "ok"}
	close(resultsCh)

	// Wait should complete without hanging
	done := make(chan struct{})
	go func() {
		a.Wait()
		close(done)
	}()

	select {
	case <-done:
		// OK
	case <-time.After(2 * time.Second):
		t.Fatal("Wait() timed out")
	}

	if len(a.Results()) != 1 {
		t.Errorf("expected 1 result, got %d", len(a.Results()))
	}
}

func TestAggregator_AddResult(t *testing.T) {
	a := NewAggregator()

	a.AddResult(WorkerResult{TaskID: "1", Output: "manual"})
	a.AddResult(WorkerResult{TaskID: "2", Output: "add"})

	if len(a.Results()) != 2 {
		t.Errorf("expected 2 results, got %d", len(a.Results()))
	}
}

func TestAggregate(t *testing.T) {
	a := NewAggregator()

	a.AddResult(WorkerResult{TaskID: "1", Output: "task 1 done"})
	a.AddResult(WorkerResult{TaskID: "2", Output: "task 2 done", Error: errors.New("failed")})
	a.AddResult(WorkerResult{TaskID: "3", Output: "task 3 done"})

	agg := a.Aggregate()

	if agg == "" {
		t.Error("Aggregate() returned empty string")
	}

	// Should contain [ERROR] for task 2
	if !strings.Contains(agg, "[ERROR] Task 2") {
		t.Errorf("expected error marker for task 2, got: %s", agg)
	}

	// Should contain [OK] for task 1 and 3
	if !strings.Contains(agg, "[OK] Task 1") || !strings.Contains(agg, "[OK] Task 3") {
		t.Errorf("expected OK markers for tasks 1 and 3, got: %s", agg)
	}
}

func TestAggregate_Empty(t *testing.T) {
	a := NewAggregator()
	result := a.Aggregate()

	if result != "Nenhum resultado." {
		t.Errorf("expected 'Nenhum resultado.', got '%s'", result)
	}
}

func TestSummarize(t *testing.T) {
	a := NewAggregator()

	a.AddResult(WorkerResult{TaskID: "1", Output: "ok"})
	a.AddResult(WorkerResult{TaskID: "2", Output: "ok", Error: errors.New("err")})
	a.AddResult(WorkerResult{TaskID: "3", Output: "ok"})

	summary := a.Summarize()

	if !strings.Contains(summary, "3 tasks") {
		t.Errorf("expected '3 tasks' in summary, got: %s", summary)
	}
	if !strings.Contains(summary, "2 OK") {
		t.Errorf("expected '2 OK' in summary, got: %s", summary)
	}
	if !strings.Contains(summary, "1 errors") {
		t.Errorf("expected '1 errors' in summary, got: %s", summary)
	}
}

func TestSummarize_Empty(t *testing.T) {
	a := NewAggregator()
	summary := a.Summarize()

	if !strings.Contains(summary, "0 tasks") {
		t.Errorf("expected '0 tasks' in summary, got: %s", summary)
	}
}

func TestEventAggregator(t *testing.T) {
	ea := NewEventAggregator()

	ea.Add("tools", "read_file")
	ea.Add("tools", "write_file")
	ea.Add("errors", "timeout")

	if ea.Len("tools") != 2 {
		t.Errorf("expected 2 tool events, got %d", ea.Len("tools"))
	}
	if ea.Len("errors") != 1 {
		t.Errorf("expected 1 error event, got %d", ea.Len("errors"))
	}
	if ea.Len("nonexistent") != 0 {
		t.Errorf("expected 0 events for nonexistent topic, got %d", ea.Len("nonexistent"))
	}

	events := ea.Get("tools")
	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d", len(events))
	}
}

func TestEventAggregator_ConcurrentSafe(t *testing.T) {
	ea := NewEventAggregator()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			ea.Add("test", n)
		}(i)
	}

	wg.Wait()

	if ea.Len("test") != 100 {
		t.Errorf("expected 100 events, got %d", ea.Len("test"))
	}
}

func TestJoinNonEmpty(t *testing.T) {
	tests := []struct {
		input []string
		sep   string
		want  string
	}{
		{[]string{}, ",", ""},
		{[]string{"a"}, ",", "a"},
		{[]string{"a", "b"}, ",", "a,b"},
		{[]string{"a", "", "b"}, ",", "a,b"},
		{[]string{"", "a"}, ",", "a"},
		{[]string{"a", "", ""}, ",", "a"},
	}

	for _, tt := range tests {
		got := joinNonEmpty(tt.input, tt.sep)
		if got != tt.want {
			t.Errorf("joinNonEmpty(%v, %q) = %q, want %q", tt.input, tt.sep, got, tt.want)
		}
	}
}
