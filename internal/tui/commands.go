// Package tui — agent commands and input manipulation.
package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/ElioNeto/devon/internal/agent"
	"github.com/ElioNeto/devon/internal/cost"
	"github.com/ElioNeto/devon/internal/history"
	"github.com/ElioNeto/devon/internal/llm"
	tea "github.com/charmbracelet/bubbletea"
)

// ── Input / send ──────────────────────────────────────────────────────────────

func (m *appModel) sendInput() (tea.Model, tea.Cmd) {
	text := strings.TrimSpace(m.input)
	if text == "" && len(m.attachments) == 0 || m.running {
		return m, nil
	}
	m.input = ""
	m.cursor = 0
	m.statusMsg = ""

	if strings.HasPrefix(text, "/") {
		m.handleSlash(text)
		return m, nil
	}

	m.inputHist.push(text)

	// Build message — multimodal if attachments exist
	if len(m.attachments) > 0 {
		// Build a Message with ContentParts
		var parts []llm.ContentPart
		if text != "" {
			parts = append(parts, llm.NewTextPart(text))
		}
		for _, att := range m.attachments {
			parts = append(parts, llm.NewImagePartBase64(att.dataURI()))
		}
		msg := llm.Message{
			Role:         llm.RoleUser,
			ContentParts: parts,
		}

		m.messages = append(m.messages, chatMessage{
			Sender:  "user",
			Content: text + fmt.Sprintf(" [%d imagem(ns)]", len(m.attachments)),
		})
		m.toolRuns = nil
		m.running = true
		m.currentTask = text
		m.rightView = viewLogs
		m.appendLog("agent", "Iniciando tarefa com imagem(ns): "+truncate(text, 50), "")

		ctx, cancel := context.WithCancel(context.Background())
		m.cancel = cancel
		cmd := startAgentWithMessage(ctx, m.agent, msg)

		// Clear attachments data and slice after send
		for i := range m.attachments {
			m.attachments[i].Data = nil
		}
		m.attachments = nil

		return m, tea.Batch(m.spinner.Tick, cmd)
	}

	// Text-only path (legacy)
	m.messages = append(m.messages, chatMessage{Sender: "user", Content: text})
	m.toolRuns = nil
	m.running = true
	m.currentTask = text
	m.rightView = viewLogs
	m.appendLog("agent", "Iniciando tarefa: "+truncate(text, 50), "")

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
				mark = "▸"
			}
			sb.WriteString("  " + mark + " " + id + "\n")
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
				if m.toolStats == nil {
					m.toolStats = make(map[string]*toolStat)
				}
				if st, ok := m.toolStats[ev.Tool]; ok {
					st.Calls++
				} else {
					m.toolStats[ev.Tool] = &toolStat{Calls: 1}
				}
				resultDetail := "… " + truncate(firstLine(ev.Result), 30)
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
				m.appendLog("warn", ev.Tool+" falhou", ev.Err.Error())
				break
			}
		}

	case "turn_done":
		m.appendLog("ok", "testes passando", "")
		m.finalizeTurn()

	case "error":
		m.messages = append(m.messages, chatMessage{Sender: "devon", Content: "Erro: " + ev.Err.Error(), IsError: true})
		m.appendLog("warn", "Erro: "+ev.Err.Error(), "")
		m.finalizeTurn()

	case "system":
		m.messages = append(m.messages, chatMessage{Sender: "system", Content: ev.Text})
		m.appendLog("agent", ev.Text, "")

	case "confirm_request":
		m.confirm.open(ConfirmRequest{
			Tool:  ev.Tool,
			Args:  ev.Args,
			Level: ev.Tool, // placeholder; level is determined by agent
		})
		m.appendLog("warn", "Aguardando confirmação: "+ev.Tool, "")
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
			msgs = append(msgs, llm.Message{Role: llm.RoleUser, Content: llm.TextContent(cm.Content)})
		case "devon":
			role := llm.RoleAssistant
			if cm.IsError {
				role = llm.RoleTool
			}
			msgs = append(msgs, llm.Message{Role: role, Content: llm.TextContent(cm.Content)})
		}
	}
	// Note: multimodal messages with ContentParts are sent directly via
	// RunWithMessage and are NOT persisted to agentMessages() history.
	// The chatMessage for multimodal inputs includes a "[N imagem(ns)]" tag
	// so the image count is preserved in the display log.
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

func startAgentWithMessage(ctx context.Context, a *agent.Agent, msg llm.Message) tea.Cmd {
	return func() tea.Msg {
		ch := a.RunWithMessage(ctx, msg)
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
	for pos > 0 && (ru[pos-1] == ' ' || ru[pos-1] == '\n') {
		pos--
	}
	for pos > 0 && ru[pos-1] != ' ' && ru[pos-1] != '\n' {
		pos--
	}
	ru = append(ru[:pos], ru[m.cursor:]...)
	m.input = string(ru)
	m.cursor = pos
}

// newLine inserts a newline at the current cursor position for multi-line input.
func (m *appModel) newLine() {
	ru := []rune(m.input)
	if m.cursor >= len(ru) {
		m.input += "\n"
		m.cursor++
	} else {
		m.input = string(ru[:m.cursor]) + "\n" + string(ru[m.cursor:])
		m.cursor++
	}
	m.multilineRows = strings.Count(m.input, "\n") + 1
}

// ── Utilities ────────────────────────────────────────────────────────────────

// extractPath tenta extrair o path do campo args de uma tool call.
func extractPath(args string) string {
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

func shortenArgs(args string) string {
	ru := []rune(args)
	if len(ru) <= 25 {
		return args
	}
	return string(ru[:24]) + "…"
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
