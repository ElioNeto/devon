package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestNew_TrimsTrailingSlash(t *testing.T) {
	c := New("key", "http://example.com/", "model", 10*time.Second)
	if c.baseURL != "http://example.com" {
		t.Errorf("baseURL = %q, want %q", c.baseURL, "http://example.com")
	}
}

func TestNew_SetsTimeout(t *testing.T) {
	c := New("key", "http://example.com", "model", 5*time.Second)
	if c.http.Timeout != 5*time.Second {
		t.Errorf("http timeout = %v, want %v", c.http.Timeout, 5*time.Second)
	}
}

func TestNew_NoAuthHeader(t *testing.T) {
	c := New("", "http://localhost:11434/v1", "model", 10*time.Second)
	if c.apiKey != "" {
		t.Errorf("apiKey = %q, want empty", c.apiKey)
	}
}

func TestStream_HTTPError(t *testing.T) {
	c := New("key", "http://127.0.0.1:19999", "model", 1*time.Second)
	_, err := c.Stream(context.Background(), []Message{}, nil)
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

func TestStream_HTTP4xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer key" {
			t.Errorf("missing auth header")
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := New("key", srv.URL, "model", 5*time.Second)
	_, err := c.Stream(context.Background(), []Message{
		{Role: RoleUser, Content: "hi"},
	}, nil)
	if err == nil {
		t.Fatal("expected HTTP 4xx error")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error should contain 401: %v", err)
	}
}

func TestStream_ResponseOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("wrong content type: %s", r.Header.Get("Content-Type"))
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: {\"choices\":[],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":1,\"total_tokens\":2}}\n\ndata: [DONE]\n\n"))
	}))
	defer srv.Close()

	c := New("key", srv.URL, "model", 5*time.Second)
	ch, err := c.Stream(context.Background(), []Message{
		{Role: RoleUser, Content: "hi"},
	}, nil)
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}

	var events []StreamEvent
	for e := range ch {
		events = append(events, e)
	}

	if len(events) == 0 {
		t.Fatal("expected at least 1 event")
	}
	if events[len(events)-1].Type != "done" {
		t.Errorf("last event not 'done': %q", events[len(events)-1].Type)
	}
}

func TestStream_TextEvents(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n"))
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\" world\"}}]}\n"))
		w.Write([]byte("data: [DONE]\n"))
	}))
	defer srv.Close()

	c := New("", srv.URL, "model", 5*time.Second)
	ch, err := c.Stream(context.Background(), []Message{}, nil)
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}

	var texts []string
	for e := range ch {
		if e.Type == "text" {
			texts = append(texts, e.Text)
		}
	}

	if len(texts) != 2 {
		t.Fatalf("expected 2 text events, got %d", len(texts))
	}
	if texts[0] != "hello" {
		t.Errorf("text[0] = %q, want %q", texts[0], "hello")
	}
	if texts[1] != " world" {
		t.Errorf("text[1] = %q, want %q", texts[1], " world")
	}
}

func TestStream_DoneWithUsage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("data: {\"choices\":[],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":5,\"total_tokens\":15}}\n\n"))
	}))
	defer srv.Close()

	c := New("", srv.URL, "model", 5*time.Second)
	ch, err := c.Stream(context.Background(), []Message{}, nil)
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}

	var doneEvent *StreamEvent
	for e := range ch {
		if e.Type == "done" {
			doneEvent = &e
			break
		}
	}

	if doneEvent == nil {
		t.Fatal("no 'done' event received")
	}
	if doneEvent.Usage == nil {
		t.Fatal("done event has no Usage")
	}
	if doneEvent.Usage.TotalTokens != 15 {
		t.Errorf("TotalTokens = %d, want 15", doneEvent.Usage.TotalTokens)
	}
}

func TestStream_ToolCalls(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call1\",\"function\":{\"name\":\"bash\",\"arguments\":\"{\\\"command\\\":\\\"echo hi\\\"}\"}}]}}]}\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()

	c := New("", srv.URL, "model", 5*time.Second)
	ch, err := c.Stream(context.Background(), []Message{}, nil)
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}

	var toolEvent *StreamEvent
	for e := range ch {
		if e.Type == "tool_call" {
			toolEvent = &e
		}
	}

	if toolEvent == nil || toolEvent.Tool == nil {
		t.Fatal("expected tool_call event")
	}
	if toolEvent.Tool.Function.Name != "bash" {
		t.Errorf("tool name = %q, want %q", toolEvent.Tool.Function.Name, "bash")
	}
}

func TestStream_ContextCancelledDuringResponse(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}
		// Cancel while streaming
		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"first\"}}]}\n\n"))
		flusher.Flush()
		time.Sleep(50 * time.Millisecond)
	}))
	defer srv.Close()

	c := New("", srv.URL, "model", 5*time.Second)
	ch, err := c.Stream(ctx, []Message{}, nil)
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}
	// Drain channel — should close without panic
	for range ch {
	}
}

func TestStream_NoAuthHeader(t *testing.T) {
	received := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = true
		if r.Header.Get("Authorization") != "" {
			t.Error("expected no auth header when API key is empty")
		}
		w.Write([]byte("data: {\"choices\":[],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":1,\"total_tokens\":2}}\n\ndata: [DONE]\n\n"))
	}))
	defer srv.Close()

	c := New("", srv.URL, "model", 5*time.Second)
	ch, err := c.Stream(context.Background(), []Message{}, nil)
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}
	for range ch {
	}
	if !received {
		t.Error("request was not sent")
	}
}

// ------------------------------------------------------------------
//  Retry & backoff tests
// ------------------------------------------------------------------

func TestRetryDelay_ExponentialBackoff(t *testing.T) {
	delay0 := retryDelay(nil, 0, 20*time.Millisecond, 200*time.Millisecond)
	delay1 := retryDelay(nil, 1, 20*time.Millisecond, 200*time.Millisecond)
	delay3 := retryDelay(nil, 3, 20*time.Millisecond, 200*time.Millisecond)

	if delay0 < 20*time.Millisecond {
		t.Errorf("delay[0] = %v, want >= 20ms", delay0)
	}
	if delay1 < delay0 {
		t.Errorf("delay[1] = %v, want >= delay[0] = %v", delay1, delay0)
	}
	if delay3 > 200*time.Millisecond {
		t.Errorf("delay[3] = %v, want <= 200ms cap", delay3)
	}
}

func TestRetryDelay_RetryAfterHeader(t *testing.T) {
	resp := &http.Response{Header: make(http.Header)}
	resp.Header.Set("Retry-After", "10")

	delay := retryDelay(resp, 0, 5*time.Second, 60*time.Second)

	if delay < 10*time.Second {
		t.Errorf("delay = %v, want >= 10s", delay)
	}
}

func TestClient_Stream_429ThenSuccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping retry test in short mode")
	}
	var callCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)
		if n <= 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":{"message":"rate limited"}}`))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n"))
		w.Write([]byte("data: [DONE]\n"))
	}))
	defer srv.Close()

	client := New("test-key", srv.URL, "test-model", 10*time.Second)

	ch, err := client.Stream(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil)
	if err != nil {
		t.Fatalf("Stream returned error: %v", err)
	}

	var texts []string
	for ev := range ch {
		if ev.Type == "text" {
			texts = append(texts, ev.Text)
		}
	}
	if len(texts) != 1 || texts[0] != "hello" {
		t.Errorf("texts = %v, want [hello]", texts)
	}
	if n := callCount.Load(); n != 2 {
		t.Errorf("callCount = %d, want 2", n)
	}
}

func TestClient_Stream_Persistent429(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	var callCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":{"message":"rate limited"}}`))
	}))
	defer srv.Close()

	client := New("test-key", srv.URL, "test-model", 5*time.Second)

	_, err := client.Stream(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil)
	if err == nil {
		t.Fatal("expected error after persistent 429")
	}
	t.Logf("error (expected): %v", err)
}

func TestClient_Stream_503ThenSuccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping retry test in short mode")
	}
	var callCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)
		if n <= 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"error":{"message":"upstream error"}}`))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n"))
		w.Write([]byte("data: [DONE]\n"))
	}))
	defer srv.Close()

	client := New("test-key", srv.URL, "test-model", 10*time.Second)

	ch, err := client.Stream(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil)
	if err != nil {
		t.Fatalf("Stream returned error: %v", err)
	}

	var texts []string
	for ev := range ch {
		if ev.Type == "text" {
			texts = append(texts, ev.Text)
		}
	}
	if len(texts) != 1 || texts[0] != "ok" {
		t.Errorf("texts = %v, want [ok]", texts)
	}
	if n := callCount.Load(); n != 2 {
		t.Errorf("callCount = %d, want 2", n)
	}
}

func TestClient_Stream_401ImmediateFail(t *testing.T) {
	var callCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"message":"invalid api key"}}`))
	}))
	defer srv.Close()

	client := New("test-key", srv.URL, "test-model", 5*time.Second)

	_, err := client.Stream(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil)
	if err == nil {
		t.Fatal("expected error on 401")
	}
	if callCount.Load() != 1 {
		t.Errorf("callCount = %d, want 1 (no retry on 401)", callCount.Load())
	}
	if !strings.Contains(err.Error(), "401") && !strings.Contains(err.Error(), "invalid api key") {
		t.Errorf("error = %q, want 401 or 'invalid api key'", err.Error())
	}
}

func TestExtractErrorMessage(t *testing.T) {
	t.Run("metadata raw takes precedence", func(t *testing.T) {
		body := []byte(`{"error":{"message":"outer","metadata":{"raw":"inner raw"}}}`)
		got := extractErrorMessage(body)
		if got != "inner raw" {
			t.Errorf("got %q, want 'inner raw'", got)
		}
	})

	t.Run("falls back to error.message", func(t *testing.T) {
		body := []byte(`{"error":{"message":"rate limited by upstream"}}`)
		got := extractErrorMessage(body)
		if got != "rate limited by upstream" {
			t.Errorf("got %q, want 'rate limited by upstream'", got)
		}
	})

	t.Run("falls back to raw body string", func(t *testing.T) {
		body := []byte(`plain error text`)
		got := extractErrorMessage(body)
		if got != "plain error text" {
			t.Errorf("got %q, want 'plain error text'", got)
		}
	})

	t.Run("empty body returns empty", func(t *testing.T) {
		got := extractErrorMessage(nil)
		if got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}

func TestIsRetryableStatus(t *testing.T) {
	tests := []struct {
		code int
		want bool
	}{
		{400, false},
		{401, false},
		{403, false},
		{404, false},
		{429, true},
		{500, true},
		{502, true},
		{503, true},
		{504, true},
	}
	for _, tc := range tests {
		got := isRetryableStatus(tc.code)
		if got != tc.want {
			t.Errorf("isRetryableStatus(%d) = %v, want %v", tc.code, got, tc.want)
		}
	}
}

func TestHTTPStatusError(t *testing.T) {
	resp := &http.Response{StatusCode: 429}
	err := &httpStatusError{
		StatusCode: 429,
		Message:    "rate limited",
		Response:   resp,
	}
	s := err.Error()
	if !strings.Contains(s, "429") {
		t.Errorf("error string = %q, want '429'", s)
	}
	if !strings.Contains(s, "rate limited") {
		t.Errorf("error string = %q, want 'rate limited'", s)
	}
}

func TestClient_Stream_ContextCancelledDuringRetry(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	var callCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.Header().Set("Retry-After", "10")
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":{"message":"rate limited"}}`))
	}))
	defer srv.Close()

	client := New("test-key", srv.URL, "test-model", 10*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := client.Stream(ctx, []Message{{Role: RoleUser, Content: "hi"}}, nil)
	if err == nil {
		t.Fatal("expected error from context cancellation")
	}
}

func TestClient_doRequest_BodyAndHeaders(t *testing.T) {
	var capturedCT string
	var capturedBody []byte
	var capturedAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCT = r.Header.Get("Content-Type")
		capturedAuth = r.Header.Get("Authorization")
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: [DONE]\n"))
	}))
	defer srv.Close()

	client := New("my-key", srv.URL, "test", 5*time.Second)
	body, _ := json.Marshal(ChatRequest{
		Model:    "test",
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
		Stream:   true,
	})

	resp, err := client.doRequest(context.Background(), body)
	if err != nil {
		t.Fatalf("doRequest error: %v", err)
	}
	defer resp.Body.Close()

	if capturedCT != "application/json" {
		t.Errorf("Content-Type = %q, want 'application/json'", capturedCT)
	}
	if capturedAuth != "Bearer my-key" {
		t.Errorf("Authorization = %q, want 'Bearer my-key'", capturedAuth)
	}
	if !strings.Contains(string(capturedBody), "test") {
		t.Errorf("body = %q, want to contain 'test'", string(capturedBody))
	}
}

func TestClient_Stream_5xxMaxRetries(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	var callCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprintf(w, `{"error":{"message":"bad gateway"}}`)
	}))
	defer srv.Close()

	client := New("test-key", srv.URL, "test-model", 5*time.Second)

	_, err := client.Stream(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil)
	if err == nil {
		t.Fatal("expected error after max 5xx retries")
	}
	t.Logf("error (expected): %v after %d calls", err, callCount.Load())
}
