package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderStatusBar renderiza a barra superior com informações do projeto.
func renderStatusBar(m *appModel, width int) string {
	s := m.styles

	wd := m.cfg.WorkDir
	if len(wd) > 28 {
		wd = "…" + wd[len(wd)-27:]
	}

	totalTok := m.tracker.TotalInputTokens + m.tracker.TotalOutputTokens
	costStr := formatCostStr(m.tracker.TotalCostUSD)

	parts := []string{
		s.statusKey.Render("devon") + "  ",
		s.statusKey.Render("dir:") + " " + s.statusVal.Render(wd),
		s.statusSep.Render(" │ "),
		s.statusKey.Render("modelo:") + " " + s.statusVal.Render(truncate(m.cfg.Model, 22)),
		s.statusSep.Render(" │ "),
		s.statusKey.Render("tokens:") + " " + s.statusVal.Render(formatShort(totalTok)),
		s.statusSep.Render(" │ "),
		s.statusKey.Render("custo:") + " " + s.statusVal.Render(costStr),
		s.statusSep.Render(" │ "),
		s.statusKey.Render("modo:") + " " + renderModeBadge(m),
	}

	line := strings.Join(parts, "")

	// Pad to full width
	visibleLen := lipgloss.Width(line)
	if visibleLen < width {
		line += strings.Repeat(" ", width-visibleLen)
	}

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

// renderHelp renderiza o painel de ajuda.
func renderHelp(m *appModel, width int) string {
	s := m.styles
	pairs := []struct{ k, v string }{
		{"↑↓", "navegar"},
		{"←→", "painéis"},
		{"tab", "seção"},
		{"enter", "selecionar"},
		{"x", "menu contexto"},
		{"e", "expandir"},
		{"ctrl+c", "interromper/sair"},
		{"ctrl+l", "limpar"},
		{"ctrl+k", "nova sessão"},
		{"pgup/pgdn", "rolar"},
		{"q/esc", "fechar menu"},
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
