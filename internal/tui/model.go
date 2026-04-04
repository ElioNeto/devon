// Package tui implementa a interface de usuÃ¡rio com Bubble Tea.
package tui

import (
	"context"
	"strings"

	"github.com/ElioNeto/devon/internal/agent"
	"github.com/ElioNeto/devon/internal/config"
	"github.com/ElioNeto/devon/internal/llm"
	"github.com/ElioNeto/devon/internal/tools"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// --- Model principal ---

type appModel struct {
	width  int
	height int

	// config
	cfg    *config.Config
	agent  *agent.Agent
	done   bool

	// status bar
	usage llm.Usage

	// chat messages
	messages []chatMessage

	// tool calls sendo executados
	currentToolRuns []toolRun

	// input
	input        strings.Builder
	cursorVisible bool
	cursorPos    int

	// scroll
	scrollOffset int
	maxScroll    int

	// spinner para agente processando
	spinner spinner.Model

	// agente estÃ¡ ativo
	running bool
	cancel  context.CancelFunc

	// ajuda visÃ­vel
	showHelp bool

	// estilos
	styles styles
}

// chatMessage representa uma mensagem no chat.
type chatMessage struct {
	Sender    string // "user" | "devon" | "system" | "tool"
	Content   string
	ToolRuns  []toolRun // para mensagens do devon com tool calls
	IsError   bool
}

// toolRun representa a execuÃ§Ã£o de uma tool com status.
type toolRun struct {
	Name   string
	Args   string
	Result string
	Status string // "running" | "done" | "error"
}

type styles struct {
	header      lipgloss.Style
	statusBar   lipgloss.Style
	chatArea    lipgloss.Style
	inputBar    lipgloss.Style
	userMsg     lipgloss.Style
	devonMsg    lipgloss.Style
	systemMsg   lipgloss.Style
	errorMsg    lipgloss.Style
	toolRunning lipgloss.Style
	toolDone    lipgloss.Style
	toolError   lipgloss.Style
	helpText    lipgloss.Style
}

func makeStyles() styles {
	s := styles{}
	s.header = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#8B5CF6")).
		Bold(true).
		Padding(0, 1)

	s.statusBar = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#E2E8F0")).
		Background(lipgloss.Color("#4B5563")).
		Padding(0, 1)

	s.inputBar = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#E2E8F0")).
		Bold(true).
		Padding(0, 1)

	s.userMsg = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#93C5FD")).
		PaddingLeft(1)

	s.devonMsg = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#86EFAC")).
		PaddingLeft(1)

	s.systemMsg = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#A5B4FC")).
		Italic(true).
		PaddingLeft(1)

	s.errorMsg = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FCA5A5")).
		PaddingLeft(1)

	s.toolRunning = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FBBF24")) // amarelo

	s.toolDone = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#4ADE80")) // verde

	s.toolError = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F87171")) // vermelho

	s.helpText = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF")).
		Faint(true)

	return s
}

func newModel(cfg *config.Config) appModel {
	s := spinner.New()
	s.Type = spinner.MiniDot

	registry := tools.NewRegistry()
	client := llm.New(cfg.APIKey, cfg.BaseURL, cfg.Model, cfg.Timeout)
	agt := agent.New(cfg, client, registry)

	return appModel{
		cfg:     cfg,
		agent:   agt,
		spinner: s,
		styles:  makeStyles(),
	}
}

// --- Mensagens ---

type userMsg string
type agentEventMsg agent.Event
type windowSizeMsg tea.WindowSizeMsg
type agentDoneMsg struct{}

// --- Commands ---

func newUserInput() tea.Cmd {
	// input handled via View/Update directly
	return nil
}

func sendToAgent(ctx context.Context, a *agent.Agent, input string) tea.Cmd {
	return func() tea.Msg {
		ch := a.Run(ctx, input)
		for ev := range ch {
			tea.Send(agentEventMsg(ev))
		}
		return agentDoneMsg{}
	}
}

// --- Init ---

func (m appModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg {
			// Mensagem de boas-vindas
			return agentEventMsg(agent.Event{
				Type: "system",
				Text: "Devon pronto. Digite sua mensagem e pressione Enter.",
			})
		},
	)
}

// --- Update ---

func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		if m.showHelp {
			if msg.String() == "esc" || msg.String() == "?" {
				m.showHelp = false
			} else {
				m.showHelp = false // qualquer tecla fecha a ajuda
			}
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c":
			if m.running && m.cancel != nil {
				m.cancel()
				m.running = false
				m.currentToolRuns = nil
				m.addMessage(chatMessage{
					Sender:  "system",
					Content: "Agente interrompido.",
				})
				return m, nil
			}
			return m, tea.Quit

		case "ctrl+l":
			m.messages = nil
			m.currentToolRuns = nil
			m.scrollOffset = 0
			return m, nil

		case "ctrl+k":
			m.messages = nil
			m.currentToolRuns = nil
			m.scrollOffset = 0
			m.usage = llm.Usage{}
			m.agent = agent.New(
				m.cfg,
				llm.New(m.cfg.APIKey, m.cfg.BaseURL, m.cfg.Model, m.cfg.Timeout),
				tools.NewRegistry(),
			)
			m.addMessage(chatMessage{
				Sender:  "system",
				Content: "Nova sessÃ£o iniciada.",
			})
			return m, nil

		case "enter":
			if m.input.Len() == 0 || m.running {
				return m, nil
			}
			input := m.input.String()
			m.input.Reset()
			m.cursorPos = 0
			m.addMessage(chatMessage{
				Sender:  "user",
				Content: input,
			})
			m.scrollOffset = 0
			m.currentToolRuns = nil
			m.running = true
			ctx, cancel := context.WithCancel(context.Background())
			m.cancel = cancel
			cmds = append(cmds, sendToAgent(ctx, m.agent, input))
			m.spinner.Type = spinner.MiniDot

		case "ctrl+backspace", "ctrl+w":
			if m.input.Len() > 0 {
				m.deletePreviousWord()
			}

		case "backspace":
			if m.input.Len() > 0 && m.cursorPos > 0 {
				m.deleteCharBefore()
			}

		case "ctrl+u":
			m.input.Reset()
			m.cursorPos = 0

		case "left":
			if m.cursorPos > 0 {
				m.cursorPos--
			}

		case "right":
			if m.cursorPos < m.input.Len() {
				m.cursorPos++
			}

		case "?":
			m.showHelp = true
			return m, nil

		case "up", "down", "ctrl+e", "ctrl+a", "home", "end":
			// handled by standard input, ignore

		default:
			if msg.Type == tea.KeyRunes {
				m.insertRune(msg.Runes[0])
			}

		case "pgup":
			if m.scrollOffset < m.maxScroll {
				m.scrollOffset++
			}

		case "pgdown":
			if m.scrollOffset > 0 {
				m.scrollOffset--
			}
		}

	case agentEventMsg:
		ev := agent.Event(msg)
		switch ev.Type {
		case "text":
			// Adiciona ou append ao Ãºltimo texto do devon
			if len(m.messages) == 0 || m.messages[len(m.messages)-1].Sender != "devon" {
				m.addMessage(chatMessage{
					Sender:  "devon",
					Content: ev.Text,
				})
			} else {
				idx := len(m.messages) - 1
				m.messages[idx].Content += ev.Text
			}

		case "tool_start":
			m.currentToolRuns = append(m.currentToolRuns, toolRun{
				Name:   ev.Tool,
				Args:   ev.Args,
				Status: "running",
			})

		case "tool_done":
			for i, tr := range m.currentToolRuns {
				if tr.Name == ev.Tool && tr.Status == "running" {
					m.currentToolRuns[i].Result = ev.Result
					m.currentToolRuns[i].Status = "done"
					break
				}
			}

		case "tool_error":
			for i, tr := range m.currentToolRuns {
				if tr.Name == ev.Tool && tr.Status == "running" {
					m.currentToolRuns[i].Result = ev.Err.Error()
					m.currentToolRuns[i].Status = "error"
					break
				}
			}

		case "turn_done":
			if m.cancel != nil {
				m.cancel()
			}
			m.running = false
			m.currentToolRuns = nil
			m.spinner.Type = spinner.MiniDot

		case "error":
			m.addMessage(chatMessage{
				Sender:  "devon",
				Content: "Erro: " + ev.Err.Error(),
				IsError: true,
			})
			if m.cancel != nil {
				m.cancel()
			}
			m.running = false
			m.currentToolRuns = nil

		case "system":
			m.addMessage(chatMessage{
				Sender:  "system",
				Content: ev.Text,
			})
		}
		m.scrollOffset = 0
		return m, m.spinner.Tick

	case agentDoneMsg:
		m.running = false
		m.currentToolRuns = nil
		if m.cancel != nil {
			m.cancel()
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// --- View ---

func (m appModel) View() string {
	if m.height == 0 || m.width == 0 {
		return "Iniciando Devon..."
	}

	availableHeight := m.height - 3 // header + status + input

	var b strings.Builder

	// Header
	b.WriteString(m.renderHeader())
	b.WriteByte('\n')

	// Chat area
	chatArea := m.renderChat(availableHeight - 1)
	b.WriteString(chatArea)
	b.WriteByte('\n')

	// Separator
	b.WriteString(strings.Repeat("â", m.width))
	b.WriteByte('\n')

	// Input
	b.WriteString(m.renderInput())

	// Help overlay
	if m.showHelp {
		b.WriteByte('\n')
		b.WriteString(m.renderHelp())
	}

	return b.String()
}

// --- Helpers ---

func (m *appModel) addMessage(msg chatMessage) {
	m.messages = append(m.messages, msg)
}

func (m *appModel) insertRune(r rune) {
	m.input.WriteString(string(r))
	m.cursorPos++
}

func (m *appModel) deleteCharBefore() {
	if m.cursorPos <= 0 {
		return
	}
	s := m.input.String()
	r := []rune(s)
	before := m.cursorPos
	if before <= len(r) {
		r = append(r[:before-1], r[before:]...)
		m.input.Reset()
		m.input.WriteString(string(r))
		m.cursorPos = before - 1
	}
}

func (m *appModel) deletePreviousWord() {
	s := m.input.String()
	ru := []rune(s)
	pos := m.cursorPos
	if pos == 0 {
		return
	}
	// Skip whitespace backwards
	for pos > 0 && ru[pos-1] == ' ' {
		pos--
	}
	// Skip word chars backwards
	for pos > 0 && ru[pos-1] != ' ' {
		pos--
	}
	remaining := append(ru[:pos], ru[m.cursorPos:]...)
	m.input.Reset()
	m.input.WriteString(string(remaining))
	m.cursorPos = pos
}

func (m *appModel) renderHeader() string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#8B5CF6")).
		Render("devon")

	info := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF")).
		Render(
			" > " +
				lipgloss.NewStyle().Foreground(lipgloss.Color("#E2E8F0")).Render(m.cfg.WorkDir) +
				"  modelo: " +
				lipgloss.NewStyle().Foreground(lipgloss.Color("#E2E8F0")).Render(m.cfg.Model) +
				"  tokens: " +
				lipgloss.NewStyle().Foreground(lipgloss.Color("#E2E8F0")).Render(formatTokens(m.usage)) +
				"  modo: " +
				lipgloss.NewStyle().Foreground(lipgloss.Color("#E2E8F0")).Render(m.cfg.Mode.String()))

	return lipgloss.JoinString(lipgloss.Top, title, "  ", info)
}

func (m *appModel) renderChat(availableHeight int) string {
	var lines []string

	for _, msg := range m.messages {
		switch msg.Sender {
		case "user":
			text := m.styles.userMsg.Render("> " + msg.Content)
			lines = append(lines, text)

		case "devon":
			if msg.IsError {
				text := m.styles.errorMsg.Render("â " + msg.Content)
				lines = append(lines, text)
			} else {
				text := m.styles.devonMsg.Render("â " + msg.Content)
				lines = append(lines, text)
			}

		case "system":
			text := m.styles.systemMsg.Render("Â· " + msg.Content)
			lines = append(lines, text)

		case "tool":
			text := m.styles.toolDone.Render("  " + msg.Content)
			lines = append(lines, text)
		}
	}

	// Tool runs em andamento
	if len(m.currentToolRuns) > 0 {
		for _, tr := range m.currentToolRuns {
			var line string
			switch tr.Status {
			case "running":
				line = m.spinner.View() + " " + tr.Name + "(" + shortenArgs(tr.Args) + ")"
				lines = append(lines, m.styles.toolRunning.Render("  "+line))
			case "done":
				line = "â " + tr.Name + "(" + shortenArgs(tr.Args) + ")"
				lines = append(lines, m.styles.toolDone.Render("  "+line))
			case "error":
				line = "â " + tr.Name + ": " + tr.Result
				lines = append(lines, m.styles.toolError.Render("  "+line))
			}
		}
	}

	// Truncate para caber no space disponÃ­vel
	total := len(lines)
	if total > availableHeight {
		lines = lines[total-availableHeight:]
	}

	return strings.Join(lines, "\n")
}

func (m *appModel) renderInput() string {
	prefix := " > "
	inputText := m.input.String()
	cursor := m.cursorPos

	// Split around cursor for cursor display
	if cursor >= len([]rune(inputText)) {
		// cursor at end
		return prefix + inputText + " " + m.styles.helpText.Render("|")
	}

	ru := []rune(inputText)
	before := string(ru[:cursor])
	after := string(ru[cursor:])
	_ = before
	_ = after

	// Simplified: just show the text with a cursor indicator
	display := prefix + inputText
	return m.styles.inputBar.Render(display)
}

func (m *appModel) renderHelp() string {
	help := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF")).
		Faint(true).
		Render(`
Teclas:
  Enter        Enviar mensagem
  Ctrl+C       Interromper agente / sair
  Ctrl+L       Limpar chat
  Ctrl+K       Nova sessÃ£o
  Ctrl+Backsp  Apagar palavra anterior
  Ctrl+U       Limpar input
  PgUp/PgDn    Scroll chat
  ?            Mostrar/ocultar esta ajuda
`)
	return help
}

func formatTokens(u llm.Usage) string {
	if u.TotalTokens == 0 {
		return "0"
	}
	return formatNumber(u.TotalTokens)
}

func formatNumber(n int) string {
	if n >= 1_000_000 {
		return formatNumber(n/1000_000) + "M"
	}
	if n >= 1_000 {
		return formatNumber(n/1000) + "k"
	}
	return fmt.Sprintf("%d", n)
}

func shortenArgs(args string) string {
	if len(args) <= 30 {
		return args
	}
	return args[:27] + "..."
}
