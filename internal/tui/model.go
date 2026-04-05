// Package tui implements a multi-panel Bubble Tea TUI for Devon.
// Layout inspired by lazydocker: left navigation panel + dynamic right panel.
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

// ── Estado da aplicação ──────────────────────────────────────────────────────

type appModel struct {
	// Dimensões do terminal
	width  int
	height int

	// Deps
	cfg     *config.Config
	agent   *agent.Agent
	session *history.Session
	tracker *cost.Session
	styles  uiStyles
	spinner spinner.Model

	// Painel esquerdo
	leftItems  []leftItem
	leftCursor int
	leftFocus  bool // true = foco no painel esquerdo

	// Painel direito
	rightView    rightView
	rightScroll  int
	expandedView bool

	// Dados do turno atual
	messages []chatMessage
	toolRuns []toolRun
	running  bool
	cancel   context.CancelFunc

	// Ferramenta selecionada no painel direito
	selectedTool    *toolRun
	selectedTurnIdx int

	// Histórico de turnos (para o painel lateral)
	historyTurns []historyTurn

	// Estatísticas de ferramentas
	toolStats map[string]*toolStat

	// Memória (placeholder para issue #22)
	memoryFacts []memoryFact

	// Input
	input       string
	cursor      int
	pendingInput string

	// UI state
	showHelp  bool
	showMenu  bool
	menuCursor int
	statusMsg string
	popup     string
}

type chatMessage struct {
	Sender  string // "user" | "devon" | "system"
	Content string
	IsError bool
}

type toolRun struct {
	Name   string
	Args   string
	Result string
	Status string // "running" | "done" | "error"
}

type toolStat struct {
	Calls int
	AvgMs int64
	MaxMs int64
	totalMs int64
}

type historyTurn struct {
	UserPrompt      string
	AgentReply      string
	ToolSummary     string
	Timestamp       string
	PromptTokens    int
	CompletionTokens int
	DurationMs      int64
}

type memoryFact struct {
	Category string
	Key      string
	Value    string
}

// ── Inicialização ────────────────────────────────────────────────────────────

func newModel(cfg *config.Config) appModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(colorPrimary)

	registry := tools.NewRegistry()
	client := llm.New(cfg.APIKey, cfg.BaseURL, cfg.Model, cfg.Timeout)
	agt := agent.New(cfg, client, registry)
	tracker := cost.NewSession(cfg.Model)

	session, err := history.LoadLastSession(cfg.WorkDir)
	if err != nil {
		session = nil
	}

	return appModel{
		cfg:             cfg,
		agent:           agt,
		session:         session,
		tracker:         tracker,
		spinner:         s,
		styles:          newUIStyles(),
		leftFocus:       true,
		rightView:       viewTurnoAtivo,
		selectedTurnIdx: -1,
		toolStats:       make(map[string]*toolStat),
	}
}

// ── Init ─────────────────────────────────────────────────────────────────────

func (m appModel) Init() tea.Cmd {
	welcome := "Devon pronto. Use ↑↓ para navegar, Enter para selecionar, x para menu."
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

// ── Mensagens do Bubble Tea ──────────────────────────────────────────────────

type agentEventMsg agent.Event
type agentResult struct{ events []agent.Event }

// ── Update ───────────────────────────────────────────────────────────────────

func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Menu de contexto aberto
	if m.showMenu {
		return m.updateMenu(msg)
	}

	// Popup (histórico, uso etc)
	if m.popup != "" {
		if _, ok := msg.(tea.KeyMsg); ok {
			m.popup = ""
			return m, nil
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.updateKey(msg)

	case agentEventMsg:
		m.processAgentEvent(agent.Event(msg))
		return m, m.spinner.Tick

	case agentResult:
		m.finalizeAgentResult(msg)
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m appModel) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.showHelp {
		m.showHelp = false
		return m, nil
	}

	switch msg.String() {
	case "ctrl+c":
		if m.running && m.cancel != nil {
			m.cancel()
			m.running = false
			m.toolRuns = nil
			m.messages = append(m.messages, chatMessage{Sender: "system", Content: "Agente interrompido."})
			return m, nil
		}
		return m, tea.Quit

	case "ctrl+l":
		m.messages = nil
		m.toolRuns = nil
		return m, nil

	case "ctrl+k":
		m.messages = nil
		m.toolRuns = nil
		m.historyTurns = nil
		m.tracker = cost.NewSession(m.cfg.Model)
		var err error
		m.session, err = history.CreateSession(m.cfg.WorkDir)
		if err != nil {
			m.messages = append(m.messages, chatMessage{Sender: "system", Content: "Erro ao criar sessão: " + err.Error()})
		} else {
			m.messages = append(m.messages, chatMessage{Sender: "system", Content: "Nova sessão " + m.session.ID})
		}
		return m, nil

	case "tab":
		// Ciclar seção no painel esquerdo
		m.cycleSection()
		return m, nil

	case "left":
		if !m.leftFocus {
			m.leftFocus = true
		}
		return m, nil

	case "right":
		if m.leftFocus {
			m.leftFocus = false
		}
		return m, nil

	case "up":
		if m.leftFocus {
			m.navigateLeft(-1)
		} else {
			if m.rightScroll > 0 {
				m.rightScroll--
			}
		}
		return m, nil

	case "down":
		if m.leftFocus {
			m.navigateLeft(1)
		} else {
			m.rightScroll++
		}
		return m, nil

	case "pgup":
		if m.rightScroll > 0 {
			m.rightScroll -= 10
			if m.rightScroll < 0 {
				m.rightScroll = 0
			}
		}
		return m, nil

	case "pgdown":
		m.rightScroll += 10
		return m, nil

	case "enter":
		if m.leftFocus {
			m.selectLeftItem()
			return m, nil
		}
		// Foco no input (painel direito)
		return m.sendInput()

	case "x":
		actions := contextMenuFor(&m)
		if len(actions) > 0 {
			m.showMenu = true
			m.menuCursor = 0
		}
		return m, nil

	case "e":
		m.expandedView = !m.expandedView
		return m, nil

	case "?":
		m.showHelp = true
		return m, nil

	case "q", "esc":
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

	default:
		// Qualquer tecla de texto vai para o input (independente do foco)
		if msg.Type == tea.KeyRunes && !m.leftFocus {
			for _, r := range msg.Runes {
				m.insertRune(r)
			}
		}
		// Se há pendingInput (de re-executar), manda pro input
		if m.pendingInput != "" && !m.running {
			m.input = m.pendingInput
			m.cursor = len([]rune(m.input))
			m.pendingInput = ""
			m.leftFocus = false
		}
	}

	return m, nil
}

func (m appModel) updateMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	actions := contextMenuFor(&m)
	if kMsg, ok := msg.(tea.KeyMsg); ok {
		switch kMsg.String() {
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
			// Atalho de tecla direto
			for _, a := range actions {
				if a.Key == kMsg.String() {
					a.Action(&m)
					m.showMenu = false
					break
				}
			}
		}
	}
	return m, nil
}

func (m *appModel) navigateLeft(dir int) {
	items := m.leftItems
	if len(items) == 0 {
		return
	}
	next := m.leftCursor + dir
	// Pula cabeçalhos de seção
	for next >= 0 && next < len(items) && items[next].Icon == "─" {
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
	nextSec := (current + 1) % (secTokens + 1)
	for i, item := range m.leftItems {
		if item.Section == nextSec && item.Icon != "─" {
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
	if item.Icon == "─" {
		return
	}
	m.syncRightView()
	m.leftFocus = false // move foco para painel direito após selecionar
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
			m.rightView = viewToolCall
		} else {
			m.selectedTool = nil
			m.rightView = viewTurnoAtivo
		}
	case secHistorico:
		m.selectedTurnIdx = item.Index
		m.rightView = viewHistoricoTurno
	case secFerramentas:
		m.rightView = viewFerramentas
	case secMemoria:
		m.rightView = viewMemoria
	case secTokens:
		m.rightView = viewTokens
	}
}

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
	m.rightView = viewTurnoAtivo

	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	cmd := startAgent(ctx, m.agent, text)
	return m, tea.Batch(m.spinner.Tick, cmd)
}

// ── View ──────────────────────────────────────────────────────────────────────

func (m appModel) View() string {
	if m.height == 0 || m.width == 0 {
		return "Iniciando Devon..."
	}

	const leftW = 28
	rightW := m.width - leftW - 2 // 2 = border chars
	if rightW < 20 {
		rightW = 20
	}

	statusH := 1
	inputH := 3
	helpH := 0
	if m.showHelp {
		helpH = 1
	}
	panelH := m.height - statusH - inputH - helpH
	if panelH < 5 {
		panelH = 5
	}

	var sb strings.Builder

	// Status bar
	sb.WriteString(renderStatusBar(&m, m.width))
	sb.WriteString("\n")

	// Painéis lado a lado
	left := renderLeftPanel(&m, leftW, panelH, m.leftFocus)
	right := renderRightPanel(&m, rightW, panelH, !m.leftFocus)
	sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, left, right))
	sb.WriteString("\n")

	// Barra de input
	sb.WriteString(renderInputBar(&m, m.width))

	// Ajuda
	if m.showHelp {
		sb.WriteString("\n")
		sb.WriteString(renderHelp(&m, m.width))
	}

	result := sb.String()

	// Menu de contexto como overlay
	if m.showMenu {
		return renderContextMenu(&m, m.width, m.height)
	}

	return result
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
	case "tool_start":
		m.toolRuns = append(m.toolRuns, toolRun{Name: ev.Tool, Args: ev.Args, Status: "running"})
		if _, ok := m.toolStats[ev.Tool]; !ok {
			m.toolStats[ev.Tool] = &toolStat{}
		}
	case "tool_done":
		for i, tr := range m.toolRuns {
			if tr.Name == ev.Tool && tr.Status == "running" {
				m.toolRuns[i].Result = ev.Result
				m.toolRuns[i].Status = "done"
				if st, ok := m.toolStats[ev.Tool]; ok {
					st.Calls++
				}
				break
			}
		}
	case "tool_error":
		for i, tr := range m.toolRuns {
			if tr.Name == ev.Tool && tr.Status == "running" {
				m.toolRuns[i].Result = ev.Err.Error()
				m.toolRuns[i].Status = "error"
				break
			}
		}
	case "turn_done":
		m.finalizeTurn()
	case "error":
		m.messages = append(m.messages, chatMessage{Sender: "devon", Content: "Erro: " + ev.Err.Error(), IsError: true})
		m.finalizeTurn()
	case "system":
		m.messages = append(m.messages, chatMessage{Sender: "system", Content: ev.Text})
	}
}

func (m *appModel) finalizeTurn() {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	m.running = false

	// Salva turno no histórico
	prompt := ""
	reply := ""
	for _, msg := range m.messages {
		if msg.Sender == "user" && prompt == "" {
			prompt = msg.Content
		}
		if msg.Sender == "devon" {
			reply = msg.Content
		}
	}
	toolSummary := ""
	for _, tr := range m.toolRuns {
		toolSummary += tr.Name + " "
	}

	m.historyTurns = append(m.historyTurns, historyTurn{
		UserPrompt:  prompt,
		AgentReply:  reply,
		ToolSummary: strings.TrimSpace(toolSummary),
		Timestamp:   "agora",
	})

	// Salva sessão
	if m.session != nil {
		_ = history.SaveMessages(m.cfg.WorkDir, m.session.ID, m.agentMessages(), &m.session.Usage)
	}
	m.toolRuns = nil
}

func (m *appModel) finalizeAgentResult(res agentResult) {
	for _, ev := range res.events {
		m.processAgentEvent(ev)
	}
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
	case text == "/usage" || text == "/cost":
		m.popup = m.tracker.Format()
	default:
		m.messages = append(m.messages, chatMessage{Sender: "system", Content: "Comando desconhecido: " + text})
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

// ── Utilities ─────────────────────────────────────────────────────────────────

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

func itoaF(n int) string {
	return fmt.Sprintf("%d", n)
}
