package llm

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewOpenAIProvider(t *testing.T) {
	cfg := ProviderConfig{
		Name:    "gpt-4",
		APIKey:  "sk-test",
		BaseURL: "https://api.openai.com/v1",
		Model:   "gpt-4",
		Timeout: 30 * time.Second,
	}
	p := NewOpenAIProvider(cfg.BaseURL, cfg.Model, cfg)

	if p.Name() != "gpt-4" {
		t.Errorf("Name() = %q", p.Name())
	}
	if p.Info().Provider != "openai" {
		t.Errorf("Info().Provider = %q", p.Info().Provider)
	}
	if !p.Info().SupportsTools {
		t.Error("expected SupportsTools=true")
	}
	if !p.Info().SupportsVision {
		t.Error("expected SupportsVision=true")
	}
}

func TestOpenAIProvider_Info(t *testing.T) {
	p := NewOpenAIProvider("http://example.com/v1", "model", ProviderConfig{Name: "test"})
	info := p.Info()

	if info.Name != "test" {
		t.Errorf("Info().Name = %q", info.Name)
	}
	if info.Provider != "openai" {
		t.Errorf("Info().Provider = %q", info.Provider)
	}
}

func TestOpenAIProvider_Stream_TextSSE(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); !strings.HasPrefix(got, "Bearer ") {
			t.Errorf("Authorization header = %q", got)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		events := []string{
			`{"id":"1","choices":[{"delta":{"content":"Hi"},"index":0}]}`,
			`{"id":"1","choices":[{"delta":{"content":" there"},"index":0}]}`,
			`{"id":"1","choices":[{"finish_reason":"stop","index":0}],"usage":{"prompt_tokens":3,"completion_tokens":8}}`,
		}
		for _, e := range events {
			_, _ = io.WriteString(w, "data: "+e+"\n\n")
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}))
	defer server.Close()

	p := NewOpenAIProvider(server.URL, "gpt-4", ProviderConfig{
		Name: "gpt-4", APIKey: "sk-test", Timeout: 5 * time.Second,
	})

	ch, err := p.Stream(context.Background(), []Message{{Role: RoleUser, Content: strPtr("hello")}}, nil)
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}

	var text strings.Builder
	for d := range ch {
		if d.Type == "text" {
			text.WriteString(d.Text())
		}
	}
	if text.String() != "Hi there" {
		t.Errorf("text = %q", text.String())
	}
}

func TestOpenAIProvider_Stream_ToolCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		events := []string{
			`{"id":"1","choices":[{"delta":{"tool_calls":[{"id":"tc1","type":"function","function":{"name":"bash","arguments":"{\"command\":\"echo hi\"}"}}]},"index":0}]}`,
			`{"id":"1","choices":[{"finish_reason":"stop","index":0}]}`,
		}
		for _, e := range events {
			_, _ = io.WriteString(w, "data: "+e+"\n\n")
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}))
	defer server.Close()

	p := NewOpenAIProvider(server.URL, "gpt-4", ProviderConfig{
		Name: "gpt-4", APIKey: "sk-test", Timeout: 5 * time.Second,
	})

	ch, err := p.Stream(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}

	var tools []ToolCall
	for d := range ch {
		if d.Type == "tool_call" && d.Tool != nil {
			tools = append(tools, *d.Tool)
		}
	}
	if len(tools) == 0 {
		t.Error("expected at least one tool call")
	}
	if tools[0].Function.Name != "bash" {
		t.Errorf("tool name = %q", tools[0].Function.Name)
	}
}

func TestOpenAIProvider_Stream_BadRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	p := NewOpenAIProvider(server.URL, "model", ProviderConfig{Name: "test", Timeout: time.Second})
	_, err := p.Stream(context.Background(), nil, nil)
	if err == nil {
		t.Error("expected error for 400")
	}
}

func TestOpenAIProvider_Stream_InvalidURL(t *testing.T) {
	p := NewOpenAIProvider("://invalid", "model", ProviderConfig{Name: "test", APIKey: "k", Timeout: time.Second})
	_, err := p.Stream(context.Background(), nil, nil)
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}
