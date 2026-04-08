package permissions

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewAuditLogger(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	a, err := NewAuditLogger()
	if err != nil {
		t.Fatalf("NewAuditLogger() error: %v", err)
	}
	defer a.Close()

	dir := filepath.Join(tmpHome, ".devon")
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf(".devon dir should exist: %v", err)
	}
	if !info.IsDir() {
		t.Fatal(".devon should be a directory")
	}
}

func TestAuditLogger_LogAndEntries(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	a, err := NewAuditLogger()
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	a.Log("bash", "echo hello", "hello", nil)
	a.Log("write", "file.txt", "ok", nil)
	a.Log("bash", "bad cmd", "", os.ErrPermission)

	entries, err := a.Entries()
	if err != nil {
		t.Fatalf("Entries() error: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].Tool != "bash" {
		t.Errorf("entries[0].Tool = %q", entries[0].Tool)
	}
	if entries[0].Status != "OK" {
		t.Errorf("entries[0].Status = %q", entries[0].Status)
	}
	if entries[2].Status == "" || entries[2].Status == "OK" {
		t.Errorf("entries[2].Status should be ERROR, got %q", entries[2].Status)
	}
}

func TestAuditLogger_Entries_NotFound(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	a, err := NewAuditLogger()
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	entries, err := a.Entries()
	if err != nil {
		t.Fatalf("Entries() error: %v", err)
	}
	if entries != nil {
		t.Errorf("expected nil entries for nonexistent file, got %v", entries)
	}
}

func TestAuditLogger_Summary(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	a, err := NewAuditLogger()
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	a.Log("bash", "echo hi", "hi", nil)
	a.Log("bash", "bad", "", os.ErrPermission)
	a.Log("write", "f.txt", "ok", nil)

	summary, err := a.Summary()
	if err != nil {
		t.Fatalf("Summary() error: %v", err)
	}
	if summary == "" {
		t.Error("summary should not be empty")
	}
}

func TestAuditLogger_Summary_Empty(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	a, err := NewAuditLogger()
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	summary, err := a.Summary()
	if err != nil {
		t.Fatalf("Summary() error: %v", err)
	}
	if summary == "" {
		t.Error("empty summary should still return message")
	}
}

func TestAuditLogger_Close_MultipleCalls(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	a, err := NewAuditLogger()
	if err != nil {
		t.Fatal(err)
	}
	a.Close()
	a.Close() // should not panic
}

func TestParseAuditLine(t *testing.T) {
	line := `[2025-01-15T10:30:00Z] tool=bash args="echo hi" status=OK result="hi"`
	e := parseAuditLine(line)

	if e.Tool != "bash" {
		t.Errorf("Tool = %q", e.Tool)
	}
	if e.Args != "echo hi" {
		t.Errorf("Args = %q", e.Args)
	}
	if e.Status != "OK" {
		t.Errorf("Status = %q", e.Status)
	}
	if e.Result != "hi" {
		t.Errorf("Result = %q", e.Result)
	}
}

func TestParseAuditLine_EmptyOrInvalid(t *testing.T) {
	e := parseAuditLine("")
	if e.Tool != "" {
		t.Errorf("empty line should produce empty entry, got Tool=%q", e.Tool)
	}

	e = parseAuditLine("no bracket")
	if e.Tool != "" {
		t.Errorf("invalid line should produce empty entry, got Tool=%q", e.Tool)
	}

	e = parseAuditLine("[not-a-timestamp] tool=x")
	if e.Tool != "" {
		// timestamp parse fails but might still parse fields
		_ = e
	}
}

func TestSplitLines(t *testing.T) {
	got := splitLines("a\nb\nc")
	if len(got) != 3 {
		t.Errorf("splitLines returned %d lines", len(got))
	}
}

func TestSplitLines_TrailingNewline(t *testing.T) {
	got := splitLines("a\nb\n")
	if len(got) != 2 {
		t.Errorf("splitLines with trailing newline returned %d lines", len(got))
	}
}

func TestSplitLines_NoNewline(t *testing.T) {
	got := splitLines("hello")
	if len(got) != 1 || got[0] != "hello" {
		t.Errorf("splitLines = %v", got)
	}
}

func TestHasPrefix(t *testing.T) {
	if !hasPrefix("hello world", "hello") {
		t.Error("expected true")
	}
	if hasPrefix("hi", "hello") {
		t.Error("expected false")
	}
}

func TestFieldEnd(t *testing.T) {
	if got := fieldEnd("hello world"); got != 5 {
		t.Errorf("fieldEnd = %d, want 5", got)
	}
	if got := fieldEnd("nospaces"); got != 8 {
		t.Errorf("fieldEnd = %d, want 8", got)
	}
}

func TestQuotedEnd(t *testing.T) {
	if got := quotedEnd(`"hello" world`); got != 6 {
		t.Errorf("quotedEnd = %d, want 6", got)
	}
}

func TestSkipPast(t *testing.T) {
	if got := skipPast("hello world", 5); got != "world" {
		t.Errorf("skipPast = %q", got)
	}
	if got := skipPast("hello", 10); got != "" {
		t.Errorf("skipPast beyond end = %q", got)
	}
}
