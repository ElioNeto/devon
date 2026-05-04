package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/ElioNeto/devon/internal/agent"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

// ── Bubble Tea message types ──────────────────────────────────────────────────

type agentEventMsg agent.Event

type agentResult struct {
	events []agent.Event
}

// ── Left panel sections ───────────────────────────────────────────────────────

type leftSection int

const (
	secTurno       leftSection = iota // Tasks (turno ativo + tool calls)
	secHistorico                      // não usado no left mas mantido para syncRightView
	secFerramentas                    // Tools ativos
	secMemoria                        // Files Modified
	secTokens                         // Memory / Context
)

type leftItem struct {
	Label      string
	StatusKind string // "header"|"running"|"waiting"|"pending"|"done"|"error"|"failed"|"system"|"file"|"mem"|"prog"
	Section    leftSection
	Index      int
	Meta       string // texto alinhado à direita (tempo, diff)
}

// ── Log event (right panel) ───────────────────────────────────────────────────

type logEvent struct {
	Ts     string // HH:MM:SS
	Actor  string // "agent" | "tool" | "warn" | "ok"
	Msg    string
	Detail string // inline detail colorido (ex: "→ 3.2kb" ou "… created")
}

// fileChange rastreia um arquivo modificado.
type fileChange struct {
	Path   string
	Status string // "M" | "A" | "D"
	Lines  int    // +lines para A/M, -lines para D
}

// ── buildLeftItems ────────────────────────────────────────────────────────────

func buildLeftItems(m *appModel) []leftItem {
	var items []leftItem

	// ══ Tasks ════════════════════════════════════════════════
	items = append(items, leftItem{Label: "Tarefas", StatusKind: "header", Section: secTurno})

	if m.running {
		task := truncate(m.currentTask, 22)
		if task == "" {
			task = "processando..."
		}
		items = append(items, leftItem{
			Label:      task,
			StatusKind: "running",
			Section:    secTurno,
			Index:      0,
			Meta:       m.spinner.View(),
		})
	}

	for i, ht := range m.historyTurns {
		lbl := truncate(firstLine(ht.UserPrompt), 22)
		if lbl == "" {
			lbl = fmt.Sprintf("turno %d", i+1)
		}
		kind := "done"
		meta := ""
		if ht.Elapsed > 0 {
			meta = fmtElapsed(ht.Elapsed)
		}
		items = append(items, leftItem{
			Label:      lbl,
			StatusKind: kind,
			Section:    secTurno,
			Index:      i,
			Meta:       meta,
		})
	}

	if len(m.pendingTasks) > 0 {
		for _, pt := range m.pendingTasks {
			items = append(items, leftItem{
				Label:      truncate(pt.Label, 22),
				StatusKind: pt.Status,
				Section:    secTurno,
				Meta:       pt.Meta,
			})
		}
	}

	// ══ Ferramentas ══════════════════════════════════════════
	items = append(items, leftItem{Label: "Ferramentas", StatusKind: "header", Section: secFerramentas})

	if len(m.toolRuns) == 0 {
		items = append(items, leftItem{Label: "  —", StatusKind: "system", Section: secFerramentas})
	} else {
		for i, tr := range m.toolRuns {
			meta := ""
			switch tr.Status {
			case "done":
				meta = formatShort(tr.DurationMs) + "ms"
			case "running":
				meta = "—"
			}
			items = append(items, leftItem{
				Label:      truncate(tr.Name, 22),
				StatusKind: tr.Status,
				Section:    secFerramentas,
				Index:      i + 1,
				Meta:       meta,
			})
		}
	}

	// ══ Files Modified ════════════════════════════════════════
	items = append(items, leftItem{Label: "Arquivos Modificados", StatusKind: "header", Section: secMemoria})

	if len(m.fileChanges) == 0 {
		items = append(items, leftItem{Label: "  —", StatusKind: "system", Section: secMemoria})
	} else {
		for _, fc := range m.fileChanges {
			sign := "+"
			if fc.Status == "D" {
				sign = "-"
			}
			meta := fmt.Sprintf("%s%d", sign, fc.Lines)
			items = append(items, leftItem{
				Label:      truncate(fc.Path, 22),
				StatusKind: "file:" + fc.Status,
				Section:    secMemoria,
				Meta:       meta,
			})
		}
	}

	// ══ Memória / Contexto ════════════════════════════════════
	items = append(items, leftItem{Label: "Memória / Contexto", StatusKind: "header", Section: secTokens})

	if m.tracker != nil {
		totalTok := m.tracker.TotalInputTokens + m.tracker.TotalOutputTokens
		maxCtx := m.maxContextTokens
		if maxCtx <= 0 {
			maxCtx = 32000
		}
		items = append(items, leftItem{
			Label:      "janela de contexto",
			StatusKind: "system",
			Section:    secTokens,
			Meta:       fmt.Sprintf("%s / %s", formatShort(totalTok), formatShort(maxCtx)),
		})
		// barra de progresso
		items = append(items, leftItem{
			Label:      progressBar(totalTok, maxCtx, 20),
			StatusKind: "prog",
			Section:    secTokens,
		})
		items = append(items, leftItem{
			Label:      "estimativa de custo",
			StatusKind: "system",
			Section:    secTokens,
			Meta:       fmt.Sprintf("$%.4f", m.tracker.TotalCostUSD),
		})
		if len(m.fileChanges) > 0 {
			items = append(items, leftItem{
				Label:      "arquivos no contexto",
				StatusKind: "system",
				Section:    secTokens,
				Meta:       fmt.Sprintf("%d", len(m.fileChanges)),
			})
		}

		// Token usage chart — sparkline
		if len(m.tokenPerTurn) > 1 {
			chartW := m.width/3 - 4
			if chartW < 8 {
				chartW = 8
			}
			sparkline := Sparkline(m.tokenPerTurn, chartW)
			maxTok := 0
			for _, v := range m.tokenPerTurn {
				if v > maxTok {
					maxTok = v
				}
			}
			items = append(items, leftItem{
				Label:      "tokens/turno " + sparkline,
				StatusKind: "system",
				Section:    secTokens,
				Meta:       fmtShort(maxTok),
			})
			// Horizontal bars for last 5 turns
			n := len(m.tokenPerTurn)
			if n > 5 {
				n = 5
			}
			start := len(m.tokenPerTurn) - n
			for i := start; i < len(m.tokenPerTurn); i++ {
				turnLabel := fmt.Sprintf("T%d", i+1)
				barW := chartW - 8
				if barW < 5 {
					barW = 5
				}
				bar := HorzBar(turnLabel, m.tokenPerTurn[i], maxTok, barW, 4)
				items = append(items, leftItem{
					Label:      " " + bar,
					StatusKind: "system",
					Section:    secTokens,
				})
			}
		}
	}

	return items
}

// ── progressBar ───────────────────────────────────────────────────────────────

func progressBar(val, max, width int) string {
	if max <= 0 || width <= 0 {
		return strings.Repeat("░", width)
	}
	filled := int(float64(val) / float64(max) * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

// ── Right panel: tab enum ─────────────────────────────────────────────────────

type rightView int

const (
	viewLogs   rightView = iota // tab 1: Logs — stream de eventos
	viewDiff                    // tab 2: Diff — último diff de arquivo
	viewConfig                  // tab 3: Config — variáveis / sessão
	viewSteps                   // tab 4: Steps — histórico de turno selecionado
)

// ── Context menu ──────────────────────────────────────────────────────────────

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
				m.appendLog("system", "Agente interrompido.", "")
			}
		}})
	}
	actions = append(actions, menuAction{"Nova sessão [Ctrl+K]", "n", func(m *appModel) {
		m.messages = nil
		m.toolRuns = nil
		m.historyTurns = nil
		m.fileChanges = nil
		m.logEvents = nil
	}})
	actions = append(actions, menuAction{"Limpar chat [Ctrl+L]", "l", func(m *appModel) {
		m.messages = nil
		m.toolRuns = nil
		m.logEvents = nil
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

	innerW := width - 2 // subtrair bordas

	var lines []string

	// Header da sessão (linha 0 do painel)
	modelLabel := truncate(m.cfg.Model, 12)
	if m.router != nil {
		activeType := m.agent.ActiveTaskType()
		activeModel := m.agent.ActiveModel()
		if activeModel == "" {
			activeModel = m.cfg.Model
		}
		modelLabel = string(activeType) + ":" + truncate(activeModel, 8)
	}
	sessionLine := s.configKey.Render(" devon ") +
		s.configVal.Render("v"+appVersion) +
		"  " +
		s.statusVal.Render("sessão "+truncate(m.sessionID(), 8)) +
		"  " +
		s.statusVal.Render("model "+modelLabel) +
		"  tokens " +
		s.configVal.Render(fmtShort(m.totalTokens()))
	lines = append(lines, sessionLine)
	lines = append(lines, "")

	for idx, item := range items {
		var line string

		if item.StatusKind == "header" {
			// ─Tasks─
			dash := strings.Repeat("─", 2)
			line = s.itemSection.Render(dash + item.Label + dash)
		} else if item.StatusKind == "prog" {
			// barra de progresso verde
			line = " " + s.progFill.Render(item.Label)
		} else {
			// item normal: badge + label + meta
			badgeStr, labelStyle := badgeAndStyle(s, item)
			if idx == m.leftCursor {
				labelStyle = s.itemSelected
			}

			// calcular padding para alinhar Meta à direita
			labelTxt := item.Label
			prefixW := lipgloss.Width(badgeStr) + 1
			maxLabel := innerW - prefixW - lipgloss.Width(item.Meta) - 1
			if maxLabel < 4 {
				maxLabel = 4
			}
			labelTxt = truncate(labelTxt, maxLabel)
			gap := innerW - prefixW - lipgloss.Width(labelTxt) - lipgloss.Width(item.Meta)
			if gap < 1 {
				gap = 1
			}
			line = badgeStr + " " + labelStyle.Render(labelTxt) +
				strings.Repeat(" ", gap) +
				metaStyle(s, item)
		}
		lines = append(lines, line)
	}

	// pad to height
	for len(lines) < height-2 {
		lines = append(lines, "")
	}
	if len(lines) > height-2 {
		lines = lines[:height-2]
	}

	content := strings.Join(lines, "\n")
	borderStyle := s.panelBase
	if focused {
		borderStyle = s.panelFocused
	}
	return borderStyle.Width(innerW).Height(height - 2).Render(content)
}

// badgeAndStyle returns the coloured status badge string and label style for an item.
func badgeAndStyle(s uiStyles, item leftItem) (string, lipgloss.Style) {
	switch item.StatusKind {
	case "running":
		return s.statusRunning.Render("● execut. "), s.itemNormal
	case "waiting":
		return s.statusWaiting.Render("● aguard. "), s.itemNormal
	case "pending":
		return s.statusPending.Render("● pendente"), s.itemNormal
	case "done":
		return s.statusDone.Render("● concl.  "), s.itemNormal
	case "error", "failed":
		return s.statusError.Render("● " + item.StatusKind + " "), s.itemNormal
	case "active":
		return s.statusRunning.Render("● ativo "), s.itemNormal
	case "file:M":
		return s.fileModified.Render("M"), s.itemNormal
	case "file:A":
		return s.fileAdded.Render("A"), s.itemNormal
	case "file:D":
		return s.fileDeleted.Render("D"), s.itemNormal
	case "system":
		return " ", s.itemNormal
	default:
		return " ", s.itemNormal
	}
}

// metaStyle renders the right-aligned meta text with appropriate color.
func metaStyle(s uiStyles, item leftItem) string {
	if item.Meta == "" {
		return ""
	}
	switch item.StatusKind {
	case "file:A":
		return s.fileAdded.Render(item.Meta)
	case "file:D":
		return s.fileDeleted.Render(item.Meta)
	case "running":
		return s.statusRunning.Render(item.Meta)
	case "error", "failed":
		return s.statusError.Render(item.Meta)
	default:
		return s.fileLines.Render(item.Meta)
	}
}

// ── Rendering: Right Panel ────────────────────────────────────────────────────

func renderRightPanel(m *appModel, width, height int, focused bool) string {
	s := m.styles
	innerW := width - 2

	// ── Tab bar
	tabs := []struct {
		label string
		view  rightView
	}{
		{"Logs", viewLogs},
		{"Diff", viewDiff},
		{"Config", viewConfig},
		{"Etapas", viewSteps},
	}
	var tabParts []string
	for _, t := range tabs {
		if m.rightView == t.view {
			tabParts = append(tabParts, s.tabActive.Render(t.label))
		} else {
			tabParts = append(tabParts, s.tabInactive.Render(t.label))
		}
	}
	tabBar := s.itemSection.Render("─") + strings.Join(tabParts, s.statusSep.Render(" ")) +
		s.itemSection.Render(strings.Repeat("─", max(0, innerW-30)))

	contentH := height - 2 - 1 // height - borders - tabbar
	if contentH < 1 {
		contentH = 1
	}

	var content string
	switch m.rightView {
	case viewLogs:
		content = renderViewLogs(m, innerW, contentH)
	case viewDiff:
		content = renderViewDiff(m, innerW, contentH)
	case viewConfig:
		content = renderViewConfig(m, innerW, contentH)
	case viewSteps:
		content = renderViewSteps(m, innerW, contentH)
	}

	full := tabBar + "\n" + content
	borderStyle := s.panelBase
	if focused {
		borderStyle = s.panelFocused
	}
	return borderStyle.Width(innerW).Height(height - 2).Render(full)
}

// ── View: Logs ────────────────────────────────────────────────────────────────

func renderViewLogs(m *appModel, width, height int) string {
	s := m.styles

	events := m.logEvents
	if len(events) == 0 {
		return s.sysMsg.Render("  Aguardando eventos…")
	}

	// janela deslizante: mostra as últimas `height` linhas com scroll
	var lines []string
	for _, ev := range events {
		line := renderLogLine(s, ev, width)
		lines = append(lines, line)
	}

	start := 0
	if len(lines) > height {
		start = len(lines) - height - m.rightScroll
		if start < 0 {
			start = 0
		}
	}
	end := start + height
	if end > len(lines) {
		end = len(lines)
	}

	return strings.Join(lines[start:end], "\n")
}

func renderLogLine(s uiStyles, ev logEvent, _ int) string {
	ts := s.actorTs.Render(ev.Ts)

	var actor string
	switch ev.Actor {
	case "agent":
		actor = s.actorAgent.Render("[agente]")
	case "tool":
		actor = s.actorTool.Render("[tool]  ")
	case "warn":
		actor = s.actorWarn.Render("[aviso] ")
	case "ok":
		actor = s.actorOk.Render("[ok]   ")
	default:
		actor = s.sysMsg.Render("[sist.] ")
	}

	msg := ev.Msg
	if ev.Detail != "" {
		msg += " " + ev.Detail
	}

	return ts + " " + actor + " " + msg
}

// ── View: Diff ────────────────────────────────────────────────────────────────

func renderViewDiff(m *appModel, width, height int) string {
	s := m.styles
	if m.lastDiff == "" {
		return s.sysMsg.Render("  Nenhum diff disponível.")
	}
	lines := strings.Split(m.lastDiff, "\n")
	avail := height
	start := 0
	if len(lines) > avail {
		start = len(lines) - avail - m.rightScroll
		if start < 0 {
			start = 0
		}
	}
	end := start + avail
	if end > len(lines) {
		end = len(lines)
	}
	var out []string
	for _, l := range lines[start:end] {
		if strings.HasPrefix(l, "+") {
			out = append(out, s.diffAdd.Render(l))
		} else if strings.HasPrefix(l, "-") {
			out = append(out, s.diffDel.Render(l))
		} else if strings.HasPrefix(l, "@@") {
			out = append(out, s.diffHunk.Render(l))
		} else {
			out = append(out, l)
		}
	}
	_ = width
	return strings.Join(out, "\n")
}

// ── View: Config ──────────────────────────────────────────────────────────────

func renderViewConfig(m *appModel, width, height int) string {
	s := m.styles
	var lines []string

	kv := func(k, v string) {
		lines = append(lines, s.configKey.Render(fmt.Sprintf("  %-18s", k))+s.configVal.Render(v))
	}

	kv("sessão", m.sessionID())
	kv("modelo", m.cfg.Model)
	kv("modo", m.cfg.Mode.String())
	kv("dir trabalho", truncate(m.cfg.WorkDir, width-22))
	lines = append(lines, "")

	if m.tracker != nil {
		kv("tokens entrada", formatShort(m.tracker.TotalInputTokens))
		kv("tokens saída", formatShort(m.tracker.TotalOutputTokens))
		kv("total tokens", formatShort(m.tracker.TotalInputTokens+m.tracker.TotalOutputTokens))
		kv("custo USD", fmt.Sprintf("$%.4f", m.tracker.TotalCostUSD))
		kv("requisições", fmt.Sprintf("%d", m.tracker.TotalRequests))
	}
	lines = append(lines, "")

	if len(m.memoryFacts) > 0 {
		lines = append(lines, s.itemSection.Render("──Memória──"))
		for _, f := range m.memoryFacts {
			lines = append(lines, s.configKey.Render(fmt.Sprintf("  [%s] %s: ", f.Category, f.Key))+
				s.configVal.Render(truncate(f.Value, width-30)))
		}
	}

	// trim to height
	if len(lines) > height {
		lines = lines[:height]
	}
	return strings.Join(lines, "\n")
}

// ── View: Steps ───────────────────────────────────────────────────────────────

func renderViewSteps(m *appModel, width, height int) string {
	s := m.styles
	var lines []string

	idx := m.selectedTurnIdx
	if idx < 0 || idx >= len(m.historyTurns) {
		if len(m.messages) == 0 {
			return s.sysMsg.Render("  Nenhum turno selecionado.")
		}
		// mostra turno ativo
		for _, msg := range m.messages {
			switch msg.Sender {
			case "user":
				lines = append(lines, s.userMsg.Render("▸ "+truncate(msg.Content, width-4)))
			case "devon":
				if msg.IsError {
					for _, l := range wrapLine(msg.Content, width-4) {
						lines = append(lines, s.errMsg.Render("  "+l))
					}
				} else {
					md := renderMarkdown(msg.Content, width-4)
					for _, l := range strings.Split(md, "\n") {
						lines = append(lines, "  "+l)
					}
				}
			case "system":
				lines = append(lines, s.sysMsg.Render("  "+truncate(msg.Content, width-4)))
			}
			lines = append(lines, "")
		}
		if len(lines) > height {
			lines = lines[len(lines)-height:]
		}
		return strings.Join(lines, "\n")
	}

	ht := m.historyTurns[idx]
	lines = append(lines, s.configKey.Render(fmt.Sprintf("  Turno %d — %s", idx+1, ht.Timestamp)))
	lines = append(lines, "")
	lines = append(lines, s.configKey.Render("  Prompt:"))
	for _, l := range wrapLine(ht.UserPrompt, width-4) {
		lines = append(lines, s.userMsg.Render("    "+l))
	}
	lines = append(lines, "")
	lines = append(lines, s.configKey.Render("  Resposta:"))
	if ht.AgentReply != "" {
		md := renderMarkdown(ht.AgentReply, width-4)
		for _, l := range strings.Split(md, "\n") {
			lines = append(lines, "    "+l)
		}
	}
	if ht.ToolSummary != "" {
		lines = append(lines, "")
		lines = append(lines, s.configKey.Render("  Tools: ")+s.sysMsg.Render(ht.ToolSummary))
	}
	if ht.PromptTokens+ht.CompletionTokens > 0 {
		lines = append(lines, "")
		lines = append(lines, s.configKey.Render("  Tokens: ")+
			s.configVal.Render(fmt.Sprintf("in=%s out=%s",
				formatShort(ht.PromptTokens),
				formatShort(ht.CompletionTokens))))
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	return strings.Join(lines, "\n")
}

// ── Help overlay ──────────────────────────────────────────────────────────────

func renderHelp(m *appModel, width int) string {
	s := m.styles
	hints := AllHints()
	half := (len(hints) + 1) / 2

	var left, right []string
	left = append(left, s.itemSection.Render("──Ajuda──"))
	left = append(left, "")
	for _, h := range hints[:half] {
		left = append(left, s.keyStyle.Render(fmt.Sprintf("  %-18s", h.Keys))+s.helpStyle.Render(h.Action))
	}
	if len(hints) > half {
		right = append(right, s.itemSection.Render("──Mais──"))
		right = append(right, "")
		for _, h := range hints[half:] {
			right = append(right, s.keyStyle.Render(fmt.Sprintf("  %-18s", h.Keys))+s.helpStyle.Render(h.Action))
		}
	}

	bottom := s.sysMsg.Render("  Pressione qualquer tecla para fechar.")
	joined := lipgloss.JoinHorizontal(lipgloss.Top, strings.Join(left, "\n"), strings.Join(right, "\n"))
	body := lipgloss.JoinVertical(lipgloss.Left, joined, "", bottom)
	return s.menuStyle.Width(min(width-4, 60)).Render(body)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// renderMarkdown renders a string as Markdown using Glamour.
func renderMarkdown(text string, width int) string {
	r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(width-2),
	)
	if err != nil {
		return text
	}
	if out, err := r.Render(text); err == nil {
		return strings.TrimRight(out, "\n ")
	}
	return text
}

// appendLog adiciona um evento ao stream de logs do painel direito.
func (m *appModel) appendLog(actor, msg, detail string) {
	m.logEvents = append(m.logEvents, logEvent{
		Ts:     time.Now().Format("15:04:05"),
		Actor:  actor,
		Msg:    msg,
		Detail: detail,
	})
}

// sessionID retorna o ID curto da sessão ativa.
func (m *appModel) sessionID() string {
	if m.session != nil {
		return truncate(m.session.ID, 8)
	}
	return "—"
}

// totalTokens retorna o total de tokens da sessão.
func (m *appModel) totalTokens() int {
	if m.tracker == nil {
		return 0
	}
	return m.tracker.TotalInputTokens + m.tracker.TotalOutputTokens
}

// fmtElapsed formata uma duração em ms para exibição.
func fmtElapsed(ms int64) string {
	if ms >= 60000 {
		return fmt.Sprintf("%dm%ds", ms/60000, (ms%60000)/1000)
	}
	if ms >= 1000 {
		return fmt.Sprintf("%ds", ms/1000)
	}
	return fmt.Sprintf("%dms", ms)
}

// max returns the larger of two ints.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
