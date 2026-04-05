package tui

import (
	"fmt"
	"strings"

	"github.com/ElioNeto/devon/internal/agent"
	"github.com/charmbracelet/lipgloss"
)

// ── Bubble Tea message types ──────────────────────────────────────────────────

type agentEventMsg agent.Event

type agentResult struct {
	events []agent.Event
}

// ── Left panel: sections & items ─────────────────────────────────────────────

type leftSection int

const (
	secTurno      leftSection = iota // current turn + tool calls
	secHistorico                     // past turns
	secFerramentas                   // tool stats
	secMemoria                       // memory facts
	secTokens                        // token chart
)

type leftItem struct {
	Label      string
	StatusKind string // "header" | "running" | "done" | "error" | "system" | ""
	Section    leftSection
	Index      int // position within section (0 = section header item)
}

// buildLeftItems creates the full flat list for the left panel.
func buildLeftItems(m *appModel) []leftItem {
	var items []leftItem

	// ── Seção: Turno Ativo ──────────────────────────────────
	items = append(items, leftItem{Label: "Turno Ativo", StatusKind: "header", Section: secTurno})
	if m.running {
		items = append(items, leftItem{Label: "  " + m.spinner.View() + " processando...", StatusKind: "running", Section: secTurno})
	} else if len(m.messages) > 0 {
		last := m.messages[len(m.messages)-1]
		lbl := truncate(firstLine(last.Content), 28)
		kind := "done"
		if last.IsError {
			kind = "error"
		}
		items = append(items, leftItem{Label: "  " + lbl, StatusKind: kind, Section: secTurno})
	}
	for i, tr := range m.toolRuns {
		icon := "⚙"
		switch tr.Status {
		case "running":
			icon = "⟳"
		case "done":
			icon = "✓"
		case "error":
			icon = "✗"
		}
		lbl := fmt.Sprintf("  %s %s %s", icon, truncate(tr.Name, 14), truncate(shortenArgs(tr.Args), 10))
		items = append(items, leftItem{Label: lbl, StatusKind: tr.Status, Section: secTurno, Index: i + 1})
	}

	// ── Seção: Histórico ────────────────────────────────────
	items = append(items, leftItem{Label: "Histórico", StatusKind: "header", Section: secHistorico})
	if len(m.historyTurns) == 0 {
		items = append(items, leftItem{Label: "  (sem histórico)", StatusKind: "system", Section: secHistorico})
	} else {
		for i, ht := range m.historyTurns {
			lbl := truncate(firstLine(ht.UserPrompt), 28)
			if lbl == "" {
				lbl = "(turno " + fmt.Sprint(i+1) + ")"
			}
			items = append(items, leftItem{Label: "  " + lbl, StatusKind: "done", Section: secHistorico, Index: i})
		}
	}

	// ── Seção: Ferramentas ──────────────────────────────────
	items = append(items, leftItem{Label: "Ferramentas", StatusKind: "header", Section: secFerramentas})
	if len(m.toolStats) == 0 {
		items = append(items, leftItem{Label: "  (sem chamadas)", StatusKind: "system", Section: secFerramentas})
	} else {
		for name, st := range m.toolStats {
			lbl := fmt.Sprintf("  %-14s %2dx", truncate(name, 14), st.Calls)
			items = append(items, leftItem{Label: lbl, StatusKind: "done", Section: secFerramentas})
		}
	}

	// ── Seção: Memória ──────────────────────────────────────
	items = append(items, leftItem{Label: "Memória", StatusKind: "header", Section: secMemoria})
	if len(m.memoryFacts) == 0 {
		items = append(items, leftItem{Label: "  (vazio)", StatusKind: "system", Section: secMemoria})
	} else {
		for _, f := range m.memoryFacts {
			lbl := fmt.Sprintf("  %s: %s", truncate(f.Key, 10), truncate(f.Value, 16))
			items = append(items, leftItem{Label: lbl, StatusKind: "", Section: secMemoria})
		}
	}

	// ── Seção: Tokens ───────────────────────────────────────
	items = append(items, leftItem{Label: "Tokens", StatusKind: "header", Section: secTokens})
	if m.tracker != nil {
		totalTok := m.tracker.TotalInputTokens + m.tracker.TotalOutputTokens
		items = append(items, leftItem{
			Label:      fmt.Sprintf("  total: %s", formatShort(totalTok)),
			StatusKind: "system",
			Section:    secTokens,
		})
		if len(m.tokenPerTurn) > 0 {
			spark := Sparkline(m.tokenPerTurn, 20)
			items = append(items, leftItem{Label: "  " + spark, StatusKind: "", Section: secTokens})
		}
	}

	return items
}

// ── Right panel views ─────────────────────────────────────────────────────────

type rightView int

const (
	viewTurnoAtivo    rightView = iota
	viewToolCall
	viewHistoricoTurno
	viewFerramentas
	viewMemoria
	viewTokens
)

// ── Context menu (action-func style for updateMenu) ───────────────────────────

type menuAction struct {
	Label  string
	Key    string
	Action func(*appModel)
}

func contextMenuFor(m *appModel) []menuAction {
	var actions []menuAction
	if m.running {
		actions = append(actions, menuAction{"Interromper agente", "i", func(m *appModel) {
			if m.cancel != nil {
				m.cancel()
				m.running = false
				m.messages = append(m.messages, chatMessage{Sender: "system", Content: "Agente interrompido."})
			}
		}})
	}
	actions = append(actions, menuAction{"Nova sessão [Ctrl+K]", "n", func(m *appModel) {
		m.messages = nil
		m.toolRuns = nil
		m.historyTurns = nil
	}})
	actions = append(actions, menuAction{"Limpar chat [Ctrl+L]", "l", func(m *appModel) {
		m.messages = nil
		m.toolRuns = nil
	}})
	actions = append(actions, menuAction{"Ver uso de tokens", "t", func(m *appModel) {
		if m.tracker != nil {
			m.popup = m.tracker.Format()
		}
	}})
	actions = append(actions, menuAction{"Ajuda", "?", func(m *appModel) {
		m.showHelp = true
	}})
	return actions
}

// ── Rendering: Left Panel ─────────────────────────────────────────────────────

func renderLeftPanel(m *appModel, width, height int, focused bool) string {
	s := m.styles
	items := buildLeftItems(m)
	m.leftItems = items

	var lines []string

	for idx, item := range items {
		var line string
		if item.StatusKind == "header" {
			// Section header: ─── Label ───
			titleW := width - 4
			if titleW < 4 {
				titleW = 4
			}
			dash := strings.Repeat("─", 2)
			lbl := s.itemSection.Render(item.Label)
			line = dash + lbl + dash
		} else {
			labelStyle := s.itemNormal
			if idx == m.leftCursor {
				labelStyle = s.itemSelected
			}
			// Status icon on left
			var statusSfx string
			switch item.StatusKind {
			case "running":
				statusIcon := s.toolRunning.Render("●")
				statusLine := item.Label
				pad := width - lipgloss.Width(statusLine) - 2
				if pad < 0 {
					pad = 0
				}
				line = labelStyle.Render(statusLine) + strings.Repeat(" ", pad) + statusIcon
				lines = append(lines, line)
				continue
			case "done":
				statusIcon := s.toolDone.Render("●")
				pad := width - lipgloss.Width(item.Label) - 2
				if pad < 0 {
					pad = 0
				}
				line = labelStyle.Render(item.Label) + strings.Repeat(" ", pad) + statusIcon
				lines = append(lines, line)
				continue
			case "error":
				statusIcon := s.toolError.Render("●")
				pad := width - lipgloss.Width(item.Label) - 2
				if pad < 0 {
					pad = 0
				}
				line = labelStyle.Render(item.Label) + strings.Repeat(" ", pad) + statusIcon
				lines = append(lines, line)
				continue
			default:
				_ = statusSfx
				line = labelStyle.Width(width - 1).Render(item.Label)
			}
		}
		lines = append(lines, line)
	}

	// Pad to height
	for len(lines) < height {
		lines = append(lines, "")
	}
	if len(lines) > height {
		lines = lines[:height]
	}

	content := strings.Join(lines, "\n")

	borderStyle := s.panelBase
	if focused {
		borderStyle = s.panelFocused
	}
	return borderStyle.
		Width(width - 2).
		Height(height - 2).
		Render(content)
}

// ── Rendering: Right Panel ────────────────────────────────────────────────────

func renderRightPanel(m *appModel, width, height int, focused bool) string {
	s := m.styles
	var content string

	switch m.rightView {
	case viewTurnoAtivo:
		content = renderViewTurno(m, width-4, height-2)
	case viewToolCall:
		content = renderViewToolCall(m, width-4, height-2)
	case viewHistoricoTurno:
		content = renderViewHistorico(m, width-4, height-2)
	case viewFerramentas:
		content = renderViewFerramentas(m, width-4, height-2)
	case viewMemoria:
		content = renderViewMemoria(m, width-4, height-2)
	case viewTokens:
		content = renderViewTokens(m, width-4, height-2)
	default:
		content = renderViewTurno(m, width-4, height-2)
	}

	borderStyle := s.panelBase
	if focused {
		borderStyle = s.panelFocused
	}
	return borderStyle.
		Width(width - 2).
		Height(height - 2).
		Render(content)
}

// ── Right Panel Views ─────────────────────────────────────────────────────────

func renderViewTurno(m *appModel, width, height int) string {
	s := m.styles
	var lines []string

	// Title row
	titleStr := s.itemSection.Render("─── Turno Ativo ───")
	lines = append(lines, titleStr)
	lines = append(lines, "")

	if len(m.messages) == 0 {
		lines = append(lines, s.sysMsg.Render("  Nenhuma mensagem ainda."))
		lines = append(lines, "")
		lines = append(lines, s.sysMsg.Render("  Digite uma mensagem abaixo e pressione Enter."))
		lines = append(lines, s.sysMsg.Render("  Use x para o menu de contexto."))
	} else {
		// Show messages, scrolled
		var msgLines []string
		for _, msg := range m.messages {
			switch msg.Sender {
			case "user":
				msgLines = append(msgLines, s.userMsg.Render("▸ "+truncate(msg.Content, width-4)))
			case "devon":
				if msg.IsError {
					for _, l := range wrapLine(msg.Content, width-4) {
						msgLines = append(msgLines, s.errMsg.Render("  "+l))
					}
				} else {
					for _, l := range wrapLine(msg.Content, width-4) {
						msgLines = append(msgLines, s.agentMsg.Render("  "+l))
					}
				}
			case "system":
				msgLines = append(msgLines, s.sysMsg.Render("  "+truncate(msg.Content, width-4)))
			}
			msgLines = append(msgLines, "")
		}

		// Apply scroll offset
		avail := height - 3
		start := 0
		if len(msgLines) > avail {
			start = len(msgLines) - avail - m.rightScroll*2
			if start < 0 {
				start = 0
			}
		}
		visible := msgLines[start:]
		if len(visible) > avail {
			visible = visible[:avail]
		}
		lines = append(lines, visible...)
	}

	// Tool runs
	if len(m.toolRuns) > 0 {
		lines = append(lines, "")
		lines = append(lines, s.itemSection.Render("─── Ferramentas ───"))
		for _, tr := range m.toolRuns {
			var style lipgloss.Style
			var icon string
			switch tr.Status {
			case "running":
				style = s.toolRunning
				icon = "⟳"
			case "done":
				style = s.toolDone
				icon = "✓"
			case "error":
				style = s.toolError
				icon = "✗"
			default:
				style = s.sysMsg
				icon = "·"
			}
			line := fmt.Sprintf("  %s %-16s %s", icon, truncate(tr.Name, 16), truncate(shortenArgs(tr.Args), width-24))
			lines = append(lines, style.Render(line))
			if tr.Status != "running" && tr.Result != "" {
				result := truncate(firstLine(tr.Result), width-6)
				lines = append(lines, s.sysMsg.Render("    → "+result))
			}
		}
	}

	return strings.Join(lines, "\n")
}

func renderViewToolCall(m *appModel, width, height int) string {
	s := m.styles
	var lines []string

	if m.selectedTool == nil {
		return s.sysMsg.Render("  Selecione uma tool call no painel esquerdo.")
	}
	tr := m.selectedTool

	lines = append(lines, s.itemSection.Render("─── Tool Call: "+tr.Name+" ───"))
	lines = append(lines, "")

	// Status badge
	var statusStyle lipgloss.Style
	switch tr.Status {
	case "running":
		statusStyle = s.toolRunning
	case "done":
		statusStyle = s.toolDone
	case "error":
		statusStyle = s.toolError
	default:
		statusStyle = s.sysMsg
	}
	lines = append(lines, s.configKey.Render("Status: ")+statusStyle.Render(tr.Status))
	lines = append(lines, "")

	// Args section
	lines = append(lines, s.configKey.Render("Input:"))
	for _, l := range wrapLine(tr.Args, width-4) {
		lines = append(lines, s.configVal.Render("  "+l))
	}
	lines = append(lines, "")

	// Result section
	if tr.Result != "" {
		lines = append(lines, s.configKey.Render("Output:"))
		resultLines := strings.Split(tr.Result, "\n")
		maxResultLines := height - len(lines) - 2
		if maxResultLines < 3 {
			maxResultLines = 3
		}
		shown := resultLines
		if len(shown) > maxResultLines {
			shown = shown[:maxResultLines]
			shown = append(shown, s.sysMsg.Render("  ... (truncado, "+fmt.Sprint(len(resultLines)-maxResultLines)+" linhas)"))
		}
		for _, l := range shown {
			// Basic diff coloring
			if strings.HasPrefix(l, "+") {
				lines = append(lines, s.diffAdd.Render(l))
			} else if strings.HasPrefix(l, "-") {
				lines = append(lines, s.diffDel.Render(l))
			} else if strings.HasPrefix(l, "@@") {
				lines = append(lines, s.diffHunk.Render(l))
			} else {
				lines = append(lines, s.sysMsg.Render("  "+l))
			}
		}
	}

	return strings.Join(lines, "\n")
}

func renderViewHistorico(m *appModel, width, height int) string {
	s := m.styles
	var lines []string

	idx := m.selectedTurnIdx
	if idx < 0 || idx >= len(m.historyTurns) {
		lines = append(lines, s.sysMsg.Render("  Selecione um turno no painel esquerdo."))
		return strings.Join(lines, "\n")
	}

	ht := m.historyTurns[idx]
	lines = append(lines, s.itemSection.Render(fmt.Sprintf("─── Turno %d ───", idx+1)))
	lines = append(lines, "")
	lines = append(lines, s.configKey.Render("Prompt:"))
	for _, l := range wrapLine(ht.UserPrompt, width-4) {
		lines = append(lines, s.userMsg.Render("  "+l))
	}
	lines = append(lines, "")
	lines = append(lines, s.configKey.Render("Resposta:"))
	for _, l := range wrapLine(ht.AgentReply, width-4) {
		lines = append(lines, s.agentMsg.Render("  "+l))
	}
	if ht.ToolSummary != "" {
		lines = append(lines, "")
		lines = append(lines, s.configKey.Render("Tools: ")+s.sysMsg.Render(ht.ToolSummary))
	}
	if ht.PromptTokens+ht.CompletionTokens > 0 {
		lines = append(lines, "")
		lines = append(lines, s.configKey.Render("Tokens: ")+
			s.configVal.Render(fmt.Sprintf("in=%s out=%s",
				formatShort(ht.PromptTokens),
				formatShort(ht.CompletionTokens))))
	}

	_ = height
	return strings.Join(lines, "\n")
}

func renderViewFerramentas(m *appModel, width, height int) string {
	s := m.styles
	var lines []string

	lines = append(lines, s.itemSection.Render("─── Ferramentas ───"))
	lines = append(lines, "")

	if len(m.toolStats) == 0 {
		lines = append(lines, s.sysMsg.Render("  Nenhuma ferramenta usada ainda."))
		return strings.Join(lines, "\n")
	}

	// Table header
	heatWidth := width - 4
	lines = append(lines, s.tableHeader.Render(
		fmt.Sprintf("  %-20s %6s %8s %8s", "Ferramenta", "Calls", "AvgMs", "MaxMs"),
	))
	lines = append(lines, s.tableHeader.Render(strings.Repeat("─", min(heatWidth, 50))))

	maxCalls := 1
	for _, st := range m.toolStats {
		if st.Calls > maxCalls {
			maxCalls = st.Calls
		}
	}

	for name, st := range m.toolStats {
		barW := int(float64(st.Calls) / float64(maxCalls) * 20)
		bar := strings.Repeat("█", barW) + strings.Repeat("░", 20-barW)
		row := fmt.Sprintf("  %-20s %6d |%s| %5dms",
			truncate(name, 20), st.Calls, bar, st.MaxMs)
		lines = append(lines, s.tableRow.Render(row))
	}

	_ = height
	return strings.Join(lines, "\n")
}

func renderViewMemoria(m *appModel, width, height int) string {
	s := m.styles
	var lines []string

	lines = append(lines, s.itemSection.Render("─── Memória ───"))
	lines = append(lines, "")

	if len(m.memoryFacts) == 0 {
		lines = append(lines, s.sysMsg.Render("  Nenhum fato memorizado."))
		return strings.Join(lines, "\n")
	}

	for _, f := range m.memoryFacts {
		key := s.configKey.Render(fmt.Sprintf("  [%s] %s: ", f.Category, f.Key))
		val := s.configVal.Render(truncate(f.Value, width-30))
		lines = append(lines, key+val)
	}

	_ = height
	return strings.Join(lines, "\n")
}

func renderViewTokens(m *appModel, width, height int) string {
	s := m.styles
	var lines []string

	lines = append(lines, s.itemSection.Render("─── Uso de Tokens ───"))
	lines = append(lines, "")

	if m.tracker == nil {
		lines = append(lines, s.sysMsg.Render("  Sem dados."))
		return strings.Join(lines, "\n")
	}

	// KPIs
	totalIn := m.tracker.TotalInputTokens
	totalOut := m.tracker.TotalOutputTokens
	total := totalIn + totalOut
	lines = append(lines, s.configKey.Render("  Input:  ")+s.configVal.Render(formatShort(totalIn)))
	lines = append(lines, s.configKey.Render("  Output: ")+s.configVal.Render(formatShort(totalOut)))
	lines = append(lines, s.configKey.Render("  Total:  ")+s.configVal.Render(formatShort(total)))
	lines = append(lines, s.configKey.Render("  Custo:  ")+s.configVal.Render(m.tracker.Format()))
	lines = append(lines, "")

	// Sparkline
	if len(m.tokenPerTurn) > 1 {
		lines = append(lines, s.configKey.Render("  Tokens por turno:"))
		spark := Sparkline(m.tokenPerTurn, width-6)
		lines = append(lines, "  "+s.configVal.Render(spark))
		lines = append(lines, "")
	}

	// Bar chart por turno
	if len(m.tokenPerTurn) > 0 {
		lines = append(lines, s.configKey.Render("  Detalhes:"))
		maxT := 1
		for _, v := range m.tokenPerTurn {
			if v > maxT {
				maxT = v
			}
		}
		for i, v := range m.tokenPerTurn {
			barW := int(float64(v) / float64(maxT) * float64(width-20))
			if barW < 1 {
				barW = 1
			}
			bar := strings.Repeat("█", barW)
			row := fmt.Sprintf("  T%-3d %s %s", i+1, bar, formatShort(v))
			lines = append(lines, s.tableRow.Render(row))
		}
	}

	_ = height
	return strings.Join(lines, "\n")
}

// ── Help overlay ──────────────────────────────────────────────────────────────

func renderHelp(m *appModel, width int) string {
	s := m.styles
	var lines []string
	lines = append(lines, s.itemSection.Render("─── Ajuda ───"))
	lines = append(lines, "")
	for _, h := range AllHints() {
		lines = append(lines, s.keyStyle.Render(fmt.Sprintf("  %-18s", h.Keys))+s.helpStyle.Render(h.Action))
	}
	lines = append(lines, "")
	lines = append(lines, s.sysMsg.Render("  Pressione qualquer tecla para fechar."))
	return s.menuStyle.Width(min(width-4, 60)).Render(strings.Join(lines, "\n"))
}
