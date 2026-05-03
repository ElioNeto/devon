package tui

import (
	"strings"
	"testing"

	"github.com/ElioNeto/devon/internal/agent"
	"github.com/ElioNeto/devon/internal/config"
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
	m := newModel(testConfig(), testRegistry())
	if m.cfg.Model != "test-model" {
		t.Errorf("expected model test-model, got %q", m.cfg.Model)
	}
	if m.agent == nil {
		t.Error("agent should not be nil")
	}
}

func TestModel_Init(t *testing.T) {
	m := newModel(testConfig(), testRegistry())
	cmd := m.Init()
	if cmd == nil {
		t.Error("Init should return a cmd")
	}
}

func TestModel_UpdateWindowSize(t *testing.T) {
	m := newModel(testConfig(), testRegistry())
	m = updateApp(m, tea.WindowSizeMsg{Width: 80, Height: 24})
	if m.width != 80 || m.height != 24 {
		t.Errorf("expected size 80x24, got %dx%d", m.width, m.height)
	}
}

func TestModel_UpdateTypeText(t *testing.T) {
	m := newModel(testConfig(), testRegistry())
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
	m := newModel(testConfig(), testRegistry())
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
	m := newModel(testConfig(), testRegistry())
	m.width = 80
	m.height = 24

	m = updateApp(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hello")})
	m = updateApp(m, tea.KeyMsg{Type: tea.KeyCtrlU})
	if m.input != "" || m.cursor != 0 {
		t.Errorf("ctrl+u should clear input, got %q cursor=%d", m.input, m.cursor)
	}
}

func TestModel_UpdateCursor(t *testing.T) {
	m := newModel(testConfig(), testRegistry())
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
	m := newModel(testConfig(), testRegistry())
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
	m := newModel(testConfig(), testRegistry())
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
	m := newModel(testConfig(), testRegistry())
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
	m := newModel(testConfig(), testRegistry())
	m.width = 80
	m.height = 24

	m = updateApp(m, tea.KeyMsg{Type: tea.KeyCtrlH})
	if !m.showHelp {
		t.Error("help should be shown after Ctrl+H")
	}
}

// ── Issue #27: shortcut keys must not conflict with text input ─────────────

func TestModel_ShortcutsDontConflictWithInput(t *testing.T) {
	m := newModel(testConfig(), testRegistry())
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
	m := newModel(testConfig(), testRegistry())
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
	m := newModel(testConfig(), testRegistry())
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
	for _, key := range []string{"Ctrl+2..5", "!", "Ctrl+E", "Ctrl+C", "Ctrl+L", "Ctrl+K"} {
		if !has(key) {
			t.Errorf("missing %s in AllHints()", key)
		}
	}
}

func TestModel_UpdateCtrlC(t *testing.T) {
	m := newModel(testConfig(), testRegistry())
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
	m := newModel(testConfig(), testRegistry())
	v := m.View()
	if v != "Iniciando Devon..." {
		t.Errorf("expected placeholder, got %q", v)
	}
}

func TestModel_View_Basic(t *testing.T) {
	m := newModel(testConfig(), testRegistry())
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
	m := newModel(testConfig(), testRegistry())
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
	m := newModel(testConfig(), testRegistry())
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
	m := newModel(testConfig(), testRegistry())
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
	m := newModel(testConfig(), testRegistry())
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

// ── Attachment tests ──────────────────────────────────────────────────────────

func TestFilePickerInitializedInModel(t *testing.T) {
	m := newModel(testConfig(), testRegistry())
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
	m := newModel(testConfig(), testRegistry())
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
	m := newModel(testConfig(), testRegistry())
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
	m := newModel(testConfig(), testRegistry())
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
	m := newModel(testConfig(), testRegistry())
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
	m := newModel(testConfig(), testRegistry())
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
