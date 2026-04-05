// Package tui implementa a interface com Bubble Tea para o Devon.
package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/ElioNeto/devon/internal/agent"
	"github.com/ElioNeto/devon/internal/config"
	"github.com/ElioNeto/devon/internal/cost"
	"github.com/ElioNeto/devon/internal/history"
	"github.com/ElioNeto/devon/internal/llm"
	"github.com/ElioNeto/devon/internal/tools"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// appModel is the main Bubble Tea model.
type appModel struct {
	width  int
	height int

	cfg     *config.Config
	agent   *agent.Agent
	session *history.Session
	tracker *cost.Session
	styles  uiStyles
	spinner spinner.Model

	// Left panel
	leftItems  []leftItem
	leftCursor int
	leftFocus  bool

	// Right panel
	rightView    rightView
	rightScroll  int
	expandedView bool

	// Log stream (right panel — Logs tab)
	logEvents []logEvent

	// Diff content (right panel — Diff tab)
	lastDiff string

	// Messages (Steps tab fallback)
	messages []chatMessage
	toolRuns []toolRun
	running  bool
	cancel   context.CancelFunc
	showHelp bool
	popup    string
	layout   layout

	// Current task label (shown in Tasks while running)
	currentTask string

	// Pending tasks (queue)
	pendingTasks []pendingTask

	// Navigation
	leftItemCount   int
	selectedTurnIdx int
	selectedTool    *toolRun

	// History turns
	historyTurns []historyTurn

	// Tool stats
	toolStats map[string]*toolStat

	// Memory facts
	memoryFacts []memoryFact

	// File changes
	fileChanges []fileChange

	// Context
	maxContextTokens int

	// Context menu
	ctxMenu    ctxMenuState
	showMenu   bool
	menuCursor int

	// Input
	input     string
	cursor    int
	scroll    int
	statusMsg string

	// Token history per turn
	tokenPerTurn []int
}

type chatMessage struct {
	Sender  string
	Content string
	IsError bool
}

type toolRun struct {
	Name       string
	Args       string
	Result     string
	Status     string // "running"|"done"|"error"
	DurationMs int
	StartedAt  int64
}

type toolStat struct {
	Calls int
	AvgMs int64
	MaxMs int64
}

type historyTurn struct {
	UserPrompt       string
	AgentReply       string
	ToolSummary      string
	Timestamp        string
	Elapsed          int64
	PromptTokens     int
	CompletionTokens int
}

type memoryFact struct {
	Category string
	Key      string
	Value    string
}

type pendingTask struct {
	Label  string
	Status string // "waiting"|"pending"|"error"
	Meta   string
}

// ── Initialization ────────────────────────────────────────────────────────────

func newModel(cfg *config.Config) appModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(colorPrimary)

	registry := tools.NewRegistry()
	client := llm.New(cfg.APIKey, cfg.BaseURL, cfg.Model, cfg.Timeout)
	agt := agent.New(cfg, client, registry)
	tracker := cost.NewSession(cfg.Model)

	maxCtx := 32000

	session, err := history.LoadLastSession(cfg.WorkDir)
	if err != nil {
		session = nil
	}

	return appModel{
		cfg:              cfg,
		agent:            agt,
		session:          session,
		tracker:          tracker,
		spinner:          s,
		styles:           newUIStyles(),
		layout:           calcLayout(0, 0),
		rightView:        viewLogs,
		toolStats:        make(map[string]*toolStat),
		selectedTurnIdx:  -1,
		leftFocus:        true,
		maxContextTokens: maxCtx,
	}
}

// ── Init ──────────────────────────────────────────────────────────────────────

func (m appModel) Init() tea.Cmd {
	welcome := "Devon pronto. ↑↓ navegar · Enter selecionar · x menu · ? ajuda"
	if m.session != nil {
		welcome = fmt.Sprintf("Sessão %s carregada.", m.session.ID)
	}
	return tea.Sequence(
		m.spinner.Tick,
		func() tea.Msg {
			return agentEventMsg(agent.Event{Type: "system", Text: welcome})
		},
	)
}

// ── Update ────────────────────────────────────────────────────────────────────

func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		if m.ctxMenu.visible && key.Type == tea.KeyEscape {
			m.ctxMenu.close()
			return m, nil
		}
		if m.popup != "" {
			m.popup = ""
			return m, nil
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.layout = calcLayout(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		if m.ctxMenu.visible {
			return m.handleCtxMenuKey(msg)
		}
		if m.showMenu {
			return m.updateMenu(msg)
		}
		if m.showHelp {
			m.showHelp = false
			return m, nil
		}
		return m.handleKey(msg)

	case agentEventMsg:
		m.processAgentEvent(agent.Event(msg))
		return m, m.spinner.Tick

	case agentResult:
		for _, ev := range msg.events {
			m.processAgentEvent(ev)
		}
		if m.session != nil {
			_ = history.SaveMessages(m.cfg.WorkDir, m.session.ID, m.agentMessages(), &m.session.Usage)
		}
		total := m.tracker.TotalInputTokens + m.tracker.TotalOutputTokens
		m.tokenPerTurn = append(m.tokenPerTurn, total)
		if m.cancel != nil {
			m.cancel()
			m.cancel = nil
		}
		m.running = false
		m.toolRuns = nil
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m appModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		if m.running && m.cancel != nil {
			m.cancel()
			m.running = false
			m.toolRuns = nil
			m.appendLog("system", "Agente interrompido.", "")
			return m, nil
		}
		return m, tea.Quit

	case "ctrl+l":
		m.messages = nil
		m.toolRuns = nil
		m.logEvents = nil
		m.scroll = 0
		return m, nil

	case "ctrl+k":
		m.messages = nil
		m.toolRuns = nil
		m.logEvents = nil
		m.fileChanges = nil
		m.scroll = 0
		m.tracker = cost.NewSession(m.cfg.Model)
		m.tokenPerTurn = nil
		var err error
		m.session, err = history.CreateSession(m.cfg.WorkDir)
		if err != nil {
			m.appendLog("system", "Erro ao criar sessão: "+err.Error(), "")
		} else {
			m.appendLog("system", "Nova sessão "+m.session.ID, "")
		}
		return m, nil

	case "tab":
		m.cycleSection()
		return m, nil

	case "shift+tab":
		m.cycleSectionBack()
		return m, nil

	case "up":
		if m.leftFocus {
			m.navigateLeft(-1)
		} else if m.rightScroll > 0 {
			m.rightScroll--
		}
		return m, nil

	case "down":
		if m.leftFocus {
			m.navigateLeft(1)
		} else {
			m.rightScroll++
		}
		return m, nil

	case "left":
		if !m.leftFocus {
			if len([]rune(m.input)) > 0 && m.cursor > 0 {
				m.cursor--
			} else {
				m.leftFocus = true
			}
		}
		return m, nil

	case "right":
		if m.leftFocus {
			m.leftFocus = false
		} else {
			if m.cursor < len([]rune(m.input)) {
				m.cursor++
			}
		}
		return m, nil

	case "pgup":
		if m.rightScroll > 0 {
			m.rightScroll -= 5
			if m.rightScroll < 0 {
				m.rightScroll = 0
			}
		}
		return m, nil

	case "pgdown":
		m.rightScroll += 5
		return m, nil

	case "1":
		m.rightView = viewLogs
		return m, nil
	case "2":
		m.rightView = viewDiff
		return m, nil
	case "3":
		m.rightView = viewConfig
		return m, nil
	case "4":
		m.rightView = viewSteps
		return m, nil

	case "enter":
		if m.leftFocus {
			m.selectLeftItem()
			return m, nil
		}
		return m.sendInput()

	case "x":
		m.handleCtxMenuOpen()
		return m, nil

	case "e":
		m.expandedView = !m.expandedView
		return m, nil

	case "?":
		m.showHelp = true
		return m, nil

	case "esc", "q":
		m.showMenu = false
		m.popup = ""
		m.showHelp = false
		return m, nil

	case "backspace":
		if !m.leftFocus && m.cursor > 0 {
			m.deleteCharBefore()
		}
		return m, nil

	case "ctrl+u":
		if !m.leftFocus {
			m.input = ""
			m.cursor = 0
		}
		return m, nil

	case "ctrl+w":
		if !m.leftFocus {
			m.deleteWord()
		}
		return m, nil

	case "home":
		m.cursor = 0
		return m, nil

	case "end":
		m.cursor = len([]rune(m.input))
		return m, nil

	default:
		if msg.Type == tea.KeyRunes && !m.leftFocus {
			for _, r := range msg.Runes {
				m.insertRune(r)
			}
		}
	}
	return m, nil
}

func (m appModel) updateMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	actions := contextMenuFor(&m)
	switch msg.String() {
	case "up":
		if m.menuCursor > 0 {
			m.menuCursor--
		}
	case "down":
		if m.menuCursor < len(actions)-1 {
			m.menuCursor++
		}
	case "enter":
		if m.menuCursor < len(actions) {
			actions[m.menuCursor].Action(&m)
		}
		m.showMenu = false
	case "esc", "q", "x":
		m.showMenu = false
	default:
		for _, a := range actions {
			if a.Key == msg.String() {
				a.Action(&m)
				m.showMenu = false
				break
			}
		}
	}
	return m, nil
}

// ── View ──────────────────────────────────────────────────────────────────────

func (m appModel) View() string {
	if m.height == 0 || m.width == 0 {
		return "Iniciando Devon..."
	}

	l := m.layout
	if l.width == 0 {
		l = calcLayout(m.width, m.height)
	}

	statusBarH := 1
	inputH := 3
	panelH := m.height - statusBarH - inputH
	if panelH < 5 {
		panelH = 5
	}

	leftW := l.leftPanelW
	if leftW <= 0 {
		leftW = m.width / 3
	}
	if leftW < 24 {
		leftW = 24
	}
	rightW := m.width - leftW
	if rightW < 20 {
		rightW = 20
	}

	leftPanel := renderLeftPanel(&m, leftW, panelH, m.leftFocus)
	rightPanel := renderRightPanel(&m, rightW, panelH, !m.leftFocus)
	panels := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)

	inputBar := renderInputBar(&m, m.width)
	statusBar := renderStatusBar(&m, m.width)

	view := lipgloss.JoinVertical(lipgloss.Left, panels, inputBar, statusBar)

	if m.showHelp {
		view = overlayCenter(view, renderHelp(&m, m.width), m.width, m.height)
	}
	if m.popup != "" {
		popupView := m.styles.menuStyle.Width(min(m.width-8, 70)).Render(m.popup)
		view = overlayCenter(view, popupView, m.width, m.height)
	}
	if m.ctxMenu.visible {
		view += "\n" + m.ctxMenu.render(m.width)
	}

	return view
}

func overlayCenter(base, overlay string, _, _ int) string {
	return base + "\n\n" + overlay
}

// ── Navigation ────────────────────────────────────────────────────────────────

func (m *appModel) navigateLeft(dir int) {
	items := buildLeftItems(m)
	m.leftItems = items
	if len(items) == 0 {
		return
	}
	next := m.leftCursor + dir
	for next >= 0 && next < len(items) && items[next].StatusKind == "header" {
		next += dir
	}
	if next >= 0 && next < len(items) {
		m.leftCursor = next
		m.syncRightView()
	}
}

func (m *appModel) cycleSection() {
	current := leftSection(-1)
	if m.leftCursor < len(m.leftItems) {
		current = m.leftItems[m.leftCursor].Section
	}
	nextSec := leftSection((int(current) + 1) % (int(secTokens) + 1))
	for i, item := range m.leftItems {
		if item.Section == nextSec && item.StatusKind != "header" {
			m.leftCursor = i
			m.syncRightView()
			return
		}
	}
}

func (m *appModel) cycleSectionBack() {
	current := leftSection(-1)
	if m.leftCursor < len(m.leftItems) {
		current = m.leftItems[m.leftCursor].Section
	}
	nextSec := leftSection((int(current) + int(secTokens)) % (int(secTokens) + 1))
	for i, item := range m.leftItems {
		if item.Section == nextSec && item.StatusKind != "header" {
			m.leftCursor = i
			m.syncRightView()
			return
		}
	}
}

func (m *appModel) selectLeftItem() {
	if m.leftCursor >= len(m.leftItems) {
		return
	}
	item := m.leftItems[m.leftCursor]
	if item.StatusKind == "header" {
		return
	}
	m.syncRightView()
	m.leftFocus = false
}

func (m *appModel) syncRightView() {
	if m.leftCursor >= len(m.leftItems) {
		return
	}
	item := m.leftItems[m.leftCursor]
	switch item.Section {
	case secTurno:
		if item.Index > 0 && item.Index-1 < len(m.toolRuns) {
			m.selectedTool = &m.toolRuns[item.Index-1]
		}
		m.rightView = viewLogs
	case secFerramentas:
		m.rightView = viewLogs
	case secMemoria:
		m.rightView = viewDiff
	case secTokens:
		m.rightView = viewConfig
	case secHistorico:
		m.selectedTurnIdx = item.Index
		m.rightView = viewSteps
	}
}

// ── Context menu ──────────────────────────────────────────────────────────────

func (m *appModel) handleCtxMenuOpen() {
	m.showMenu = true
	m.menuCursor = 0
}

func (m *appModel) handleCtxMenuKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc":
		m.ctxMenu.close()
		return m, nil
	case "up":
		if m.ctxMenu.cursor > 0 {
			m.ctxMenu.cursor--
		}
	case "down":
		if m.ctxMenu.cursor < len(m.ctxMenu.items)-1 {
			m.ctxMenu.cursor++
		}
	case "enter":
		if m.ctxMenu.cursor < len(m.ctxMenu.items) {
			action := m.ctxMenu.items[m.ctxMenu.cursor].Action
			m.ctxMenu.close()
			return m, m.executeCtxAction(action)
		}
	}
	return m, nil
}

func (m *appModel) executeCtxAction(action string) tea.Cmd {
	switch action {
	case "interrupt":
		if m.running && m.cancel != nil {
			m.cancel()
			m.running = false
			m.appendLog("system", "Agente interrompido.", "")
		}
	case "new_session":
		m.messages = nil
		m.toolRuns = nil
		m.logEvents = nil
		m.fileChanges = nil
	case "copy_response":
		if len(m.messages) > 0 {
			m.appendLog("system", "Resposta copiada.", "")
		}
	case "tool_input":
		if len(m.toolRuns) > m.leftCursor {
			tr := m.toolRuns[m.leftCursor]
			m.popup = fmt.Sprintf("Input de %s:\n  %s\n\nPressione tecla para fechar.", tr.Name, tr.Args)
		}
	case "tool_output":
		if len(m.toolRuns) > m.leftCursor {
			tr := m.toolRuns[m.leftCursor]
			m.popup = fmt.Sprintf("Output de %s:\n%s\n\nPressione tecla para fechar.", tr.Name, firstNLines(tr.Result, 30))
		}
	case "delete":
		if m.session != nil {
			if err := history.ClearSession(m.cfg.WorkDir, m.session.ID); err == nil {
				m.appendLog("system", "Sessão deletada.", "")
			}
		}
	default:
		m.appendLog("system", "Ação: "+action, "")
	}
	return nil
}

// ── Input / send ──────────────────────────────────────────────────────────────

func (m appModel) sendInput() (tea.Model, tea.Cmd) {
	text := strings.TrimSpace(m.input)
	if text == "" || m.running {
		return m, nil
	}
	m.input = ""
	m.cursor = 0
	m.statusMsg = ""

	if strings.HasPrefix(text, "/") {
		m.handleSlash(text)
		return m, nil
	}

	m.messages = append(m.messages, chatMessage{Sender: "user", Content: text})
	m.toolRuns = nil
	m.running = true
	m.currentTask = text
	m.rightView = viewLogs
	m.appendLog("agent", "Starting task: "+truncate(text, 50), "")

	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	cmd := startAgent(ctx, m.agent, text)
	return m, tea.Batch(m.spinner.Tick, cmd)
}

// ── Slash commands ────────────────────────────────────────────────────────────

func (m *appModel) handleSlash(text string) {
	switch {
	case text == "/history" || text == "/sessions":
		sessions, err := history.ListSessions(m.cfg.WorkDir)
		if err != nil {
			m.popup = "Erro: " + err.Error()
			return
		}
		if len(sessions) == 0 {
			m.popup = "Nenhuma sessão salva."
			return
		}
		var sb strings.Builder
		sb.WriteString("Sessões:\n")
		for _, id := range sessions {
			mark := " "
			if m.session != nil && m.session.ID == id {
				mark = "▶"
			}
			sb.WriteString(fmt.Sprintf("  %s %s\n", mark, id))
		}
		m.popup = sb.String()
	case text == "/clear":
		m.messages = nil
		m.toolRuns = nil
		m.historyTurns = nil
		m.logEvents = nil
		m.fileChanges = nil
	case text == "/usage" || text == "/cost":
		if m.tracker != nil {
			m.popup = m.tracker.Format()
		}
	case strings.HasPrefix(text, "/load "):
		id := strings.TrimSpace(strings.TrimPrefix(text, "/load"))
		if ses, err := history.LoadSession(m.cfg.WorkDir, id); err == nil {
			m.session = ses
			m.messages = nil
			m.logEvents = nil
			m.appendLog("system", "Sessão "+id+" carregada.", "")
			m.tracker = cost.NewSession(m.cfg.Model)
			m.tracker.TotalInputTokens = ses.Usage.PromptTokens
			m.tracker.TotalOutputTokens = ses.Usage.CompletionTokens
			m.tracker.TotalRequests = ses.Usage.Requests
			m.tracker.TotalCostUSD = cost.EstimateCost(m.cfg.Model, ses.Usage.PromptTokens, ses.Usage.CompletionTokens)
			m.tokenPerTurn = nil
		} else {
			m.appendLog("system", "Erro ao carregar sessão: "+err.Error(), "")
		}
	default:
		m.appendLog("system", "Comando desconhecido: "+text, "")
	}
}

// ── Agent event processing ────────────────────────────────────────────────────

func (m *appModel) processAgentEvent(ev agent.Event) {
	switch ev.Type {
	case "text":
		last := len(m.messages) - 1
		if last < 0 || m.messages[last].Sender != "devon" {
			m.messages = append(m.messages, chatMessage{Sender: "devon", Content: ev.Text})
		} else {
			m.messages[last].Content += ev.Text
		}
		m.appendLog("agent", truncate(ev.Text, 80), "")

	case "tool_start":
		m.toolRuns = append(m.toolRuns, toolRun{Name: ev.Tool, Args: ev.Args, Status: "running"})
		m.leftItemCount = len(m.toolRuns) + 3
		detail := ""
		if ev.Args != "" {
			detail = truncate(ev.Args, 40)
		}
		m.appendLog("tool", ev.Tool, detail)

	case "tool_done":
		for i, tr := range m.toolRuns {
			if tr.Name == ev.Tool && tr.Status == "running" {
				m.toolRuns[i].Result = ev.Result
				m.toolRuns[i].Status = "done"
				if st, ok := m.toolStats[ev.Tool]; ok {
					st.Calls++
				} else {
					m.toolStats[ev.Tool] = &toolStat{Calls: 1}
				}
				resultDetail := "… " + truncate(firstLine(ev.Result), 30)
				// captura diffs e mudanças de arquivo
				m.captureFileChange(ev.Tool, ev.Args, ev.Result)
				m.appendLog("tool", ev.Tool, resultDetail)
				break
			}
		}

	case "tool_error":
		for i, tr := range m.toolRuns {
			if tr.Name == ev.Tool && tr.Status == "running" {
				m.toolRuns[i].Result = ev.Err.Error()
				m.toolRuns[i].Status = "error"
				m.appendLog("warn", ev.Tool+" failed", ev.Err.Error())
				break
			}
		}

	case "turn_done":
		m.appendLog("ok", fmt.Sprintf("%d tests passing", len(m.toolRuns)), "")
		m.finalizeTurn()

	case "error":
		m.messages = append(m.messages, chatMessage{Sender: "devon", Content: "Erro: " + ev.Err.Error(), IsError: true})
		m.appendLog("warn", "Erro: "+ev.Err.Error(), "")
		m.finalizeTurn()

	case "system":
		m.messages = append(m.messages, chatMessage{Sender: "system", Content: ev.Text})
		m.appendLog("agent", ev.Text, "")
	}
}

// captureFileChange detecta tool calls de escrita/remoção e registra a mudança.
func (m *appModel) captureFileChange(tool, args, result string) {
	switch tool {
	case "write_file", "create_file":
		path := extractPath(args)
		if path == "" {
			return
		}
		status := "M"
		if strings.Contains(result, "created") {
			status = "A"
		}
		lines := countLines(result)
		m.upsertFileChange(path, status, lines)
		if strings.Contains(result, "diff") || strings.Contains(args, "diff") {
			m.lastDiff = result
		}
	case "delete_file", "rm":
		path := extractPath(args)
		if path == "" {
			return
		}
		m.upsertFileChange(path, "D", 0)
	}
}

func (m *appModel) upsertFileChange(path, status string, lines int) {
	for i, fc := range m.fileChanges {
		if fc.Path == path {
			m.fileChanges[i].Status = status
			m.fileChanges[i].Lines += lines
			return
		}
	}
	m.fileChanges = append(m.fileChanges, fileChange{Path: path, Status: status, Lines: lines})
}

// extractPath tenta extrair o path do campo args de uma tool call.
func extractPath(args string) string {
	// formato simples: primeiro token que parece um path
	for _, word := range strings.Fields(args) {
		if strings.Contains(word, "/") || strings.HasSuffix(word, ".ts") ||
			strings.HasSuffix(word, ".go") || strings.HasSuffix(word, ".js") ||
			strings.HasSuffix(word, ".json") || strings.HasSuffix(word, ".md") {
			return strings.Trim(word, `"',`)
		}
	}
	return ""
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

func (m *appModel) finalizeTurn() {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	m.running = false
	m.currentTask = ""

	prompt, reply, toolSummary := "", "", ""
	for _, msg := range m.messages {
		if msg.Sender == "user" && prompt == "" {
			prompt = msg.Content
		}
		if msg.Sender == "devon" {
			reply = msg.Content
		}
	}
	for _, tr := range m.toolRuns {
		toolSummary += tr.Name + " "
	}

	m.historyTurns = append(m.historyTurns, historyTurn{
		UserPrompt:  prompt,
		AgentReply:  reply,
		ToolSummary: strings.TrimSpace(toolSummary),
		Timestamp:   "agora",
	})

	if m.session != nil {
		_ = history.SaveMessages(m.cfg.WorkDir, m.session.ID, m.agentMessages(), &m.session.Usage)
	}
	m.toolRuns = nil
}

func (m *appModel) agentMessages() []llm.Message {
	var msgs []llm.Message
	for _, cm := range m.messages {
		switch cm.Sender {
		case "user":
			msgs = append(msgs, llm.Message{Role: llm.RoleUser, Content: cm.Content})
		case "devon":
			role := llm.RoleAssistant
			if cm.IsError {
				role = llm.RoleTool
			}
			msgs = append(msgs, llm.Message{Role: role, Content: cm.Content})
		}
	}
	return msgs
}

// ── Agent command ─────────────────────────────────────────────────────────────

func startAgent(ctx context.Context, a *agent.Agent, input string) tea.Cmd {
	return func() tea.Msg {
		ch := a.Run(ctx, input)
		var events []agent.Event
		for ev := range ch {
			events = append(events, ev)
		}
		return agentResult{events: events}
	}
}

// ── Input manipulation ────────────────────────────────────────────────────────

func (m *appModel) insertRune(r rune) {
	ru := []rune(m.input)
	ru = append(ru[:m.cursor], append([]rune{r}, ru[m.cursor:]...)...)
	m.input = string(ru)
	m.cursor++
}

func (m *appModel) deleteCharBefore() {
	if m.cursor <= 0 {
		return
	}
	ru := []rune(m.input)
	ru = append(ru[:m.cursor-1], ru[m.cursor:]...)
	m.input = string(ru)
	m.cursor--
}

func (m *appModel) deleteCharAfter() {
	ru := []rune(m.input)
	if m.cursor >= len(ru) {
		return
	}
	ru = append(ru[:m.cursor], ru[m.cursor+1:]...)
	m.input = string(ru)
}

func (m *appModel) deleteWord() {
	if m.cursor <= 0 {
		return
	}
	ru := []rune(m.input)
	pos := m.cursor
	for pos > 0 && ru[pos-1] == ' ' {
		pos--
	}
	for pos > 0 && ru[pos-1] != ' ' {
		pos--
	}
	ru = append(ru[:pos], ru[m.cursor:]...)
	m.input = string(ru)
	m.cursor = pos
}

// ── Utilities ────────────────────────────────────────────────────────────────

func shortenArgs(args string) string {
	if len(args) <= 25 {
		return args
	}
	return args[:22] + "…"
}

func firstLine(s string) string {
	if i := strings.Index(s, "\n"); i >= 0 {
		return s[:i]
	}
	return s
}

func firstNLines(s string, n int) string {
	lines := strings.SplitN(s, "\n", n+1)
	if len(lines) > n {
		return strings.Join(lines[:n], "\n") + "\n... (truncado)"
	}
	return s
}

func wrapLine(s string, width int) []string {
	if width <= 0 {
		return []string{s}
	}
	var lines []string
	for len([]rune(s)) > width {
		ru := []rune(s)
		idx := width
		for i := width - 1; i > 0; i-- {
			if ru[i] == ' ' {
				idx = i
				break
			}
		}
		lines = append(lines, string(ru[:idx]))
		s = strings.TrimLeft(string(ru[idx:]), " ")
	}
	if s != "" {
		lines = append(lines, s)
	}
	return lines
}

func formatTokens(n int) string { return formatShort(n) }

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func itoaF(n int) string {
	return fmt.Sprintf("%d", n)
}
