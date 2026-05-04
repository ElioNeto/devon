// renderStatusBar, renderModeBadge, renderInputBar, renderInputLine
package tui

import (
	"fmt"
	"strings"

	"github.com/ElioNeto/devon/internal/cost"
)

// appVersion é a versão exibida na status bar.
const appVersion = "0.4.1"

// renderStatusBar renderiza a barra inferior no formato OpenCode:
//
//	[model] · [provider] · tokens: [in]/[out] · ~$[cost] · [status]
func renderStatusBar(m *appModel, width int) string {
	s := m.styles

	// Agent status
	var statusStr string
	if m.running {
		hasRunningTool := false
		for _, tr := range m.toolRuns {
			if tr.Status == "running" {
				statusStr = s.statusRunning.Render(fmt.Sprintf("⚙  running %s", tr.Name))
				hasRunningTool = true
				break
			}
		}
		if !hasRunningTool {
			statusStr = s.statusKey.Render(m.spinner.View() + " thinking…")
		}
	} else {
		statusStr = s.statusSep.Render("●  idle")
	}

	// Model name
	modelStr := s.statusKey.Render(m.cfg.Model)

	// Provider
	providerName := extractProvider(m.cfg.BaseURL)
	providerStr := s.statusVal.Render(providerName)

	// Tokens
	var tokensStr string
	if m.tracker != nil {
		inStr := formatShort(m.tracker.TotalInputTokens)
		outStr := formatShort(m.tracker.TotalOutputTokens)
		tokensStr = s.statusVal.Render(fmt.Sprintf("%s/%s", inStr, outStr))
	} else {
		tokensStr = s.statusVal.Render("0/0")
	}

	// Cost
	var costStr string
	if m.tracker != nil && m.tracker.TotalCostUSD > 0 {
		costStr = s.statusVal.Render(fmt.Sprintf("~$%s", cost.FormatCost(m.tracker.TotalCostUSD)))
	} else {
		costStr = s.statusVal.Render("~$0.00")
	}

	// Build line: model · provider · tokens: in/out · ~$cost · status
	sep := s.statusSep.Render(" · ")

	tokensSection := s.statusKey.Render("tokens: ") + tokensStr

	line := fmt.Sprintf("%s%s%s%s%s%s%s%s%s",
		modelStr, sep,
		providerStr, sep,
		tokensSection, sep,
		costStr, sep,
		statusStr,
	)

	return s.statusBar.Width(width).Render(line)
}

// extractProvider returns a short provider name from the base URL.
func extractProvider(baseURL string) string {
	lower := strings.ToLower(baseURL)
	switch {
	case strings.Contains(lower, "openai"):
		return "openai"
	case strings.Contains(lower, "anthropic"):
		return "anthropic"
	case strings.Contains(lower, "ollama"), strings.Contains(lower, "11434"):
		return "ollama"
	case strings.Contains(lower, "google"), strings.Contains(lower, "gemini"):
		return "google"
	case strings.Contains(lower, "deepseek"):
		return "deepseek"
	default:
		return "api"
	}
}

// renderInputBar renderiza a barra de input.
func renderInputBar(m *appModel, width int) string {
	s := m.styles

	if m.running {
		running := m.spinner.View() + "  " + s.statusKey.Render("aguardando resposta...")
		return s.inputBar.Width(width).Render(running)
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
		return s.inputBar.Width(width).Render(badgeLine + prompt + renderInputLine(m))
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
	return s.inputBar.Width(width).Render(content)
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
