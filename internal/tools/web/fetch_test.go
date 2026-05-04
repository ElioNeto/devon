package web

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ElioNeto/devon/internal/config"
)

func TestFetchTool_Name(t *testing.T) {
	tool := &FetchTool{}
	if tool.Name() != "web_fetch" {
		t.Errorf("expected 'web_fetch', got %q", tool.Name())
	}
}

func TestFetchTool_Permission(t *testing.T) {
	tool := &FetchTool{}
	if tool.Permission().String() != "read" {
		t.Errorf("expected 'read', got %q", tool.Permission().String())
	}
}

func TestFetchTool_Description(t *testing.T) {
	tool := &FetchTool{}
	if tool.Description() == "" {
		t.Error("expected non-empty description")
	}
}

func TestFetchTool_Schema(t *testing.T) {
	tool := &FetchTool{}
	schema := tool.Schema()
	if schema == nil {
		t.Fatal("expected non-nil schema")
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(schema, &parsed); err != nil {
		t.Fatalf("invalid JSON schema: %v", err)
	}
	props, ok := parsed["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("expected properties in schema")
	}
	if _, ok := props["url"]; !ok {
		t.Error("expected 'url' property in schema")
	}
}

func TestFetchTool_Execute_EmptyURL(t *testing.T) {
	tool := &FetchTool{Config: &config.WebConfig{Enabled: true}}
	params, _ := json.Marshal(map[string]string{"url": ""})
	_, err := tool.Execute(context.Background(), params)
	if err == nil {
		t.Error("expected error for empty url, got nil")
	}
}

func TestFetchTool_Execute_NotEnabled(t *testing.T) {
	tool := &FetchTool{Config: &config.WebConfig{Enabled: false}}
	params, _ := json.Marshal(map[string]string{"url": "https://example.com"})
	_, err := tool.Execute(context.Background(), params)
	if err == nil {
		t.Error("expected error when web not enabled, got nil")
	}
}

func TestFetchTool_Execute_InvalidParams(t *testing.T) {
	tool := &FetchTool{}
	_, err := tool.Execute(context.Background(), json.RawMessage(`{invalid}`))
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestFetchTool_Execute_BackendSelection(t *testing.T) {
	t.Run("duckduckgo for fetch when no firecrawl key", func(t *testing.T) {
		backend, err := SelectBackend(&config.WebConfig{Enabled: true, Backend: "auto"})
		if err != nil {
			t.Fatalf("SelectBackend() error = %v", err)
		}
		if backend.Name() != "duckduckgo" {
			t.Errorf("expected 'duckduckgo', got %q", backend.Name())
		}
	})

	t.Run("timeout context", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 0)
		defer cancel()

		ddg := &DuckDuckGoBackend{}
		_, err := ddg.Fetch(ctx, "https://example.com")
		if err == nil {
			t.Error("expected error for cancelled context, got nil")
		}
	})

	t.Run("invalid URL scheme", func(t *testing.T) {
		ddg := &DuckDuckGoBackend{}
		_, err := ddg.Fetch(context.Background(), "ftp://not-supported.com")
		if err == nil {
			t.Error("expected error for ftp URL, got nil")
		}
	})
}
