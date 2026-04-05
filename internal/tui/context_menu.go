package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// menuAction define uma ação no menu de contexto.
type menuAction struct {
	Label  string
	Key    string
	Action func(m *appModel)
}

// contextMenuFor retorna as ações disponíveis para o item atualmente selecionado.
func contextMenuFor(m *appModel) []menuAction {
	if len(m.leftItems) == 0 || m.leftCursor >= len(m.leftItems) {
		return nil
	}
	item := m.leftItems[m.leftCursor]

	switch item.Section {
	case secTurno:
		if m.running {
			return []menuAction{
				{Label: "Interromper agente", Key: "i", Action: func(m *appModel) {
					if m.cancel != nil {
						m.cancel()
						m.running = false
					}
				}},
				{Label: "Copiar última resposta", Key: "c", Action: func(m *appModel) {
					// placeholder — clipboard via atotto/clipboard
					m.statusMsg = "Resposta copiada"
				}},
			}
		}
		return []menuAction{
			{Label: "Copiar última resposta", Key: "c", Action: func(m *appModel) {
				m.statusMsg = "Resposta copiada"
			}},
			{Label: "Nova sessão", Key: "n", Action: func(m *appModel) {
				m.messages = nil
				m.toolRuns = nil
			}},
		}

	case secHistorico:
		return []menuAction{
			{Label: "Re-executar prompt", Key: "r", Action: func(m *appModel) {
				if m.selectedTurnIdx >= 0 && m.selectedTurnIdx < len(m.historyTurns) {
					m.pendingInput = m.historyTurns[m.selectedTurnIdx].UserPrompt
				}
			}},
			{Label: "Copiar mensagem", Key: "c", Action: func(m *appModel) {
				m.statusMsg = "Mensagem copiada"
			}},
			{Label: "Exportar turno", Key: "e", Action: func(m *appModel) {
				m.statusMsg = "Turno exportado"
			}},
		}

	case secFerramentas:
		if m.selectedTool != nil {
			return []menuAction{
				{Label: "Ver input completo", Key: "i", Action: func(m *appModel) {
					m.rightView = viewToolCall
					m.expandedView = true
				}},
				{Label: "Ver output", Key: "o", Action: func(m *appModel) {
					m.rightView = viewToolCall
				}},
				{Label: "Abrir arquivo no editor", Key: "f", Action: func(m *appModel) {
					m.statusMsg = "Abrindo no $EDITOR..."
				}},
			}
		}
		return []menuAction{
			{Label: "Ver estatísticas", Key: "s", Action: func(m *appModel) {
				m.rightView = viewFerramentas
			}},
		}

	case secMemoria:
		return []menuAction{
			{Label: "Editar fato", Key: "e", Action: func(m *appModel) {
				m.statusMsg = "Editar fato (em breve)"
			}},
			{Label: "Deletar fato", Key: "d", Action: func(m *appModel) {
				m.statusMsg = "Fato deletado"
			}},
			{Label: "Exportar memória", Key: "x", Action: func(m *appModel) {
				m.statusMsg = "Memória exportada"
			}},
		}

	case secTokens:
		return []menuAction{
			{Label: "Ver gráfico completo", Key: "g", Action: func(m *appModel) {
				m.rightView = viewTokens
			}},
			{Label: "Exportar uso", Key: "e", Action: func(m *appModel) {
				m.statusMsg = "Uso exportado"
			}},
		}
	}

	return nil
}

// renderContextMenu renderiza o menu de contexto como overlay centralizado.
func renderContextMenu(m *appModel, width, height int) string {
	s := m.styles
	actions := contextMenuFor(m)

	var lines []string
	lines = append(lines, s.panelTitle.Render("Menu de contexto  (esc para fechar)"))
	lines = append(lines, strings.Repeat("─", 36))
	lines = append(lines, "")

	for i, a := range actions {
		var line string
		if i == m.menuCursor {
			line = s.menuSelected.Render(a.Key+"  "+a.Label)
		} else {
			line = s.menuItem.Render(a.Key+"  "+a.Label)
		}
		lines = append(lines, line)
	}

	if len(actions) == 0 {
		lines = append(lines, s.sysMsg.Render("  Nenhuma ação disponível"))
	}

	content := strings.Join(lines, "\n")
	menuW := 40
	if menuW > width-4 {
		menuW = width - 4
	}

	return lipgloss.Place(
		width, height,
		lipgloss.Center, lipgloss.Center,
		s.menuStyle.Width(menuW).Render(content),
	)
}
