package config

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDoctor_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"models":[]}`))
	}))
	defer srv.Close()

	cfg := &Config{
		Model:   "test-model",
		BaseURL: srv.URL,
		WorkDir: t.TempDir(),
		Mode:    ModeAuto,
	}

	err := cfg.Doctor(context.Background())
	if err != nil {
		t.Fatalf("Doctor() error: %v", err)
	}
}

func TestDoctor_Failure(t *testing.T) {
	cfg := &Config{
		Model:   "test-model",
		BaseURL: "http://127.0.0.1:19999", // no server
		WorkDir: t.TempDir(),
		Mode:    ModeAuto,
	}

	err := cfg.Doctor(context.Background())
	if err == nil {
		t.Fatal("expected error from unreachable server")
	}
}

func TestDoctor_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	cfg := &Config{
		Model:   "test-model",
		BaseURL: srv.URL,
		WorkDir: t.TempDir(),
		Mode:    ModeAuto,
	}

	err := cfg.Doctor(context.Background())
	if err == nil {
		t.Fatal("expected error from 401 response")
	}
}

func TestDoctor_ContextCancelled(t *testing.T) {
	cfg := &Config{
		Model:   "test-model",
		BaseURL: "http://localhost:11434/v1",
		WorkDir: t.TempDir(),
		Mode:    ModeAuto,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := cfg.Doctor(ctx)
	if err == nil {
		t.Fatal("expected error with cancelled context")
	}
}

func TestDoctor_WithAPIKeyAndContextDoc(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check auth header
		if r.Header.Get("Authorization") != "Bearer secret-key" {
			t.Error("missing auth header")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"models":[]}`))
	}))
	defer srv.Close()

	cfg := &Config{
		APIKey:     "secret-key",
		Model:      "test-model",
		BaseURL:    srv.URL,
		WorkDir:    t.TempDir(),
		Mode:       ModeAuto,
		ContextDoc: "project context here",
	}

	err := cfg.Doctor(context.Background())
	if err != nil {
		t.Fatalf("Doctor() error: %v", err)
	}
}
