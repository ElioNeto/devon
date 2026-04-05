package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderStatusBar renderiza a barra inferior de status — lazydocker style:
//
//	PgUp/PgDn: scroll · esc/q: close · x: menu · ← → ↑ ↓: navigate
func renderStatusBar(m *appModel, width int) string {
	s := m.styles

	var parts []string

	// Navigation hints (left side)
	navHints := []struct{ key, desc string }{
		{"PgUp/PgDn", "scroll"},
		{"esc/q", "close"},
		{"x", "menu"},
		{"← → ↑ ↓", "navigate"},
	}
	for i, h := range navHints {
		if i > 0 {
			parts = append(parts, s.statusSep.Render(","))
			parts = append(parts, " ")
		}
		parts = append(parts, s.keyStyle.Render(h.key))
		parts = append(parts, s.statusKey.Render(": "+h.desc))
	}

	leftStr := strings.Join(parts, "")

	// Right side: model + mode + cost
	rightStr := s.statusKey.Render("modelo: ") +
		s.statusVal.Render(truncate(m.cfg.Model, 20)) +
		s.statusSep.Render(" │ ") +
		s.statusKey.Render("modo: ") +
		renderModeBadge(m) +
		s.statusSep.Render(" │ ") +
		s.statusKey.Render("custo: ") +
		s.statusVal.Render(formatCostStr(m.tracker.TotalCostUSD))

	leftW := lipgloss.Width(leftStr)
	rightW := lipgloss.Width(rightStr)
	gap := width - leftW - rightW - 2
	if gap < 1 {
		gap = 1
	}

	line := leftStr + strings.Repeat(" ", gap) + rightStr
	return s.statusBar.Width(width).Render(line)
}

func renderModeBadge(m *appModel) string {
	mode := m.cfg.Mode.String()
	var color lipgloss.Color
	switch mode {
	case "yolo":
		color = colorRed
	case "safe":
		color = colorGreen
	default:
		color = colorYellow
	}
	return lipgloss.NewStyle().Foreground(color).Bold(true).Render(mode)
}

// renderHelp renderiza o painel de ajuda inline.
func renderHelp(m *appModel, width int) string {
	s := m.styles
	pairs := []struct{ k, v string }{
		{"↑↓", "navegar"},
		{"tab", "seção"},
		{"enter", "selecionar"},
		{"x", "menu contexto"},
		{"e", "expandir"},
		{"ctrl+c", "sair"},
		{"ctrl+l", "limpar"},
		{"ctrl+k", "nova sessão"},
		{"pgup/pgdn", "rolar"},
		{"q/esc", "fechar"},
		{"?", "ajuda"},
	}
	var sb strings.Builder
	for _, p := range pairs {
		sb.WriteString(s.keyStyle.Render(p.k))
		sb.WriteString(s.helpStyle.Render(" "+p.v+"  "))
	}
	line := sb.String()
	visW := lipgloss.Width(line)
	if visW < width {
		line += strings.Repeat(" ", width-visW)
	}
	return lipgloss.NewStyle().
		Background(colorSurface).
		Width(width).
		Render(line)
}

// renderInputBar renderiza a barra de input na parte inferior.
func renderInputBar(m *appModel, width int) string {
	s := m.styles
	var content string

	if m.running {
		content = m.spinner.View() + "  " + s.statusKey.Render("aguardando resposta...")
	} else if m.statusMsg != "" {
		content = s.sysMsg.Render("  " + m.statusMsg)
	} else {
		prompt := s.inputPrompt.Render("> ")
		content = prompt + renderInputLine(m)
	}

	return s.inputBar.
		Width(width - 2).
		Render(content)
}

func renderInputLine(m *appModel) string {
	ru := []rune(m.input)
	s := m.styles
	if len(ru) == 0 {
		return s.sysMsg.Render(fmt.Sprintf("envie uma mensagem  [x] menu  [?] ajuda"))
	}
	if m.cursor >= len(ru) {
		return m.input + s.cursorStyle.Render("▋")
	}
	before := string(ru[:m.cursor])
	cur := s.cursorStyle.Render(string(ru[m.cursor]))
	after := string(ru[m.cursor+1:])
	return before + cur + after
}
