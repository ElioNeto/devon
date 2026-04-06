// renderStatusBar, renderModeBadge, renderInputBar, renderInputLine
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/ElioNeto/devon/internal/cost"
)

// appVersion é a versão exibida na status bar.
const appVersion = "0.4.1"

// renderStatusBar renderiza a barra inferior — fiel à imagem:
//
//	PgUp/PgDn: scroll   esc/q: quit   Tab: focus   ↑↓: navigate   Enter: select   x: menu
func renderStatusBar(m *appModel, width int) string {
	s := m.styles

	// Status bar hints — updated for workspace tabs and command menu.
	navHints := []struct{ key, desc string }{
		{"PgUp/PgDn", "scroll"},
		{"esc/q", "quit"},
		{"Tab", "focus"},
		{"↑↓", "navigate"},
		{"Enter", "select"},
		{"!", "comandos"},
	}
	var parts []string
	for i, h := range navHints {
		if i > 0 {
			parts = append(parts, s.statusSep.Render("   "))
		}
		parts = append(parts, s.keyStyle.Render(h.key)+s.statusKey.Render(": "+h.desc))
	}
	leftStr := strings.Join(parts, "")

	rightStr := buildStatusRight(m)

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

// renderInputBar renderiza a barra de input.
func renderInputBar(m *appModel, width int) string {
	s := m.styles
	var content string

	if m.running {
		content = m.spinner.View() + "  " + s.statusKey.Render("aguardando resposta...")
	} else {
		prompt := s.inputPrompt.Render("> ")
		content = prompt + renderInputLine(m)
	}

	return s.inputBar.Width(width - 2).Render(content)
}

func renderInputLine(m *appModel) string {
	ru := []rune(m.input)
	s := m.styles
	if len(ru) == 0 {
		return s.sysMsg.Render(fmt.Sprintf("envie uma mensagem  [!] comandos  [?] ajuda"))
	}
	if m.cursor >= len(ru) {
		return m.input + s.cursorStyle.Render("▋")
	}
	before := string(ru[:m.cursor])
	cur := s.cursorStyle.Render(string(ru[m.cursor]))
	after := string(ru[m.cursor+1:])
	return before + cur + after
}

// buildStatusRight returns the right side of the status bar: model, tokens, cost, session.
func buildStatusRight(m *appModel) string {
	s := m.styles

	model := s.keyStyle.Render("model: ") + s.statusVal.Render(m.cfg.Model)

	if m.tracker == nil {
		return model
	}

	totalTokens := m.tracker.TotalInputTokens + m.tracker.TotalOutputTokens
	if totalTokens == 0 {
		return model
	}

	tokensStr := s.keyStyle.Render("tokens: ") + s.statusVal.Render(fmt.Sprint(totalTokens))

	var costStr string
	if strings.HasSuffix(strings.ToLower(m.cfg.Model), ":free") || m.tracker.TotalCostUSD == 0 {
		costStr = s.keyStyle.Render("custo: ") + s.statusVal.Render("grátis")
	} else {
		costStr = s.keyStyle.Render("custo: ") + s.statusVal.Render(cost.FormatCost(m.tracker.TotalCostUSD))
	}

	sep := s.statusSep.Render("  ")

	right := model + sep + tokensStr + sep + costStr

	if m.session != nil && m.session.ID != "" {
		id := m.session.ID
		if len(id) > 15 {
			id = id[len(id)-15:]
		}
		right += sep + s.keyStyle.Render("session: ") + s.statusVal.Render(id)
	}

	return right
}

// renderCostBar renderiza a barra de custo/progresso (usada em renderStatusBar modo verboso).
func renderCostBar(m *appModel) string {
	if m.tracker == nil {
		return ""
	}
	return cost.FormatCost(m.tracker.TotalCostUSD)
}
