package llm

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/ElioNeto/devon/internal/config"
)

// mockStreamProvider returns a pre-built error delta or text delta.
type mockStreamProvider struct {
	name        string
	errOnStream error
	errDelta    error
	text        string
	called      atomic.Int32
}

func (m *mockStreamProvider) Name() string { return m.name }
func (m *mockStreamProvider) Info() ModelInfo {
	return ModelInfo{Name: m.name}
}

func (m *mockStreamProvider) Stream(ctx context.Context, messages []Message, tools []ToolDef) (<-chan Delta, error) {
	m.called.Add(1)

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
	if p1.called.Load() != 1 {
		t.Errorf("p1 called %d times, want 1", p1.called.Load())
	}
	if p2.called.Load() != 1 {
		t.Errorf("p2 called %d times, want 1", p2.called.Load())
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
	if p2.called.Load() != 0 {
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
	if p2.called.Load() != 0 {
		t.Errorf("p2 called %d times, want 0", p2.called.Load())
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

	if text1 != "from p1" {
		t.Errorf("Stream1 text = %q, want %q", text1, "from p1")
	}
	if text2 != "from p2" {
		t.Errorf("Stream2 text = %q, want %q", text2, "from p2")
	}
	if p1.called.Load() != 1 || p2.called.Load() != 1 {
		t.Errorf("p1=%d, p2=%d, want both 1", p1.called.Load(), p2.called.Load())
	}
}

func TestRouter_RoundRobin_FallbackOn429(t *testing.T) {
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
	if p1.called.Load() != 1 || p2.called.Load() != 1 {
		t.Errorf("p1=%d, p2=%d", p1.called.Load(), p2.called.Load())
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

	if p1.called.Load()+p2.called.Load() != 10 {
		t.Errorf("total calls = %d, want 10", p1.called.Load()+p2.called.Load())
	}
}

// ── AgentRouter tests ─────────────────────────────────────────────────────────

// mockStreamer implements Streamer for AgentRouter tests.
type mockStreamer struct {
	name string
}

func (m *mockStreamer) Stream(ctx context.Context, msgs []Message, tools []ToolDef) (<-chan StreamEvent, error) {
	ch := make(chan StreamEvent, 1)
	ch <- StreamEvent{Type: "done"}
	close(ch)
	return ch, nil
}

func (m *mockStreamer) Info() ModelInfo {
	return ModelInfo{Name: m.name}
}

func TestAgentRouter_ClientFor_MappedType(t *testing.T) {
	routing := map[config.TaskType]*config.Profile{
		config.TaskTypeExplore: {Name: "fast", BaseURL: "http://localhost:11434/v1", Model: "explore-model"},
		config.TaskTypeCode:    {Name: "fast", BaseURL: "http://localhost:11434/v1", Model: "code-model"},
	}
	defaultClient := &mockStreamer{name: "default"}
	router := NewAgentRouter(routing, defaultClient)

	exploreClient := router.ClientFor(config.TaskTypeExplore)
	if exploreClient == nil {
		t.Fatal("ClientFor(explore) returned nil")
	}
	if exploreClient.Info().Name != "explore-model" {
		t.Errorf("explore client model = %q, want %q", exploreClient.Info().Name, "explore-model")
	}

	codeClient := router.ClientFor(config.TaskTypeCode)
	if codeClient == nil {
		t.Fatal("ClientFor(code) returned nil")
	}
	if codeClient.Info().Name != "code-model" {
		t.Errorf("code client model = %q, want %q", codeClient.Info().Name, "code-model")
	}
}

func TestAgentRouter_ClientFor_UnmappedType(t *testing.T) {
	routing := map[config.TaskType]*config.Profile{
		config.TaskTypeExplore: {Name: "fast", BaseURL: "http://localhost:11434/v1", Model: "explore-model"},
	}
	defaultClient := &mockStreamer{name: "default"}
	router := NewAgentRouter(routing, defaultClient)

	// Plan is not in the routing map
	planClient := router.ClientFor(config.TaskTypePlan)
	if planClient == nil {
		t.Fatal("ClientFor(plan) returned nil")
	}
	if planClient.Info().Name != "default" {
		t.Errorf("plan client model = %q, want %q", planClient.Info().Name, "default")
	}
}

func TestAgentRouter_ClientFor_NilRouting(t *testing.T) {
	defaultClient := &mockStreamer{name: "default"}
	router := NewAgentRouter(nil, defaultClient)

	// Nil routing should always return default
	for _, tt := range []config.TaskType{config.TaskTypeExplore, config.TaskTypePlan, config.TaskTypeCode} {
		client := router.ClientFor(tt)
		if client == nil {
			t.Fatalf("ClientFor(%v) returned nil", tt)
		}
		if client.Info().Name != "default" {
			t.Errorf("ClientFor(%v) model = %q, want %q", tt, client.Info().Name, "default")
		}
	}
}

func TestAgentRouter_ClientFor_EmptyRouting(t *testing.T) {
	defaultClient := &mockStreamer{name: "default"}
	router := NewAgentRouter(map[config.TaskType]*config.Profile{}, defaultClient)

	// Empty routing should return default for all types
	for _, tt := range []config.TaskType{config.TaskTypeExplore, config.TaskTypePlan, config.TaskTypeCode} {
		client := router.ClientFor(tt)
		if client == nil {
			t.Fatalf("ClientFor(%v) returned nil", tt)
		}
		if client.Info().Name != "default" {
			t.Errorf("ClientFor(%v) model = %q, want %q", tt, client.Info().Name, "default")
		}
	}
}

func TestAgentRouter_NilRouter(t *testing.T) {
	var router *AgentRouter
	client := router.ClientFor(config.TaskTypeCode)
	if client != nil {
		t.Error("ClientFor on nil router should return nil")
	}
}

func TestAgentRouter_ModelFor_Mapped(t *testing.T) {
	routing := map[config.TaskType]*config.Profile{
		config.TaskTypeExplore: {Name: "fast", BaseURL: "http://localhost:11434/v1", Model: "explore-model"},
	}
	defaultClient := &mockStreamer{name: "default"}
	router := NewAgentRouter(routing, defaultClient)

	model := router.ModelFor(config.TaskTypeExplore)
	if model != "explore-model" {
		t.Errorf("ModelFor(explore) = %q, want %q", model, "explore-model")
	}

	// Unmapped type returns default model
	model = router.ModelFor(config.TaskTypePlan)
	if model != "default" {
		t.Errorf("ModelFor(plan) = %q, want %q", model, "default")
	}
}

func TestAgentRouter_ModelFor_NilRouter(t *testing.T) {
	var router *AgentRouter
	model := router.ModelFor(config.TaskTypeCode)
	if model != "" {
		t.Errorf("ModelFor on nil router = %q, want empty", model)
	}
}

func TestAgentRouter_NewAgentRouter_NilProfile(t *testing.T) {
	routing := map[config.TaskType]*config.Profile{
		config.TaskTypeExplore: nil, // should be skipped
	}
	defaultClient := &mockStreamer{name: "default"}
	router := NewAgentRouter(routing, defaultClient)

	client := router.ClientFor(config.TaskTypeExplore)
	if client != defaultClient {
		t.Error("nil profile should result in default client")
	}
}

// config alias for tests — avoid import cycle by using the real import in test
// Since this is a _test.go file in the llm package, the import is fine.
