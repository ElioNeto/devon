package tui

import (
	"context"
	"fmt"
	"strings"

	sessionpkg "github.com/ElioNeto/devon/internal/session"
	tea "github.com/charmbracelet/bubbletea"
)

// sessionPickerState holds the state for the session picker overlay.
type sessionPickerState struct {
	visible  bool
	sessions []sessionListItem
	cursor   int
	filter   string
	done     bool // set to true when a session is selected
}

type sessionListItem struct {
	ID          string
	Task        string
	Status      string
	MessageCnt  int
	ToolCallCnt int
	Model       string
}

// sessionPickerView renders the session picker overlay.
func (m *appModel) sessionPickerView(width int) string {
	if !m.picker.visible || m.picker.done {
		return ""
	}

	s := m.styles

	var b strings.Builder
	b.WriteString(s.menuStyle.Width(min(width-8, 70)).Render("Selecionar sessão (↑/↓ navegar, Enter selecionar, Esc cancelar)"))

	if len(m.picker.sessions) == 0 {
		msg := s.menuStyle.
			Width(min(width-8, 70)).
			Render("Nenhuma sessão encontrada. Pressione Enter para iniciar nova sessão.")
		return "\n" + msg
	}

	var listItems []string
	for i, item := range m.picker.sessions {
		cursor := " "
		if i == m.picker.cursor {
			cursor = "▸"
		}

		task := item.Task
		if task == "" {
			task = "(sem tarefa)"
		}
		if len(task) > 40 {
			task = task[:40] + "…"
		}

		statusStyle := s.statusOther
		switch item.Status {
		case "active":
			statusStyle = s.statusRunning
		case "completed":
			statusStyle = s.statusDone
		case "error":
			statusStyle = s.statusError
		}

		line := fmt.Sprintf("%s %s [%s] %s — %d msgs, %d tools",
			cursor,
			item.ID,
			statusStyle.Render(item.Status),
			task,
			item.MessageCnt,
			item.ToolCallCnt,
		)

		if i == m.picker.cursor {
			listItems = append(listItems, s.menuSelected.Render(line))
		} else {
			listItems = append(listItems, s.menuItem.Render(line))
		}
	}

	b.WriteString("\n")
	b.WriteString(strings.Join(listItems, "\n"))
	b.WriteString("\n\n  ↑/↓ navegar · Enter selecionar · Esc cancelar")

	view := s.menuStyle.
		Width(min(width-8, 70)).
		Render(b.String())

	return "\n" + view
}

// initSessionPicker populates the picker with sessions from the manager.
func (m *appModel) initSessionPicker(mgr *sessionpkg.Manager) {
	sessions, err := mgr.List(context.Background(), 50)
	if err != nil || len(sessions) == 0 {
		m.picker.sessions = nil
		m.picker.cursor = 0
		return
	}

	items := make([]sessionListItem, 0, len(sessions))
	for _, s := range sessions {
		items = append(items, sessionListItem{
			ID:          s.ID,
			Task:        s.Task,
			Status:      s.Status,
			MessageCnt:  s.MessageCount,
			ToolCallCnt: s.ToolCallCount,
			Model:       s.Model,
		})
	}
	m.picker.sessions = items
	m.picker.cursor = 0
}

// handleSessionPickerKey processes key events when the session picker is active.
func (m *appModel) handleSessionPickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.picker.done {
		return m, nil
	}

	switch msg.String() {
	case "esc":
		m.picker.visible = false
		m.picker.done = true
		return m, nil

	case "up":
		if m.picker.cursor > 0 {
			m.picker.cursor--
		}
		return m, nil

	case "down":
		if len(m.picker.sessions) > 0 && m.picker.cursor < len(m.picker.sessions)-1 {
			m.picker.cursor++
		}
		return m, nil

	case "enter":
		if len(m.picker.sessions) == 0 {
			// No sessions - just close picker and start fresh
			m.picker.visible = false
			m.picker.done = true
			return m, nil
		}

		selected := m.picker.sessions[m.picker.cursor]
		if m.sessionMgr != nil {
			// Touch the session to update last_activity
			_ = m.sessionMgr.Touch(context.Background(), selected.ID)
		}

		// Set session info and load history from DB
		if m.dbStore != nil {
			m.loadSessionFromDB(context.Background(), selected.ID)
		} else {
			if m.session != nil {
				m.session.ID = selected.ID
			}
			m.messages = append(m.messages, chatMessage{
				Sender:  "system",
				Content: "Sessão " + selected.ID + " carregada.",
			})
		}
		m.picker.visible = false
		m.picker.done = true
		return m, nil
	}

	return m, nil
}
