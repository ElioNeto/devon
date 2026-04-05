package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// rightView identifica qual view está ativa no painel direito.
type rightView int

const (
	viewTurnoAtivo rightView = iota
	viewToolCall
	viewHistoricoTurno
	viewFerramentas
	viewTokens
	viewMemoria
)

// renderRightPanel roteia para a view correta com base na seleção do painel esquerdo.
func renderRightPanel(m *appModel, width, height int, focused bool) string {
	s := m.styles

	var content string
	switch m.rightView {
	case viewTurnoAtivo:
		content = renderViewTurnoAtivo(m, width-4, height-2)
	case viewToolCall:
		content = renderViewToolCall(m, width-4, height-2)
	case viewHistoricoTurno:
		content = renderViewHistorico(m, width-4, height-2)
	case viewFerramentas:
		content = renderViewFerramentas(m, width-4, height-2)
	case viewTokens:
		content = renderViewTokens(m, width-4, height-2)
	case viewMemoria:
		content = renderViewMemoria(m, width-4, height-2)
	default:
		content = renderViewTurnoAtivo(m, width-4, height-2)
	}

	panelStyle := s.panelBase
	if focused {
		panelStyle = s.panelFocused
	}
	return panelStyle.
		Width(width).
		Height(height).
		Render(content)
}

// ── View: Turno ativo ─────────────────────────────────────────────────────────

func renderViewTurnoAtivo(m *appModel, width, height int) string {
	s := m.styles
	var lines []string

	lines = append(lines, s.panelTitle.Render("▶  Turno atual — stream"))
	lines = append(lines, strings.Repeat("─", width))

	for _, msg := range m.messages {
		switch msg.Sender {
		case "user":
			for _, l := range wrapLine("› "+msg.Content, width) {
				lines = append(lines, s.userMsg.Render(l))
			}
			lines = append(lines, "")
		case "devon":
			for _, l := range wrapLine(msg.Content, width) {
				if msg.IsError {
					lines = append(lines, s.errMsg.Render(l))
				} else {
					lines = append(lines, s.agentMsg.Render(l))
				}
			}
			lines = append(lines, "")
		case "system":
			lines = append(lines, s.sysMsg.Render("  — "+msg.Content))
		}
	}

	// Tool calls ativos
	if len(m.toolRuns) > 0 {
		lines = append(lines, "")
		for _, tr := range m.toolRuns {
			switch tr.Status {
			case "running":
				lines = append(lines, s.toolRunning.Render(fmt.Sprintf("  ⏳ %s(%s)", tr.Name, shortenArgs(tr.Args))))
			case "done":
				lines = append(lines, s.toolDone.Render(fmt.Sprintf("  ✔ %s(%s)", tr.Name, shortenArgs(tr.Args))))
			case "error":
				lines = append(lines, s.toolError.Render(fmt.Sprintf("  ✘ %s: %s", tr.Name, firstLine(tr.Result))))
			}
		}
	}

	return joinLines(lines, height)
}

// ── View: Tool Call ───────────────────────────────────────────────────────────

func renderViewToolCall(m *appModel, width, height int) string {
	s := m.styles
	var lines []string

	if m.selectedTool == nil {
		if len(m.toolRuns) == 0 {
			return s.sysMsg.Render("Nenhum tool call neste turno.")
		}
		m.selectedTool = &m.toolRuns[0]
	}
	tr := m.selectedTool

	lines = append(lines, s.panelTitle.Render("⚙  Tool call — "+tr.Name))
	lines = append(lines, strings.Repeat("─", width))
	lines = append(lines, "")

	statIcon := "⏳"
	statStyle := s.toolRunning
	if tr.Status == "done" {
		statIcon = "✔"
		statStyle = s.toolDone
	} else if tr.Status == "error" {
		statIcon = "✘"
		statStyle = s.toolError
	}
	lines = append(lines, statStyle.Render(statIcon+" "+tr.Name))
	lines = append(lines, "")

	// Input
	lines = append(lines, s.statusKey.Render("Input:"))
	for _, l := range wrapLine(tr.Args, width) {
		lines = append(lines, "  "+l)
	}
	lines = append(lines, "")

	// Output / Diff
	if tr.Result != "" {
		if isDiff(tr.Result) {
			lines = append(lines, s.statusKey.Render("Diff:"))
			for _, dl := range strings.Split(tr.Result, "\n") {
				switch {
				case strings.HasPrefix(dl, "+"):
					lines = append(lines, s.diffAdd.Render(dl))
				case strings.HasPrefix(dl, "-"):
					lines = append(lines, s.diffDel.Render(dl))
				case strings.HasPrefix(dl, "@@"):
					lines = append(lines, s.diffHunk.Render(dl))
				default:
					lines = append(lines, "  "+dl)
				}
			}
		} else {
			lines = append(lines, s.statusKey.Render("Output:"))
			resultLines := strings.Split(tr.Result, "\n")
			maxShow := height - len(lines) - 2
			if maxShow < 1 {
				maxShow = 5
			}
			for i, rl := range resultLines {
				if i >= maxShow {
					lines = append(lines, s.sysMsg.Render(fmt.Sprintf("  … +%d linhas (pressione 'e' para expandir)", len(resultLines)-maxShow)))
					break
				}
				lines = append(lines, "  "+rl)
			}
		}
	}

	return joinLines(lines, height)
}

// ── View: Histórico de turno ──────────────────────────────────────────────────

func renderViewHistorico(m *appModel, width, height int) string {
	s := m.styles
	var lines []string

	if m.selectedTurnIdx < 0 || m.selectedTurnIdx >= len(m.historyTurns) {
		return s.sysMsg.Render("Nenhum turno selecionado.")
	}
	ht := m.historyTurns[m.selectedTurnIdx]

	lines = append(lines, s.panelTitle.Render(fmt.Sprintf("○  Turno %d — %s", m.selectedTurnIdx+1, ht.Timestamp)))
	lines = append(lines, strings.Repeat("─", width))
	lines = append(lines, "")
	lines = append(lines, s.statusKey.Render("Tokens:"))
	lines = append(lines, fmt.Sprintf("  %s prompt + %s completion = %s total",
		formatShort(ht.PromptTokens),
		formatShort(ht.CompletionTokens),
		formatShort(ht.PromptTokens+ht.CompletionTokens),
	))
	lines = append(lines, "")
	lines = append(lines, s.statusKey.Render("Ferramentas:"))
	lines = append(lines, "  "+ht.ToolSummary)
	lines = append(lines, "")
	lines = append(lines, s.statusKey.Render("Mensagem:"))
	lines = append(lines, "")
	for _, l := range wrapLine(ht.UserPrompt, width) {
		lines = append(lines, s.userMsg.Render("  "+l))
	}
	lines = append(lines, "")
	for _, l := range wrapLine(ht.AgentReply, width) {
		lines = append(lines, s.agentMsg.Render("  "+l))
	}

	return joinLines(lines, height)
}

// ── View: Estatísticas de ferramentas ─────────────────────────────────────────

func renderViewFerramentas(m *appModel, width, height int) string {
	s := m.styles
	var lines []string

	lines = append(lines, s.panelTitle.Render("◆  Ferramentas — estatísticas"))
	lines = append(lines, strings.Repeat("─", width))
	lines = append(lines, "")

	if len(m.toolStats) == 0 {
		lines = append(lines, s.sysMsg.Render("Nenhuma ferramenta usada ainda."))
		return joinLines(lines, height)
	}

	// Barra horizontal por ferramenta
	maxCalls := 1
	for _, st := range m.toolStats {
		if st.Calls > maxCalls {
			maxCalls = st.Calls
		}
	}

	lines = append(lines, s.statusKey.Render(fmt.Sprintf("  %-12s  %-20s  %6s  %8s  %8s", "Ferramenta", "Chamadas", "Chamadas", "Avg", "Max")))

	for name, st := range m.toolStats {
		bar := HorizBar(name, st.Calls, maxCalls, width-4, 12)
		avgStr := formatDuration(st.AvgMs)
		maxStr := formatDuration(st.MaxMs)
		lines = append(lines, s.chartBar.Render("  ")+bar+s.chartLabel.Render(fmt.Sprintf("  avg %s  max %s", avgStr, maxStr)))
	}

	// Gráfico de tempo por turno
	lines = append(lines, "")
	lines = append(lines, strings.Repeat("─", width))
	lines = append(lines, s.panelTitle.Render("Tempo por turno (ms)"))
	lines = append(lines, "")

	if len(m.historyTurns) > 0 {
		labels := make([]string, len(m.historyTurns))
		vals := make([]int, len(m.historyTurns))
		for i, ht := range m.historyTurns {
			labels[i] = fmt.Sprintf("T%d", i+1)
			vals[i] = int(ht.DurationMs)
		}
		chartH := height - len(lines) - 4
		if chartH < 3 {
			chartH = 3
		}
		for _, row := range VertBars(labels, vals, width-4, chartH) {
			lines = append(lines, "  "+row)
		}
	}

	return joinLines(lines, height)
}

// ── View: Tokens ──────────────────────────────────────────────────────────────

func renderViewTokens(m *appModel, width, height int) string {
	s := m.styles
	var lines []string

	total := m.tracker.TotalInputTokens + m.tracker.TotalOutputTokens

	lines = append(lines, s.panelTitle.Render("▦  Consumo de tokens"))
	lines = append(lines, strings.Repeat("─", width))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("  Total: %s   Custo estimado: %s",
		formatShort(total),
		formatCostStr(m.tracker.TotalCostUSD),
	))
	lines = append(lines, fmt.Sprintf("  Contexto atual: %s / %s (%d%%)",
		formatShort(total),
		formatShort(m.cfg.MaxTokens),
		safePct(total, m.cfg.MaxTokens),
	))
	lines = append(lines, "")

	// Sparkline de tokens por turno
	if len(m.historyTurns) > 0 {
		lines = append(lines, s.statusKey.Render("  Tokens por turno:"))
		vals := make([]int, len(m.historyTurns))
		for i, ht := range m.historyTurns {
			vals[i] = ht.PromptTokens + ht.CompletionTokens
		}
		lines = append(lines, "  "+lipgloss.NewStyle().Foreground(colorPrimary).Render(Sparkline(vals, width-6)))
		lines = append(lines, "")
	}

	// Gráfico de barras verticais
	if len(m.historyTurns) > 0 {
		labels := make([]string, len(m.historyTurns))
		vals := make([]int, len(m.historyTurns))
		for i, ht := range m.historyTurns {
			labels[i] = fmt.Sprintf("T%d", i+1)
			vals[i] = ht.PromptTokens + ht.CompletionTokens
		}
		chartH := height - len(lines) - 4
		if chartH < 3 {
			chartH = 3
		}
		for _, row := range VertBars(labels, vals, width-4, chartH) {
			lines = append(lines, lipgloss.NewStyle().Foreground(colorPrimary).Render("  "+row))
		}
	}

	return joinLines(lines, height)
}

// ── View: Memória ─────────────────────────────────────────────────────────────

func renderViewMemoria(m *appModel, width, height int) string {
	s := m.styles
	var lines []string

	lines = append(lines, s.panelTitle.Render("◈  Memória do projeto"))
	lines = append(lines, strings.Repeat("─", width))
	lines = append(lines, "")

	if len(m.memoryFacts) == 0 {
		lines = append(lines, s.sysMsg.Render("Nenhum fato salvo ainda."))
		lines = append(lines, "")
		lines = append(lines, s.sysMsg.Render("O agente usa 'remember' para guardar convenções,"))
		lines = append(lines, s.sysMsg.Render("decisões e padrões de erro do projeto."))
		return joinLines(lines, height)
	}

	// Agrupa por categoria
	cats := map[string][]string{}
	for _, f := range m.memoryFacts {
		cats[f.Category] = append(cats[f.Category], f.Key+": "+f.Value)
	}

	for cat, facts := range cats {
		lines = append(lines, s.statusKey.Render(fmt.Sprintf("  %s  (%d)", cat, len(facts))))
		for _, fact := range facts {
			for _, l := range wrapLine("    • "+fact, width) {
				lines = append(lines, s.agentMsg.Render(l))
			}
		}
		lines = append(lines, "")
	}

	return joinLines(lines, height)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func joinLines(lines []string, height int) string {
	for len(lines) < height {
		lines = append(lines, "")
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	return strings.Join(lines, "\n")
}

func isDiff(s string) bool {
	return strings.Contains(s, "\n+") || strings.Contains(s, "\n-") || strings.HasPrefix(s, "@@")
}

func safePct(a, b int) int {
	if b == 0 {
		return 0
	}
	return a * 100 / b
}

func formatCostStr(v float64) string {
	if v == 0 {
		return "$0.00"
	}
	if v < 0.01 {
		return fmt.Sprintf("$%.4f", v)
	}
	return fmt.Sprintf("$%.2f", v)
}
