package session_test

import (
	"strings"
	"testing"
	"time"

	"github.com/ElioNeto/devon/internal/db"
	"github.com/ElioNeto/devon/internal/session"
)

func TestExportMarkdown(t *testing.T) {
	now := time.Now()
	data := &session.ExportData{
		Session: db.SessionDetail{
			ID:       "test-session",
			Task:     "implement feature X",
			Model:    "gpt-4",
			Status:   "active",
			Duration: 15000,
		},
		Messages: []db.Message{
			{ID: 1, SessionID: "test-session", AgentID: "agent-1", Role: "user", Content: "Hello", Timestamp: now},
			{ID: 2, SessionID: "test-session", AgentID: "agent-1", Role: "assistant", Content: "Hi there!", Timestamp: now},
		},
		ToolCalls: []db.ToolCall{
			{ID: 1, SessionID: "test-session", AgentID: "agent-1", ToolName: "read_file", Arguments: `{"path":"main.go"}`, Status: "completed", Timestamp: now},
		},
		FileAccess: []db.FileAccess{
			{ID: 1, SessionID: "test-session", FilePath: "/project/main.go", AccessType: "read", Timestamp: now},
		},
	}

	// Markdown export
	md, err := session.ExportMarkdown(data)
	if err != nil {
		t.Fatalf("ExportMarkdown failed: %v", err)
	}

	// Verify content
	if !strings.Contains(md, "test-session") {
		t.Error("expected session ID in output")
	}
	if !strings.Contains(md, "implement feature X") {
		t.Error("expected task in output")
	}
	if !strings.Contains(md, "gpt-4") {
		t.Error("expected model in output")
	}
	if !strings.Contains(md, "Hello") {
		t.Error("expected message content in output")
	}
	if !strings.Contains(md, "read_file") {
		t.Error("expected tool call in output")
	}
	if !strings.Contains(md, "main.go") {
		t.Error("expected file access in output")
	}
}

func TestExportJSON(t *testing.T) {
	now := time.Now()
	data := &session.ExportData{
		Session: db.SessionDetail{
			ID:       "json-session",
			Task:     "test task",
			Model:    "claude-3",
			Status:   "completed",
			Duration: 5000,
		},
		Messages: []db.Message{
			{ID: 1, SessionID: "json-session", AgentID: "agent-1", Role: "user", Content: "test", Timestamp: now},
		},
	}

	jsonBytes, err := session.ExportJSON(data)
	if err != nil {
		t.Fatalf("ExportJSON failed: %v", err)
	}

	jsonStr := string(jsonBytes)
	if !strings.Contains(jsonStr, "json-session") {
		t.Error("expected session ID in JSON output")
	}
	if !strings.Contains(jsonStr, "claude-3") {
		t.Error("expected model in JSON output")
	}
	if !strings.Contains(jsonStr, "test task") {
		t.Error("expected task in JSON output")
	}
	if !strings.Contains(jsonStr, "\"messages\"") {
		t.Error("expected messages array in JSON output")
	}
}

func TestExportNilData(t *testing.T) {
	_, err := session.ExportMarkdown(nil)
	if err == nil {
		t.Error("expected error for nil data in Markdown export")
	}

	_, err = session.ExportJSON(nil)
	if err == nil {
		t.Error("expected error for nil data in JSON export")
	}
}

func TestExportEmptyData(t *testing.T) {
	data := &session.ExportData{
		Session: db.SessionDetail{
			ID: "empty-session",
		},
	}

	// Markdown with no messages, tool calls, or file access should still produce header
	md, err := session.ExportMarkdown(data)
	if err != nil {
		t.Fatalf("ExportMarkdown failed: %v", err)
	}
	if !strings.Contains(md, "empty-session") {
		t.Error("expected session ID in output")
	}

	// JSON with minimal data
	jsonBytes, err := session.ExportJSON(data)
	if err != nil {
		t.Fatalf("ExportJSON failed: %v", err)
	}
	if !strings.Contains(string(jsonBytes), "empty-session") {
		t.Error("expected session ID in JSON output")
	}
}
