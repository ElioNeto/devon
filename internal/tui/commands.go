// Package tui — agent commands and input manipulation.
package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

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
	m.multilineRows = 1
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

		// Inject conversation history into agent before Run
		if msgs := m.agentMessages(); len(msgs) > 1 {
			m.agent.SetConversation(msgs[:len(msgs)-1])
		} else if len(msgs) == 1 {
			m.agent.ResetHistory()
		}

		ctx, cancel := context.WithCancel(context.Background())
		m.cancel = cancel
		cmd := m.startAgentWithMessage(ctx, msg)

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

	// Inject conversation history into agent before Run
	if msgs := m.agentMessages(); len(msgs) > 1 {
		m.agent.SetConversation(msgs[:len(msgs)-1])
	} else if len(msgs) == 1 {
		m.agent.ResetHistory()
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	cmd := m.startAgent(ctx, text)
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
		m.agent.ResetHistory()
	case text == "/usage" || text == "/cost":
		var sb strings.Builder
		if m.tracker != nil {
			sb.WriteString(m.tracker.Format())
			sb.WriteString("\n\n")
		}
		if m.agent != nil {
			sb.WriteString(m.agent.UsageStats())
		} else {
			sb.WriteString("Agente não inicializado.")
		}
		m.popup = sb.String()
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
		m.isGenerating = true
		last := len(m.messages) - 1
		if last < 0 || m.messages[last].Sender != "devon" {
			m.messages = append(m.messages, chatMessage{Sender: "devon", Content: ev.Text})
			m.appendLog("agent", truncate(ev.Text, 80), "")
		} else {
			m.messages[last].Content += ev.Text
		}

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
		// NOTE: state cleanup (isGenerating, running, toolRuns) is handled by finalizeTurn().
		// This case only emits log events so turn_done flow is visible in the UI.
		// The Summary field is set by the agent based on task type:
		//   - "resposta enviada" for conversational turns (no tool calls)
		//   - "tarefa concluída" for task completion (tools were used)
		// Falls back to "testes passando" for backward compatibility.
		msg := ev.Summary
		if msg == "" {
			msg = "testes passando"
		}
		m.appendLog("ok", msg, "")
		if last := len(m.messages) - 1; last >= 0 && m.messages[last].Sender == "devon" {
			m.appendLog("agent", truncate(m.messages[last].Content, 80), "")
		}

	case "error":
		m.isGenerating = false
		m.messages = append(m.messages, chatMessage{Sender: "devon", Content: "Erro: " + ev.Err.Error(), IsError: true})
		m.appendLog("warn", "Erro: "+ev.Err.Error(), "")
		m.running = false
		// NOTE: finalizeTurn is NOT called here anymore — the caller handles it.
		// toolRuns is NOT reset here so finalizeTurn can build the toolSummary.

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

// ── Finalize turn ─────────────────────────────────────────────────────────────

func (m *appModel) finalizeTurn() {
	m.isGenerating = false
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	m.running = false
	m.currentTask = ""
	m.agentCh = nil
	m.turnNumber++

	// Track tokens
	total := m.tracker.TotalInputTokens + m.tracker.TotalOutputTokens
	m.tokenPerTurn = append(m.tokenPerTurn, total)

	// Snapshot toolRuns before any cleanup — agentMessages() and persistSessionToDB() both need them
	toolRunsSnapshot := make([]toolRun, len(m.toolRuns))
	copy(toolRunsSnapshot, m.toolRuns)

	// Build history turn — use the LAST user message as prompt (multi-turn support) and LAST devon as reply
	prompt, reply, toolSummary := "", "", ""
	for _, msg := range m.messages {
		if msg.Sender == "user" {
			prompt = msg.Content // last user message = current turn's prompt
		}
		if msg.Sender == "devon" {
			reply = msg.Content // last devon message = current turn's reply
		}
	}
	for _, tr := range toolRunsSnapshot {
		toolSummary += tr.Name + " "
	}

	// NOTE: The full agent reply was already logged by the turn_done handler
	// in processAgentEvent. Do NOT add another appendLog here — it would
	// duplicate the log line. The reply variable is still needed below for
	// historyTurns.

	m.historyTurns = append(m.historyTurns, historyTurn{
		UserPrompt:  prompt,
		AgentReply:  reply,
		ToolSummary: strings.TrimSpace(toolSummary),
		Timestamp:   "agora",
	})

	// Save messages — agentMessages() includes tool results from m.toolRuns (still available)
	if m.session != nil {
		_ = history.SaveMessages(m.cfg.WorkDir, m.session.ID, m.agentMessages(), &m.session.Usage)
	}

	// Persist session state to DB for crash recovery — uses m.toolRuns
	m.persistSessionToDB()

	// Safe to clear toolRuns now — all consumers (toolSummary, agentMessages, persistSessionToDB) have completed
	m.toolRuns = nil
}

// ── Streaming agent commands ──────────────────────────────────────────────────

// startAgent launches the agent and returns a streaming command.
// The agent channel is stored in m.agentCh for subsequent listenAgent calls.
func (m *appModel) startAgent(ctx context.Context, input string) tea.Cmd {
	ch := m.agent.Run(ctx, input)
	m.agentCh = ch
	return listenAgent(ch)
}

// startAgentWithMessage launches the agent with a structured message.
func (m *appModel) startAgentWithMessage(ctx context.Context, msg llm.Message) tea.Cmd {
	ch := m.agent.RunWithMessage(ctx, msg)
	m.agentCh = ch
	return listenAgent(ch)
}

// listenAgent returns a tea.Cmd that reads one event from the channel.
// When the channel is closed, it returns agentDoneMsg.
func listenAgent(ch <-chan agent.Event) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return agentDoneMsg{}
		}
		return agentEventMsg(ev)
	}
}

// cursorBlinkCmd returns a tea.Cmd that toggles the cursor visibility every 500ms.
func (m *appModel) cursorBlinkCmd() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return cursorBlinkMsg{}
	})
}

// ── captureFileChange detecta tool calls de escrita/remoção e registra a mudança.
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

// persistSessionToDB saves the current session data to the DB for crash recovery.
func (m *appModel) persistSessionToDB() {
	if m.dbStore == nil || m.sessionMgr == nil || m.session == nil {
		return
	}

	sessionID := m.session.ID
	if sessionID == "" {
		return
	}

	ctx := context.Background()

	// Ensure session exists in DB
	_ = m.dbStore.CreateSession(ctx, sessionID)

	// Update session metadata with task and model
	task := ""
	for _, msg := range m.messages {
		if msg.Sender == "user" && task == "" {
			task = truncate(msg.Content, 200)
		}
	}
	_ = m.sessionMgr.Update(ctx, sessionID, task, m.cfg.Model, "active")

	// Persist messages
	for _, cm := range m.messages {
		switch cm.Sender {
		case "user":
			_ = m.dbStore.PutMessage(ctx, "tui", sessionID, "user", cm.Content)
		case "devon":
			role := "assistant"
			if cm.IsError {
				role = "tool"
			}
			_ = m.dbStore.PutMessage(ctx, "tui", sessionID, role, cm.Content)
		}
	}

	// Persist tool runs
	for _, tr := range m.toolRuns {
		status := tr.Status
		result := tr.Result
		var errStr string
		if status == "error" {
			errStr = result
		}
		_, _ = m.dbStore.PutToolCall(ctx, "tui", sessionID, tr.Name, tr.Args, status, result, errStr)
	}

	// Persist cost summary
	if m.tracker != nil {
		tokens := map[string]int{
			"prompt_tokens":     m.tracker.TotalInputTokens,
			"completion_tokens": m.tracker.TotalOutputTokens,
			"total_tokens":      m.tracker.TotalInputTokens + m.tracker.TotalOutputTokens,
			"requests":          m.tracker.TotalRequests,
		}
		_ = m.dbStore.UpdateCostSummary(ctx, sessionID, m.tracker.TotalCostUSD, tokens)
	}
}

func (m *appModel) agentMessages() []llm.Message {
	var msgs []llm.Message
	for _, cm := range m.messages {
		var role llm.Role
		switch cm.Sender {
		case "user":
			role = llm.RoleUser
		case "devon":
			if cm.IsError {
				role = llm.RoleTool
			} else {
				role = llm.RoleAssistant
			}
		case "system":
			role = llm.RoleSystem
		default:
			// Skip unknown senders
			continue
		}
		if len(cm.ContentParts) > 0 {
			msgs = append(msgs, llm.Message{Role: role, ContentParts: cm.ContentParts})
		} else {
			msgs = append(msgs, llm.Message{Role: role, Content: llm.TextContent(cm.Content)})
		}
	}
	// Append completed tool runs as tool-role messages
	for _, tr := range m.toolRuns {
		if tr.Status == "done" || tr.Status == "error" {
			msgs = append(msgs, llm.Message{Role: llm.RoleTool, Content: llm.TextContent(tr.Result)})
		}
	}
	return msgs
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

// deleteToEnd removes characters from cursor to the end of the current line.
// If cursor is at/past the end, no-op.
func (m *appModel) deleteToEnd() {
	ru := []rune(m.input)
	if m.cursor >= len(ru) {
		return
	}
	// Find next \n from cursor position, or end of string
	end := len(ru)
	for i := m.cursor; i < len(ru); i++ {
		if ru[i] == '\n' {
			end = i
			break
		}
	}
	ru = append(ru[:m.cursor], ru[end:]...)
	m.input = string(ru)
	m.multilineRows = strings.Count(m.input, "\n") + 1
}

// deleteToStart removes characters from the start of the current line to cursor.
// If cursor is already at line start, no-op.
func (m *appModel) deleteToStart() {
	ru := []rune(m.input)
	if m.cursor <= 0 {
		return
	}
	// Find previous \n before cursor, or 0
	start := 0
	for i := m.cursor - 1; i >= 0; i-- {
		if ru[i] == '\n' {
			start = i + 1
			break
		}
	}
	if start == m.cursor {
		return
	}
	ru = append(ru[:start], ru[m.cursor:]...)
	m.input = string(ru)
	m.cursor = start
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
