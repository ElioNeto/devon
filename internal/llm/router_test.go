package llm

import (
	"context"
	"net/http"
	"sync"
	"testing"
)

// mockStreamProvider returns a pre-built error delta or text delta.
type mockStreamProvider struct {
	name  string
	// errOnStream: if set, Stream returns nil, err immediately
	errOnStream error
	// errDelta: if set, Stream returns a channel that emits this error first
	errDelta error
	text     string
	called   int
}

func (m *mockStreamProvider) Name() string { return m.name }
func (m *mockStreamProvider) Info() ModelInfo {
	return ModelInfo{Name: m.name}
}

func (m *mockStreamProvider) Stream(ctx context.Context, messages []Message, tools []ToolDef) (<-chan Delta, error) {
	m.called++

	if m.errOnStream != nil {
		return nil, m.errOnStream
	}

	ch := make(chan Delta, 16)
	go func() {
		defer close(ch)
		if m.errDelta != nil {
			sendDelta(ctx, ch, ErrorDelta(m.errDelta))
			return
		}
		if m.text != "" {
			sendDelta(ctx, ch, Text(m.text))
		}
		sendDelta(ctx, ch, DoneDelta())
	}()
	return ch, nil
}

func TestRouter_FirstStrategy_FallbackOn429(t *testing.T) {
	p1 := &mockStreamProvider{
		name:        "primary",
		errOnStream: &httpStatusError{StatusCode: http.StatusTooManyRequests, Message: "rate limit"},
	}
	p2 := &mockStreamProvider{
		name: "fallback",
		text: "Hello from p2",
	}

	router := NewRouter([]Provider{p1, p2}, StrategyFirst)
	ch, err := router.Stream(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}

	var text string
	for d := range ch {
		if d.Type == "text" {
			text += d.Text()
		}
	}

	if text != "Hello from p2" {
		t.Errorf("text = %q", text)
	}
	if p1.called != 1 {
		t.Errorf("p1 called %d times, want 1", p1.called)
	}
	if p2.called != 1 {
		t.Errorf("p2 called %d times, want 1", p2.called)
	}
}

func TestRouter_FirstStrategy_ImmediateFailNonRetryable(t *testing.T) {
	p1 := &mockStreamProvider{
		name:        "primary",
		errOnStream: &httpStatusError{StatusCode: http.StatusUnauthorized, Message: "nope"},
	}
	p2 := &mockStreamProvider{
		name: "fallback",
		text: "should not reach",
	}

	router := NewRouter([]Provider{p1, p2}, StrategyFirst)
	_, err := router.Stream(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	// p2 should NOT be called because 401 is non-retryable
	if p2.called != 0 {
		t.Error("p2 should not be called for non-retryable error")
	}
}

func TestRouter_FirstStrategy_AllFail(t *testing.T) {
	p1 := &mockStreamProvider{
		name:        "p1",
		errOnStream: &httpStatusError{StatusCode: http.StatusBadGateway},
	}
	p2 := &mockStreamProvider{
		name:        "p2",
		errOnStream: &httpStatusError{StatusCode: http.StatusServiceUnavailable},
	}

	router := NewRouter([]Provider{p1, p2}, StrategyFirst)
	_, err := router.Stream(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected error when all providers fail")
	}
}

func TestRouter_FirstStrategy_FirstSucceeds(t *testing.T) {
	p1 := &mockStreamProvider{text: "from p1"}
	p2 := &mockStreamProvider{text: "from p2"}

	router := NewRouter([]Provider{p1, p2}, StrategyFirst)
	ch, err := router.Stream(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("Stream error: %v", err)
	}

	var text string
	for d := range ch {
		if d.Type == "text" {
			text += d.Text()
		}
	}
	if text != "from p1" {
		t.Errorf("text = %q, want %q", text, "from p1")
	}
	if p2.called != 0 {
		t.Errorf("p2 called %d times, want 0", p2.called)
	}
}

func TestRouter_RoundRobin_Distributes(t *testing.T) {
	p1 := &mockStreamProvider{
		name: "p1",
		text: "from p1",
	}
	p2 := &mockStreamProvider{
		name: "p2",
		text: "from p2",
	}

	router := NewRouter([]Provider{p1, p2}, StrategyRoundRobin)

	// First call
	ch1, err := router.Stream(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("Stream1 error: %v", err)
	}
	var text1 string
	for d := range ch1 {
		if d.Type == "text" {
			text1 += d.Text()
		}
	}

	// Second call
	ch2, err := router.Stream(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("Stream2 error: %v", err)
	}
	var text2 string
	for d := range ch2 {
		if d.Type == "text" {
			text2 += d.Text()
		}
	}

	// First call should go to p1 (current=0), then current becomes 1
	// Second call goes to p2 (current=1), then current becomes 0
	if text1 != "from p1" {
		t.Errorf("Stream1 text = %q, want %q", text1, "from p1")
	}
	if text2 != "from p2" {
		t.Errorf("Stream2 text = %q, want %q", text2, "from p2")
	}
	if p1.called != 1 || p2.called != 1 {
		t.Errorf("p1=%d, p2=%d, want both 1", p1.called, p2.called)
	}
}

func TestRouter_RoundRobin_FallbackOn429(t *testing.T) {
	// In round-robin mode, first call uses p1 and advances current to 1
	// If p1 returns 429, we try p2
	p1 := &mockStreamProvider{
		name:        "p1",
		errOnStream: &httpStatusError{StatusCode: http.StatusTooManyRequests},
	}
	p2 := &mockStreamProvider{
		name: "p2",
		text: "from p2",
	}

	router := NewRouter([]Provider{p1, p2}, StrategyRoundRobin)
	ch, err := router.Stream(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("Stream error: %v", err)
	}

	var text string
	for d := range ch {
		if d.Type == "text" {
			text += d.Text()
		}
	}

	if text != "from p2" {
		t.Errorf("text = %q", text)
	}
	if p1.called != 1 || p2.called != 1 {
		t.Errorf("p1=%d, p2=%d", p1.called, p2.called)
	}
}

func TestRouter_Name(t *testing.T) {
	r := NewRouter(nil, StrategyFirst)
	if r.Name() != "router" {
		t.Errorf("Name() = %q", r.Name())
	}
}

func TestRouter_ConcurrentSafety(t *testing.T) {
	p1 := &mockStreamProvider{name: "p1", text: "p1"}
	p2 := &mockStreamProvider{name: "p2", text: "p2"}
	router := NewRouter([]Provider{p1, p2}, StrategyRoundRobin)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch, err := router.Stream(context.Background(), nil, nil)
			if err != nil {
				t.Errorf("Stream error: %v", err)
				return
			}
			for range ch {
			}
		}()
	}
	wg.Wait()

	if p1.called+p2.called != 10 {
		t.Errorf("total calls = %d, want 10", p1.called+p2.called)
	}
}
