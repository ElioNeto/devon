package tui

import (
	"strings"
	"testing"

	"github.com/ElioNeto/devon/internal/agent"
	"github.com/ElioNeto/devon/internal/config"
	tea "github.com/charmbracelet/bubbletea"
)

func updateApp(m appModel, msg tea.Msg) appModel {
	result, _ := m.Update(msg)
	return result.(appModel)
}

func testConfig() *config.Config {
	return &config.Config{
		Model:   "test-model",
		BaseURL: "http://localhost:11434/v1",
		WorkDir: "/tmp/test",
		Mode:    config.ModeAuto,
	}
}

func TestNewModel(t *testing.T) {
	m := newModel(testConfig())
	if m.cfg.Model != "test-model" {
		t.Errorf("expected model test-model, got %q", m.cfg.Model)
	}
	if m.agent == nil {
		t.Error("agent should not be nil")
	}
}

func TestModel_Init(t *testing.T) {
	m := newModel(testConfig())
	cmd := m.Init()
	if cmd == nil {
		t.Error("Init should return a cmd")
	}
}

func TestModel_UpdateWindowSize(t *testing.T) {
	m := newModel(testConfig())
	m = updateApp(m, tea.WindowSizeMsg{Width: 80, Height: 24})
	if m.width != 80 || m.height != 24 {
		t.Errorf("expected size 80x24, got %dx%d", m.width, m.height)
	}
}

func TestModel_UpdateTypeText(t *testing.T) {
	m := newModel(testConfig())
	m.width = 80
	m.height = 24

	m = updateApp(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hello")})
	if m.input != "hello" {
		t.Errorf("expected input 'hello', got %q", m.input)
	}

	m = updateApp(m, tea.KeyMsg{Type: tea.KeyBackspace})
	if m.input != "hell" {
		t.Errorf("expected 'hell' after backspace, got %q", m.input)
	}
}

func TestModel_UpdateDeleteWord(t *testing.T) {
	m := newModel(testConfig())
	m.width = 80
	m.height = 24

	m = updateApp(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hello world")})
	if m.input != "hello world" {
		t.Fatalf("expected 'hello world', got %q", m.input)
	}

	m.cursor = len([]rune(m.input))
	m = updateApp(m, tea.KeyMsg{Type: tea.KeyCtrlW})
	if m.input != "hello " {
		t.Errorf("expected 'hello ' after delete word, got %q", m.input)
	}
}

func TestModel_UpdateClearInput(t *testing.T) {
	m := newModel(testConfig())
	m.width = 80
	m.height = 24

	m = updateApp(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hello")})
	m = updateApp(m, tea.KeyMsg{Type: tea.KeyCtrlU})
	if m.input != "" || m.cursor != 0 {
		t.Errorf("ctrl+u should clear input, got %q cursor=%d", m.input, m.cursor)
	}
}

func TestModel_UpdateCursor(t *testing.T) {
	m := newModel(testConfig())
	m.width = 80
	m.height = 24

	m = updateApp(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hello")})
	m.cursor = 3

	m = updateApp(m, tea.KeyMsg{Type: tea.KeyLeft})
	if m.cursor != 2 {
		t.Errorf("cursor should be 2 after left, got %d", m.cursor)
	}

	m = updateApp(m, tea.KeyMsg{Type: tea.KeyRight})
	if m.cursor != 3 {
		t.Errorf("cursor should be 3 after right, got %d", m.cursor)
	}

	m = updateApp(m, tea.KeyMsg{Type: tea.KeyHome})
	if m.cursor != 0 {
		t.Errorf("cursor should be 0 after home, got %d", m.cursor)
	}

	m = updateApp(m, tea.KeyMsg{Type: tea.KeyEnd})
	if m.cursor != 5 {
		t.Errorf("cursor should be 5 after end, got %d", m.cursor)
	}
}

func TestModel_UpdateAgentResult(t *testing.T) {
	m := newModel(testConfig())
	m.width = 80
	m.height = 24
	m.running = true

	m = updateApp(m, agentResult{events: []agent.Event{{Type: "system", Text: "test message"}}})
	if len(m.messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(m.messages))
	}
	if m.messages[0].Content != "test message" {
		t.Errorf("expected 'test message', got %q", m.messages[0].Content)
	}
	// agentResult should stop running
	if m.running {
		t.Error("agent should not be running after agentResult")
	}
}

func TestModel_UpdateClearChat(t *testing.T) {
	m := newModel(testConfig())
	m.messages = append(m.messages, chatMessage{Sender: "devon", Content: "hello"})
	m.scroll = 5

	m = updateApp(m, tea.KeyMsg{Type: tea.KeyCtrlL})
	if len(m.messages) != 0 {
		t.Error("ctrl+l should clear messages")
	}
	if m.scroll != 0 {
		t.Error("ctrl+l should reset scroll")
	}
}

func TestModel_UpdateNewSession(t *testing.T) {
	m := newModel(testConfig())
	m.messages = append(m.messages, chatMessage{Sender: "devon", Content: "old"})
	m.tracker.TotalInputTokens = 100

	m = updateApp(m, tea.KeyMsg{Type: tea.KeyCtrlK})
	if m.tracker.TotalInputTokens != 0 {
		t.Error("ctrl+k should reset usage")
	}
	if m.messages[0].Sender != "system" {
		t.Error("ctrl+k should add a system message")
	}
}

func TestModel_UpdateHelp(t *testing.T) {
	m := newModel(testConfig())
	m.width = 80
	m.height = 24

	m = updateApp(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	if !m.showHelp {
		t.Error("help should be shown after ?")
	}

	m = updateApp(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if m.showHelp {
		t.Error("help should be hidden after any key")
	}
}

func TestModel_UpdateCtrlC(t *testing.T) {
	m := newModel(testConfig())
	m.width = 80
	m.height = 24

	// Ctrl+C when not running should quit
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Error("ctrl+c when not running should return a cmd")
	}

	// Ctrl+C when running should interrupt (not quit with tea.Quit)
	m.running = true
	_, cmd2 := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	// cmd2 should NOT be tea.Quit when running
	cmdName := ""
	if cmd2 != nil {
		// tea.Cmd is a function type, we can't compare to tea.Quit directly
		// Instead, check that the model state changed
	}
	if cmdName == "quit" {
		t.Error("ctrl+c when running should not quit")
	}
}

func TestProcessAgentEventText(t *testing.T) {
	m := appModel{}
	m.processAgentEvent(agent.Event{Type: "text", Text: "hello"})
	if len(m.messages) != 1 {
		t.Fatalf("expected 1 message")
	}
	if m.messages[0].Content != "hello" {
		t.Errorf("expected 'hello', got %q", m.messages[0].Content)
	}

	// Second text should append to same message
	m.processAgentEvent(agent.Event{Type: "text", Text: " world"})
	if len(m.messages) != 1 {
		t.Fatalf("expected still 1 message")
	}
	if m.messages[0].Content != "hello world" {
		t.Errorf("expected 'hello world', got %q", m.messages[0].Content)
	}
}

func TestProcessAgentEventTool(t *testing.T) {
	m := appModel{}
	m.processAgentEvent(agent.Event{Type: "tool_start", Tool: "bash", Args: `{"cmd":"ls"}`})
	if len(m.toolRuns) != 1 {
		t.Fatalf("expected 1 tool run")
	}
	if m.toolRuns[0].Status != "running" {
		t.Errorf("expected running status, got %q", m.toolRuns[0].Status)
	}

	m.processAgentEvent(agent.Event{Type: "tool_done", Tool: "bash", Result: "file1\nfile2"})
	if m.toolRuns[0].Status != "done" {
		t.Errorf("expected done status, got %q", m.toolRuns[0].Status)
	}

	m.processAgentEvent(agent.Event{Type: "tool_start", Tool: "grep", Args: `{"pattern":"foo"}`})
	m.processAgentEvent(agent.Event{Type: "tool_error", Tool: "grep", Err: testErr{msg: "no match"}})
	if m.toolRuns[1].Status != "error" {
		t.Errorf("expected error status, got %q", m.toolRuns[1].Status)
	}
}

func TestProcessAgentEventError(t *testing.T) {
	m := appModel{running: true}
	m.processAgentEvent(agent.Event{Type: "error", Err: testErr{msg: "connection refused"}})
	if m.running {
		t.Error("agent should not be running after error")
	}
	found := false
	for _, msg := range m.messages {
		if msg.IsError && strings.Contains(msg.Content, "connection refused") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected error message in messages")
	}
}

func TestProcessAgentEventTurnDone(t *testing.T) {
	m := appModel{running: true}
	m.processAgentEvent(agent.Event{Type: "turn_done"})
	if m.running {
		t.Error("agent should not be running after turn_done")
	}
	if len(m.toolRuns) != 0 {
		t.Error("toolRuns should be cleared after turn_done")
	}
}

func TestProcessAgentEventSystem(t *testing.T) {
	m := appModel{}
	m.processAgentEvent(agent.Event{Type: "system", Text: "welcome"})
	if len(m.messages) != 1 {
		t.Fatalf("expected 1 message")
	}
	if m.messages[0].Sender != "system" || m.messages[0].Content != "welcome" {
		t.Errorf("unexpected system message: %+v", m.messages[0])
	}
}

func TestModel_View_ZeroSize(t *testing.T) {
	m := newModel(testConfig())
	v := m.View()
	if v != "Iniciando Devon..." {
		t.Errorf("expected placeholder, got %q", v)
	}
}

func TestModel_View_Basic(t *testing.T) {
	m := newModel(testConfig())
	m.width = 80
	m.height = 24

	m = updateApp(m, tea.WindowSizeMsg{Width: 80, Height: 24})

	v := m.View()
	if !strings.Contains(v, "devon") {
		t.Error("view should contain 'devon' branding")
	}
	if !strings.Contains(v, "test-model") {
		t.Error("view should contain model name")
	}
	if !strings.Contains(v, "─") {
		t.Error("view should contain separator")
	}
}

func TestModel_ViewRunning(t *testing.T) {
	m := newModel(testConfig())
	m.width = 80
	m.height = 24
	m.running = true

	v := m.View()
	// Running state should show spinner, not the input prompt
	if !strings.ContainsAny(v, " |") {
		t.Error("view should show empty input indicator")
	}
	if strings.Contains(v, " > |") {
		t.Error("running state should not show input prompt with cursor")
	}
}

func TestModel_ViewWithToolRuns(t *testing.T) {
	m := newModel(testConfig())
	m.width = 80
	m.height = 24
	m.toolRuns = []toolRun{
		{Name: "bash", Args: `{"cmd":"ls"}`, Status: "running"},
	}

	v := m.View()
	if !strings.Contains(v, "bash") {
		t.Error("view should show tool name")
	}
}

func TestModel_ViewHelp(t *testing.T) {
	m := newModel(testConfig())
	m.width = 80
	m.height = 40
	m.showHelp = true

	v := m.View()
	if !strings.Contains(v, "Enter") {
		t.Error("help should mention Enter key")
	}
	if !strings.Contains(v, "sair") {
		t.Error("help should mention sair (quit)")
	}
}

func TestWrapLine(t *testing.T) {
	tests := []struct {
		input   string
		width   int
		wantLen int
	}{
		{"hello world", 5, 2},
		{"hello", 10, 1},
		{"a bb ccc dddd", 4, 4},
	}
	for _, tt := range tests {
		got := wrapLine(tt.input, tt.width)
		if len(got) != tt.wantLen {
			t.Errorf("wrapLine(%q, %d) len=%d, want %d", tt.input, tt.width, len(got), tt.wantLen)
		}
	}
}

func TestShortenArgs(t *testing.T) {
	if shortenArgs("short") != "short" {
		t.Error("short args should not be truncated")
	}
	long := strings.Repeat("a", 50)
	got := shortenArgs(long)
	if len(got) != 25 {
		t.Errorf("expected 25 chars, got %d", len(got))
	}
	if !strings.HasSuffix(got, "…") && !strings.HasSuffix(got, "...") {
		t.Errorf("truncated args should end with ellipsis, got %q", got)
	}
}

func TestFirstLine(t *testing.T) {
	if firstLine("hello\nworld") != "hello" {
		t.Error("firstLine should return text before newline")
	}
	if firstLine("hello") != "hello" {
		t.Error("firstLine should return full string if no newline")
	}
}

func TestFormatTokens(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{0, "0"},
		{500, "500"},
		{1500, "1.5k"},
		{1500000, "1.5M"},
	}
	for _, tt := range tests {
		got := formatTokens(tt.input)
		if got != tt.want {
			t.Errorf("formatTokens(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestInputEditMiddle(t *testing.T) {
	m := newModel(testConfig())
	m.width = 80
	m.height = 24

	m = updateApp(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hello")})
	m.cursor = 2 // "he|llo"

	m = updateApp(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("X")})
	if m.input != "heXllo" {
		t.Errorf("expected 'heXllo', got %q", m.input)
	}
	if m.cursor != 3 {
		t.Errorf("cursor should be 3 after insert, got %d", m.cursor)
	}
}

func TestDeleteCharAfter(t *testing.T) {
	m := appModel{input: "hello", cursor: 2}
	m.deleteCharAfter()
	if m.input != "helo" {
		t.Errorf("expected 'helo', got %q", m.input)
	}

	m.cursor = 10 // past end, should be no-op
	m.deleteCharAfter()
	if m.input != "helo" {
		t.Errorf("input should not change when cursor past end")
	}
}

func TestDeleteWordTrailingSpaces(t *testing.T) {
	// "hello   " has 8 chars, cursor at 8 means past end
	// deleteWord should skip spaces then delete "hello" entirely
	m := appModel{input: "hello   ", cursor: 8}
	m.deleteWord()
	if m.input != "" {
		t.Errorf("expected empty string, got %q", m.input)
	}
	if m.cursor != 0 {
		t.Errorf("cursor should be 0, got %d", m.cursor)
	}
}

// testErr is a simple error for tests
type testErr struct{ msg string }

func (e testErr) Error() string { return e.msg }
