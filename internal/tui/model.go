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

// appModel is the main Bubble Tea model with multi-panel layout.
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
	showHelp bool
	spinner  spinner.Model
	styles   styles
	popup    string     // overlay for /history, /usage etc
	layout   layout     // calculated layout
	expands  bool       // if detailed view is expanded

	// Painel esquerdo: navegação
	leftFocus      leftPanelSection
	leftSectionIdx int // índice da seção visível (0..N)
	leftItemCount  int // quantos itens na seção (para navegação)
	leftCursor     int // posição do cursor na seção

	// Painel direito: conteúdo
	rightContent rightPanel

	// Menu de contexto
	ctxMenu ctxMenuState

	// Histórico de tokens por turno (para gráfico)
	tokenPerTurn []int // tokens por turno
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

type styles struct {
	title       lipgloss.Style
	info        lipgloss.Style
	statusBar   lipgloss.Style
	leftPanel   lipgloss.Style
	rightPanel  lipgloss.Style
	panelSep    lipgloss.Style
	userMsg     lipgloss.Style
	devonMsg    lipgloss.Style
	sysMsg      lipgloss.Style
	errMsg      lipgloss.Style
	toolRunning lipgloss.Style
	toolDone    lipgloss.Style
	toolError   lipgloss.Style
	inputPrefix lipgloss.Style
	cursorS     lipgloss.Style
	helpText    lipgloss.Style
	popup       lipgloss.Style
	sectionSel  lipgloss.Style // selected section tab
	sectionTab  lipgloss.Style // non-selected section tab
	highlight   lipgloss.Style
	barChart    lipgloss.Style
	tokenChart  lipgloss.Style
	focusBorder lipgloss.Style
}

func newStyles() styles {
	s := styles{}
	s.title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#8B5CF6"))
	s.info = lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF"))
	s.statusBar = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF")).
		Background(lipgloss.Color("#1E1E2E")).
		Padding(0, 1)
	s.leftPanel = lipgloss.NewStyle().PaddingLeft(1)
	s.rightPanel = lipgloss.NewStyle().PaddingLeft(1).PaddingRight(1).PaddingTop(0)
	s.panelSep = lipgloss.NewStyle().Foreground(lipgloss.Color("#334155"))
	s.userMsg = lipgloss.NewStyle().Foreground(lipgloss.Color("#93C5FD")).PaddingLeft(1)
	s.devonMsg = lipgloss.NewStyle().Foreground(lipgloss.Color("#86EFAC")).PaddingLeft(1)
	s.sysMsg = lipgloss.NewStyle().Foreground(lipgloss.Color("#A5B4FC")).Italic(true).PaddingLeft(1)
	s.errMsg = lipgloss.NewStyle().Foreground(lipgloss.Color("#FCA5A5")).PaddingLeft(1)
	s.toolRunning = lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBF24"))
	s.toolDone = lipgloss.NewStyle().Foreground(lipgloss.Color("#4ADE80"))
	s.toolError = lipgloss.NewStyle().Foreground(lipgloss.Color("#F87171"))
	s.inputPrefix = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#8B5CF6"))
	s.cursorS = lipgloss.NewStyle().Foreground(lipgloss.Color("#8B5CF6"))
	s.helpText = lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF")).Faint(true)
	s.popup = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#E2E8F0")).
		Background(lipgloss.Color("#1E293B")).
		Padding(1, 2).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#8B5CF6"))
	s.sectionSel = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#8B5CF6")).
		Bold(true).
		PaddingRight(1).
		BorderBottom(true).
		BorderBottomForeground(lipgloss.Color("#8B5CF6"))
	s.sectionTab = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280")).
		PaddingRight(1)
	s.highlight = lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBF24"))
	s.barChart = lipgloss.NewStyle().Foreground(lipgloss.Color("#3B82F6"))
	s.tokenChart = lipgloss.NewStyle()
	s.focusBorder = lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#475569")).
		Padding(0, 1)
	return s
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
		cfg:        cfg,
		agent:      agt,
		session:    session,
		tracker:    tracker,
		spinner:    s,
		styles:     newStyles(),
		layout:     calcLayout(0, 0),
		rightContent: RightChat,
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

func (m *appModel) handleSlash(text string) {
	switch {
	case text == "/history" || text == "/sessions":
		m.leftFocus = SectionHistory
		m.leftCursor = 0
		m.showHistoryPopup()
	case text == "/clear":
		if m.session != nil {
			_ = history.ClearSession(m.cfg.WorkDir, m.session.ID)
		}
		m.messages = nil
		m.toolRuns = nil
		m.scroll = 0
		m.tracker = cost.NewSession(m.cfg.Model)
		m.tokenPerTurn = nil
		var err error
		m.session, err = history.LoadLastSession(m.cfg.WorkDir)
		if err == nil {
			m.messages = append(m.messages, chatMessage{Sender: "system", Content: "Sessao limpa. Nova sessao " + m.session.ID})
		}
	case text == "/usage" || text == "/cost":
		m.showUsagePopup()
	case strings.HasPrefix(text, "/load "):
		id := strings.TrimSpace(strings.TrimPrefix(text, "/load"))
		if ses, err := history.LoadSession(m.cfg.WorkDir, id); err == nil {
			m.session = ses
			m.messages = nil
			m.messages = append(m.messages, chatMessage{Sender: "system", Content: "Sessao " + id + " carregada. Mensagens: " + itoaF(len(ses.Messages))})
			m.tracker = cost.NewSession(m.cfg.Model)
			m.tracker.TotalInputTokens = ses.Usage.PromptTokens
			m.tracker.TotalOutputTokens = ses.Usage.CompletionTokens
			m.tracker.TotalRequests = ses.Usage.Requests
			m.tracker.TotalCostUSD = cost.EstimateCost(m.cfg.Model, ses.Usage.PromptTokens, ses.Usage.CompletionTokens)
			m.tokenPerTurn = nil
		} else {
			m.messages = append(m.messages, chatMessage{Sender: "system", Content: "Erro ao carregar sessao: " + err.Error()})
		}
	case text == "/context":
		m.rightContent = RightContext
	default:
		m.messages = append(m.messages, chatMessage{Sender: "system", Content: "Comando desconhecido. Use: /history /load <id> /clear /usage /context"})
	}
}

func (m *appModel) showHistoryPopup() {
	sessions, err := history.ListSessions(m.cfg.WorkDir)
	if err != nil {
		m.popup = "Erro ao listar sessoes: " + err.Error()
		return
	}
	if len(sessions) == 0 {
		m.popup = "Nenhuma sessao salva."
		return
	}
	sb := strings.Builder{}
	sb.WriteString("Sessoes salvas:\n")
	for _, id := range sessions {
		marker := " "
		if m.session != nil && m.session.ID == id {
			marker = ">"
		}
		sb.WriteString(fmt.Sprintf("  %s %s\n", marker, id))
	}
	sb.WriteString("\nUse /load <id> para retomar ou ESC para fechar.")
	m.popup = sb.String()
}

func (m *appModel) showUsagePopup() {
	if m.tracker == nil {
		m.popup = "Sem dados de uso."
		return
	}
	m.popup = m.tracker.Format() + "\n\nPressione qualquer tecla para fechar."
}

// --- Update ---

func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle escape (close menus)
	if key, ok := msg.(tea.KeyMsg); ok {
		// Close context menu on Escape
		if m.ctxMenu.visible && key.Type == tea.KeyEscape {
			m.ctxMenu.close()
			return m, nil
		}
		// Dismiss popup on any key
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
		// If context menu is visible, handle its keys
		if m.ctxMenu.visible {
			return m.handleCtxMenuKey(msg)
		}

		if m.showHelp {
			m.showHelp = false
			return m, nil
		}

		switch msg.String() {
		case KeyQuit:
			if m.running && m.cancel != nil {
				m.cancel()
				m.running = false
				m.toolRuns = nil
				m.messages = append(m.messages, chatMessage{Sender: "system", Content: "Agente interrompido."})
				return m, nil
			}
			return m, tea.Quit

		case KeyClear:
			m.messages = nil
			m.toolRuns = nil
			m.scroll = 0
			return m, nil

		case KeyNewSession:
			m.messages = nil
			m.toolRuns = nil
			m.scroll = 0
			m.tracker = cost.NewSession(m.cfg.Model)
			m.tokenPerTurn = nil
			var err error
			m.session, err = history.CreateSession(m.cfg.WorkDir)
			if err != nil {
				m.messages = append(m.messages, chatMessage{Sender: "system", Content: "Erro ao criar sessao: " + err.Error()})
			} else {
				m.messages = append(m.messages, chatMessage{Sender: "system", Content: "Nova sessao " + m.session.ID})
			}
			return m, nil

		case "tab":
			// Cycle left panel section
			m.leftFocus = leftPanelSection((int(m.leftFocus) + 1) % 4)
			m.leftCursor = 0
			m.rightContent = m.mapSectionToRightPanel()
			return m, nil

		case "shift+tab":
			m.leftFocus = leftPanelSection((int(m.leftFocus) + 3) % 4)
			m.leftCursor = 0
			m.rightContent = m.mapSectionToRightPanel()
			return m, nil

		case KeyUp:
			if m.leftCursor > 0 {
				m.leftCursor--
			}
			return m, nil

		case KeyDown:
			if m.leftCursor < m.leftItemCount-1 {
				m.leftCursor++
			}
			return m, nil

		case KeyContext:
			m.handleCtxMenuOpen()
			return m, nil

		case KeyExpand:
			m.expands = !m.expands
			return m, nil

		case KeySend:
			if len(m.input) == 0 || m.running {
				return m, nil
			}
			text := m.input
			m.input = ""
			m.cursor = 0

			// Check for slash commands
			if strings.HasPrefix(text, "/") {
				m.handleSlash(text)
				return m, nil
			}

			m.messages = append(m.messages, chatMessage{Sender: "user", Content: text})
			m.scroll = 0
			m.toolRuns = nil
			m.running = true
			ctx, cancel := context.WithCancel(context.Background())
			m.cancel = cancel
			cmd := startAgent(ctx, m.agent, text)
			return m, tea.Batch(m.spinner.Tick, cmd)

		case KeyClearInput:
			m.input = ""
			m.cursor = 0

		case KeyDeleteWord, KeyDeleteWordBS:
			m.deleteWord()

		case KeyBackspace:
			if m.cursor > 0 {
				m.deleteCharBefore()
			}

		case KeyDelete:
			if m.cursor < len([]rune(m.input)) {
				m.deleteCharAfter()
			}

		case "left":
			if m.cursor > 0 {
				m.cursor--
			}

		case "right":
			if m.cursor < len([]rune(m.input)) {
				m.cursor++
			}

		case "home":
			m.cursor = 0

		case "end":
			m.cursor = len([]rune(m.input))

		case KeyPageUp:
			if len(m.messages) > 0 {
				m.scroll++
			}

		case KeyPageDown:
			if m.scroll > 0 {
				m.scroll--
			}

		case KeyHelp:
			m.showHelp = true
			return m, nil

		default:
			if msg.Type == tea.KeyRunes {
				for _, r := range msg.Runes {
					m.insertRune(r)
				}
			}
		}

	case agentEventMsg:
		m.processAgentEvent(agent.Event(msg))
		return m, m.spinner.Tick

	case agentResult:
		for _, ev := range msg.events {
			m.processAgentEvent(ev)
		}
		// Save session
		if m.session != nil {
			messages := m.agentMessages()
			if err := history.SaveMessages(m.cfg.WorkDir, m.session.ID, messages, &m.session.Usage); err != nil {
				m.messages = append(m.messages, chatMessage{Sender: "system", Content: "Erro ao salvar sessao: " + err.Error()})
			}
		}
		// Track tokens per turn
		total := m.tracker.TotalInputTokens + m.tracker.TotalOutputTokens
		m.tokenPerTurn = append(m.tokenPerTurn, total)

		if m.cancel != nil {
			m.cancel()
			m.cancel = nil
		}
		m.running = false
		m.toolRuns = nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *appModel) mapSectionToRightPanel() rightPanel {
	switch m.leftFocus {
	case SectionSession:
		return RightSession
	case SectionHistory:
		return RightHistory
	case SectionTools:
		return RightTools
	case SectionTokens:
		return RightTokens
	default:
		return RightChat
	}
}

func (m *appModel) handleCtxMenuOpen() {
	// Build context menu based on current state
	switch m.leftFocus {
	case SectionSession:
		if m.running {
			cm := buildContextMenu("turn_active", "")
			if cm != nil {
				m.ctxMenu = *cm
			}
		} else if m.session != nil {
			cm := buildContextMenu("session", m.session.ID)
			if cm != nil {
				m.ctxMenu = *cm
			}
		}
	case SectionTools:
		// Context menu for tool calls
		if len(m.toolRuns) > 0 {
			idx := m.leftCursor % len(m.toolRuns)
			cm := buildContextMenu("tool_call", m.toolRuns[idx].Name)
			if cm != nil {
				m.ctxMenu = *cm
			}
		}
	default:
		// Generic
		if m.session != nil {
			cm := buildContextMenu("session", m.session.ID)
			if cm != nil {
				m.ctxMenu = *cm
			}
		}
	}
}

// handleCtxMenuKey handles key events when context menu is visible.
func (m *appModel) handleCtxMenuKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc":
		m.ctxMenu.close()
		return m, nil
	case KeyUp:
		if m.ctxMenu.cursor > 0 {
			m.ctxMenu.cursor--
		}
	case KeyDown:
		if m.ctxMenu.cursor < len(m.ctxMenu.items)-1 {
			m.ctxMenu.cursor++
		}
	case KeySend:
		// Execute selected action
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
			m.messages = append(m.messages, chatMessage{Sender: "system", Content: "Agente interrompido."})
		}
	case "new_session":
		return func() tea.Msg { return tea.KeyMsg{Type: tea.KeyCtrlK} }
	case "copy_response":
		if len(m.messages) > 0 {
			last := m.messages[len(m.messages)-1]
			_ = last // would copy to clipboard in real impl
			m.messages = append(m.messages, chatMessage{Sender: "system", Content: "Resposta copiada."})
		}
	case "tool_input":
		if len(m.toolRuns) > m.leftCursor {
			tr := m.toolRuns[m.leftCursor]
			m.popup = fmt.Sprintf("Input de %s:\n  %s\n\nPressione tecla para fechar.", tr.Name, tr.Args)
		}
	case "tool_output":
		if len(m.toolRuns) > m.leftCursor {
			tr := m.toolRuns[m.leftCursor]
			lines := firstNLines(tr.Result, 30)
			m.popup = fmt.Sprintf("Output de %s:\n%s\n\nPressione tecla para fechar.", tr.Name, lines)
		}
	case "delete":
		if m.session != nil {
			if err := history.ClearSession(m.cfg.WorkDir, m.session.ID); err == nil {
				m.messages = append(m.messages, chatMessage{Sender: "system", Content: "Sessao deletada."})
			}
		}
	default:
		m.messages = append(m.messages, chatMessage{Sender: "system", Content: "Acao: " + action})
	}
	return nil
}

// agentMessages returns the current messages in a format suitable for history.
func (m *appModel) agentMessages() []llm.Message {
	var msgs []llm.Message
	for _, cm := range m.messages {
		switch cm.Sender {
		case "user":
			msgs = append(msgs, llm.Message{Role: llm.RoleUser, Content: cm.Content})
		case "devon":
			role := llm.RoleAssistant
			if cm.IsError {
				role = llm.RoleTool // treat errors as tool messages
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

	var sb strings.Builder

	// Header
	sb.WriteString(m.renderHeader())
	sb.WriteString("\n" + strings.Repeat("─", m.width-2) + "\n")

	// Multi-panel area
	sb.WriteString(m.renderMultiPanel())

	// Input bar
	sb.WriteString("─\n")
	if m.running {
		sb.WriteString(" " + m.spinner.View() + " ")
	} else {
		sb.WriteString(m.styles.inputPrefix.Render(" > "))
	}
	sb.WriteString(m.renderInputLine())

	// Status bar
	sb.WriteString("\n")
	sb.WriteString(m.renderStatusBar())

	// Help
	if m.showHelp {
		sb.WriteString("\n")
		sb.WriteString(renderHelp(&m, m.width))
	}

	// Popup overlay
	if m.popup != "" {
		sb.WriteString("\n\n")
		sb.WriteString(m.styles.popup.Render(m.popup))
	}

	// Context menu overlay
	if m.ctxMenu.visible {
		sb.WriteString("\n\n")
		sb.WriteString(m.ctxMenu.render(m.width))
	}

	return sb.String()
}

// renderStatusBar renders the bottom status line.
func (m *appModel) renderStatusBar() string {
	section := m.leftFocus.String()
	mode := "padrao"
	if m.expands {
		mode = "expandido"
	}
	return m.styles.statusBar.Render(
		fmt.Sprintf(" aba: %s  modo: %s  sessao: %s", section, mode, m.shortSessionID()),
	)
}

func (m *appModel) shortSessionID() string {
	if m.session == nil {
		return "(nenhuma)"
	}
	n := len(m.session.ID)
	if n > 8 {
		n = 8
	}
	return m.session.ID[:n]
}

// --- Multi-panel rendering ---

func (m *appModel) renderMultiPanel() string {
	l := m.layout
	if l.width == 0 {
		l = calcLayout(m.width, m.height)
	}

	leftW := l.leftPanelW
	rightW := l.width - leftW
	if rightW < 20 {
		rightW = 20
		leftW = m.width - rightW
	}

	height := l.height - l.headerH - l.separatorH - l.inputH - l.statusBarH - 1
	if height < 1 {
		height = 1
	}

	leftLines := m.renderLeftPanel(leftW, height)
	rightLines := m.renderRightPanel(m.rightContent, rightW, height)

	// Side-by-side alignment
	leftBlock := m.styles.leftPanel.Render(leftLines)
	rightBlock := m.styles.rightPanel.Render(rightLines)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftBlock, m.styles.panelSep.Render(" │ "), rightBlock)
}

// renderLeftPanel renders the left panel with section tabs and items.
func (m *appModel) renderLeftPanel(w, h int) string {
	var sb strings.Builder

	// Section tabs
	tabs := []leftPanelSection{SectionSession, SectionHistory, SectionTools, SectionTokens}
	tabParts := make([]string, len(tabs))
	for i, t := range tabs {
		label := t.String()
		if t == m.leftFocus {
			tabParts[i] = m.styles.sectionSel.Render("▸ " + label)
		} else {
			tabParts[i] = m.styles.sectionTab.Render("  " + label)
		}
	}
	sb.WriteString(strings.Join(tabParts, " "))
	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("─", w) + "\n")

	// Section content (scrollable within remaining space)
	contentH := h - 2
	content := m.renderCurrentSection(w, contentH)

	// Apply scroll
	contentLines := strings.Split(content, "\n")
	if len(contentLines) > contentH {
		start := m.leftCursor
		if start < 0 {
			start = 0
		}
		end := start + contentH
		if end > len(contentLines) {
			end = len(contentLines)
			start = end - contentH
			if start < 0 {
				start = 0
			}
		}
		contentLines = contentLines[start:end]
	}

	for _, line := range contentLines {
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	return sb.String()
}

func (m *appModel) renderCurrentSection(w, h int) string {
	switch m.leftFocus {
	case SectionSession:
		return m.renderSessionSection(w, h)
	case SectionHistory:
		return m.renderHistorySection(w, h)
	case SectionTools:
		return m.renderToolsSection(w, h)
	case SectionTokens:
		return m.renderTokensSection(w, h)
	default:
		return "(vazio)"
	}
}

func (m *appModel) renderSessionSection(w, h int) string {
	var lines []string

	// Session info
	if m.session != nil {
		lines = append(lines, m.styles.highlight.Render("  ID: "+m.session.ID[:min(16, len(m.session.ID))]))
		lines = append(lines, fmt.Sprintf("  Mensagens: %d", len(m.messages)))
		lines = append(lines, fmt.Sprintf("  Ferramentas usadas: %d", len(m.toolRuns)))
	} else {
		lines = append(lines, m.styles.highlight.Render("  Sem sessão ativa"))
	}

	// Recent tool runs summary
	lines = append(lines, "")
	lines = append(lines, "  Ferramentas ativas:")
	if len(m.toolRuns) == 0 {
		lines = append(lines, "    (nenhuma)")
		if m.running {
			lines = append(lines, "    "+m.spinner.View()+" agente pensando...")
		}
	}
	for i, tr := range m.toolRuns {
		prefix := "  "
		if i == m.leftCursor {
			prefix = m.styles.highlight.Render("▸ ")
		} else {
			prefix = "  "
		}
		switch tr.Status {
		case "running":
			lines = append(lines, prefix+m.spinner.View()+" "+tr.Name+" "+shortenArgs(tr.Args))
		case "done":
			lines = append(lines, prefix+"✓ "+tr.Name+" "+shortenArgs(tr.Args))
		case "error":
			lines = append(lines, prefix+"✗ "+tr.Name+": "+firstLine(tr.Result))
		default:
			lines = append(lines, prefix+tr.Name)
		}
	}

	m.leftItemCount = len(lines)
	return strings.Join(lines, "\n")
}

func (m *appModel) renderHistorySection(w, h int) string {
	var lines []string

	sessions, err := history.ListSessions(m.cfg.WorkDir)
	if err != nil {
		lines = append(lines, " Erro ao listar sessoes: "+err.Error())
		m.leftItemCount = len(lines)
		return strings.Join(lines, "\n")
	}

	if len(sessions) == 0 {
		lines = append(lines, " Nenhuma sessao salva.")
		lines = append(lines, "")
		lines = append(lines, " Use /history para ver sessoes salvas.")
		m.leftItemCount = len(lines)
		return strings.Join(lines, "\n")
	}

	for i, id := range sessions {
		prefix := "  "
		current := ""
		if m.session != nil && m.session.ID == id {
			prefix = " ▸ "
			current = " (atual)"
		}
		if i == m.leftCursor {
			prefix = m.styles.highlight.Render("▸ ")
		}
		lines = append(lines, prefix+id[:min(16, len(id))]+current)
	}

	lines = append(lines, "")
	lines = append(lines, " Use x para acoes, /load <id> para carregar")
	m.leftItemCount = len(lines)
	return strings.Join(lines, "\n")
}

func (m *appModel) renderToolsSection(w, h int) string {
	// Collect tool usage stats
	type toolStat struct {
		name  string
		count int
	}
	seen := map[string]*toolStat{}
	for _, tr := range m.toolRuns {
		if _, ok := seen[tr.Name]; !ok {
			seen[tr.Name] = &toolStat{name: tr.Name}
		}
		seen[tr.Name].count++
	}

	// Calculate bar widths
	labelW := 12
	if w-labelW-5 > 10 {
		labelW = 12
	}
	barW := w - labelW - 5

	rows := make([]BarRow, 0, len(seen))
	names := []string{"bash", "read", "write", "glob", "grep"}
	for _, name := range names {
		if stat, ok := seen[name]; ok {
			rows = append(rows, BarRow{
				Label: name,
				Value: stat.count,
				Meta:  fmt.Sprintf("  %dx", stat.count),
			})
			delete(seen, name)
		}
	}
	for name, stat := range seen {
		rows = append(rows, BarRow{
			Label: name,
			Value: stat.count,
			Meta:  fmt.Sprintf("  %dx", stat.count),
		})
	}

	lines := m.layout.BarChartASCII(rows, barW)
	m.leftItemCount = len(strings.Split(lines, "\n"))
	return lines
}

func (m *appModel) renderTokensSection(w, h int) string {
	var lines []string

	tokens := m.tracker.TotalInputTokens + m.tracker.TotalOutputTokens
	costStr := cost.FormatCost(m.tracker.TotalCostUSD)

	lines = append(lines, m.styles.highlight.Render("  Consumo de tokens"))
	lines = append(lines, fmt.Sprintf("  Total: %s tokens  Custo: %s", formatTokens(tokens), costStr))
	lines = append(lines, "")

	if len(m.tokenPerTurn) > 0 {
		lines = append(lines, "  Tokens por turno:")
		// Sparkline
		w2 := w - 4
		if w2 > 30 {
			w2 = 30
		}
		spark := Sparkline(m.tokenPerTurn, w2)
		lines = append(lines, "  "+spark)

		for i, v := range m.tokenPerTurn {
			lines = append(lines, fmt.Sprintf("    Turno %d: %s tokens", i+1, formatTokens(v)))
		}
	} else {
		lines = append(lines, "  Nenhum turno registrado ainda.")
	}

	m.leftItemCount = len(lines)
	return strings.Join(lines, "\n")
}

// renderRightPanel renders the right panel content based on the active type.
func (m *appModel) renderRightPanel(typ rightPanel, w, h int) string {
	switch typ {
	case RightSession:
		return m.renderRightSession(w, h)
	case RightHistory:
		return m.renderRightHistory(w, h)
	case RightTools:
		return m.renderRightTools(w, h)
	case RightTokens:
		return m.renderRightTokens(w, h)
	case RightContext:
		return m.renderRightContext(w, h)
	case RightMemory:
		return m.renderRightMemory(w, h)
	default:
		return m.renderRightChat(w, h)
	}
}

func (m *appModel) renderRightChat(w, h int) string {
	// Chat area with messages scrolling
	maxChatH := h
	if maxChatH < 1 {
		maxChatH = 1
	}

	// Build chat lines
	chat := m.buildChatLines(maxChatH)

	var sb strings.Builder
	for _, line := range chat {
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	if len(chat) == 0 {
		sb.WriteString(m.styles.info.Render("  Aguardando mensagem..."))
		sb.WriteString("\n")
	}

	return sb.String()
}

func (m *appModel) renderRightSession(w, h int) string {
	var lines []string

	sessionTitle := "  Sessao Atual"
	if m.session != nil {
		sessionTitle = "  Sessao: " + m.session.ID[:min(16, len(m.session.ID))]
	}
	lines = append(lines, m.styles.highlight.Render(sessionTitle))
	lines = append(lines, "  "+strings.Repeat("─", w-4))

	if m.session == nil {
		lines = append(lines, "  Nenhuma sessao carregada.")
	} else {
		lines = append(lines, fmt.Sprintf("  Criada: %s", m.session.CreatedAt.Format("2006-01-02 15:04")))
		lines = append(lines, fmt.Sprintf("  Atualizada: %s", m.session.UpdatedAt.Format("2006-01-02 15:04")))
		lines = append(lines, "")
		lines = append(lines, "  Mensagens na sessão:")
		lines = append(lines, fmt.Sprintf("    Prompt tokens: %d", m.session.Usage.PromptTokens))
		lines = append(lines, fmt.Sprintf("    Completion: %d", m.session.Usage.CompletionTokens))
		lines = append(lines, fmt.Sprintf("    Requests: %d", m.session.Usage.Requests))
	}

	return strings.Join(lines, "\n")
}

func (m *appModel) renderRightHistory(w, h int) string {
	sessions, err := history.ListSessions(m.cfg.WorkDir)
	if err != nil {
		return "  Erro: " + err.Error()
	}

	var lines []string
	lines = append(lines, m.styles.highlight.Render("  Historico de Sessoes"))
	lines = append(lines, "  "+strings.Repeat("─", w-4))

	if len(sessions) == 0 {
		lines = append(lines, "  Nenhuma sessao encontrada.")
	} else {
		for i, id := range sessions {
			current := ""
			if m.session != nil && m.session.ID == id {
				current = " ◀ atual"
			}
			prefix := "  "
			if i == m.leftCursor {
				prefix = "▸ "
			}
			lines = append(lines, fmt.Sprintf("%s %s%s", prefix, id[:min(16, len(id))], current))
		}
	}

	lines = append(lines, "")
	lines = append(lines, "  Use /load <id> para retomar uma sessao.")
	return strings.Join(lines, "\n")
}

func (m *appModel) renderRightTools(w, h int) string {
	// Full tool stats with ASCII bar chart
	type toolStat struct {
		name  string
		count int
	}
	seen := map[string]*toolStat{}
	for _, tr := range m.toolRuns {
		if _, ok := seen[tr.Name]; !ok {
			seen[tr.Name] = &toolStat{name: tr.Name}
		}
		seen[tr.Name].count++
	}

	var lines []string
	lines = append(lines, m.styles.highlight.Render("  Uso de Ferramentas"))
	lines = append(lines, "  "+strings.Repeat("─", w-4))

	if len(seen) == 0 {
		lines = append(lines, "  Nenhuma ferramenta usada nesta sessao.")
	} else {
		barW := w - 18
		if barW < 10 {
			barW = 10
		}
		rows := make([]BarRow, 0, len(seen))
		for name, stat := range seen {
			rows = append(rows, BarRow{
				Label: name,
				Value: stat.count,
				Meta:  fmt.Sprintf("%d calls", stat.count),
			})
		}
		lines = append(lines, m.layout.BarChartASCII(rows, barW))
	}

	return strings.Join(lines, "\n")
}

func (m *appModel) renderRightTokens(w, h int) string {
	var lines []string
	total := m.tracker.TotalInputTokens + m.tracker.TotalOutputTokens
	costStr := cost.FormatCost(m.tracker.TotalCostUSD)

	lines = append(lines, m.styles.highlight.Render("  Consumo de Tokens"))
	lines = append(lines, "  "+strings.Repeat("─", w-4))
	lines = append(lines, fmt.Sprintf("  Input:    %s tokens", formatTokens(m.tracker.TotalInputTokens)))
	lines = append(lines, fmt.Sprintf("  Output:   %s tokens", formatTokens(m.tracker.TotalOutputTokens)))
	lines = append(lines, fmt.Sprintf("  Total:    %s tokens", formatTokens(total)))
	lines = append(lines, fmt.Sprintf("  Requests: %d", m.tracker.TotalRequests))
	lines = append(lines, fmt.Sprintf("  Custo:    %s", costStr))
	lines = append(lines, "")

	if len(m.tokenPerTurn) > 0 {
		lines = append(lines, "  Tokens por turno (sparkline):")
		w2 := w - 6
		if w2 > 40 {
			w2 = 40
		}
		lines = append(lines, "  "+Sparkline(m.tokenPerTurn, w2))
		lines = append(lines, "")
		for i, v := range m.tokenPerTurn {
			lines = append(lines, fmt.Sprintf("    Turno %d: %s tokens", i+1, formatTokens(v)))
		}
	} else {
		lines = append(lines, "  Nenhum turno registrado ainda.")
	}

	return strings.Join(lines, "\n")
}

func (m *appModel) renderRightContext(w, h int) string {
	var lines []string
	lines = append(lines, m.styles.highlight.Render("  Contexto do Projeto"))
	lines = append(lines, "  "+strings.Repeat("─", w-4))

	wd := m.cfg.WorkDir
	lines = append(lines, fmt.Sprintf("  Diretorio: %s", wd))
	lines = append(lines, fmt.Sprintf("  Modelo:    %s", m.cfg.Model))
	lines = append(lines, fmt.Sprintf("  Modo:      %s", m.cfg.Mode.String()))

	if m.cfg.ContextDoc != "" {
		n := len(m.cfg.ContextDoc)
		if n > 200 {
			n = 200
		}
		lines = append(lines, "")
		lines = append(lines, "  Conteudo DEVON.md:")
		lines = append(lines, "  "+m.cfg.ContextDoc[:n])
	}

	return strings.Join(lines, "\n")
}

func (m *appModel) renderRightMemory(w, h int) string {
	return m.styles.info.Render("  Memoria: em implementacao (depende da issue #22)")
}

func (m *appModel) renderHeader() string {
	wd := m.cfg.WorkDir
	if len(wd) > 30 {
		wd = "..." + wd[len(wd)-27:]
	}
	tokens := formatTokens(m.tracker.TotalInputTokens + m.tracker.TotalOutputTokens)
	costStr := cost.FormatCost(m.tracker.TotalCostUSD)
	sessionID := ""
	if m.session != nil {
		sessionID = " [" + m.session.ID[:min(len(m.session.ID), 8)] + "]"
	}
	title := m.styles.title.Render("devon" + sessionID)
	info := m.styles.info.Render(fmt.Sprintf("  %s  modelo: %s  tokens: %s  custo: %s  modo: %s", wd, m.cfg.Model, tokens, costStr, m.cfg.Mode.String()))
	return lipgloss.JoinHorizontal(lipgloss.Top, title, info)
}

func firstNLines(s string, n int) string {
	lines := strings.SplitN(s, "\n", n+1)
	if len(lines) > 1 {
		return strings.Join(lines[:n], "\n") + "\n... (truncado)"
	}
	return s
}

// --- Agent event processing ---

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
		m.leftItemCount = len(m.toolRuns) + 3 // update for navigation
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
		if m.cancel != nil {
			m.cancel()
			m.cancel = nil
		}
		m.running = false
		m.toolRuns = nil
		// Track tokens per turn
		if m.tracker != nil {
			total := m.tracker.TotalInputTokens + m.tracker.TotalOutputTokens
			m.tokenPerTurn = append(m.tokenPerTurn, total)
		}
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

// buildChatLines creates the rendered chat lines for the chat view.
func (m *appModel) buildChatLines(maxH int) []string {
	var lines []string

	for _, msg := range m.messages {
		switch msg.Sender {
		case "user":
			for _, line := range wrapLine(" > "+msg.Content, m.width-m.layout.leftPanelW-6) {
				lines = append(lines, m.styles.userMsg.Render(line))
			}
		case "devon":
			prefix := " > "
			if msg.IsError {
				prefix = " x "
			}
			for _, line := range wrapLine(prefix+msg.Content, m.width-m.layout.leftPanelW-6) {
				if msg.IsError {
					lines = append(lines, m.styles.errMsg.Render(line))
				} else {
					lines = append(lines, m.styles.devonMsg.Render(line))
				}
			}
		case "system":
			for _, line := range wrapLine(" - "+msg.Content, m.width-m.layout.leftPanelW-6) {
				lines = append(lines, m.styles.sysMsg.Render(line))
			}
		}
	}

	// Active tool runs
	for _, tr := range m.toolRuns {
		switch tr.Status {
		case "running":
			line := m.spinner.View() + " " + tr.Name + "(" + shortenArgs(tr.Args) + ")"
			lines = append(lines, m.styles.toolRunning.Render("  "+line))
		case "done":
			line := " / " + tr.Name + "(" + shortenArgs(tr.Args) + ")"
			lines = append(lines, m.styles.toolDone.Render("  "+line))
		case "error":
			line := " x " + tr.Name + ": " + firstLine(tr.Result)
			lines = append(lines, m.styles.toolError.Render("  "+line))
		}
	}

	// Apply scroll
	if len(lines) > maxH {
		start := m.scroll
		if start < 0 {
			start = 0
		}
		end := start + maxH
		if end > len(lines) {
			end = len(lines)
			start = end - maxH
			if start < 0 {
				start = 0
			}
		}
		lines = lines[start:end]
	}

	return lines
}

func (m *appModel) renderInputLine() string {
	ru := []rune(m.input)
	if len(ru) == 0 {
		return m.styles.cursorS.Render(" |")
	}
	if m.cursor >= len(ru) {
		return m.input + m.styles.cursorS.Render(" |")
	}
	before := string(ru[:m.cursor])
	cur := string(ru[m.cursor])
	after := string(ru[m.cursor+1:])
	return before + m.styles.cursorS.Render(cur) + after
}

func (m *appModel) renderHelp() string {
	hints := AllHints()
	lines := make([]string, len(hints)+2)
	lines[0] = m.styles.helpText.Render("  Atalhos de teclado:")
	lines[1] = m.styles.helpText.Render(strings.Repeat("─", m.width-2))
	for i, h := range hints {
		key := m.styles.highlight.Render(fmt.Sprintf("  %s", h.Keys))
		action := m.styles.helpText.Render(h.Action)
		lines[i+2] = fmt.Sprintf("%s → %s", key, action)
	}
	return strings.Join(lines, "\n")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
