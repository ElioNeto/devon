package tui

import "fmt"

// ContextAction representa uma ação no menu de contexto.
type ContextAction struct {
	Label  string
	Action string // identificador para execução
}

// ctxMenuState mantém o estado do menu de contexto.
type ctxMenuState struct {
	visible bool
	title   string
	items   []ContextAction
	cursor  int
}

func (m *ctxMenuState) close() {
	m.visible = false
	m.cursor = 0
}

func (m *ctxMenuState) render(width int) string {
	if !m.visible {
		return ""
	}
	lines := make([]string, 0, len(m.items)+3)
	sep := "─"
	lines = append(lines, fmt.Sprintf(" %s %s %s", sep, m.title, sep))
	for i, item := range m.items {
		prefix := "  "
		if i == m.cursor {
			prefix = "▸ "
		}
		lines = append(lines, fmt.Sprintf("%s%s", prefix, item.Label))
	}
	lines = append(lines, " Esc para fechar")
	s := ""
	for i, line := range lines {
		if i > 0 {
			s += "\n"
		}
		s += line
	}
	return s
}
