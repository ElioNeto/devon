package llm

import (
	"context"
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

func TestOpenAIProvider_Stream_ConnRefused(t *testing.T) {
	p := NewOpenAIProvider("http://localhost:19999/v1", "model", ProviderConfig{
		Name:    "test",
		APIKey:  "k",
		Timeout: 500 * time.Millisecond,
	})
	_, err := p.Stream(context.Background(), []Message{{Role: RoleUser, Content: strPtr("hi")}}, nil)
	if err == nil {
		t.Error("expected error for unreachable endpoint")
	}
}

func TestOpenAIProvider_Stream_InvalidURL(t *testing.T) {
	p := NewOpenAIProvider("://invalid", "model", ProviderConfig{
		Name:    "test",
		APIKey:  "k",
		Timeout: time.Second,
	})
	_, err := p.Stream(context.Background(), nil, nil)
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

func TestOpenAIProvider_Stream_EmptyAPIKey(t *testing.T) {
	p := NewOpenAIProvider("http://localhost:19999/v1", "model", ProviderConfig{
		Name:    "test",
		Timeout: 500 * time.Millisecond,
	})
	_, err := p.Stream(context.Background(), nil, nil)
	if err == nil {
		t.Error("expected error")
	}
}
