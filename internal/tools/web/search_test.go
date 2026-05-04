package web

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ElioNeto/devon/internal/config"
)

func TestSearchTool_Name(t *testing.T) {
	tool := &SearchTool{}
	if tool.Name() != "web_search" {
		t.Errorf("expected 'web_search', got %q", tool.Name())
	}
}

func TestSearchTool_Permission(t *testing.T) {
	tool := &SearchTool{}
	if tool.Permission().String() != "read" {
		t.Errorf("expected 'read', got %q", tool.Permission().String())
	}
}

func TestSearchTool_Description(t *testing.T) {
	tool := &SearchTool{}
	if tool.Description() == "" {
		t.Error("expected non-empty description")
	}
}

func TestSearchTool_Schema(t *testing.T) {
	tool := &SearchTool{}
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
	if _, ok := props["query"]; !ok {
		t.Error("expected 'query' property in schema")
	}
}

func TestSearchTool_Execute_EmptyQuery(t *testing.T) {
	tool := &SearchTool{Config: &config.WebConfig{Enabled: true}}
	params, _ := json.Marshal(map[string]string{"query": ""})
	_, err := tool.Execute(context.Background(), params)
	if err == nil {
		t.Error("expected error for empty query, got nil")
	}
}

func TestSearchTool_Execute_NotEnabled(t *testing.T) {
	tool := &SearchTool{Config: &config.WebConfig{Enabled: false}}
	params, _ := json.Marshal(map[string]string{"query": "test"})
	_, err := tool.Execute(context.Background(), params)
	if err == nil {
		t.Error("expected error when web not enabled, got nil")
	}
}

func TestSearchTool_Execute_InvalidParams(t *testing.T) {
	tool := &SearchTool{}
	_, err := tool.Execute(context.Background(), json.RawMessage(`{invalid}`))
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestSearchTool_Execute_BackendSelection(t *testing.T) {
	t.Run("duckduckgo when firecrawl key absent", func(t *testing.T) {
		backend, err := SelectBackend(&config.WebConfig{Enabled: true, Backend: "auto"})
		if err != nil {
			t.Fatalf("SelectBackend() error = %v", err)
		}
		if backend.Name() != "duckduckgo" {
			t.Errorf("expected 'duckduckgo', got %q", backend.Name())
		}
	})

	t.Run("respect explicit duckduckgo backend", func(t *testing.T) {
		backend, err := SelectBackend(&config.WebConfig{Enabled: true, Backend: "duckduckgo"})
		if err != nil {
			t.Fatalf("SelectBackend() error = %v", err)
		}
		if backend.Name() != "duckduckgo" {
			t.Errorf("expected 'duckduckgo', got %q", backend.Name())
		}
	})

	t.Run("nil config returns error", func(t *testing.T) {
		_, err := SelectBackend(nil)
		if err == nil {
			t.Error("expected error for nil config, got nil")
		}
	})

	t.Run("disabled config returns error", func(t *testing.T) {
		_, err := SelectBackend(&config.WebConfig{Enabled: false})
		if err == nil {
			t.Error("expected error for disabled config, got nil")
		}
	})
}
