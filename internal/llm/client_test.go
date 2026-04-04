package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
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
