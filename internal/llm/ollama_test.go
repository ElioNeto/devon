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

func TestNewOllamaProvider(t *testing.T) {
	cfg := ProviderConfig{
		Name:    "qwen2.5",
		APIKey:  "",
		BaseURL: "http://localhost:11434/v1",
		Model:   "qwen2.5-coder:32b",
		Timeout: 10 * time.Second,
	}
	p := NewOllamaProvider(cfg.BaseURL, cfg.Model, cfg)

	if p.Name() != "qwen2.5" {
		t.Errorf("Name() = %q", p.Name())
	}
	if p.Info().Provider != "ollama" {
		t.Errorf("Info().Provider = %q", p.Info().Provider)
	}
	if !p.Info().SupportsTools {
		t.Error("expected SupportsTools=true")
	}
	if p.Info().SupportsVision {
		t.Error("expected SupportsVision=false for ollama")
	}
}

func TestOllamaProvider_Info(t *testing.T) {
	p := NewOllamaProvider("http://localhost:11434/v1", "model", ProviderConfig{Name: "test"})
	info := p.Info()

	if info.Name != "test" {
		t.Errorf("Info().Name = %q", info.Name)
	}
	if info.InputCost != 0 {
		t.Errorf("Info().InputCost = %f, want 0", info.InputCost)
	}
	if info.OutputCost != 0 {
		t.Errorf("Info().OutputCost = %f, want 0", info.OutputCost)
	}
}

func TestOllamaProvider_Stream_TextSSE(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		events := []string{
			`{"id":"1","choices":[{"delta":{"content":"Hello"},"index":0}]}`,
			`{"id":"1","choices":[{"delta":{"content":", world"},"index":0}]}`,
			`{"id":"1","choices":[{"finish_reason":"stop","index":0}],"usage":{"prompt_tokens":5,"completion_tokens":10}}`,
		}
		for _, e := range events {
			_, _ = io.WriteString(w, "data: "+e+"\n\n")
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}))
	defer server.Close()

	p := NewOllamaProvider(server.URL, "model", ProviderConfig{Name: "test", Timeout: 5 * time.Second})
	ch, err := p.Stream(context.Background(), []Message{{Role: RoleUser, Content: strPtr("hi")}}, nil)
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}

	var text strings.Builder
	var gotUsage *Usage
	for d := range ch {
		switch d.Type {
		case "text":
			text.WriteString(d.Text())
		case "done":
			gotUsage = d.Usage
		}
	}
	if text.String() != "Hello, world" {
		t.Errorf("text = %q", text.String())
	}
	if gotUsage == nil || gotUsage.PromptTokens != 5 {
		t.Errorf("usage = %+v", gotUsage)
	}
}

func TestOllamaProvider_Stream_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	p := NewOllamaProvider(server.URL, "model", ProviderConfig{Name: "test", Timeout: time.Second})
	_, err := p.Stream(context.Background(), []Message{{Role: RoleUser, Content: strPtr("hi")}}, nil)
	if err == nil {
		t.Error("expected error for 401")
	}
}

func TestOllamaProvider_Stream_InvalidURL(t *testing.T) {
	p := NewOllamaProvider("://invalid", "model", ProviderConfig{Name: "test", Timeout: time.Second})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := p.Stream(ctx, nil, nil)
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

func TestOllamaProvider_Stream_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		// Block until client disconnects
		<-r.Context().Done()
	}))
	defer server.Close()

	p := NewOllamaProvider(server.URL, "model", ProviderConfig{Name: "test", Timeout: 5 * time.Second})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	ch, err := p.Stream(ctx, nil, nil)
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}
	for range ch {
	}
}
