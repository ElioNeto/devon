// Package tui implements a Bubble Tea TUI for Devon.
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

	cfg      *config.Config
	agent    *agent.Agent
	session  *history.Session
	tracker  *cost.Session
	messages []chatMessage
	toolRuns []toolRun
	input    string
	cursor   int
	scroll   int
	running  bool
	cancel   context.CancelFunc
	showHelp bool
	spinner  spinner.Model
	styles   styles
	popup    string // overlay for /history, /usage etc
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
	userMsg     lipgloss.Style
	devonMsg    lipgloss.Style
	sysMsg      lipgloss.Style
	errMsg      lipgloss.Style
	toolRunning lipgloss.Style
	toolDone    lipgloss.Style
	toolError   lipgloss.Style
	inputPrefix lipgloss.Style
	cursor      lipgloss.Style
	helpText    lipgloss.Style
	popup       lipgloss.Style
}

func newStyles() styles {
	s := styles{}
	s.title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#8B5CF6"))
	s.info = lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF"))
	s.userMsg = lipgloss.NewStyle().Foreground(lipgloss.Color("#93C5FD")).PaddingLeft(1)
	s.devonMsg = lipgloss.NewStyle().Foreground(lipgloss.Color("#86EFAC")).PaddingLeft(1)
	s.sysMsg = lipgloss.NewStyle().Foreground(lipgloss.Color("#A5B4FC")).Italic(true).PaddingLeft(1)
	s.errMsg = lipgloss.NewStyle().Foreground(lipgloss.Color("#FCA5A5")).PaddingLeft(1)
	s.toolRunning = lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBF24"))
	s.toolDone = lipgloss.NewStyle().Foreground(lipgloss.Color("#4ADE80"))
	s.toolError = lipgloss.NewStyle().Foreground(lipgloss.Color("#F87171"))
	s.inputPrefix = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#8B5CF6"))
	s.cursor = lipgloss.NewStyle().Foreground(lipgloss.Color("#8B5CF6"))
	s.helpText = lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF")).Faint(true)
	s.popup = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#E2E8F0")).
		Background(lipgloss.Color("#1E293B")).
		Padding(1, 2).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#8B5CF6"))
	return s
}

func newModel(cfg *config.Config) appModel {
	s := spinner.New()
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#8B5CF6"))
	registry := tools.NewRegistry()
	client := llm.New(cfg.APIKey, cfg.BaseURL, cfg.Model, cfg.Timeout)
	agt := agent.New(cfg, client, registry)
	tracker := cost.NewSession(cfg.Model)

	// Try to load last session
	session, err := history.LoadLastSession(cfg.WorkDir)
	if err != nil {
		session = nil // start fresh if history fails
	}

	return appModel{
		cfg:     cfg,
		agent:   agt,
		session: session,
		tracker: tracker,
		spinner: s,
		styles:  newStyles(),
	}
}

// --- Messages ---

type agentEventMsg agent.Event

// --- Init ---

func (m appModel) Init() tea.Cmd {
	welcomeText := "Devon pronto. Digite sua mensagem e pressione Enter."
	if m.session != nil {
		welcomeText = fmt.Sprintf("Sessao %s carregada. Digite sua mensagem ou use comandos: /history /usage /clear", m.session.ID)
	}
	return tea.Sequence(
		m.spinner.Tick,
		func() tea.Msg {
			return agentEventMsg(agent.Event{Type: "system", Text: welcomeText})
		},
	)
}

// --- Slash command handling ---

func (m *appModel) handleSlash(text string) {
	switch {
	case text == "/history" || text == "/sessions":
		m.showHistory()
	case text == "/clear":
		if m.session != nil {
			_ = history.ClearSession(m.cfg.WorkDir, m.session.ID)
		}
		m.messages = nil
		m.toolRuns = nil
		m.scroll = 0
		m.tracker = cost.NewSession(m.cfg.Model)
		var err error
		m.session, err = history.LoadLastSession(m.cfg.WorkDir)
		if err == nil {
			m.messages = append(m.messages, chatMessage{Sender: "system", Content: "Sessao limpa. Nova sessao " + m.session.ID})
		}
	case text == "/usage" || text == "/cost":
		m.showUsage()
	case strings.HasPrefix(text, "/load "):
		id := strings.TrimSpace(strings.TrimPrefix(text, "/load"))
		if ses, err := history.LoadSession(m.cfg.WorkDir, id); err == nil {
			m.session = ses
			m.messages = nil
			m.messages = append(m.messages, chatMessage{Sender: "system", Content: "Sessao " + id + " carregada. Mensagens: " + itoaF(len(ses.Messages))})
			// Track usage from session
			m.tracker = cost.NewSession(m.cfg.Model)
			m.tracker.TotalInputTokens = ses.Usage.PromptTokens
			m.tracker.TotalOutputTokens = ses.Usage.CompletionTokens
			m.tracker.TotalRequests = ses.Usage.Requests
			m.tracker.TotalCostUSD = cost.EstimateCost(m.cfg.Model, ses.Usage.PromptTokens, ses.Usage.CompletionTokens)
		} else {
			m.messages = append(m.messages, chatMessage{Sender: "system", Content: "Erro ao carregar sessao: " + err.Error()})
		}
	default:
		m.messages = append(m.messages, chatMessage{Sender: "system", Content: "Comando desconhecido. Use: /history /load <id> /clear /usage"})
	}
}

func (m *appModel) showHistory() {
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

func (m *appModel) showUsage() {
	if m.tracker == nil {
		m.popup = "Sem dados de uso."
		return
	}
	m.popup = m.tracker.Format() + "\n\nPressione qualquer tecla para fechar."
}

// --- Update ---

func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle popup dismissal
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
			m.scroll = 0
			return m, nil

		case "ctrl+k":
			m.messages = nil
			m.toolRuns = nil
			m.scroll = 0
			m.tracker = cost.NewSession(m.cfg.Model)
			var err error
			m.session, err = history.CreateSession(m.cfg.WorkDir) // new session
			if err != nil {
				m.messages = append(m.messages, chatMessage{Sender: "system", Content: "Erro ao criar sessao: " + err.Error()})
			} else {
				m.messages = append(m.messages, chatMessage{Sender: "system", Content: "Nova sessao " + m.session.ID})
			}
			return m, nil

		case "enter":
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

		case "ctrl+u":
			m.input = ""
			m.cursor = 0

		case "ctrl+w", "ctrl+backspace":
			m.deleteWord()

		case "backspace":
			if m.cursor > 0 {
				m.deleteCharBefore()
			}

		case "delete":
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

		case "pgup":
			if len(m.messages) > 0 {
				m.scroll++
			}

		case "pgdown":
			if m.scroll > 0 {
				m.scroll--
			}

		case "?":
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
		m.scroll = 0
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
			msgs = append(msgs, llm.Message{Role: role, Content: cm.Content})
		}
	}
	return msgs
}

// --- View ---

func (m appModel) View() string {
	if m.height == 0 || m.width == 0 {
		return "Iniciando Devon..."
	}

	headerH := 2
	inputH := 1
	helpH := 10
	maxChatH := m.height - headerH - inputH - 1 // minus separator

	if m.showHelp {
		maxChatH -= helpH
	}

	if maxChatH < 1 {
		maxChatH = 1
	}

	var sb strings.Builder

	// Header
	sb.WriteString(m.renderHeader())
	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("─", m.width))
	sb.WriteString("\n")

	// Chat
	chatLines := m.buildChatLines(maxChatH)
	for _, line := range chatLines {
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	// Input bar
	if m.running {
		sb.WriteString(" " + m.spinner.View() + " ")
	} else {
		sb.WriteString(m.styles.inputPrefix.Render(" > "))
	}
	sb.WriteString(m.renderInputLine())

	// Help
	if m.showHelp {
		sb.WriteString("\n")
		sb.WriteString(m.renderHelp())
	}

	// Popup overlay
	if m.popup != "" {
		sb.WriteString("\n")
		sb.WriteString(m.styles.popup.Render(m.popup))
	}

	return sb.String()
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
		sessionID = " [" + m.session.ID[:min(len(m.session.ID), 16)] + "]"
	}
	title := m.styles.title.Render("devon" + sessionID)
	info := m.styles.info.Render(fmt.Sprintf(" > %s  modelo: %s  tokens: %s  custo: %s  modo: %s", wd, m.cfg.Model, tokens, costStr, m.cfg.Mode.String()))
	return lipgloss.JoinHorizontal(lipgloss.Top, title, "  ", info)
}

func (m *appModel) buildChatLines(maxH int) []string {
	var lines []string

	for _, msg := range m.messages {
		switch msg.Sender {
		case "user":
			for _, line := range wrapLine(" > "+msg.Content, m.width) {
				lines = append(lines, m.styles.userMsg.Render(line))
			}
		case "devon":
			prefix := " > "
			if msg.IsError {
				prefix = " x "
			}
			for _, line := range wrapLine(prefix+msg.Content, m.width) {
				if msg.IsError {
					lines = append(lines, m.styles.errMsg.Render(line))
				} else {
					lines = append(lines, m.styles.devonMsg.Render(line))
				}
			}
		case "system":
			for _, line := range wrapLine(" - "+msg.Content, m.width) {
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
		return m.styles.cursor.Render(" |")
	}
	if m.cursor >= len(ru) {
		return m.input + m.styles.cursor.Render(" |")
	}
	before := string(ru[:m.cursor])
	cur := string(ru[m.cursor])
	after := string(ru[m.cursor+1:])
	return before + m.styles.cursor.Render(cur) + after
}

func (m *appModel) renderHelp() string {
	return m.styles.helpText.Render(
		"  Enter: enviar  Ctrl+C: sair  Ctrl+L: limpar  Ctrl+K: nova sessao  Ctrl+W: apagar palavra  Ctrl+U: limpar input  PgUp/PgDn: scroll  /history /usage /clear /load",
	)
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
	case "tool_done":
		for i, tr := range m.toolRuns {
			if tr.Name == ev.Tool && tr.Status == "running" {
				m.toolRuns[i].Result = ev.Result
				m.toolRuns[i].Status = "done"
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
	case "error":
		m.messages = append(m.messages, chatMessage{Sender: "devon", Content: "Erro: " + ev.Err.Error(), IsError: true})
		if m.cancel != nil {
			m.cancel()
			m.cancel = nil
		}
		m.running = false
		m.toolRuns = nil
	case "system":
		m.messages = append(m.messages, chatMessage{Sender: "system", Content: ev.Text})
	}
}

// --- Input manipulation ---

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

// --- Commands ---

type agentResult struct {
	events []agent.Event
}

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

// --- Utilities ---

func shortenArgs(args string) string {
	if len(args) <= 25 {
		return args
	}
	return args[:22] + "..."
}

func firstLine(s string) string {
	if i := strings.Index(s, "\n"); i >= 0 {
		return s[:i]
	}
	return s
}

func formatTokens(n int) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}

func wrapLine(s string, width int) []string {
	if width <= 0 {
		return []string{s}
	}
	var lines []string
	for len(s) > width {
		idx := strings.LastIndex(s[:width], " ")
		if idx < 0 {
			idx = width
		}
		lines = append(lines, s[:idx])
		s = strings.TrimLeft(s[idx:], " ")
	}
	if s != "" {
		lines = append(lines, s)
	}
	return lines
}

func itoaF(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
