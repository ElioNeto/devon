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
		{"Shift+Enter", "↵"},
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

// renderInputBar renderiza a barra de input.
func renderInputBar(m *appModel, width int) string {
	s := m.styles

	if m.running {
		running := m.spinner.View() + "  " + s.statusKey.Render("aguardando resposta...")
		return s.inputBar.Width(width - 2).Render(running)
	}

	// Attachment badges — rendered above the prompt line
	var badgeLine string
	if len(m.attachments) > 0 {
		badgeStyle := s.badge.Copy()
		var badges []string
		for _, att := range m.attachments {
			badge := badgeStyle.Render(att.attachmentBadge())
			badges = append(badges, badge)
		}
		badgeLine = strings.Join(badges, " ") + "\n"
	}

	if !strings.Contains(m.input, "\n") {
		prompt := s.inputPrompt.Render("> ")
		return s.inputBar.Width(width - 2).Render(badgeLine + prompt + renderInputLine(m))
	}

	// Multi-line input: stack rows vertically.
	rows := strings.Split(m.input, "\n")
	multilineRows := len(rows)
	if multilineRows > 6 {
		multilineRows = 6
	}

	var lines []string
	for i := 0; i < multilineRows; i++ {
		prefix := "> "
		if i == 0 {
			prefix = s.inputPrompt.Render(prefix)
		} else {
			prefix = s.inputPrompt.Render("  ")
		}
		lineContent := rows[i]
		// Reconstruct cursor for this line if the cursor is on it.
		lineStart := runeOffset(m.input, i)
		lineEnd := lineStart + len([]rune(rows[i]))
		isCursorLine := m.cursor >= lineStart && m.cursor <= lineEnd
		if isCursorLine {
			col := m.cursor - lineStart
			lines = append(lines, prefix+renderInputLineWithCursor(lineContent, col, s))
		} else {
			lines = append(lines, prefix+lineContent)
		}
	}

	m.multilineRows = multilineRows
	content := strings.Join(lines, "\n")
	if badgeLine != "" {
		content = badgeLine + content
	}
	return s.inputBar.Width(width - 2).Render(content)
}

// runeOffset returns the rune index where line `lineIdx` starts in `s`.
func runeOffset(s string, lineIdx int) int {
	ru := []rune(s)
	lineStart := 0
	for i := 0; i < len(ru) && lineIdx > 0; i++ {
		if ru[i] == '\n' {
			lineStart = i + 1
			lineIdx--
		}
	}
	return lineStart
}

// renderInputLineWithCursor renders a single line with the cursor highlighted.
func renderInputLineWithCursor(line string, col int, s uiStyles) string {
	ru := []rune(line)
	if col >= len(ru) {
		return line + s.cursorStyle.Render("▋")
	}
	before := string(ru[:col])
	cur := s.cursorStyle.Render(string(ru[col]))
	after := string(ru[col+1:])
	return before + cur + after
}

func renderInputLine(m *appModel) string {
	ru := []rune(m.input)
	s := m.styles
	if len(ru) == 0 {
		return s.sysMsg.Render("envie uma mensagem  [!] comandos  [?] ajuda")
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

	model := s.keyStyle.Render("modelo: ") + s.statusVal.Render(m.cfg.Model)

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
		right += sep + s.keyStyle.Render("sessão: ") + s.statusVal.Render(id)
	}

	return right
}
