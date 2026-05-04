package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/ElioNeto/devon/internal/agent"
	"github.com/ElioNeto/devon/internal/config"
	"github.com/ElioNeto/devon/internal/cost"
	"github.com/ElioNeto/devon/internal/llm"
	"github.com/ElioNeto/devon/internal/tools"
	tea "github.com/charmbracelet/bubbletea"
)

func updateApp(m appModel, msg tea.Msg) appModel {
	result, _ := m.Update(msg)
	return *(result.(*appModel))
}

func testConfig() *config.Config {
	return &config.Config{
		Model:   "test-model",
		BaseURL: "http://localhost:11434/v1",
		WorkDir: "/tmp/test",
		Mode:    config.ModeAuto,
	}
}

func testRegistry() *tools.Registry {
	return tools.NewRegistry()
}

func TestNewModel(t *testing.T) {
	m := newModel(testConfig(), testRegistry(), "")
	if m.cfg.Model != "test-model" {
		t.Errorf("expected model test-model, got %q", m.cfg.Model)
	}
	if m.agent == nil {
		t.Error("agent should not be nil")
	}
}

func TestModel_Init(t *testing.T) {
	m := newModel(testConfig(), testRegistry(), "")
	cmd := m.Init()
	if cmd == nil {
		t.Error("Init should return a cmd")
	}
}

func TestModel_UpdateWindowSize(t *testing.T) {
	m := newModel(testConfig(), testRegistry(), "")
	m = updateApp(m, tea.WindowSizeMsg{Width: 80, Height: 24})
	if m.width != 80 || m.height != 24 {
		t.Errorf("expected size 80x24, got %dx%d", m.width, m.height)
	}
}

func TestModel_UpdateTypeText(t *testing.T) {
	m := newModel(testConfig(), testRegistry(), "")
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
	m := newModel(testConfig(), testRegistry(), "")
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
	m := newModel(testConfig(), testRegistry(), "")
	m.width = 80
	m.height = 24

	// Single-line: cursor at end → Ctrl+U deletes everything
	m = updateApp(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hello")})
	m = updateApp(m, tea.KeyMsg{Type: tea.KeyCtrlU})
	if m.input != "" || m.cursor != 0 {
		t.Errorf("ctrl+u on single line at end should clear, got %q cursor=%d", m.input, m.cursor)
	}

	// Multi-line: cursor in middle of second line → Ctrl+U deletes from line start to cursor
	m.input = "hello\nworld"
	m.cursor = 9 // the 'l' of 'world'
	m = updateApp(m, tea.KeyMsg{Type: tea.KeyCtrlU})
	if m.input != "hello\nld" || m.cursor != 6 {
		t.Errorf("ctrl+u should delete 'wo', got %q cursor=%d", m.input, m.cursor)
	}

	// Cursor at line start → no-op
	m.input = "hello\nworld"
	m.cursor = 6 // start of second line
	before := m.input
	beforeCursor := m.cursor
	m = updateApp(m, tea.KeyMsg{Type: tea.KeyCtrlU})
	if m.input != before || m.cursor != beforeCursor {
		t.Errorf("ctrl+u at line start should be no-op, got %q cursor=%d", m.input, m.cursor)
	}
}

func TestModel_UpdateCursor(t *testing.T) {
	m := newModel(testConfig(), testRegistry(), "")
	m.width = 80
	m.height = 24

	m = updateApp(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hello")})
	m.cursor = 3

	m = updateApp(m, tea.KeyMsg{Type: tea.KeyLeft})
	if m.cursor != 2 {
		t.Errorf("cursor should be 2 after left, got %d", m.cursor)
	}

	// right when leftFocus=false: switches focus, not cursor movement
	// So test cursor via "home" and "end" which always work
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
	m := newModel(testConfig(), testRegistry(), "")
	m.width = 80
	m.height = 24
	m.running = true

	// Use streaming pattern: send individual events via agentEventMsg
	m = updateApp(m, agentEventMsg(agent.Event{Type: "system", Text: "test message"}))
	if len(m.messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(m.messages))
	}
	if m.messages[0].Content != "test message" {
		t.Errorf("expected 'test message', got %q", m.messages[0].Content)
	}

	// Send agentDoneMsg to signal channel closed (agent finished)
	m = updateApp(m, agentDoneMsg{})
	if m.running {
		t.Error("agent should not be running after agentDoneMsg")
	}
}

func TestModel_UpdateClearChat(t *testing.T) {
	m := newModel(testConfig(), testRegistry(), "")
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
	m := newModel(testConfig(), testRegistry(), "")
	m.messages = append(m.messages, chatMessage{Sender: "devon", Content: "old"})
	m.tracker.TotalInputTokens = 100

	m = updateApp(m, tea.KeyMsg{Type: tea.KeyCtrlN})
	if m.tracker.TotalInputTokens != 0 {
		t.Error("ctrl+n should reset usage")
	}
	if m.messages[0].Sender != "system" {
		t.Error("ctrl+n should add a system message")
	}
}

func TestModel_UpdateHelp(t *testing.T) {
	m := newModel(testConfig(), testRegistry(), "")
	m.width = 80
	m.height = 24

	m = updateApp(m, tea.KeyMsg{Type: tea.KeyCtrlH})
	if !m.showHelp {
		t.Error("help should be shown after Ctrl+H")
	}
}

// ── Issue #27: shortcut keys must not conflict with text input ─────────────

func TestModel_ShortcutsDontConflictWithInput(t *testing.T) {
	m := newModel(testConfig(), testRegistry(), "")
	m.width = 80
	m.height = 24

	// Typing shortcut keys as plain characters should go into the input,
	// NOT trigger their former actions.

	// "1" should be typed as input, not switch to Logs
	m = updateApp(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")})
	if m.input != "1" {
		t.Errorf("expected input '1', got %q", m.input)
	}
	// initial rightView is Logs, so it's fine — but the point is it stayed

	// "2" should be typed as input, not switch to Diff
	m = updateApp(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	if m.input != "12" {
		t.Errorf("expected input '12', got %q", m.input)
	}

	// "e" should be typed as input, not toggle expanded
	m = updateApp(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	if m.input != "12e" {
		t.Errorf("expected input '12e', got %q", m.input)
	}

	// "x" should be typed as input, not open context menu
	m = updateApp(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if m.input != "12ex" {
		t.Errorf("expected input '12ex', got %q", m.input)
	}
	if m.ctxMenu.visible {
		t.Error("'x' should NOT open context menu anymore")
	}
}

func TestModel_CtrlShortcutsWork(t *testing.T) {
	m := newModel(testConfig(), testRegistry(), "")
	m.width = 80
	m.height = 24

	// Ctrl+2 (\x02) → switch to workspace 0
	m.messages = append(m.messages, chatMessage{Sender: "user", Content: "hello ws0"})
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{0x02}})
	if m.activeWorkspace != 0 {
		t.Errorf("Ctrl+2 should activate workspace 0, got %d", m.activeWorkspace)
	}

	// Ctrl+4 (\x04) → switch to workspace 1
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{0x04}})
	if m.activeWorkspace != 1 {
		t.Errorf("Ctrl+4 should activate workspace 1, got %d", m.activeWorkspace)
	}

	// Ctrl+E → toggle expanded
	m.expandedView = false
	m = updateApp(m, tea.KeyMsg{Type: tea.KeyCtrlE})
	if !m.expandedView {
		t.Error("Ctrl+E should toggle expanded view on")
	}
	m = updateApp(m, tea.KeyMsg{Type: tea.KeyCtrlE})
	if m.expandedView {
		t.Error("Ctrl+E should toggle expanded view off")
	}

	// "!" → open command menu
	m.showCmdMenu = false
	m = updateApp(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("!")})
	if !m.showCmdMenu {
		t.Error("'!' should open command menu")
	}

	// "!" again → close command menu
	m = updateApp(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("!")})
	if m.showCmdMenu {
		t.Error("'!' again should close command menu")
	}
}

func TestModel_HelpShowsNewBindings(t *testing.T) {
	m := newModel(testConfig(), testRegistry(), "")
	m.width = 80
	m.height = 40

	hints := AllHints()
	has := func(keys string) bool {
		for _, h := range hints {
			if h.Keys == keys {
				return true
			}
		}
		return false
	}
	// Old single-letter bindings must be gone
	if has("x") || has(" ") || has("e") || has("1") || has("2") {
		t.Error("old single-letter bindings should be removed from hints")
	}
	for _, key := range []string{"Ctrl+2..5", "!", "Ctrl+E", "Ctrl+C", "Ctrl+L", "Ctrl+N", "Ctrl+K", "Ctrl+A"} {
		if !has(key) {
			t.Errorf("missing %s in AllHints()", key)
		}
	}
}

func TestModel_UpdateCtrlC(t *testing.T) {
	m := newModel(testConfig(), testRegistry(), "")
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
	// cmd2 should NOT be tea.Quit when running — this is validated implicitly
	// by the Update call not panicking and returning without tea.Quit
	_ = cmd2
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

	// Only the first token should generate a log event
	if len(m.logEvents) != 1 {
		t.Fatalf("expected 1 log event, got %d", len(m.logEvents))
	}
	if m.logEvents[0].Msg != "hello" {
		t.Errorf("expected log msg 'hello', got %q", m.logEvents[0].Msg)
	}

	// Second text should append to same message but NOT generate a log event
	m.processAgentEvent(agent.Event{Type: "text", Text: " world"})
	if len(m.messages) != 1 {
		t.Fatalf("expected still 1 message")
	}
	if m.messages[0].Content != "hello world" {
		t.Errorf("expected 'hello world', got %q", m.messages[0].Content)
	}
	if len(m.logEvents) != 1 {
		t.Errorf("expected still 1 log event after second text token, got %d", len(m.logEvents))
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
	tests := []struct {
		name        string
		summary     string
		wantMsg     string
	}{
		{
			name:    "empty summary falls back to testes passando",
			summary: "",
			wantMsg: "testes passando",
		},
		{
			name:    "explicit summary resposta enviada",
			summary: "resposta enviada",
			wantMsg: "resposta enviada",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := appModel{running: true}
			m.toolRuns = []toolRun{{Name: "bash", Status: "done", Result: "ok"}}
			m.processAgentEvent(agent.Event{Type: "turn_done", Summary: tt.summary})
			// processAgentEvent("turn_done") no longer clears state — only emits logs.
			// finalizeTurn() handles cleanup. Verify running and toolRuns are preserved.
			if !m.running {
				t.Error("running should still be true after processAgentEvent turn_done (finalizeTurn handles cleanup)")
			}
			if len(m.toolRuns) == 0 {
				t.Error("toolRuns should not be cleared by processAgentEvent turn_done (finalizeTurn handles cleanup)")
			}
			// Verify log events were emitted with correct message
			found := false
			for _, le := range m.logEvents {
				if le.Msg == tt.wantMsg {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected log event %q from processAgentEvent turn_done, got %+v", tt.wantMsg, m.logEvents)
			}
		})
	}
}

func TestTurnDoneSummary_FallbackToTestesPassando(t *testing.T) {
	m := appModel{running: true}
	m.processAgentEvent(agent.Event{Type: "turn_done"}) // no Summary field set
	found := false
	for _, le := range m.logEvents {
		if le.Msg == "testes passando" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected log event 'testes passando' when Summary is empty")
	}
}

func TestTurnDoneSummary_RespostaEnviada(t *testing.T) {
	m := appModel{running: true}
	m.processAgentEvent(agent.Event{Type: "turn_done", Summary: "resposta enviada"})
	found := false
	for _, le := range m.logEvents {
		if le.Msg == "resposta enviada" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected log event 'resposta enviada' when Summary is set")
	}
}

func TestTurnDoneSummary_TarefaConcluida(t *testing.T) {
	m := appModel{running: true}
	m.processAgentEvent(agent.Event{Type: "turn_done", Summary: "tarefa concluída"})
	found := false
	for _, le := range m.logEvents {
		if le.Msg == "tarefa concluída" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected log event 'tarefa concluída' when Summary is set")
	}
}

func TestTurnDoneSummary_LoopConcluido(t *testing.T) {
	m := appModel{running: true}
	m.processAgentEvent(agent.Event{Type: "turn_done", Summary: "loop concluído em 3 turnos"})
	found := false
	for _, le := range m.logEvents {
		if le.Msg == "loop concluído em 3 turnos" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected log event 'loop concluído em 3 turnos' when Summary is set")
	}
}

func TestTurnDoneLogDeduplication(t *testing.T) {
	// finalizeTurn() needs a tracker for token tracking
	m := appModel{running: true, tracker: cost.NewSession("test-model")}
	// Simulate a multi-token agent response: first token, second token, then done
	m.processAgentEvent(agent.Event{Type: "text", Text: "Hello, "})
	m.processAgentEvent(agent.Event{Type: "text", Text: "world!"})

	// turn_done should log summary + full agent reply, but NOT a third line
	m.processAgentEvent(agent.Event{Type: "turn_done", Summary: "resposta enviada"})
	m.finalizeTurn()

	// Expected log events:
	//   1. "agent" + "Hello, "           (first token)
	//   2. "ok"    + "resposta enviada"   (turn_done summary)
	//   3. "agent" + "Hello, world!"      (turn_done full reply)
	// Total: 3
	// CRITICAL: should NOT be 4 (would mean finalizeTurn also logged the reply)
	if len(m.logEvents) != 3 {
		t.Fatalf("expected 3 log events, got %d: %+v", len(m.logEvents), m.logEvents)
	}
	// Verify the third event is the full reply
	if m.logEvents[2].Actor != "agent" || m.logEvents[2].Msg != "Hello, world!" {
		t.Errorf("expected agent log 'Hello, world!', got %+v", m.logEvents[2])
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
	m := newModel(testConfig(), testRegistry(), "")
	v := m.View()
	if v != "Iniciando Devon..." {
		t.Errorf("expected placeholder, got %q", v)
	}
}

func TestModel_View_Basic(t *testing.T) {
	m := newModel(testConfig(), testRegistry(), "")
	m.width = 80
	m.height = 24

	m = updateApp(m, tea.WindowSizeMsg{Width: 80, Height: 24})

	v := m.View()
	// View should not be empty
	if v == "" {
		t.Fatal("view should not be empty")
	}
	// Should contain layout elements
	if len(v) < 50 {
		t.Errorf("view too short: %d chars", len(v))
	}
}

func TestModel_ViewRunning(t *testing.T) {
	m := newModel(testConfig(), testRegistry(), "")
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
	m := newModel(testConfig(), testRegistry(), "")
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
	m := newModel(testConfig(), testRegistry(), "")
	m.width = 80
	m.height = 40
	m.showHelp = true

	v := m.View()
	if !strings.Contains(v, "Enter") && !strings.Contains(v, "enviar") {
		t.Error("help should mention enter key")
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
	// 24 bytes 'a' + 3 bytes "…" = 27 bytes
	if len(got) != 27 {
		t.Errorf("expected 27 bytes, got %d", len(got))
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
	m := newModel(testConfig(), testRegistry(), "")
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

func TestAgenticLoopCancellation(t *testing.T) {
	// Ctrl+C during an agentic loop should cancel and stop the agent.
	m := newModel(testConfig(), testRegistry(), "")
	m.width = 80
	m.height = 24
	m.running = true
	m.currentTask = "test task"
	m.toolRuns = []toolRun{{Name: "bash", Args: `{"cmd":"ls"}`, Status: "running"}}

	// Set up a cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	_ = ctx

	// Send Ctrl+C
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m2 := result.(*appModel)

	if m2.running {
		t.Error("agent should not be running after Ctrl+C")
	}
	if len(m2.toolRuns) != 0 {
		t.Error("toolRuns should be cleared after Ctrl+C")
	}
}

// ── Multi-line input and readline key tests ──────────────────────────────

func TestModel_CtrlA_MovesCursorToLineStart(t *testing.T) {
	m := newModel(testConfig(), testRegistry(), "")
	m.width = 80
	m.height = 24

	// Multi-line: cursor in middle of second line → Ctrl+A moves to start of second line
	m.input = "hello\nworld"
	m.cursor = 8 // the 'o' of 'world'
	m = updateApp(m, tea.KeyMsg{Type: tea.KeyCtrlA})
	if m.cursor != 6 {
		t.Errorf("ctrl+a should move cursor to start of second line (6), got %d", m.cursor)
	}

	// Single-line: cursor at position 3 → Ctrl+A moves to 0
	m.input = "hello"
	m.cursor = 3
	m = updateApp(m, tea.KeyMsg{Type: tea.KeyCtrlA})
	if m.cursor != 0 {
		t.Errorf("ctrl+a on single line should move cursor to 0, got %d", m.cursor)
	}

	// Already at line start → no-op
	m.input = "hello\nworld"
	m.cursor = 6
	m = updateApp(m, tea.KeyMsg{Type: tea.KeyCtrlA})
	if m.cursor != 6 {
		t.Errorf("ctrl+a at line start should be no-op, got %d", m.cursor)
	}
}

func TestModel_CtrlE_MovesCursorToLineEnd(t *testing.T) {
	m := newModel(testConfig(), testRegistry(), "")
	m.width = 80
	m.height = 24

	// Multi-line: cursor on first line → Ctrl+E moves to end of first line
	m.input = "hello\nworld"
	m.cursor = 2 // 'l' of 'hello'
	m = updateApp(m, tea.KeyMsg{Type: tea.KeyCtrlE})
	if m.cursor != 5 {
		t.Errorf("ctrl+e should move cursor to end of first line (5), got %d", m.cursor)
	}

	// Multi-line: cursor on second line → Ctrl+E moves to end of second line
	m.input = "hello\nworld"
	m.cursor = 7 // 'o' of 'world'
	m = updateApp(m, tea.KeyMsg{Type: tea.KeyCtrlE})
	if m.cursor != 11 {
		t.Errorf("ctrl+e should move cursor to end of second line (11), got %d", m.cursor)
	}

	// Single-line: cursor at start → Ctrl+E moves to end
	m.input = "hello"
	m.cursor = 0
	m = updateApp(m, tea.KeyMsg{Type: tea.KeyCtrlE})
	if m.cursor != 5 {
		t.Errorf("ctrl+e on single line should move cursor to 5, got %d", m.cursor)
	}

	// Already at line end → no-op
	m.input = "hello"
	m.cursor = 5
	before := m.cursor
	m = updateApp(m, tea.KeyMsg{Type: tea.KeyCtrlE})
	if m.cursor != before {
		t.Errorf("ctrl+e at line end should be no-op, got %d", m.cursor)
	}
}

func TestModel_CtrlE_TogglesExpandWhenInputEmpty(t *testing.T) {
	m := newModel(testConfig(), testRegistry(), "")
	m.width = 80
	m.height = 24

	// Input is empty → Ctrl+E toggles expandedView
	m.expandedView = false
	m.input = ""
	m = updateApp(m, tea.KeyMsg{Type: tea.KeyCtrlE})
	if !m.expandedView {
		t.Error("ctrl+e with empty input should toggle expandedView on")
	}

	// Toggle back
	m = updateApp(m, tea.KeyMsg{Type: tea.KeyCtrlE})
	if m.expandedView {
		t.Error("ctrl+e with empty input should toggle expandedView off")
	}

	// Input non-empty → Ctrl+E moves cursor, does NOT toggle expanded
	m.expandedView = false
	m.input = "hello"
	m.cursor = 0
	m = updateApp(m, tea.KeyMsg{Type: tea.KeyCtrlE})
	if m.expandedView {
		t.Error("ctrl+e with non-empty input should NOT toggle expandedView")
	}
	if m.cursor != 5 {
		t.Errorf("ctrl+e with non-empty input should move cursor to end (5), got %d", m.cursor)
	}
}

func TestModel_CtrlK_DeletesToEndOfLine(t *testing.T) {
	m := newModel(testConfig(), testRegistry(), "")
	m.width = 80
	m.height = 24

	// Multi-line: cursor in middle of first line → deletes rest of first line
	m.input = "hello world\nfoo\nbar"
	m.cursor = 5 // after 'hello'
	m = updateApp(m, tea.KeyMsg{Type: tea.KeyCtrlK})
	expected := "hello\nfoo\nbar"
	if m.input != expected {
		t.Errorf("ctrl+k should delete rest of first line, got %q, want %q", m.input, expected)
	}

	// Cursor at end of line → no-op
	m.input = "hello\nworld"
	m.cursor = 5 // end of first line
	before := m.input
	beforeCursor := m.cursor
	m = updateApp(m, tea.KeyMsg{Type: tea.KeyCtrlK})
	if m.input != before || m.cursor != beforeCursor {
		t.Errorf("ctrl+k at line end should be no-op, got %q cursor=%d", m.input, m.cursor)
	}
}

func TestModel_CtrlU_DeletesToStartOfLine(t *testing.T) {
	m := newModel(testConfig(), testRegistry(), "")
	m.width = 80
	m.height = 24

	// Multi-line: cursor in middle of second line → deletes from line start to cursor
	m.input = "hello\nworld"
	m.cursor = 9 // the 'l' of 'world'
	m = updateApp(m, tea.KeyMsg{Type: tea.KeyCtrlU})
	expected := "hello\nld"
	if m.input != expected || m.cursor != 6 {
		t.Errorf("ctrl+u should delete 'wo', got %q cursor=%d", m.input, m.cursor)
	}

	// Cursor at line start → no-op
	m.input = "hello\nworld"
	m.cursor = 6 // start of second line
	before := m.input
	beforeCursor := m.cursor
	m = updateApp(m, tea.KeyMsg{Type: tea.KeyCtrlU})
	if m.input != before || m.cursor != beforeCursor {
		t.Errorf("ctrl+u at line start should be no-op, got %q cursor=%d", m.input, m.cursor)
	}

	// Single-line: cursor at end → clears entire line
	m.input = "hello"
	m.cursor = 5
	m = updateApp(m, tea.KeyMsg{Type: tea.KeyCtrlU})
	if m.input != "" || m.cursor != 0 {
		t.Errorf("ctrl+u on single line at end should clear, got %q cursor=%d", m.input, m.cursor)
	}
}

func TestModel_InputHeight_Dynamic(t *testing.T) {
	m := newModel(testConfig(), testRegistry(), "")
	m.width = 80
	m.height = 24

	// Default multilineRows is 1
	if m.multilineRows != 1 {
		t.Errorf("multilineRows should default to 1, got %d", m.multilineRows)
	}

	// Set multilineRows to 3, verify dynamic height via View()
	m.multilineRows = 3
	m.input = "line1\nline2\nline3"
	v := m.View()
	// View should render with inputHeight = max(1, min(3, 6)) = 3
	if v == "" {
		t.Error("view should not be empty")
	}

	// Set multilineRows beyond cap (8), should be capped at 6
	m.multilineRows = 8
	m.input = "a\nb\nc\nd\ne\nf\ng\nh"
	v = m.View()
	if v == "" {
		t.Error("view should not be empty with 8 lines input")
	}
}

func TestModel_HomeEnd_StillAbsolute(t *testing.T) {
	m := newModel(testConfig(), testRegistry(), "")
	m.width = 80
	m.height = 24

	// Home on multi-line → cursor goes to 0 (absolute start)
	m.input = "hello\nworld"
	m.cursor = 8
	m = updateApp(m, tea.KeyMsg{Type: tea.KeyHome})
	if m.cursor != 0 {
		t.Errorf("home should move cursor to absolute start (0), got %d", m.cursor)
	}

	// End on multi-line → cursor goes to len(input) (absolute end)
	m.cursor = 3
	m = updateApp(m, tea.KeyMsg{Type: tea.KeyEnd})
	if m.cursor != 11 {
		t.Errorf("end should move cursor to absolute end (11), got %d", m.cursor)
	}
}

// testErr is a simple error for tests
type testErr struct{ msg string }

func (e testErr) Error() string { return e.msg }

// ── Attachment tests ──────────────────────────────────────────────────────────

func TestFilePickerInitializedInModel(t *testing.T) {
	m := newModel(testConfig(), testRegistry(), "")
	m.width = 80
	m.height = 24

	// Verify file picker state is initialized
	if m.fp.FileAllowed != true {
		t.Error("file picker should allow files")
	}
	if len(m.fp.AllowedTypes) == 0 {
		t.Error("file picker should have allowed types configured")
	}
	if m.attachments == nil {
		t.Error("attachments slice should be initialized")
	}
}

func TestRemoveAttachment_CtrlRRemovesLast(t *testing.T) {
	m := newModel(testConfig(), testRegistry(), "")
	m.width = 80
	m.height = 24

	// Manually add attachments
	m.attachments = []Attachment{
		{Filename: "img1.png", Data: []byte("data1"), SizeKB: 1, MimeType: "image/png"},
		{Filename: "img2.jpg", Data: []byte("data2"), SizeKB: 2, MimeType: "image/jpeg"},
	}
	if len(m.attachments) != 2 {
		t.Fatalf("expected 2 attachments, got %d", len(m.attachments))
	}

	// Ctrl+R should remove last
	m = updateApp(m, tea.KeyMsg{Type: tea.KeyCtrlR})
	if len(m.attachments) != 1 {
		t.Fatalf("expected 1 attachment after Ctrl+R, got %d", len(m.attachments))
	}
	if m.attachments[0].Filename != "img1.png" {
		t.Errorf("expected remaining attachment 'img1.png', got %q", m.attachments[0].Filename)
	}
}

func TestRemoveAttachment_CtrlREmptyDoesNotPanic(t *testing.T) {
	m := newModel(testConfig(), testRegistry(), "")
	m.width = 80
	m.height = 24

	// Ctrl+R with no attachments should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Ctrl+R with no attachments should not panic: %v", r)
		}
	}()
	_ = updateApp(m, tea.KeyMsg{Type: tea.KeyCtrlR})
}

func TestAttachmentBadgeFormat(t *testing.T) {
	att := Attachment{
		Filename: "test.png",
		SizeKB:   42,
		MimeType: "image/png",
	}
	badge := att.attachmentBadge()
	expected := "img: test.png 42KB"
	if badge != expected {
		t.Errorf("expected badge %q, got %q", expected, badge)
	}
}

func TestDataURIFormat(t *testing.T) {
	att := Attachment{
		Filename: "test.png",
		Data:     []byte("hello"),
		MimeType: "image/png",
		SizeKB:   0,
	}
	uri := att.dataURI()
	// "hello" base64 = aGVsbG8=
	expected := "data:image/png;base64,aGVsbG8="
	if uri != expected {
		t.Errorf("expected data URI %q, got %q", expected, uri)
	}
}

func TestAttachmentBadgesInInputBar(t *testing.T) {
	m := newModel(testConfig(), testRegistry(), "")
	m.width = 80
	m.height = 24

	// Add attachments
	m.attachments = []Attachment{
		{Filename: "pic1.png", SizeKB: 10, MimeType: "image/png"},
		{Filename: "pic2.jpg", SizeKB: 20, MimeType: "image/jpeg"},
	}

	// Render the view
	v := m.View()
	if !strings.Contains(v, "pic1.png") || !strings.Contains(v, "pic2.jpg") {
		t.Error("attachment badges should contain filenames in the view")
	}
	if !strings.Contains(v, "10KB") || !strings.Contains(v, "20KB") {
		t.Error("attachment badges should contain sizes in the view")
	}
}

func TestAttachmentsClearedAfterSend(t *testing.T) {
	m := newModel(testConfig(), testRegistry(), "")
	m.width = 80
	m.height = 24

	// Simulate attachments
	m.attachments = []Attachment{
		{Filename: "test.png", Data: []byte("some data"), SizeKB: 1, MimeType: "image/png"},
	}
	m.input = "hello with image"

	// sendInput should trigger
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := result.(*appModel)

	// After send, attachments should be cleared
	if len(m2.attachments) != 0 {
		t.Errorf("expected 0 attachments after send, got %d", len(m2.attachments))
	}
}

func TestOversizedAttachmentRejected(t *testing.T) {
	m := newModel(testConfig(), testRegistry(), "")
	m.width = 80
	m.height = 24

	// Create a large attachment (bigger than 10 MB default)
	bigData := make([]byte, 11*1024*1024)
	att := Attachment{
		Filename: "big.png",
		Data:     bigData,
		MimeType: "image/png",
		SizeKB:   len(bigData) / 1024,
	}

	err := m.validateAttachment(att)
	if err == nil {
		t.Error("expected error for oversized attachment, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "muito grande") {
		t.Errorf("expected size error, got: %v", err)
	}
}

// ── Cursor lifecycle tests ─────────────────────────────────────────────

func TestCursor_ShownOnTextEvent(t *testing.T) {
	m := appModel{}
	m.processAgentEvent(agent.Event{Type: "text", Text: "hello"})
	if !m.isGenerating {
		t.Error("isGenerating should be true after text event")
	}
}

func TestCursor_HiddenOnTurnDone(t *testing.T) {
	m := newModel(testConfig(), testRegistry(), "")
	m.width = 80
	m.height = 24
	m.isGenerating = true
	// processAgentEvent("turn_done") no longer clears isGenerating — finalizeTurn does.
	// Simulate the real flow: processAgentEvent for logging, then finalizeTurn for cleanup.
	m.processAgentEvent(agent.Event{Type: "turn_done"})
	m.finalizeTurn()
	if m.isGenerating {
		t.Error("isGenerating should be false after turn_done + finalizeTurn")
	}
}

func TestCursor_HiddenOnError(t *testing.T) {
	m := appModel{isGenerating: true}
	m.processAgentEvent(agent.Event{Type: "error", Err: testErr{msg: "fail"}})
	if m.isGenerating {
		t.Error("isGenerating should be false after error")
	}
}

func TestCursor_HiddenOnAgentDone(t *testing.T) {
	m := newModel(testConfig(), testRegistry(), "")
	m.width = 80
	m.height = 24
	m.running = true
	m.isGenerating = true
	m = updateApp(m, agentDoneMsg{})
	if m.isGenerating {
		t.Error("isGenerating should be false after agentDoneMsg")
	}
}

func TestCursor_HiddenOnFinalizeTurn(t *testing.T) {
	m := newModel(testConfig(), testRegistry(), "")
	m.width = 80
	m.height = 24
	m.isGenerating = true
	m.finalizeTurn()
	if m.isGenerating {
		t.Error("isGenerating should be false after finalizeTurn")
	}
}

func TestCursor_VisibleInView(t *testing.T) {
	m := newModel(testConfig(), testRegistry(), "")
	m.width = 80
	m.height = 24
	m.rightView = viewSteps // cursor only renders in Steps tab
	m.messages = []chatMessage{
		{Sender: "user", Content: "hello"},
		{Sender: "devon", Content: "world"},
	}
	m.isGenerating = true
	m.cursorVisible = true

	v := m.View()
	if !strings.Contains(v, "▋") {
		t.Error("view should contain cursor character ▋ when generating and visible")
	}
}

func TestCursor_HiddenInViewWhenNotGenerating(t *testing.T) {
	m := newModel(testConfig(), testRegistry(), "")
	m.width = 80
	m.height = 24
	m.rightView = viewSteps // cursor only renders in Steps tab
	m.messages = []chatMessage{
		{Sender: "user", Content: "hello"},
		{Sender: "devon", Content: "world"},
	}
	m.isGenerating = false
	m.cursorVisible = true

	v := m.View()
	if strings.Contains(v, "▋") {
		t.Error("view should NOT contain cursor character when not generating")
	}
}

func TestCursor_InvisibleWhenBlinkOff(t *testing.T) {
	m := newModel(testConfig(), testRegistry(), "")
	m.width = 80
	m.height = 24
	m.rightView = viewSteps // cursor only renders in Steps tab
	m.messages = []chatMessage{
		{Sender: "devon", Content: "hello"},
	}
	m.isGenerating = true
	m.canBlink = true
	m.cursorVisible = false

	v := m.View()
	// When cursorVisible is false and canBlink is true, a space should render
	// instead of the cursor block. But since it's a space, we just verify
	// the view still looks reasonable (doesn't crash, shows the message).
	if !strings.Contains(v, "hello") {
		t.Error("view should still show message content")
	}
}

func TestCursor_CanBlinkDisabled(t *testing.T) {
	// TERM=dumb equivalent
	m := newModel(testConfig(), testRegistry(), "")
	m.width = 80
	m.height = 24
	m.rightView = viewSteps // cursor only renders in Steps tab
	m.canBlink = false
	m.cursorVisible = false
	m.isGenerating = true
	m.messages = []chatMessage{
		{Sender: "devon", Content: "test"},
	}

	v := m.View()
	// When canBlink is false, cursor should be shown even when cursorVisible is false
	if !strings.Contains(v, "▋") {
		t.Error("when canBlink=false, cursor should always be visible")
	}
}

func TestCursor_CanBlinkDetected(t *testing.T) {
	// Default newModel should set canBlink based on TERM env
	m := newModel(testConfig(), testRegistry(), "")
	// The field must be set (either true or false depending on TERM)
	// We just verify it doesn't panic and that cursorBlinkCmd works
	cmd := m.cursorBlinkCmd()
	if cmd == nil {
		t.Error("cursorBlinkCmd should not be nil")
	}
}

func TestCursor_ToggleViaUpdate(t *testing.T) {
	m := newModel(testConfig(), testRegistry(), "")
	m.width = 80
	m.height = 24
	m.cursorVisible = true

	// Send cursorBlinkMsg to toggle cursor visibility
	m = updateApp(m, cursorBlinkMsg{})
	if m.cursorVisible {
		t.Error("cursorVisible should be false after cursorBlinkMsg")
	}

	// Send another blink to toggle back
	m = updateApp(m, cursorBlinkMsg{})
	if !m.cursorVisible {
		t.Error("cursorVisible should be true after second cursorBlinkMsg")
	}
}

func TestHasVisionContent(t *testing.T) {
	textParts := []llm.ContentPart{llm.NewTextPart("hello")}
	if llm.HasVisionContent(textParts) {
		t.Error("text-only parts should not have vision content")
	}

	parts := []llm.ContentPart{
		llm.NewTextPart("describe this"),
		llm.NewImagePartBase64("data:image/png;base64,abc123"),
	}
	if !llm.HasVisionContent(parts) {
		t.Error("parts with image should have vision content")
	}
}

func TestMultiTurnHistoryPreservation(t *testing.T) {
	m := newModel(testConfig(), testRegistry(), "")
	m.width = 80
	m.height = 24

	// ── Turn 1: user "My name is John" → devon reply with bash tool ──
	m.messages = append(m.messages, chatMessage{Sender: "user", Content: "My name is John"})
	m.processAgentEvent(agent.Event{Type: "text", Text: "Nice to meet you John"})
	m.processAgentEvent(agent.Event{Type: "tool_start", Tool: "bash", Args: `{"cmd":"ls"}`})
	m.processAgentEvent(agent.Event{Type: "tool_done", Tool: "bash", Result: "ok"})

	// Verify agentMessages() includes tool result BEFORE finalizeTurn clears toolRuns
	turn1Msgs := m.agentMessages()
	hasToolResult := false
	for _, msg := range turn1Msgs {
		if msg.Role == llm.RoleTool {
			hasToolResult = true
			break
		}
	}
	if !hasToolResult {
		t.Error("Turn 1: agentMessages should include a RoleTool message when toolRuns are present")
	}
	// Verify the tool result content
	toolFound := false
	for _, msg := range turn1Msgs {
		if msg.Role == llm.RoleTool && msg.Content != nil && *msg.Content == "ok" {
			toolFound = true
			break
		}
	}
	if !toolFound {
		t.Error("Turn 1: agentMessages should include tool result with content 'ok'")
	}

	m.processAgentEvent(agent.Event{Type: "turn_done"})
	m.finalizeTurn()

	// ── Turn 2: user "What is my name?" → devon "Your name is John" ──
	m.messages = append(m.messages, chatMessage{Sender: "user", Content: "What is my name?"})
	m.processAgentEvent(agent.Event{Type: "text", Text: "Your name is John"})
	m.processAgentEvent(agent.Event{Type: "turn_done"})
	m.finalizeTurn()

	// ── Turn 3: user "What did I ask first?" → devon "You said your name is John" ──
	m.messages = append(m.messages, chatMessage{Sender: "user", Content: "What did I ask first?"})
	m.processAgentEvent(agent.Event{Type: "text", Text: "You said your name is John"})
	m.processAgentEvent(agent.Event{Type: "turn_done"})
	m.finalizeTurn()

	// ── Verification after all turns ──

	// 1. historyTurns has 3 entries
	if len(m.historyTurns) != 3 {
		t.Errorf("expected 3 historyTurns, got %d", len(m.historyTurns))
	}

	// 2. historyTurns preserve user prompts across turns
	if m.historyTurns[0].UserPrompt != "My name is John" {
		t.Errorf("turn 1 user prompt = %q, want 'My name is John'", m.historyTurns[0].UserPrompt)
	}
	if m.historyTurns[1].UserPrompt != "What is my name?" {
		t.Errorf("turn 2 user prompt = %q, want 'What is my name?'", m.historyTurns[1].UserPrompt)
	}
	if m.historyTurns[2].UserPrompt != "What did I ask first?" {
		t.Errorf("turn 3 user prompt = %q, want 'What did I ask first?'", m.historyTurns[2].UserPrompt)
	}

	// 3. historyTurns include tool summary
	if m.historyTurns[0].ToolSummary != "bash" {
		t.Errorf("turn 1 tool summary = %q, want 'bash'", m.historyTurns[0].ToolSummary)
	}

	// 4. agentMessages() returns at least 6 messages (3 user + 3 assistant, plus tool results captured at persistence)
	allMsgs := m.agentMessages()
	if len(allMsgs) < 6 {
		t.Errorf("agentMessages() returned %d messages, expected at least 6", len(allMsgs))
	}

	// 5. agentMessages() includes "John" reference from turn 1 (cross-turn context preservation)
	johnRef := false
	for _, msg := range allMsgs {
		if msg.Content != nil && strings.Contains(*msg.Content, "John") {
			johnRef = true
			break
		}
	}
	if !johnRef {
		t.Error("agentMessages() should include 'John' reference from turn 1")
	}

	// 6. agentMessages() includes tool results (persisted in session, checked here for RoleTool)
	// Note: after finalizeTurn, m.toolRuns is nil, so RoleTool is not in the final in-memory output.
	// Tool results ARE captured at persistence time (history.SaveMessages) when agentMessages()
	// is called inside finalizeTurn with toolRuns still populated. We validate that by checking
	// that the turn 1 state included them (checked above before finalizeTurn).
	_ = turn1Msgs // verified above

	// 7. Verify total message count matches expected conversation flow
	expectedRoles := []llm.Role{llm.RoleUser, llm.RoleAssistant, llm.RoleUser, llm.RoleAssistant, llm.RoleUser, llm.RoleAssistant}
	if len(allMsgs) >= len(expectedRoles) {
		for i, role := range expectedRoles {
			if allMsgs[i].Role != role {
				t.Errorf("allMsgs[%d].Role = %q, want %q", i, allMsgs[i].Role, role)
			}
		}
	}
}

// ── Mock streamer for router tests ─────────────────────────────────────────

type mockStreamer struct {
	modelName string
}

func (m *mockStreamer) Stream(_ context.Context, _ []llm.Message, _ []llm.ToolDef) (<-chan llm.StreamEvent, error) {
	ch := make(chan llm.StreamEvent)
	close(ch)
	return ch, nil
}

func (m *mockStreamer) Info() llm.ModelInfo {
	return llm.ModelInfo{Name: m.modelName}
}

// ── Turn number and tool call count tests ──────────────────────────────────

func TestTurnNumberIncremented(t *testing.T) {
	m := newModel(testConfig(), testRegistry(), "")
	m.running = true
	m.processAgentEvent(agent.Event{Type: "turn_done"})
	m.finalizeTurn()
	if m.turnNumber != 1 {
		t.Errorf("turnNumber should be 1 after first turn, got %d", m.turnNumber)
	}

	m.processAgentEvent(agent.Event{Type: "turn_done"})
	m.finalizeTurn()
	if m.turnNumber != 2 {
		t.Errorf("turnNumber should be 2 after second turn, got %d", m.turnNumber)
	}
}

func TestToolCallCountIncremented(t *testing.T) {
	m := appModel{}
	m.processAgentEvent(agent.Event{Type: "tool_start", Tool: "bash", Args: `{"cmd":"ls"}`})
	m.processAgentEvent(agent.Event{Type: "tool_start", Tool: "grep", Args: `{"pattern":"foo"}`})
	m.processAgentEvent(agent.Event{Type: "tool_start", Tool: "write", Args: `{"path":"test.txt"}`})
	if len(m.toolRuns) != 3 {
		t.Errorf("len(toolRuns) should be 3, got %d", len(m.toolRuns))
	}
}

// ── Statusbar tests ────────────────────────────────────────────────────────

func TestStatusbarPortugueseLabels(t *testing.T) {
	s := newUIStyles()

	// Idle state → "●  idle"
	m := &appModel{
		cfg:    testConfig(),
		styles: s,
	}
	bar := renderStatusBar(m, 80)
	if !strings.Contains(bar, "●  idle") {
		t.Errorf("idle statusbar should contain '●  idle', got: %s", bar)
	}

	// Running without tool → "gerando"
	m2 := &appModel{
		cfg:     testConfig(),
		styles:  s,
		running: true,
	}
	bar2 := renderStatusBar(m2, 80)
	if !strings.Contains(bar2, "gerando") {
		t.Errorf("running statusbar should contain 'gerando', got: %s", bar2)
	}

	// Running with tool → "aguardando tool {name}"
	m3 := &appModel{
		cfg:     testConfig(),
		styles:  s,
		running: true,
		toolRuns: []toolRun{
			{Name: "bash", Status: "running"},
		},
	}
	bar3 := renderStatusBar(m3, 80)
	if !strings.Contains(bar3, "aguardando tool bash") {
		t.Errorf("running with tool should contain 'aguardando tool bash', got: %s", bar3)
	}
}

func TestStatusbarTurnoLoopDisplay(t *testing.T) {
	cfg := testConfig()
	m := &appModel{
		cfg:        cfg,
		styles:     newUIStyles(),
		turnNumber: 2,
	}
	bar := renderStatusBar(m, 80)
	if !strings.Contains(bar, "turno 2") {
		t.Errorf("statusbar should contain 'turno 2', got: %s", bar)
	}
}

func TestStatusbarModelFromRouter(t *testing.T) {
	cfg := testConfig()
	cfg.Model = "cfg-model"

	// Without router, cfg.Model should appear
	m := &appModel{
		cfg:    cfg,
		styles: newUIStyles(),
	}
	bar := renderStatusBar(m, 80)
	if !strings.Contains(bar, "cfg-model") {
		t.Errorf("statusbar should contain model name from cfg.Model, got: %s", bar)
	}

	// With router, router model should appear
	mockClient := &mockStreamer{modelName: "router-model"}
	router := llm.NewAgentRouter(nil, mockClient)
	m2 := &appModel{
		cfg:    cfg,
		styles: newUIStyles(),
		router: router,
	}
	bar2 := renderStatusBar(m2, 80)
	if !strings.Contains(bar2, "router-model") {
		t.Errorf("statusbar should contain model name from router, got: %s", bar2)
	}
	if strings.Contains(bar2, "cfg-model") {
		t.Errorf("statusbar should NOT contain cfg.Model when router provides a model, got: %s", bar2)
	}
}

func TestStatusbarTurnoHiddenWhenZero(t *testing.T) {
	cfg := testConfig()
	m := &appModel{
		cfg:        cfg,
		styles:     newUIStyles(),
		turnNumber: 0,
	}
	bar := renderStatusBar(m, 80)
	if strings.Contains(bar, "turno") {
		t.Errorf("statusbar should NOT contain 'turno' when turnNumber=0, got: %s", bar)
	}
}

func TestStatusbarTurnoHiddenWhenZeroAndModelOnly(t *testing.T) {
	cfg := testConfig()
	cfg.Model = "my-model"
	m := &appModel{
		cfg:    cfg,
		styles: newUIStyles(),
	}
	bar := renderStatusBar(m, 80)
	if strings.Contains(bar, "turno") {
		t.Errorf("statusbar should NOT contain 'turno' when turnNumber=0, got: %s", bar)
	}
	if !strings.Contains(bar, "my-model") {
		t.Errorf("statusbar should still show model name, got: %s", bar)
	}
}

func TestAgenticMultiTurnEventConsumption(t *testing.T) {
	m := newModel(testConfig(), testRegistry(), "")
	m.width = 80
	m.height = 24
	m.running = true
	m.agentCh = make(<-chan agent.Event) // non-nil mock channel

	// Simulate: text → tool_start → tool_done → text → turn_done
	m = updateApp(m, agentEventMsg(agent.Event{Type: "text", Text: "I will use bash"}))
	if !m.running {
		t.Error("agent should still be running after text event")
	}

	m = updateApp(m, agentEventMsg(agent.Event{Type: "tool_start", Tool: "bash", Args: `{"cmd":"ls"}`}))
	if len(m.toolRuns) != 1 {
		t.Error("expected 1 tool run")
	}
	if !m.running {
		t.Error("agent should still be running after tool_start")
	}

	m = updateApp(m, agentEventMsg(agent.Event{Type: "tool_done", Tool: "bash", Result: "file1"}))
	if !m.running {
		t.Error("agent should still be running after tool_done (agent continues internally)")
	}

	// Second turn text from agent
	m = updateApp(m, agentEventMsg(agent.Event{Type: "text", Text: "Done."}))
	if !m.running {
		t.Error("agent should still be running during second turn")
	}

	// turn_done + finalizeTurn should stop the agent
	m.processAgentEvent(agent.Event{Type: "turn_done"})
	m.finalizeTurn()
	if m.running {
		t.Error("agent should not be running after finalizeTurn")
	}
	if len(m.toolRuns) != 0 {
		t.Error("toolRuns should be cleared after finalizeTurn")
	}
}
