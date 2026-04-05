package tui

import "fmt"

// ContextAction representa uma ação no menu de contexto.
type ContextAction struct {
	Label string
	Action string // identificador para execução
}

// ctxMenuState mantém o estado do menu de contexto.
type ctxMenuState struct {
	visible bool
	title   string
	items   []ContextAction
	cursor  int
	target  string // alvo contextual (session ID, tool name, etc.)
}

func (m *ctxMenuState) open(title string, target string, items []ContextAction) {
	m.visible = true
	m.title = title
	m.target = target
	m.items = items
	m.cursor = 0
}

func (m *ctxMenuState) close() {
	m.visible = false
	m.cursor = 0
}

func buildContextMenu(item string, detail string) *ctxMenuState {
	m := &ctxMenuState{}
	switch item {
	case "turn_active":
		m.open("Turno Ativo", "current_turn", []ContextAction{
			{"Interromper agente", "interrupt"},
			{"Copiar resposta", "copy_response"},
			{"Nova sessão", "new_session"},
		})
	case "tool_call":
		m.open("Tool Call: "+detail, detail, []ContextAction{
			{"Ver input completo", "tool_input"},
			{"Ver output", "tool_output"},
			{"Copiar resultado", "copy_result"},
		})
	case "history_turn":
		m.open("Turno Histórico: "+detail, detail, []ContextAction{
			{"Re-executar prompt", "replay"},
			{"Copiar mensagem", "copy_msg"},
			{"Exportar turno", "export_turn"},
		})
	case "session":
		m.open("Sessão: "+detail, detail, []ContextAction{
			{"Renomear", "rename"},
			{"Exportar", "export"},
			{"Deletar", "delete"},
		})
	case "memory_fact":
		m.open("Memória", detail, []ContextAction{
			{"Editar fato", "edit_fact"},
			{"Deletar fato", "delete_fact"},
		})
	default:
		return nil
	}
	return m
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
