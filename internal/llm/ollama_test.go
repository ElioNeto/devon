package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
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

func TestOllamaProvider_Stream_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	p := NewOllamaProvider(server.URL, "model", ProviderConfig{Name: "test", Timeout: time.Second})
	_, err := p.Stream(context.Background(), []Message{{Role: RoleUser, Content: strPtr("hi")}}, nil)
	if err == nil {
		t.Error("expected error for server error")
	}
}

func TestOllamaProvider_Stream_InvalidURL(t *testing.T) {
	p := NewOllamaProvider("://invalid", "model", ProviderConfig{Name: "test", Timeout: time.Second})
	_, err := p.Stream(context.Background(), nil, nil)
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}
