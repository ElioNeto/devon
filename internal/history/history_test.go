package history

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ElioNeto/devon/internal/llm"
)

func tempDir(t *testing.T) string {
	t.Helper()
	d, err := os.MkdirTemp("", "devon-history-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(d) })
	return d
}

func TestSessionDir_CreatesDirectory(t *testing.T) {
	// Use temp dir to avoid polluting real ~/.devon
	home := os.Getenv("HOME")
	tmpHome := tempDir(t)
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", home)

	workDir := "/some/project/path"
	dir, err := SessionDir(workDir)
	if err != nil {
		t.Fatalf("SessionDir() error: %v", err)
	}
	if !filepath.IsAbs(dir) {
		t.Errorf("SessionDir() returned relative path: %q", dir)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("session directory does not exist: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("session path is not a directory")
	}
}

func TestSessionDir_SameInputSameOutput(t *testing.T) {
	home := os.Getenv("HOME")
	tmpHome := tempDir(t)
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", home)

	dir1, _ := SessionDir("/same/input")
	dir2, _ := SessionDir("/same/input")
	if dir1 != dir2 {
		t.Errorf("SessionDir not deterministic: %q vs %q", dir1, dir2)
	}
}

func TestSessionDir_DifferentInputDifferentOutput(t *testing.T) {
	home := os.Getenv("HOME")
	tmpHome := tempDir(t)
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", home)

	dir1, _ := SessionDir("/project/a")
	dir2, _ := SessionDir("/project/b")
	if dir1 == dir2 {
		t.Error("different inputs produced same session dir")
	}
}

func TestCreateSession(t *testing.T) {
	home := os.Getenv("HOME")
	tmpHome := tempDir(t)
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", home)

	dir, _ := sessionDir("/test/project")
	s, err := createSession(dir)
	if err != nil {
		t.Fatalf("createSession() error: %v", err)
	}
	if s.ID == "" {
		t.Error("session ID should not be empty")
	}
	if s.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
	// File should exist
	path := sessionFile(dir, s.ID)
	if _, err := os.Stat(path); err != nil {
		t.Errorf("session file not found: %v", err)
	}
}

func TestLoadLastSession_NoSession(t *testing.T) {
	home := os.Getenv("HOME")
	tmpHome := tempDir(t)
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", home)

	s, err := LoadLastSession("/test/new-session")
	if err != nil {
		t.Fatalf("LoadLastSession() error: %v", err)
	}
	if s == nil {
		t.Fatal("session should not be nil")
	}
	if len(s.Messages) != 0 {
		t.Errorf("new session should have no messages, got %d", len(s.Messages))
	}
}

func TestSaveAndLoadSession(t *testing.T) {
	home := os.Getenv("HOME")
	tmpHome := tempDir(t)
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", home)

	s, err := createSession(sessionDirMust(t, "/test/session"))
	if err != nil {
		t.Fatal(err)
	}

	s.Messages = []llm.Message{
		{Role: llm.RoleUser, Content: llm.TextContent("hello")},
		{Role: llm.RoleAssistant, Content: llm.TextContent("hi")},
	}
	s.Usage = UsageSummary{TotalTokens: 100, Requests: 2}

	if err := Save("/test/session", s); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := LoadSession("/test/session", s.ID)
	if err != nil {
		t.Fatalf("LoadSession() error: %v", err)
	}
	if len(loaded.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(loaded.Messages))
	}
	if loaded.Messages[0].Content == nil || *loaded.Messages[0].Content != "hello" {
		t.Errorf("expected first message %q, got %q", "hello", *loaded.Messages[0].Content)
	}
	if loaded.Usage.TotalTokens != 100 {
		t.Errorf("expected 100 tokens, got %d", loaded.Usage.TotalTokens)
	}
}

func TestListSessions(t *testing.T) {
	home := os.Getenv("HOME")
	tmpHome := tempDir(t)
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", home)

	workDir := "/test/list"
	dir := sessionDirMust(t, workDir)
	createSessionMust(t, dir)
	time.Sleep(2 * time.Millisecond)
	createSessionMust(t, dir)

	sessions, err := ListSessions(workDir)
	if err != nil {
		t.Fatalf("ListSessions() error: %v", err)
	}
	if len(sessions) < 2 {
		t.Errorf("expected >= 2 sessions, got %d", len(sessions))
	}
}

func TestClearSession(t *testing.T) {
	home := os.Getenv("HOME")
	tmpHome := tempDir(t)
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", home)

	workDir := "/test/clear"
	dir := sessionDirMust(t, workDir)
	s := createSessionMust(t, dir)

	if err := ClearSession(workDir, s.ID); err != nil {
		t.Fatalf("ClearSession() error: %v", err)
	}

	sessions, _ := ListSessions(workDir)
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions after clear, got %d", len(sessions))
	}
}

func TestAppendMessage(t *testing.T) {
	home := os.Getenv("HOME")
	tmpHome := tempDir(t)
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", home)

	workDir := "/test/append"
	dir := sessionDirMust(t, workDir)
	s := createSessionMust(t, dir)

	msg := llm.Message{Role: llm.RoleUser, Content: llm.TextContent("test")}
	if err := AppendMessage(workDir, s.ID, msg, nil); err != nil {
		t.Fatalf("AppendMessage() error: %v", err)
	}

	sessions, _ := ListSessions(workDir)
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session")
	}
}

func TestSaveMessagesJSONL(t *testing.T) {
	home := os.Getenv("HOME")
	tmpHome := tempDir(t)
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", home)

	workDir := "/test/jsonl"
	_ = sessionDirMust(t, workDir)

	msg := llm.Message{Role: llm.RoleUser, Content: llm.TextContent("hello world")}
	id := "20250101T120000Z"

	if err := SaveMessagesJSONL(workDir, id, msg); err != nil {
		t.Fatalf("SaveMessagesJSONL() error: %v", err)
	}

	messages, err := LoadMessagesJSONL(workDir, id)
	if err != nil {
		t.Fatalf("LoadMessagesJSONL() error: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	if messages[0].Content == nil || *messages[0].Content != "hello world" {
		t.Errorf("expected 'hello world', got %q", *messages[0].Content)
	}
}

func TestLoadSession_NonExistent(t *testing.T) {
	home := os.Getenv("HOME")
	tmpHome := tempDir(t)
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", home)

	_, err := LoadSession("/test/none", "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}

func TestLoadLastSession_LoadsMostRecent(t *testing.T) {
	home := os.Getenv("HOME")
	tmpHome := tempDir(t)
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", home)

	workDir := "/test/recent"
	dir := sessionDirMust(t, workDir)

	// Create two sessions with different timestamps
	createSessionMust(t, dir)
	time.Sleep(10 * time.Millisecond)
	s2 := createSessionMust(t, dir)

	// LoadLastSession should return the most recent
	loaded, err := LoadLastSession(workDir)
	if err != nil {
		t.Fatalf("LoadLastSession() error: %v", err)
	}
	if loaded.ID != s2.ID {
		t.Errorf("expected most recent session %q, got %q", s2.ID, loaded.ID)
	}
}

func TestUsageSummary_AddUsage(t *testing.T) {
	u := &UsageSummary{}
	u.AddUsage(&llm.Usage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
	})
	u.AddUsage(&llm.Usage{
		PromptTokens:     200,
		CompletionTokens: 100,
		TotalTokens:      300,
	})
	if u.PromptTokens != 300 {
		t.Errorf("expected 300 prompt tokens, got %d", u.PromptTokens)
	}
	if u.CompletionTokens != 150 {
		t.Errorf("expected 150 completion tokens, got %d", u.CompletionTokens)
	}
	if u.TotalTokens != 450 {
		t.Errorf("expected 450 total tokens, got %d", u.TotalTokens)
	}
	if u.Requests != 2 {
		t.Errorf("expected 2 requests, got %d", u.Requests)
	}
}

// --- helpers ---

func sessionDirMust(t *testing.T, workDir string) string {
	t.Helper()
	dir, err := sessionDir(workDir)
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func createSessionMust(t *testing.T, dir string) *Session {
	t.Helper()
	s, err := createSession(dir)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestCreateSession_Public(t *testing.T) {
	home := os.Getenv("HOME")
	tmpHome := tempDir(t)
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", home)

	s, err := CreateSession("/test/public-create")
	if err != nil {
		t.Fatalf("CreateSession() error: %v", err)
	}
	if s.ID == "" {
		t.Error("session ID should not be empty")
	}
	if s.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}

func TestSaveMessages(t *testing.T) {
	home := os.Getenv("HOME")
	tmpHome := tempDir(t)
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", home)

	workDir := "/test/save-messages"
	dir := sessionDirMust(t, workDir)
	s := createSessionMust(t, dir)

	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: llm.TextContent("hello")},
		{Role: llm.RoleAssistant, Content: llm.TextContent("hi there")},
	}
	usage := &UsageSummary{PromptTokens: 50, CompletionTokens: 25, TotalTokens: 75, Requests: 1}

	if err := SaveMessages(workDir, s.ID, msgs, usage); err != nil {
		t.Fatalf("SaveMessages() error: %v", err)
	}

	loaded, err := LoadSession(workDir, s.ID)
	if err != nil {
		t.Fatalf("LoadSession() error: %v", err)
	}
	if len(loaded.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(loaded.Messages))
	}
	if loaded.Usage.PromptTokens != 50 {
		t.Errorf("expected 50 prompt tokens, got %d", loaded.Usage.PromptTokens)
	}
}

func TestSaveMessages_NilUsage(t *testing.T) {
	home := os.Getenv("HOME")
	tmpHome := tempDir(t)
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", home)

	workDir := "/test/save-nil-usage"
	dir := sessionDirMust(t, workDir)
	s := createSessionMust(t, dir)

	msgs := []llm.Message{{Role: llm.RoleUser, Content: llm.TextContent("test")}}

	if err := SaveMessages(workDir, s.ID, msgs, nil); err != nil {
		t.Fatalf("SaveMessages() error: %v", err)
	}

	loaded, err := LoadSession(workDir, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(loaded.Messages))
	}
}

func TestLoadMessages(t *testing.T) {
	home := os.Getenv("HOME")
	tmpHome := tempDir(t)
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", home)

	workDir := "/test/load-messages"
	dir := sessionDirMust(t, workDir)
	s := createSessionMust(t, dir)

	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: llm.TextContent("hello")},
	}
	if err := SaveMessages(workDir, s.ID, msgs, nil); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadMessages(workDir, s.ID)
	if err != nil {
		t.Fatalf("LoadMessages() error: %v", err)
	}
	if len(loaded) != 1 {
		t.Errorf("expected 1 message, got %d", len(loaded))
	}
	if loaded[0].Content == nil || *loaded[0].Content != "hello" {
		t.Errorf("expected 'hello', got %q", *loaded[0].Content)
	}
}

func TestLoadMessages_NonExistent(t *testing.T) {
	home := os.Getenv("HOME")
	tmpHome := tempDir(t)
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", home)

	_, err := LoadMessages("/test/load-nonexistent", "nosuchid")
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}

func TestGetSessionInfo(t *testing.T) {
	home := os.Getenv("HOME")
	tmpHome := tempDir(t)
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", home)

	workDir := "/test/get-session-info"
	dir := sessionDirMust(t, workDir)
	s := createSessionMust(t, dir)

	info, err := GetSessionInfo(workDir)
	if err != nil {
		t.Fatalf("GetSessionInfo() error: %v", err)
	}
	if info == nil {
		t.Fatal("GetSessionInfo returned nil")
	}
	if info.ID != s.ID {
		t.Errorf("expected session ID %q, got %q", s.ID, info.ID)
	}
}

func TestGetSessionInfo_NewProject(t *testing.T) {
	home := os.Getenv("HOME")
	tmpHome := tempDir(t)
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", home)

	info, err := GetSessionInfo("/test/get-new-project")
	if err != nil {
		t.Fatalf("GetSessionInfo() error: %v", err)
	}
	if info == nil {
		t.Fatal("GetSessionInfo returned nil")
	}
	if info.ID == "" {
		t.Error("expected session ID for new project")
	}
}

func TestAppendMessage_WithUsage(t *testing.T) {
	home := os.Getenv("HOME")
	tmpHome := tempDir(t)
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", home)

	workDir := "/test/append-usage"
	dir := sessionDirMust(t, workDir)
	s := createSessionMust(t, dir)

	usage := &UsageSummary{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15, Requests: 1}
	msg := llm.Message{Role: llm.RoleAssistant, Content: llm.TextContent("response")}

	if err := AppendMessage(workDir, s.ID, msg, usage); err != nil {
		t.Fatalf("AppendMessage() error: %v", err)
	}

	// Append another to test accumulation
	msg2 := llm.Message{Role: llm.RoleUser, Content: llm.TextContent("follow up")}
	usage2 := &UsageSummary{PromptTokens: 20, CompletionTokens: 10, TotalTokens: 30, Requests: 1}
	if err := AppendMessage(workDir, s.ID, msg2, usage2); err != nil {
		t.Fatalf("AppendMessage() second call error: %v", err)
	}

	loaded, err := LoadSession(workDir, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(loaded.Messages))
	}
}

func TestAppendMessage_MalformedJSON(t *testing.T) {
	home := os.Getenv("HOME")
	tmpHome := tempDir(t)
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", home)

	workDir := "/test/append-malformed"
	dir := sessionDirMust(t, workDir)

	// Write a malformed JSON session file
	id := "malformed-session"
	data := []byte(`not valid json`)
	sessionPath := filepath.Join(dir, id+".json")
	if err := os.WriteFile(sessionPath, data, 0o600); err != nil {
		t.Fatal(err)
	}

	msg := llm.Message{Role: llm.RoleUser, Content: llm.TextContent("test")}
	err := AppendMessage(workDir, id, msg, nil)
	if err == nil {
		t.Error("expected error for malformed session file")
	}
}

func TestSaveMessagesJSONL_MultipleMessages(t *testing.T) {
	home := os.Getenv("HOME")
	tmpHome := tempDir(t)
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", home)

	workDir := "/test/jsonl-multi"
	_ = sessionDirMust(t, workDir)
	id := "jsonl-multi-session"

	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: llm.TextContent("first")},
		{Role: llm.RoleAssistant, Content: llm.TextContent("second")},
		{Role: llm.RoleUser, Content: llm.TextContent("third")},
	}

	for _, msg := range msgs {
		if err := SaveMessagesJSONL(workDir, id, msg); err != nil {
			t.Fatalf("SaveMessagesJSONL() error: %v", err)
		}
	}

	loaded, err := LoadMessagesJSONL(workDir, id)
	if err != nil {
		t.Fatalf("LoadMessagesJSONL() error: %v", err)
	}
	if len(loaded) != 3 {
		t.Errorf("expected 3 messages, got %d", len(loaded))
	}
}

func TestLoadMessagesJSONL_MalformedLines(t *testing.T) {
	home := os.Getenv("HOME")
	tmpHome := tempDir(t)
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", home)

	workDir := "/test/jsonl-malformed"
	dir := sessionDirMust(t, workDir)
	id := "malformed-jsonl"

	// Write a JSONL with a bad line
	jsonlPath := filepath.Join(dir, id+".jsonl")
	data := []byte("not json\n{\"role\":\"user\",\"content\":\"ok\"}\n")
	if err := os.WriteFile(jsonlPath, data, 0o600); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadMessagesJSONL(workDir, id)
	if err != nil {
		t.Fatalf("LoadMessagesJSONL() error: %v", err)
	}
	// Should skip malformed line and load the valid one
	if len(loaded) != 1 {
		t.Errorf("expected 1 message (malformed skipped), got %d", len(loaded))
	}
}

func TestClearSession_NonExistent(t *testing.T) {
	home := os.Getenv("HOME")
	tmpHome := tempDir(t)
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", home)

	err := ClearSession("/test/clear-nonexistent", "nosuchid")
	if err != nil {
		t.Errorf("ClearSession on nonexistent should not error, got: %v", err)
	}
}
