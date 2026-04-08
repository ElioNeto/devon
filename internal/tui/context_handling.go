// Package tui — context menu handling.
package tui

import (
	"fmt"

	"github.com/ElioNeto/devon/internal/history"
	tea "github.com/charmbracelet/bubbletea"
)

// ── Context menu ──────────────────────────────────────────────────────────────

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
			m.appendLog("system", "Agente interrompido.", "")
		}
	case "new_session":
		m.messages = nil
		m.toolRuns = nil
		m.logEvents = nil
		m.fileChanges = nil
	case "copy_response":
		if len(m.messages) > 0 {
			m.appendLog("system", "Resposta copiada.", "")
		}
	case "tool_input":
		if len(m.toolRuns) > m.leftCursor {
			tr := m.toolRuns[m.leftCursor]
			m.popup = fmt.Sprintf("Input de %s:\n  %s\n\nPressione tecla para fechar.", tr.Name, tr.Args)
		}
	case "tool_output":
		if len(m.toolRuns) > m.leftCursor {
			tr := m.toolRuns[m.leftCursor]
			m.popup = fmt.Sprintf("Output de %s:\n%s\n\nPressione tecla para fechar.", tr.Name, firstNLines(tr.Result, 30))
		}
	case "delete":
		if m.session != nil {
			if err := history.ClearSession(m.cfg.WorkDir, m.session.ID); err == nil {
				m.appendLog("system", "Sessão deletada.", "")
			}
		}
	default:
		m.appendLog("system", "Ação: "+action, "")
	}
	return nil
}
