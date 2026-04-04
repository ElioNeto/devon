package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestBashTool_NameDescSchema(t *testing.T) {
	tool := &BashTool{}
	if tool.Name() != "bash" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "bash")
	}
	if tool.Description() == "" {
		t.Error("Description() is empty")
	}
	if len(tool.Schema()) == 0 {
		t.Error("Schema() is empty")
	}
}

func TestBashTool_Execute_Success(t *testing.T) {
	tool := &BashTool{Timeout: 5 * time.Second}
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"command":"echo hello"}`))
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if result != "hello" {
		t.Errorf("Execute() = %q, want %q", result, "hello")
	}
}

func TestBashTool_Execute_NoOutput(t *testing.T) {
	tool := &BashTool{Timeout: 5 * time.Second}
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"command":"true"}`))
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if result != "(no output)" {
		t.Errorf("Execute() = %q, want %q", result, "(no output)")
	}
}

func TestBashTool_Execute_Error(t *testing.T) {
	tool := &BashTool{Timeout: 5 * time.Second}
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"command":"exit 1"}`))
	if err == nil {
		t.Fatal("expected error from exit 1")
	}
}

func TestBashTool_Execute_EmptyCommand(t *testing.T) {
	tool := &BashTool{}
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"command":""}`))
	if err == nil {
		t.Fatal("expected error for empty command")
	}
}

func TestBashTool_Execute_InvalidParams(t *testing.T) {
	tool := &BashTool{}
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"cmd":"echo"}`))
	if err == nil {
		t.Fatal("expected error for invalid params")
	}
}

func TestBashTool_Execute_Timeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping timeout test in short mode")
	}
	tool := &BashTool{Timeout: 100 * time.Millisecond}
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"command":"sleep 10"}`))
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestBashTool_Execute_Stderr(t *testing.T) {
	tool := &BashTool{Timeout: 5 * time.Second}
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"command":"echo error >&2 && echo ok"}`))
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty output with stderr")
	}
}

func TestSanitizeOutput_Truncates(t *testing.T) {
	s := make([]byte, 40*1024)
	for i := range s {
		s[i] = 'a'
	}
	result := sanitizeOutput(string(s))
	if len(result) > 40*1024 {
		t.Errorf("sanitizeOutput did not truncate: length %d", len(result))
	}
}

func TestSanitizeOutput_NoTruncate(t *testing.T) {
	short := "short output"
	if got := sanitizeOutput(short); got != short {
		t.Errorf("sanitizeOutput(%q) = %q, want %q", short, got, short)
	}
}

func TestBashTool_Execute_StderrOnly(t *testing.T) {
	tool := &BashTool{Timeout: 5 * time.Second}
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"command":"echo error >&2"}`))
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	result = strings.TrimSpace(result)
	if result != "error" {
		t.Errorf("stderr-only result = %q, want 'error'", result)
	}
}

func TestBashTool_Execute_StderrAndStdout(t *testing.T) {
	tool := &BashTool{Timeout: 5 * time.Second}
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"command":"echo err >&2; echo ok"}`))
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !strings.Contains(result, "err") || !strings.Contains(result, "ok") {
		t.Errorf("expected both stderr and stdout in output: %q", result)
	}
}
