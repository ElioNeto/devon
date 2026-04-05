package tui

import (
	"fmt"

	"github.com/ElioNeto/devon/internal/cost"
	"github.com/ElioNeto/devon/internal/history"
	"github.com/charmbracelet/lipgloss"
)

// cmdMenuAction describes an entry in the command menu (opened with "!").
type cmdMenuAction struct {
	Label  string
	Action func(*appModel)
}

// cmdMenuActions returns the available command-menu items for the current model state.
var cmdMenuActions = []cmdMenuAction{
	{"Limpar chat [Ctrl+L]", clearChatAction},
	{"Nova sessão [Ctrl+K]", newSessionAction},
	{"Novo workspace", nextFreeWorkspaceAction},
	{"Ver uso de tokens", viewTokensAction},
}

func (m *appModel) toggleCmdMenu() {
	m.showCmdMenu = !m.showCmdMenu
	if m.showCmdMenu && m.cmdMenuCursor == 0 {
		// reset cursor on open
	}
}

func (m *appModel) execCmdMenuAction() {
	if m.cmdMenuCursor < len(cmdMenuActions) {
		cmdMenuActions[m.cmdMenuCursor].Action(m)
	}
	m.showCmdMenu = false
	m.cmdMenuCursor = 0
}

// ── Actions ───────────────────────────────────────────────────────────────────

func clearChatAction(m *appModel) {
	m.messages = nil
	m.toolRuns = nil
	m.logEvents = nil
	m.rightScroll = 0
}

func newSessionAction(m *appModel) {
	m.messages = nil
	m.toolRuns = nil
	m.historyTurns = nil
	m.fileChanges = nil
	m.logEvents = nil
	m.scroll = 0
	m.tracker = cost.NewSession(m.cfg.Model)
	m.tokenPerTurn = nil
	var err error
	m.session, err = history.CreateSession(m.cfg.WorkDir)
	if err != nil {
		m.messages = append(m.messages, chatMessage{Sender: "system", Content: "Erro: " + err.Error()})
	} else {
		m.messages = append(m.messages, chatMessage{Sender: "system", Content: "Nova sessão " + m.session.ID})
	}
}

func nextFreeWorkspaceAction(m *appModel) {
	slot := m.nextFreeWorkspace()
	if slot >= 0 {
		m.switchWorkspace(slot)
	}
}

func viewTokensAction(m *appModel) {
	if m.tracker != nil {
		m.popup = m.tracker.Format()
	}
}

func (m *appModel) nextFreeWorkspace() int {
	for i := range m.workspaceSlots {
		s := &m.workspaceSlots[i]
		if s.session == nil && len(s.messages) == 0 {
			return i
		}
	}
	return -1
}

func (m *appModel) switchWorkspace(idx int) {
	if idx < 0 || idx >= len(m.workspaceSlots) {
		return
	}
	// Save current workspace state into the slot
	m.saveWorkspaceState(m.activeWorkspace)
	// Load target workspace state
	m.loadWorkspaceState(idx)
	m.activeWorkspace = idx
}

func (m *appModel) saveWorkspaceState(slot int) {
	s := &m.workspaceSlots[slot]
	s.session = m.session
	s.messages = m.messages
	s.toolRuns = m.toolRuns
	s.logEvents = m.logEvents
	s.fileChanges = m.fileChanges
	s.memoryFacts = m.memoryFacts
	s.historyTurns = m.historyTurns
	s.tracker = m.tracker
	s.tokenPerTurn = m.tokenPerTurn
	s.currentTask = m.currentTask
	s.pendingTasks = m.pendingTasks
	s.running = m.running
}

func (m *appModel) loadWorkspaceState(idx int) {
	s := &m.workspaceSlots[idx]
	m.session = s.session
	m.messages = s.messages
	m.toolRuns = s.toolRuns
	m.logEvents = s.logEvents
	m.fileChanges = s.fileChanges
	m.memoryFacts = s.memoryFacts
	m.historyTurns = s.historyTurns
	m.tracker = s.tracker
	m.tokenPerTurn = s.tokenPerTurn
	m.currentTask = s.currentTask
	m.pendingTasks = s.pendingTasks
	m.running = s.running
}

// cmdMenuRenders the command menu as an overlay panel.
func renderCmdMenuOverlay(m *appModel, width int) string {
	overlayW := 36
	if overlayW > width-4 {
		overlayW = width - 4
	}

	title := "Comandos"
	line := fmt.Sprintf("─ %s ─", title)
	pad := overlayW - renderWidth(line)
	if pad < 1 {
		pad = 1
	}
	line = line + spaces(pad)

	lines := []string{line}
	for i, a := range cmdMenuActions {
		cursor := "  "
		if i == m.cmdMenuCursor {
			cursor = "▸ "
		}
		entry := cursor + a.Label
		lines = append(lines, entry)
	}
	lines = append(lines, "")
	lines = append(lines, "  Esc para fechar")

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorPrimary).
		Padding(0, 1).
		Width(overlayW - 2).
		Render(fmt.Sprintf("%s", joinLines(lines)))
}

func joinLines(ss []string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += "\n"
		}
		result += s
	}
	return result
}

func spaces(n int) string {
	if n <= 0 {
		return ""
	}
	s := ""
	for i := 0; i < n; i++ {
		s += " "
	}
	return s
}

func renderWidth(s string) int {
	if lipglossWidth == nil {
		return len(s)
	}
	return lipglossWidth(s)
}

var lipglossWidth = func(s string) int { return lipgloss.Width(s) }
