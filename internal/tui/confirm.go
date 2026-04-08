// Package tui renders and manages the confirmation overlay.
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ConfirmRequest is emitted by the agent when a tool needs user confirmation.
type ConfirmRequest struct {
	Tool  string
	Args  string
	Level string // "read" | "write" | "execute"
}

// confirmState tracks the overlay state.
type confirmState struct {
	visible bool
	req     ConfirmRequest
	choices []string // rendered choice labels
	cursor  int
}

func (c *confirmState) open(req ConfirmRequest) {
	c.visible = true
	c.req = req
	c.choices = []string{
		"[y] Sim — executar esta vez",
		"[n] Não — bloquear",
		"[a] Sempre — aprovar nesta sessão",
	}
	c.cursor = 0
}

func (c *confirmState) close() {
	c.visible = false
	c.cursor = 0
}

func renderConfirmOverlay(m *appModel, width int) string {
	panelW := 50
	if panelW > width-4 {
		panelW = width - 4
	}

	lines := []string{}

	// Header: tool name
	title := fmt.Sprintf("⚡ %s", m.confirm.req.Tool)
	lines = append(lines, m.styles.badgeTool.Render(title))

	// Permission level badge
	level := m.confirm.req.Level
	lvlStyle := m.styles.badgeRead
	switch level {
	case "write":
		lvlStyle = m.styles.badgeWrite
	case "execute":
		lvlStyle = m.styles.badgeExecute
	}
	lines = append(lines, lvlStyle.Render(level))
	lines = append(lines, "")

	// Args preview (first few lines)
	preview := formatArgsPreview(m.confirm.req.Args)
	if preview != "" {
		lines = append(lines, "Argumentos:")
		lines = append(lines, preview)
		lines = append(lines, "")
	}

	lines = append(lines, "Permissão:")

	// Choices
	for i, choice := range m.confirm.choices {
		cursor := "  "
		if i == m.confirm.cursor {
			cursor = ">"
		}
		lines = append(lines, fmt.Sprintf("%s %s", cursor, choice))
	}

	lines = append(lines, "")
	lines = append(lines, "y/n/a ou ↑↓ Enter")

	content := joinLines(lines)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorPrimary).
		Padding(0, 1).
		Width(panelW - 2).
		Render(content)
}

func formatArgsPreview(args string) string {
	if args == "" {
		return ""
	}

	s := args
	// Try to truncate for readability
	const maxLines = 8
	truncated := false
	lines := strings.Split(s, "\n")
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		truncated = true
	}

	result := strings.Join(lines, "\n")
	if len(result) > 120 {
		// Truncate to first 120 chars per line
		truncated = true
		out := ""
		for _, line := range strings.Split(result, "\n") {
			if len(line) > 120 {
				line = line[:120] + "…"
			}
			if out != "" {
				out += "\n"
			}
			out += line
		}
		result = out
	}

	if truncated {
		result += "\n…"
	}
	return result
}
