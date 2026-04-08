package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAnthropicProvider_CreatesCorrectProvider(t *testing.T) {
	cfg := ProviderConfig{
		Name:    "claude-sonnet",
		APIKey:  "test-key",
		BaseURL: "", // empty → defaults to anthropic
		Model:   "claude-sonnet-4-20250514",
		Timeout: 10 * time.Second,
	}
	p := NewAnthropicProvider(cfg)

	if p.Name() != "claude-sonnet" {
		t.Errorf("Name() = %q, want %q", p.Name(), "claude-sonnet")
	}
	if p.Info().Provider != "anthropic" {
		t.Errorf("Info().Provider = %q", p.Info().Provider)
	}
	if !p.Info().SupportsTools {
		t.Error("expected SupportsTools to be true")
	}
	if !p.Info().SupportsVision {
		t.Error("expected SupportsVision for claude-3+ model")
	}
}

func TestAnthropicProvider_Stream_TextSSE(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify headers
		if got := r.Header.Get("x-api-key"); got != "test-key" {
			t.Errorf("x-api-key = %q", got)
		}
		if got := r.Header.Get("anthropic-version"); got != anthropicAPIVersion {
			t.Errorf("anthropic-version = %q", got)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}

		// Send SSE events: content_block_delta with text
		events := []string{
			`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
			`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`,
			`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":", world!"}}`,
			`{"type":"message_stop"}`,
		}

		for _, e := range events {
			_, _ = io.WriteString(w, "data: "+e+"\n\n")
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}))
	defer server.Close()

	p := NewAnthropicProvider(ProviderConfig{
		Name:    "test",
		APIKey:  "test-key",
		BaseURL: server.URL,
		Model:   "claude-sonnet",
	})

	msgs := []Message{
		{Role: RoleUser, Content: strPtr("Say hi")},
	}

	ch, err := p.Stream(context.Background(), msgs, nil)
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}

	var textBuf strings.Builder
	for d := range ch {
		if d.Type == "text" {
			textBuf.WriteString(d.Text())
		}
	}

	got := textBuf.String()
	want := "Hello, world!"
	if got != want {
		t.Errorf("text = %q, want %q", got, want)
	}
}

func TestAnthropicProvider_Stream_ToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}

		events := []string{
			`{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"tool1","name":"file_write","input":{}}}`,
			`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"path\":\"hello.go\"}"}}`,
			`{"type":"content_block_stop","index":0}`,
			`{"type":"message_delta","usage":{"output_tokens":10}}`,
		}
		for _, e := range events {
			_, _ = io.WriteString(w, "data: "+e+"\n\n")
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}))
	defer server.Close()

	p := NewAnthropicProvider(ProviderConfig{
		Name:    "test",
		APIKey:  "k",
		BaseURL: server.URL,
		Model:   "claude-sonnet",
	})

	ch, err := p.Stream(context.Background(), []Message{
		{Role: RoleUser, Content: strPtr("create hello.go")},
	}, nil)
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}

	var toolCalls []ToolCall
	var gotUsage *Usage
	for d := range ch {
		switch d.Type {
		case "tool_call":
			if d.Tool != nil {
				toolCalls = append(toolCalls, *d.Tool)
			}
		case "done":
			gotUsage = d.Usage
		}
	}

	if len(toolCalls) == 0 {
		t.Fatal("got 0 tool calls")
	}
	tc := toolCalls[0]
	if tc.Type != "function" {
		t.Errorf("Tool type = %q, want %q", tc.Type, "function")
	}
	if tc.Function.Name != "file_write" {
		t.Errorf("Tool name = %q, want %q", tc.Function.Name, "file_write")
	}
	if !strings.Contains(tc.Function.Arguments, "hello.go") {
		t.Errorf("Tool args = %q, want to contain hello.go", tc.Function.Arguments)
	}
	if gotUsage == nil || gotUsage.CompletionTokens != 10 {
		t.Errorf("usage = %+v, want 10 output tokens", gotUsage)
	}
}

func TestAnthropicProvider_Stream_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":{"message":"Invalid API Key"}}`)
	}))
	defer server.Close()

	p := NewAnthropicProvider(ProviderConfig{
		Name:    "test",
		APIKey:  "bad-key",
		BaseURL: server.URL,
		Model:   "claude-sonnet",
	})

	_, err := p.Stream(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error = %q, want to contain 401", err.Error())
	}
}

func TestAnthropicProvider_buildBody(t *testing.T) {
	p := NewAnthropicProvider(ProviderConfig{
		Name:   "test",
		Model:  "claude-sonnet",
		APIKey: "k",
	})
	msgs := []Message{
		{Role: RoleSystem, Content: strPtr("You are helpful")},
		{Role: RoleUser, Content: strPtr("What is 2+2?")},
	}
	tools := []ToolDef{
		{
			Type: "function",
			Function: ToolDefFunc{
				Name:        "calc",
				Description: "Do math",
				Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
			},
		},
	}

	body, err := p.buildBody(msgs, tools)
	if err != nil {
		t.Fatalf("buildBody error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if parsed["system"] != "You are helpful" {
		t.Errorf("system = %q", parsed["system"])
	}
	if parsed["model"] != "claude-sonnet" {
		t.Errorf("model = %v", parsed["model"])
	}
	if parsed["stream"] != true {
		t.Error("stream should be true")
	}

	// Check tools use input_schema instead of parameters
	toolsArr := parsed["tools"].([]any)
	firstTool := toolsArr[0].(map[string]any)
	if _, ok := firstTool["input_schema"]; !ok {
		t.Errorf("tool missing input_schema: %v", firstTool)
	}
	if _, ok := firstTool["parameters"]; ok {
		t.Error("tool should NOT have 'parameters' key (anthropic uses input_schema)")
	}
}

func TestAnthropicProvider_buildBody_SystemMerges(t *testing.T) {
	p := NewAnthropicProvider(ProviderConfig{
		Name: "test", Model: "claude", APIKey: "k",
	})

	msgs := []Message{
		{Role: RoleSystem, Content: strPtr("line1")},
		{Role: RoleUser, Content: strPtr("hello")},
		{Role: RoleSystem, Content: strPtr("line2")},
	}

	body, err := p.buildBody(msgs, nil)
	if err != nil {
		t.Fatal(err)
	}

	var parsed map[string]any
	_ = json.Unmarshal(body, &parsed)
	expected := "line1\nline2"
	if parsed["system"] != expected {
		t.Errorf("system = %q, want %q", parsed["system"], expected)
	}
}

func TestAnthropicProvider_buildBody_TagsVision(t *testing.T) {
	cases := []struct {
		model      string
		wantVision bool
	}{
		{"claude-3-opus", true},
		{"claude-3-sonnet", true},
		{"claude-3-5-sonnet", true},
		{"claude-2", false},
		{"claude-instant", false},
	}

	for _, tc := range cases {
		p := NewAnthropicProvider(ProviderConfig{
			Name: "test", Model: tc.model, APIKey: "k",
		})

		if got := p.Info().SupportsVision; got != tc.wantVision {
			t.Errorf("model=%s: SupportsVision=%v, want=%v", tc.model, got, tc.wantVision)
		}
	}
}

func strPtr(s string) *string { return &s }
