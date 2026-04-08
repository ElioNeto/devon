package llm

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// AtomicClient collects all Deltas from a Provider into a single response,
// with automatic retry on transient errors.
type AtomicClient struct {
	Provider   Provider
	MaxRetries int
	BaseDelay  time.Duration
}

// ChatResponse is the complete response after collecting all deltas.
type ChatResponse struct {
	Text      string
	ToolCalls []ToolCall
	Usage     *Usage
}

func (a *AtomicClient) ensureDefaults() {
	if a.MaxRetries <= 0 {
		a.MaxRetries = 3
	}
	if a.BaseDelay <= 0 {
		a.BaseDelay = 100 * time.Millisecond
	}
}

// Send performs the request and collects all Deltas into a ChatResponse.
// It retries transient errors with exponential backoff.
func (a *AtomicClient) Send(ctx context.Context, messages []Message, tools []ToolDef) (*ChatResponse, error) {
	a.ensureDefaults()

	var lastErr error
	delay := a.BaseDelay

	for attempt := 0; attempt < a.MaxRetries; attempt++ {
		resp, err := a.collectDeltas(ctx, messages, tools)
		if err == nil {
			return resp, nil
		}

		lastErr = err

		if !isRetryable(err) {
			return nil, err
		}

		select {
		case <-time.After(delay):
			delay *= 2
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return nil, lastErr
}

func (a *AtomicClient) collectDeltas(ctx context.Context, messages []Message, tools []ToolDef) (*ChatResponse, error) {
	ch, err := a.Provider.Stream(ctx, messages, tools)
	if err != nil {
		return nil, err
	}

	resp := &ChatResponse{}
	for delta := range ch {
		switch delta.Type {
		case "text":
			resp.Text += delta.Text()
		case "tool_call":
			if delta.Tool != nil {
				resp.ToolCalls = append(resp.ToolCalls, *delta.Tool)
			}
		case "done":
			if delta.Usage != nil {
				resp.Usage = delta.Usage
			}
		case "error":
			if delta.Err != nil {
				return nil, delta.Err
			}
		}
	}

	return resp, nil
}

// isRetryable returns true for HTTP 429, 500, 502, 503 and context/network errors.
// It returns false for 400, 401, 403, 404.
func isRetryable(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	var httpErr *httpStatusError
	if errors.As(err, &httpErr) {
		switch httpErr.StatusCode {
		case http.StatusTooManyRequests, // 429
			http.StatusInternalServerError,   // 500
			http.StatusBadGateway,            // 502
			http.StatusServiceUnavailable,    // 503
			http.StatusGatewayTimeout:        // 504
			return true
		default:
			return false
		}
	}

	// Anthropic error: check for HTTP status code in formatted error
	// e.g., "anthropic: HTTP 429: ..."
	var fmtErr interface{ Unwrap() error }
	if errors.As(err, &fmtErr) {
		return isRetryable(fmtErr.Unwrap())
	}

	// Network errors that are not wrapped with status code
	// (dial error, TLS error, etc.) are retryable
	return true
}
