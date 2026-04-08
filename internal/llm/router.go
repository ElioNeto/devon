package llm

import (
	"context"
	"sync"
)

// RouterStrategy defines how the Router selects providers.
type RouterStrategy string

const (
	// StrategyFirst tries providers in order, falling back on 429/5xx errors.
	StrategyFirst RouterStrategy = "first"
	// StrategyRoundRobin distributes calls evenly among providers.
	StrategyRoundRobin RouterStrategy = "round_robin"
)

// Router delegates Stream to one of several providers with fallback logic.
type Router struct {
	providers []Provider
	strategy  RouterStrategy
	mu        sync.Mutex
	current   int
}

// NewRouter creates a Router with the given providers and strategy.
func NewRouter(providers []Provider, strategy RouterStrategy) *Router {
	return &Router{
		providers: providers,
		strategy:  strategy,
	}
}

func (r *Router) Name() string    { return "router" }
func (r *Router) Info() ModelInfo { return ModelInfo{Name: "router"} }

// Stream delegates to a provider based on the strategy.
// In "first" mode it tries providers in order, falling back on 429/5xx.
// In "round_robin" mode it picks the next provider, falling back on 429/5xx.
// Returns error only if all providers fail.
func (r *Router) Stream(ctx context.Context, messages []Message, tools []ToolDef) (<-chan Delta, error) {
	r.mu.Lock()
	start := r.current
	if r.strategy == StrategyRoundRobin {
		r.current = (r.current + 1) % len(r.providers)
	}
	r.mu.Unlock()

	n := len(r.providers)
	var lastErr error

	for i := 0; i < n; i++ {
		idx := (start + i) % n
		p := r.providers[idx]

		ch, err := p.Stream(ctx, messages, tools)
		if err != nil {
			lastErr = err
			if !isRetryable(err) {
				return nil, err
			}
			continue
		}

		// Check for error delta in the first message from channel
		deltaCh := make(chan Delta, 32)
		go func(providerCh <-chan Delta, fallback bool) {
			defer close(deltaCh)
			for d := range providerCh {
				if d.Type == "error" && d.Err != nil {
					lastErr = d.Err
					if !isRetryable(d.Err) {
						// Non-retryable error — don't try to forward
						return
					}
					if fallback {
						// Skip this provider, don't emit error delta upstream
						continue
					}
				}
				select {
				case deltaCh <- d:
				case <-ctx.Done():
					return
				}
			}
		}(ch, true)

		return deltaCh, nil
	}

	return nil, lastErr
}
