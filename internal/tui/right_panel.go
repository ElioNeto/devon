package tui

import (
	"fmt"
	"strings"
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
		content = renderViewConfig(m, width-4, height-2)
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
		content = renderViewConfig(m, width-4, height-2)
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

// ── View: Config (lazydocker-style right panel) ───────────────────────────────
//
// Layout mirrors the reference image:
//
//	Name:     <none>
//	ID:       sha256:...
//	Tags:     <none>:<none>
//	Size:     144.02MB
//	Created:  Sat, 29 Jun 2019 ...
//
//	ID         TAG   SIZE   COMMAND
//	abc123...  ...   679kB  COPY ...

func renderViewConfig(m *appModel, width, height int) string {
	s := m.styles
	var lines []string

	// ── Header box: Config ────────────────────────────────────
	lines = append(lines, s.panelTitle.Render("Config"))
	lines = append(lines, "")

	// Determine selected item details
	var name, id, tags, size, created string

	if m.session != nil {
		name = m.cfg.Model
		id = "sess:" + m.session.ID
		tags = m.cfg.Mode.String() + ":" + m.cfg.WorkDir
		tokens := m.tracker.TotalInputTokens + m.tracker.TotalOutputTokens
		size = formatShort(tokens) + " tokens"
		created = m.session.CreatedAt.Format("Mon, 02 Jan 2006 15:04:05")
	} else {
		name = "<none>"
		id = "<none>"
		tags = "<none>:<none>"
		size = "0"
		created = "—"
	}

	// Render key-value pairs aligned
	keyW := 9
	pairs := []struct{ k, v string }{
		{"Name:", name},
		{"ID:", id},
		{"Tags:", tags},
		{"Size:", size},
		{"Created:", created},
	}
	for _, p := range pairs {
		key := s.configKey.Render(fmt.Sprintf("%-*s", keyW, p.k))
		val := s.configVal.Render(p.v)
		lines = append(lines, key+" "+val)
	}
	lines = append(lines, "")

	// ── Layer table ───────────────────────────────────────────
	// Column widths
	colID := 12
	colTag := 14
	colSize := 12
	colCmd := width - colID - colTag - colSize - 3
	if colCmd < 10 {
		colCmd = 10
	}

	headerLine := fmt.Sprintf("%-*s %-*s %-*s %s",
		colID, "ID",
		colTag, "TAG",
		colSize, "SIZE",
		"COMMAND",
	)
	lines = append(lines, s.tableHeader.Render(headerLine))

	// Rows from tool runs (layers / commands)
	if len(m.toolRuns) == 0 && len(m.messages) == 0 {
		// Example placeholder rows (shown when idle, like lazydocker)
		examples := []struct{ id, tag, size, cmd string }{
			{"<missing>", "", "0B", "ENV DEVON_READY=1"},
			{"<missing>", "", "0B", "CMD [\"devon\"]"},
		}
		for _, ex := range examples {
			rowStr := fmt.Sprintf("%-*s %-*s %-*s %s",
				colID, ex.id,
				colTag, ex.tag,
				colSize, ex.size,
				truncate(ex.cmd, colCmd),
			)
			lines = append(lines, s.tableRowMissing.Render(rowStr))
		}
	} else {
		for i, tr := range m.toolRuns {
			// Derive a short hash-like ID from index
			shortID := fmt.Sprintf("%08x", i*0xdeadbeef%0xffffffff)
			if len(shortID) > colID-1 {
				shortID = shortID[:colID-1]
			}

			// Tag = status
			var tag string
			switch tr.Status {
			case "running":
				tag = s.statusRunning.Render("running")
			case "done":
				tag = s.toolDone.Render("done")
			case "error":
				tag = s.toolError.Render("error")
			default:
				tag = "—"
			}

			resultSize := formatShort(len(tr.Result)) + "B"

			cmdStr := truncate(tr.Name+"("+tr.Args+")", colCmd)

			// We can't mix styled tag into a single format string cleanly,
			// so build manually.
			idPart := fmt.Sprintf("%-*s ", colID, shortID)
			tagPart := tag + strings.Repeat(" ", max(0, colTag-len(tr.Status)))
			sizePart := fmt.Sprintf("%-*s ", colSize, resultSize)

			var rowStyle = s.tableRow
			if i == m.rightScroll {
				rowStyle = s.tableRowSel
			}
			lines = append(lines, rowStyle.Render(idPart)+tagPart+rowStyle.Render(sizePart+cmdStr))
		}
	}

	// Active messages (chat stream)
	if len(m.messages) > 0 {
		lines = append(lines, "")
		lines = append(lines, s.panelTitle.Render("Stream"))
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

	lines = append(lines, s.configKey.Render("Input:"))
	for _, l := range wrapLine(tr.Args, width) {
		lines = append(lines, "  "+l)
	}
	lines = append(lines, "")

	if tr.Result != "" {
		if isDiff(tr.Result) {
			lines = append(lines, s.configKey.Render("Diff:"))
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
			lines = append(lines, s.configKey.Render("Output:"))
			resultLines := strings.Split(tr.Result, "\n")
			maxShow := height - len(lines) - 2
			if maxShow < 1 {
				maxShow = 5
			}
			for i, rl := range resultLines {
				if i >= maxShow {
					lines = append(lines, s.sysMsg.Render(fmt.Sprintf("  … +%d linhas", len(resultLines)-maxShow)))
					break
				}
				lines = append(lines, "  "+rl)
			}
		}
	}

	return joinLines(lines, height)
}

// ── View: Histórico ───────────────────────────────────────────────────────────

func renderViewHistorico(m *appModel, width, height int) string {
	s := m.styles
	var lines []string

	if m.selectedTurnIdx < 0 || m.selectedTurnIdx >= len(m.historyTurns) {
		return s.sysMsg.Render("Nenhum turno selecionado.")
	}
	ht := m.historyTurns[m.selectedTurnIdx]

	lines = append(lines, s.panelTitle.Render(fmt.Sprintf("Turno %d — %s", m.selectedTurnIdx+1, ht.Timestamp)))
	lines = append(lines, strings.Repeat("─", width))
	lines = append(lines, "")
	lines = append(lines, s.configKey.Render("Tokens:"))
	lines = append(lines, fmt.Sprintf("  %s prompt + %s completion",
		formatShort(ht.PromptTokens),
		formatShort(ht.CompletionTokens),
	))
	lines = append(lines, "")
	lines = append(lines, s.configKey.Render("Ferramentas: ")+ht.ToolSummary)
	lines = append(lines, "")
	lines = append(lines, s.configKey.Render("Prompt:"))
	for _, l := range wrapLine(ht.UserPrompt, width) {
		lines = append(lines, s.userMsg.Render("  "+l))
	}
	lines = append(lines, "")
	lines = append(lines, s.configKey.Render("Resposta:"))
	for _, l := range wrapLine(ht.AgentReply, width) {
		lines = append(lines, s.agentMsg.Render("  "+l))
	}

	return joinLines(lines, height)
}

// ── View: Ferramentas ─────────────────────────────────────────────────────────

func renderViewFerramentas(m *appModel, width, height int) string {
	s := m.styles
	var lines []string

	lines = append(lines, s.panelTitle.Render("Ferramentas — estatísticas"))
	lines = append(lines, strings.Repeat("─", width))
	lines = append(lines, "")

	if len(m.toolStats) == 0 {
		lines = append(lines, s.sysMsg.Render("Nenhuma ferramenta usada ainda."))
		return joinLines(lines, height)
	}

	maxCalls := 1
	for _, st := range m.toolStats {
		if st.Calls > maxCalls {
			maxCalls = st.Calls
		}
	}

	for name, st := range m.toolStats {
		bar := HorizBar(name, st.Calls, maxCalls, width-4, 12)
		avgStr := formatDuration(st.AvgMs)
		maxStr := formatDuration(st.MaxMs)
		lines = append(lines, s.tableRow.Render("  ")+bar+s.configKey.Render(fmt.Sprintf("  avg %s  max %s", avgStr, maxStr)))
	}

	return joinLines(lines, height)
}

// ── View: Tokens ──────────────────────────────────────────────────────────────

func renderViewTokens(m *appModel, width, height int) string {
	s := m.styles
	var lines []string

	total := m.tracker.TotalInputTokens + m.tracker.TotalOutputTokens

	lines = append(lines, s.panelTitle.Render("Consumo de tokens"))
	lines = append(lines, strings.Repeat("─", width))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("  Total: %s   Custo: %s",
		formatShort(total),
		formatCostStr(m.tracker.TotalCostUSD),
	))
	lines = append(lines, "")

	if len(m.historyTurns) > 0 {
		lines = append(lines, s.configKey.Render("  Tokens por turno:"))
		vals := make([]int, len(m.historyTurns))
		for i, ht := range m.historyTurns {
			vals[i] = ht.PromptTokens + ht.CompletionTokens
		}
		lines = append(lines, "  "+s.statusRunning.Render(Sparkline(vals, width-6)))
		lines = append(lines, "")
	}

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
			lines = append(lines, s.statusRunning.Render("  "+row))
		}
	}

	return joinLines(lines, height)
}

// ── View: Memória ─────────────────────────────────────────────────────────────

func renderViewMemoria(m *appModel, width, height int) string {
	s := m.styles
	var lines []string

	lines = append(lines, s.panelTitle.Render("Memória do projeto"))
	lines = append(lines, strings.Repeat("─", width))
	lines = append(lines, "")

	if len(m.memoryFacts) == 0 {
		lines = append(lines, s.sysMsg.Render("Nenhum fato salvo ainda."))
		lines = append(lines, "")
		lines = append(lines, s.sysMsg.Render("O agente usa 'remember' para guardar convenções."))
		return joinLines(lines, height)
	}

	cats := map[string][]string{}
	for _, f := range m.memoryFacts {
		cats[f.Category] = append(cats[f.Category], f.Key+": "+f.Value)
	}

	for cat, facts := range cats {
		lines = append(lines, s.configKey.Render(fmt.Sprintf("  %s  (%d)", cat, len(facts))))
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
