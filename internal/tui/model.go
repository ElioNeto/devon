// Package tui implementa a interface visual tipo lazydocker para o Devon.
package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

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

// ═══════════════════ Model ════════════════════════════════════════════

type appModel struct {
	width  int
	height int

	cfg     *config.Config
	agent   *agent.Agent
	session *history.Session
	tracker *cost.Session
	styles  uiStyles
	spinner spinner.Model
	layout  layout

	pane     int // 0=left, 1=right
	tab      int // 0=Sessão, 1=Histórico, 2=Ferramentas, 3=TokenTime
	tabIndex int // cursor within current section
	scroll   int // scroll for right panel content

	input  string
	cursor int

	messages   []chatMessage
	toolRuns   []toolRun
	historyTurns []historyTurn
	toolStats  map[string]*toolStat
	memoryFacts []memoryFact
	tokenTime  []tokenSnapshot
	costByTool map[string]float64

	running  bool
	cancel   context.CancelFunc
	showHelp bool
	popup    string
	ctxMenu  ctxMenuState
}

type chatMessage struct {
	Sender  string
	Content string
	IsError bool
}

type toolRun struct {
	Name      string
	Args      string
	Result    string
	Status    string
	StartedAt time.Time
	Duration  time.Duration
}

type toolStat struct {
	Calls int
	Total time.Duration
	Max   time.Duration
}

type historyTurn struct {
	UserPrompt   string
	AgentReply   string
	ToolSummary  string
	Timestamp    string
	PromptTokens int
	OutputTokens int
}

type memoryFact struct {
	Category string
	Key      string
	Value    string
}

type tokenSnapshot struct {
	Time   time.Time
	Turns  int
	Input  int
	Output int
}

type agentEventMsg agent.Event

type agentResult struct {
	events []agent.Event
}

type tickMsg struct{}

// ═══════════════════ Initialization ═══════════════════════════════════

func newModel(cfg *config.Config) appModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(colorPrimary)

	registry := tools.NewRegistry()
	client := llm.New(cfg.APIKey, cfg.BaseURL, cfg.Model, cfg.Timeout)
	agt := agent.New(cfg, client, registry)
	tracker := cost.NewSession(cfg.Model)

	session, _ := history.LoadLastSession(cfg.WorkDir)

	return appModel{
		cfg:       cfg,
		agent:     agt,
		session:   session,
		tracker:   tracker,
		spinner:   s,
		styles:    newUIStyles(),
		toolStats: make(map[string]*toolStat),
		costByTool: make(map[string]float64),
		tab:      0,
		pane:     0,
	}
}

func (m appModel) Init() tea.Cmd {
	welcome := "Devon pronto. Tab alterna painéis, ↑↓ navega, digite para enviar."
	if m.session != nil {
		welcome = fmt.Sprintf("Sessão %s carregada.", m.session.ID[:min(12, len(m.session.ID))])
	}
	return tea.Sequence(
		m.spinner.Tick,
		func() tea.Msg {
			return agentEventMsg(agent.Event{Type: "system", Text: welcome})
		},
		tea.Tick(time.Second, func(time.Time) tea.Msg { return tickMsg{} }),
	)
}

// ═══════════════════ Update ═══════════════════════════════════════════

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
			msgs := m.agentMessages()
			_ = history.SaveMessages(m.cfg.WorkDir, m.session.ID, msgs, &m.session.Usage)
		}
		if m.tracker != nil {
			now := time.Now()
			m.tokenTime = append(m.tokenTime, tokenSnapshot{
				Time:   now,
				Turns:  len(m.historyTurns) + 1,
				Input:  m.tracker.TotalInputTokens,
				Output: m.tracker.TotalOutputTokens,
			})
			if len(m.tokenTime) > 120 {
				m.tokenTime = m.tokenTime[len(m.tokenTime)-120:]
			}
		}
		m.finalizeTurn()
		if m.cancel != nil {
			m.cancel()
			m.cancel = nil
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tickMsg:
		return m, tea.Tick(time.Second, func(time.Time) tea.Msg { return tickMsg{} })
	}

	return m, nil
}

func (m appModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := msg.String()
	switch s {
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
		m.tracker = cost.NewSession(m.cfg.Model)
		m.session, _ = history.CreateSession(m.cfg.WorkDir)
		m.messages = append(m.messages, chatMessage{Sender: "system", Content: "Nova sessão criada."})
		return m, nil

	case "ctrl+tab":
		m.pane = (m.pane + 1) % 2
		return m, nil

	case "ctrl+h":
		m.showHelp = true
		return m, nil

	case "ctrl+q":
		if m.popup != "" {
			m.popup = ""
			return m, nil
		}
		if m.ctxMenu.visible {
			m.ctxMenu.close()
			return m, nil
		}
		return m, tea.Quit

	case "ctrl+e":
		if m.tabIndex > 0 {
			m.tabIndex--
		}
		return m, nil

	case "ctrl+n":
		m.tabIndex++
		return m, nil

	case "pgup":
		if m.scroll >= 5 {
			m.scroll -= 5
		} else {
			m.scroll = 0
		}
		return m, nil

	case "pgdown":
		m.scroll += 5
		return m, nil

	case "enter":
		text := strings.TrimSpace(m.input)
		if text == "" || m.running {
			return m, nil
		}
		m.input = ""
		m.cursor = 0

		if strings.HasPrefix(text, "/") {
			m.handleSlash(text)
			return m, nil
		}

		m.messages = append(m.messages, chatMessage{Sender: "user", Content: text})
		m.toolRuns = nil
		m.running = true
		ctx, cancel := context.WithCancel(context.Background())
		m.cancel = cancel
		return m, tea.Batch(m.spinner.Tick, startAgent(ctx, m.agent, text))

	case "backspace":
		if m.cursor > 0 {
			m.deleteCharBefore()
		}
		return m, nil

	case "delete":
		if m.cursor < len([]rune(m.input)) {
			m.deleteCharAfter()
		}
		return m, nil

	case "ctrl+u":
		m.input = ""
		m.cursor = 0
		return m, nil

	case "ctrl+w", "ctrl+backspace":
		m.deleteWord()
		return m, nil

	case "left":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil

	case "right":
		if m.cursor < len([]rune(m.input)) {
			m.cursor++
		}
		return m, nil

	case "home":
		m.cursor = 0
		return m, nil

	case "end":
		m.cursor = len([]rune(m.input))
		return m, nil

	default:
		if msg.Type == tea.KeyRunes {
			for _, r := range msg.Runes {
				m.insertRune(r)
			}
		} else if msg.Type == tea.KeySpace {
			m.insertRune(' ')
		}
		return m, nil
	}
}

// ═══════════════════ View ═════════════════════════════════════════════

func (m appModel) View() string {
	if m.height == 0 || m.width == 0 {
		return "Iniciando Devon..."
	}

	l := m.layout
	if l.width == 0 {
		l = calcLayout(m.width, m.height)
	}

	var sb strings.Builder

	// Header
	sb.WriteString(m.renderHeader(l.width))
	sb.WriteString("\n")

	// Panels
	leftP := m.renderLeft(l.leftW, l.panelH)
	rightP := m.renderRight(l.rightW, l.panelH)
	sep := m.styles.panelTitle.Render(" │ ")
	panels := lipgloss.JoinHorizontal(lipgloss.Top, leftP, sep, rightP)
	sb.WriteString(panels)

	// Divider
	sb.WriteString("\n" + strings.Repeat("─", m.width))

	// Input
	sb.WriteString("\n")
	sb.WriteString(m.renderInput(l.width))

	// Footer
	sb.WriteString("\n")
	sb.WriteString(m.renderFooter(l.width))

	// Overlays
	if m.showHelp {
		sb.WriteString("\n")
		sb.WriteString(renderHelp(m, l.width))
	}
	if m.popup != "" {
		sb.WriteString("\n\n")
		sb.WriteString(m.styles.menuStyle.Render(m.popup))
	}
	if m.ctxMenu.visible {
		sb.WriteString("\n\n")
		sb.WriteString(m.ctxMenu.render(l.width))
	}

	return sb.String()
}

// ═══════════════════ Header ═══════════════════════════════════════════

func (m *appModel) renderHeader(w int) string {
	tokens := fmtShort(m.tracker.TotalInputTokens + m.tracker.TotalOutputTokens)
	costStr := cost.FormatCost(m.tracker.TotalCostUSD)
	sess := ""
	if m.session != nil {
		sess = " [" + m.session.ID[:min(8, len(m.session.ID))] + "]"
	}
	wd := m.cfg.WorkDir
	if len(wd) > 25 {
		wd = "…" + wd[len(wd)-24:]
	}

	left := m.styles.badge.Render("devon" + sess)
	right := fmt.Sprintf("  %s  modelo:%s  tokens:%s  custo:%s", wd, m.cfg.Model, tokens, costStr)
	gap := w - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

// ═══════════════════ Left Panel ═══════════════════════════════════════

var tabLabels = []string{"■ Sessão", "◆ Histórico", "▲ Ferramentas", "◇ TokenTime"}

func (m *appModel) renderLeft(w, h int) string {
	var lines []string

	// Tabs
	for i, label := range tabLabels {
		if i == m.tab && m.pane == 0 {
			lines = append(lines, " ▸"+m.styles.itemSelected.Render(label+"  "))
		} else {
			lines = append(lines, "  "+m.styles.itemSection.Render(label+"  "))
		}
	}
	lines = append(lines, strings.Repeat("─", w))

	ch := h - 3
	if ch < 1 {
		ch = 1
	}

	switch m.tab {
	case 0:
		lines = append(lines, m.renderSessionTab(w, ch)...)
	case 1:
		lines = append(lines, m.renderHistoryTab(w, ch)...)
	case 2:
		lines = append(lines, m.renderToolsTab(w, ch)...)
	case 3:
		lines = append(lines, m.renderTokenTimeTab(w, ch)...)
	}

	content := joinH(lines, h)
	return m.styles.panelFocused.Width(w).Height(h).Render(content)
}

func (m *appModel) renderSessionTab(w, h int) []string {
	var lines []string
	if m.running {
		lines = append(lines, " "+m.spinner.View()+" agente trabalhando…")
		lines = append(lines, "")
	}

	for i, tr := range m.toolRuns {
		pfx := "  "
		if i == m.cursor {
			pfx = "▸ "
		}
		switch tr.Status {
		case "running":
			lines = append(lines, pfx+m.styles.toolRunning.Render("⏳ "+tr.Name)+" "+Trunc(Trunc(tr.Args, w-25), w-25))
		case "done":
			lines = append(lines, pfx+m.styles.toolDone.Render("✔ "+tr.Name)+" "+Trunc(Trunc(tr.Args, w-25), w-25))
		case "error":
			lines = append(lines, pfx+m.styles.toolError.Render("✘ "+tr.Name)+": "+firstLine(tr.Result))
		}
	}

	if !m.running && len(m.toolRuns) == 0 {
		if len(m.messages) > 0 {
			lines = append(lines, "")
			for _, msg := range m.messages {
				pfx := "  "
				switch msg.Sender {
				case "user":
					lines = append(lines, pfx+m.styles.userMsg.Render("› "+Trunc(msg.Content, w-5)))
				case "devon":
					if msg.IsError {
						lines = append(lines, pfx+m.styles.errMsg.Render("✘ "+Trunc(msg.Content, w-5)))
					} else {
						lines = append(lines, pfx+Trunc(msg.Content, w-4))
					}
				case "system":
					lines = append(lines, pfx+m.styles.sysMsg.Render("— "+Trunc(msg.Content, w-5)))
				}
			}
		} else {
			lines = append(lines, "")
			lines = append(lines, m.styles.sysMsg.Render(" Digite sua pergunta abaixo."))
		}
	}
	return lines
}

func (m *appModel) renderHistoryTab(w, h int) []string {
	var lines []string
	if len(m.historyTurns) == 0 {
		lines = append(lines, m.styles.sysMsg.Render("Nenhum turno ainda."))
		return lines
	}
	for i, ht := range m.historyTurns {
		pfx := "  "
		if i == m.cursor {
			pfx = "▸ "
		}
		tok := ht.PromptTokens + ht.OutputTokens
		lines = append(lines, fmt.Sprintf("%sTurno %d  (%s toks)", pfx, i+1, fmtShort(tok)))
		lines = append(lines, "     "+Trunc(ht.UserPrompt, w-7))
		if ht.ToolSummary != "" {
			lines = append(lines, "     tools: "+ht.ToolSummary)
		}
		lines = append(lines, "")
	}
	return lines
}

func (m *appModel) renderToolsTab(w, h int) []string {
	var lines []string
	if len(m.toolStats) == 0 {
		lines = append(lines, m.styles.sysMsg.Render("Nenhuma ferramenta usada."))
		return lines
	}
	maxC := 1
	for _, st := range m.toolStats {
		if st.Calls > maxC {
			maxC = st.Calls
		}
	}
	barW := w - 20
	if barW < 4 {
		barW = 4
	}
	for name, st := range m.toolStats {
		bar := HorzBar(name, st.Calls, maxC, barW, 10)
		lines = append(lines, "  "+bar+"  "+formatDuration(st.Total))
	}
	if len(m.costByTool) > 0 {
		lines = append(lines, "")
		lines = append(lines, m.styles.panelTitle.Render("Custo por ferramenta:"))
		for name, c := range m.costByTool {
			lines = append(lines, fmt.Sprintf("  %-12s %s", name, cost.FormatCost(c)))
		}
	}
	return lines
}

func (m *appModel) renderTokenTimeTab(w, h int) []string {
	var lines []string
	total := m.tracker.TotalInputTokens + m.tracker.TotalOutputTokens
	lines = append(lines, fmt.Sprintf("  Total: %s tokens  |  Custo: %s", fmtShort(total), cost.FormatCost(m.tracker.TotalCostUSD)))
	lines = append(lines, "")

	if len(m.tokenTime) > 1 {
		lines = append(lines, "  Input tokens:  "+Sparkline(m.tokenTimeInput(), max(w-18, 10)))
		lines = append(lines, "  Output tokens: "+Sparkline(m.tokenTimeOutput(), max(w-18, 10)))
		lines = append(lines, "")

		// ASCII area chart
		totalVals := m.tokenTimeTotal()
		chH := min(h-len(lines)-3, 10)
		if chH > 3 {
			for j, row := range LineChart(totalVals, max(w-10, 10), chH, "  ") {
				if j == 0 {
					lines = append(lines, fmt.Sprintf("%6s├%s", fmtShort(maxInt(totalVals)), row))
				} else {
					lines = append(lines, "       │"+row)
				}
			}
			lines = append(lines, "       └"+strings.Repeat(" ", max(w-18, 10))+fmt.Sprintf("%d snaps", len(m.tokenTime)))
		}
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("  Período: %s (%d snapshots, %d turnos)",
			m.tokenTime[0].Time.Format("15:04:05"), len(m.tokenTime), len(m.historyTurns)+1))

		// Cost per tool
		if len(m.costByTool) > 0 {
			lines = append(lines, "")
			lines = append(lines, m.styles.panelTitle.Render("Custo por ferramenta:"))
			mc := 0.0
			for _, c := range m.costByTool {
				if c > mc {
					mc = c
				}
			}
			for name, c := range m.costByTool {
				barW := min(w-16, 30)
				if mc > 0 {
					fill := min(barW, max(1, int(c/mc*float64(barW))))
					lines = append(lines, fmt.Sprintf("  %-12s [%s] %s", name+strings.Repeat(" ", max(0,10-len(name))),
						strings.Repeat("█", fill)+strings.Repeat("░", max(0,barW-fill)), cost.FormatCost(c)))
				} else {
					lines = append(lines, fmt.Sprintf("  %-12s %s", name, cost.FormatCost(c)))
				}
			}
		}
	} else {
		lines = append(lines, m.styles.sysMsg.Render("Envie mensagens para gerar o gráfico temporal."))
	}
	return lines
}

// Token time helpers
func (m *appModel) tokenTimeInput() []int {
	vals := make([]int, len(m.tokenTime))
	for i, ts := range m.tokenTime {
		vals[i] = ts.Input
	}
	return vals
}
func (m *appModel) tokenTimeOutput() []int {
	vals := make([]int, len(m.tokenTime))
	for i, ts := range m.tokenTime {
		vals[i] = ts.Output
	}
	return vals
}
func (m *appModel) tokenTimeTotal() []int {
	vals := make([]int, len(m.tokenTime))
	for i, ts := range m.tokenTime {
		vals[i] = ts.Input + ts.Output
	}
	return vals
}

// ═══════════════════ Right Panel ═════════════════════════════════════

func (m *appModel) renderRight(w, h int) string {
	var lines []string
	switch m.tab {
	case 0:
		lines = m.renderRightSession(w)
	case 1:
		lines = m.renderRightHistory(w)
	case 2:
		lines = m.renderRightTools(w)
	case 3:
		lines = m.renderRightTokenTime(w)
	default:
		lines = m.renderRightSession(w)
	}
	content := joinH(lines, h)
	return m.styles.panelBase.Width(w).Height(h).Render(content)
}

func (m *appModel) renderRightSession(w int) []string {
	var lines []string
	lines = append(lines, m.styles.configKey.Render("Sessão Atual"))
	lines = append(lines, strings.Repeat("─", w))
	if m.session != nil {
		pairs := []struct{ k, v string }{
			{"ID:", m.session.ID[:min(16, len(m.session.ID))]},
			{"Modelo:", m.cfg.Model},
			{"Mensagens:", fmt.Sprintf("%d", len(m.messages))},
			{"Tokens:", fmtShort(m.tracker.TotalInputTokens + m.tracker.TotalOutputTokens)},
			{"Custo:", cost.FormatCost(m.tracker.TotalCostUSD)},
			{"Criada:", m.session.CreatedAt.Format("15:04:05 02/01/2006")},
		}
		for _, p := range pairs {
			lines = append(lines, "  "+m.styles.configKey.Render(fmt.Sprintf("%-12s", p.k))+" "+m.styles.configVal.Render(p.v))
		}
	} else {
		lines = append(lines, m.styles.sysMsg.Render("Nenhuma sessão ativa."))
	}
	if len(m.messages) > 0 {
		lines = append(lines, "")
		lines = append(lines, m.styles.configKey.Render("Chat:"))
		start := len(m.messages) - 25
		if start < 0 {
			start = 0
		}
		for i := start; i < len(m.messages); i++ {
			msg := m.messages[i]
			switch msg.Sender {
			case "user":
				lines = append(lines, m.styles.userMsg.Render("  › "+Trunc(msg.Content, w-7)))
			case "devon":
				if msg.IsError {
					lines = append(lines, m.styles.errMsg.Render("  ✘ "+Trunc(msg.Content, w-7)))
				} else {
					lines = append(lines, "  "+Trunc(msg.Content, w-5))
				}
			case "system":
				lines = append(lines, m.styles.sysMsg.Render("  — "+Trunc(msg.Content, w-7)))
			}
		}
	}
	return lines
}

func (m *appModel) renderRightHistory(w int) []string {
	var lines []string
	lines = append(lines, m.styles.configKey.Render("Histórico de Turnos"))
	lines = append(lines, strings.Repeat("─", w))
	if len(m.historyTurns) == 0 {
		lines = append(lines, "", m.styles.sysMsg.Render("Nenhum turno registrado."))
		return lines
	}
	idx := m.cursor
	if idx >= len(m.historyTurns) {
		idx = len(m.historyTurns) - 1
	}
	if idx < 0 {
		return lines
	}
	ht := m.historyTurns[idx]
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("  Turno %d — %s", idx+1, ht.Timestamp))
	lines = append(lines, fmt.Sprintf("  Tokens: %s in / %s out", fmtShort(ht.PromptTokens), fmtShort(ht.OutputTokens)))
	lines = append(lines, "  Tools: "+ht.ToolSummary)
	lines = append(lines, "")
	lines = append(lines, m.styles.configKey.Render("Prompt:"))
	for _, l := range wrapLines(ht.UserPrompt, w-4) {
		lines = append(lines, m.styles.userMsg.Render("  "+l))
	}
	lines = append(lines, "")
	lines = append(lines, m.styles.configKey.Render("Resposta:"))
	for _, l := range wrapLines(ht.AgentReply, w-4) {
		lines = append(lines, m.styles.agentMsg.Render("  "+l))
	}
	return lines
}

func (m *appModel) renderRightTools(w int) []string {
	var lines []string
	lines = append(lines, m.styles.configKey.Render("Ferramentas — detalhe"))
	lines = append(lines, strings.Repeat("─", w))
	if len(m.toolStats) == 0 {
		lines = append(lines, "", m.styles.sysMsg.Render("Nenhuma ferramenta usada."))
		return lines
	}
	lines = append(lines, "")
	lines = append(lines, m.styles.tableHeader.Render(fmt.Sprintf("  %-12s %6s %10s %10s", "Ferramenta", "Calls", "Total", "Max")))
	for name, st := range m.toolStats {
		lines = append(lines, fmt.Sprintf("  %-12s %6d %10s %10s", name, st.Calls, formatDuration(st.Total), formatDuration(st.Max)))
	}
	return lines
}

func (m *appModel) renderRightTokenTime(w int) []string {
	var lines []string
	lines = append(lines, m.styles.configKey.Render("Consumo de Tokens"))
	lines = append(lines, strings.Repeat("─", w))
	total := m.tracker.TotalInputTokens + m.tracker.TotalOutputTokens
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("  Total: %s  |  Input: %s  |  Output: %s",
		fmtShort(total), fmtShort(m.tracker.TotalInputTokens), fmtShort(m.tracker.TotalOutputTokens)))
	lines = append(lines, fmt.Sprintf("  Cust:  |  Requests: %d", m.tracker.TotalRequests))
	lines = append(lines, "")

	if len(m.tokenTime) > 1 {
		lines = append(lines, m.styles.configKey.Render("Sparkline input:"))
		lines = append(lines, "  "+Sparkline(m.tokenTimeInput(), max(w-8, 10)))
		lines = append(lines, m.styles.configKey.Render("Sparkline output:"))
		lines = append(lines, "  "+Sparkline(m.tokenTimeOutput(), max(w-8, 10)))

		totalVals := m.tokenTimeTotal()
		chH := 8
		for j, row := range LineChart(totalVals, max(w-10, 10), chH, "  ") {
			if j == 0 {
				lines = append(lines, fmt.Sprintf("%6s├%s", fmtShort(maxInt(totalVals)), row))
			} else {
				lines = append(lines, "       │"+row)
			}
		}
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("  Período: %s → %s (%d snapshots)",
			m.tokenTime[0].Time.Format("15:04"),
			m.tokenTime[len(m.tokenTime)-1].Time.Format("15:04"),
			len(m.tokenTime)))
	} else {
		lines = append(lines, m.styles.sysMsg.Render("Envie mensagens para gerar o gráfico."))
	}

	// Cost by tool
	if len(m.costByTool) > 0 {
		lines = append(lines, "")
		lines = append(lines, m.styles.configKey.Render("Custo por ferramenta:"))
		mc := 0.0
		for _, c := range m.costByTool {
			if c > mc {
				mc = c
			}
		}
		barW := min(w-16, 30)
		for name, c := range m.costByTool {
			if mc > 0 {
				fill := min(barW, max(1, int(c/mc*float64(barW))))
				lines = append(lines, fmt.Sprintf("  %-12s [%s] %s", name+strings.Repeat(" ", max(0, 10-len(name))),
					strings.Repeat("█", fill)+strings.Repeat("░", max(0, barW-fill)), cost.FormatCost(c)))
			} else {
				lines = append(lines, fmt.Sprintf("  %-12s %s", name, cost.FormatCost(c)))
			}
		}
	}
	return lines
}

// ═══════════════════ Input / Footer ═══════════════════════════════════

func (m *appModel) renderInput(w int) string {
	pfx := m.styles.inputPrompt.Render("> ")
	if m.running {
		return m.styles.statusBar.Render(m.spinner.View() + "  aguardando resposta do agente…")
	}
	ru := []rune(m.input)
	var inputStr string
	if len(ru) == 0 {
		inputStr = m.styles.statusKey.Render("envie sua mensagem…")
	} else if m.cursor >= len(ru) {
		inputStr = m.input + m.styles.cursorStyle.Render("▋")
	} else {
		before := string(ru[:m.cursor])
		cur := m.styles.cursorStyle.Render(string(ru[m.cursor]))
		after := string(ru[m.cursor+1:])
		inputStr = before + cur + after
	}
	return m.styles.inputBar.Width(w).Render(" " + pfx + inputStr)
}

func (m *appModel) renderFooter(w int) string {
	hints := []struct{ k string }{
		{"Ctrl+Tab"}, {"Ctrl+N/E"}, {"Enter"}, {"Ctrl+Q"}, {"Ctrl+H"}, {"PgUp/Dn"}, {"Ctrl+L"}, {"Ctrl+K"},
	}
	labels := []string{"painel", "navegar", "enviar", "sair", "ajuda", "rolar", "limpar", "nova sessão"}
	parts := make([]string, 0, len(hints)*2)
	for i, h := range hints {
		if i > 0 {
			parts = append(parts, m.styles.statusSep.Render("│"))
		}
		parts = append(parts, m.styles.keyStyle.Render(h.k))
		parts = append(parts, m.styles.helpStyle.Render(labels[i]))
	}
	rightPart := m.styles.statusKey.Render(fmt.Sprintf(" %s", m.cfg.Model))
	leftStr := strings.Join(parts, " ")
	gap := w - lipgloss.Width(leftStr) - lipgloss.Width(rightPart) - 4
	if gap < 1 {
		gap = 1
	}
	return m.styles.statusBar.Width(w).Render(leftStr + strings.Repeat(" ", gap) + rightPart)
}

// ═══════════════════ Help ═════════════════════════════════════════════

func renderHelp(m appModel, w int) string {
	pairs := []struct{ k, v string }{
		{"Ctrl+Tab", "alterna painel esquerdo/direito"},
		{"Ctrl+N/E", "próximo/anterior item"},
		{"Enter", "enviar mensagem"},
		{"Ctrl+Q", "sair (ou fecha popup)"},
		{"Ctrl+C", "interromper agente / sair"},
		{"Ctrl+L", "limpar chat"},
		{"Ctrl+K", "nova sessão"},
		{"Ctrl+U", "limpar input"},
		{"Ctrl+W", "apagar palavra"},
		{"Ctrl+H", "toggle ajuda"},
		{"PgUp/Dn", "rolar"},
	}
	var lines []string
	lines = append(lines, m.styles.menuStyle.Render("Atalhos de teclado"))
	for _, p := range pairs {
		lines = append(lines, fmt.Sprintf("  %-16s→ %s", m.styles.keyStyle.Render(p.k), p.v))
	}
	return joinH(lines, 15)
}

// ═══════════════════ Agent Processing ═════════════════════════════════

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
		m.toolRuns = append(m.toolRuns, toolRun{
			Name:      ev.Tool,
			Args:      ev.Args,
			Status:    "running",
			StartedAt: time.Now(),
		})
	case "tool_done":
		for i, tr := range m.toolRuns {
			if tr.Name == ev.Tool && tr.Status == "running" {
				dur := time.Since(tr.StartedAt)
				m.toolRuns[i].Result = ev.Result
				m.toolRuns[i].Status = "done"
				m.toolRuns[i].Duration = dur
				if m.toolStats == nil {
					m.toolStats = make(map[string]*toolStat)
				}
				if st, ok := m.toolStats[ev.Tool]; ok {
					st.Calls++
					st.Total += dur
					if dur > st.Max {
						st.Max = dur
					}
				} else {
					m.toolStats[ev.Tool] = &toolStat{Calls: 1, Total: dur, Max: dur}
				}
				// Track cost per tool
				if m.tracker != nil {
					_ = m.tracker.TotalInputTokens + m.tracker.TotalOutputTokens
				}
				break
			}
		}
	case "tool_error":
		for i, tr := range m.toolRuns {
			if tr.Name == ev.Tool && tr.Status == "running" {
				m.toolRuns[i].Result = ev.Err.Error()
				m.toolRuns[i].Status = "error"
				m.toolRuns[i].Duration = time.Since(tr.StartedAt)
				break
			}
		}
	case "turn_done":
		if m.cancel != nil {
			m.cancel()
			m.cancel = nil
		}
		m.running = false
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

	prompt, reply := "", ""
	var toolNames []string
	for _, msg := range m.messages {
		if msg.Sender == "user" && prompt == "" {
			prompt = msg.Content
		}
		if msg.Sender == "devon" {
			reply = msg.Content
		}
	}
	for _, tr := range m.toolRuns {
		toolNames = append(toolNames, tr.Name)
	}

	m.historyTurns = append(m.historyTurns, historyTurn{
		UserPrompt:  prompt,
		AgentReply:  reply,
		ToolSummary: strings.Join(toolNames, ", "),
		Timestamp:   time.Now().Format("15:04"),
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

// ═══════════════════ Slash Commands ═══════════════════════════════════

func (m *appModel) handleSlash(text string) {
	switch {
	case text == "/history":
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
	case text == "/usage":
		m.popup = m.tracker.Format()
	case strings.HasPrefix(text, "/load "):
		id := strings.TrimSpace(strings.TrimPrefix(text, "/load"))
		if ses, err := history.LoadSession(m.cfg.WorkDir, id); err == nil {
			m.session = ses
			m.messages = nil
			m.tracker = cost.NewSession(m.cfg.Model)
			m.tracker.TotalInputTokens = ses.Usage.PromptTokens
			m.tracker.TotalOutputTokens = ses.Usage.CompletionTokens
		}
	default:
		m.messages = append(m.messages, chatMessage{Sender: "system", Content: "Comando desconhecido: " + text})
	}
}

// ═══════════════════ Context Menu ═════════════════════════════════════

func (m *appModel) handleCtxMenuOpen() {
	if m.running {
		cm := buildContextMenu("turn_active", "")
		if cm != nil {
			m.ctxMenu = *cm
		}
		return
	}
	if m.session != nil {
		cm := buildContextMenu("session", m.session.ID)
		if cm != nil {
			m.ctxMenu = *cm
		}
	}
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
			m.messages = append(m.messages, chatMessage{Sender: "system", Content: "Agente interrompido."})
		}
	case "new_session":
		return func() tea.Msg { return tea.KeyMsg{Type: tea.KeyCtrlK} }
	case "copy_response":
		m.messages = append(m.messages, chatMessage{Sender: "system", Content: "Resposta copiada."})
	case "delete":
		if m.session != nil {
			if err := history.ClearSession(m.cfg.WorkDir, m.session.ID); err == nil {
				m.messages = append(m.messages, chatMessage{Sender: "system", Content: "Sessão deletada."})
			}
		}
	default:
		m.messages = append(m.messages, chatMessage{Sender: "system", Content: "Ação: " + action})
	}
	return nil
}

// ═══════════════════ Agent Command ═══════════════════════════════════╦

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

// ═══════════════════ Input Manipulation ═══════════════════════════════

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

// ═══════════════════ Helpers ═════════════════════════════════════════

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

func wrapLines(s string, width int) []string {
	if width <= 0 {
		return []string{s}
	}
	var out []string
	for len([]rune(s)) > width {
		ru := []rune(s)
		idx := width
		for i := width - 1; i > 0; i-- {
			if ru[i] == ' ' {
				idx = i
				break
			}
		}
		out = append(out, string(ru[:idx]))
		s = strings.TrimLeft(string(ru[idx:]), " ")
	}
	if s != "" {
		out = append(out, s)
	}
	return out
}

func wrapLine(s string, width int) []string {
	return wrapLines(s, width)
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

func maxInt(s []int) int {
	m := 0
	for _, v := range s {
		if v > m {
			m = v
		}
	}
	return m
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
