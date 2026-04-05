// renderStatusBar, renderModeBadge, renderInputBar, renderInputLine
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/ElioNeto/devon/internal/cost"
)

// Trunc is a package-level helper for safely truncating display strings.
func Trunc(s string, n int) string {
	return truncate(s, n)
}

// renderStatusBar renderiza a barra inferior de status — lazydocker style:
//
//	PgUp/PgDn: scroll · esc/q: fechar · x: menu · ← → ↑ ↓: navegar
func renderStatusBar(m *appModel, width int) string {
	s := m.styles

	var parts []string
	navHints := []struct{ key, desc string }{
		{"PgUp/PgDn", "scroll"},
		{"esc/q", "fechar"},
		{"x", "menu"},
		{"← → ↑ ↓", "navegar"},
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

	rightStr := s.statusKey.Render("modelo: ") +
		s.statusVal.Render(Trunc(m.cfg.Model, 20)) +
		s.statusSep.Render(" │ ") +
		s.statusKey.Render("modo: ") +
		renderModeBadge(m) +
		s.statusSep.Render(" │ ") +
		s.statusKey.Render("custo: ") +
		s.statusVal.Render(cost.FormatCost(m.tracker.TotalCostUSD))

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

// renderInputBar renderiza a barra de input na parte inferior.
func renderInputBar(m *appModel, width int) string {
	s := m.styles
	var content string

	if m.running {
		content = m.spinner.View() + "  " + s.statusKey.Render("aguardando resposta...")
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
