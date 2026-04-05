package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ── Section types ────────────────────────────────────────────────────────────

type leftSection int

const (
	secTurno leftSection = iota
	secHistorico
	secFerramentas
	secMemoria
	secTokens
)

var sectionLabels = []string{
	"Sessão atual",
	"Histórico",
	"Ferramentas",
	"Memória",
	"Tokens",
}

// ── Item model ───────────────────────────────────────────────────────────────

type leftItem struct {
	Section    leftSection
	Label      string
	SubLabel   string
	Index      int
	Icon       string
	StatusKind string // "running" | "exited" | "" (for section headers: "header")
	CPU        string // optional right-aligned metric
}

// ── Build items ──────────────────────────────────────────────────────────────

func buildLeftItems(m *appModel) []leftItem {
	var items []leftItem

	// ── Services (Sessão atual / tool runs) ───────────────────
	items = append(items, leftItem{Section: secTurno, Label: "Services", StatusKind: "header"})
	if m.running {
		items = append(items, leftItem{
			Section: secTurno, Label: "agent", StatusKind: "running",
			CPU: "…", Index: 0,
		})
	}
	for i, tr := range m.toolRuns {
		kind := "running"
		if tr.Status == "done" {
			kind = "running"
		} else if tr.Status == "error" {
			kind = "exited"
		}
		items = append(items, leftItem{
			Section:    secTurno,
			Label:      tr.Name,
			SubLabel:   shortenArgs(tr.Args),
			StatusKind: kind,
			CPU:        "",
			Index:      i + 1,
		})
	}

	// ── Standalone Containers (histórico de turnos) ───────────
	items = append(items, leftItem{Section: secHistorico, Label: "Standalone Containers", StatusKind: "header"})
	for i, ht := range m.historyTurns {
		lbl := truncate(ht.UserPrompt, 18)
		kind := "running"
		if i < len(m.historyTurns)-1 {
			kind = "exited"
		}
		items = append(items, leftItem{
			Section:    secHistorico,
			Label:      lbl,
			StatusKind: kind,
			Index:      i,
		})
	}

	// ── Images (ferramentas disponíveis) ──────────────────────
	items = append(items, leftItem{Section: secFerramentas, Label: "Images", StatusKind: "header"})
	for name, stat := range m.toolStats {
		items = append(items, leftItem{
			Section:  secFerramentas,
			Label:    name,
			SubLabel: fmt.Sprintf("%dx", stat.Calls),
			CPU:      fmt.Sprintf("%d", stat.Calls),
			Index:    0,
		})
	}
	if len(m.toolStats) == 0 {
		items = append(items, leftItem{Section: secFerramentas, Label: "<none>", SubLabel: "<none>", CPU: "0"})
	}

	// ── Volumes (tokens) ──────────────────────────────────────
	items = append(items, leftItem{Section: secTokens, Label: "Volumes", StatusKind: "header"})
	if m.tracker != nil {
		items = append(items, leftItem{
			Section:  secTokens,
			Label:    "tokens",
			SubLabel: formatShort(m.tracker.TotalInputTokens + m.tracker.TotalOutputTokens),
			Index:    0,
		})
	}

	return items
}

// ── Renderer ─────────────────────────────────────────────────────────────────

func renderLeftPanel(m *appModel, width, height int, focused bool) string {
	items := buildLeftItems(m)
	m.leftItems = items

	s := m.styles
	var lines []string

	for i, item := range items {
		// Section header row — lazydocker style ─ Services ──
		if item.StatusKind == "header" {
			avail := width - 4 - len(item.Label)
			if avail < 0 {
				avail = 0
			}
			headerLine := "─" + item.Label + strings.Repeat("─", avail)
			lines = append(lines, s.itemSection.Width(width-2).Render(headerLine))
			continue
		}

		isSelected := i == m.leftCursor

		// Status badge: "running" or "exited (N)"
		var badge string
		switch item.StatusKind {
		case "running":
			badge = s.statusRunning.Render("running")
		case "exited":
			badge = s.statusExited.Render("exited (1)")
		default:
			badge = ""
		}

		// Right-side CPU/metric
		cpuPart := ""
		if item.CPU != "" {
			cpuPart = s.statusVal.Render(item.CPU + "%")
		}

		// Compose line: " running label        0.01%"
		labelTrunc := truncate(item.Label, width-22)
		var lineStr string
		if badge != "" {
			lineStr = badge + " " + labelTrunc
		} else {
			lineStr = "         " + labelTrunc
		}
		if cpuPart != "" {
			// pad to align right
			pureLen := lipgloss.Width(badge) + 1 + len(labelTrunc)
			pad := width - 4 - pureLen - lipgloss.Width(cpuPart)
			if pad > 0 {
				lineStr += strings.Repeat(" ", pad)
			}
			lineStr += cpuPart
		}

		var rendered string
		if isSelected && focused {
			rendered = lipgloss.NewStyle().
				Background(colorSurface2).
				Width(width - 2).
				Render(lineStr)
		} else if isSelected {
			rendered = s.itemSelected.Width(width - 2).Render(lineStr)
		} else {
			rendered = s.itemNormal.Width(width - 2).Render(lineStr)
		}
		lines = append(lines, rendered)
	}

	// Pad to fill height
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width-2))
	}
	if len(lines) > height {
		lines = lines[:height]
	}

	content := strings.Join(lines, "\n")
	panelStyle := s.panelBase
	if focused {
		panelStyle = s.panelFocused
	}
	return panelStyle.
		Width(width).
		Height(height).
		Render(content)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func formatDuration(ms int64) string {
	if ms >= 1000 {
		return fmt.Sprintf("%.1fs", float64(ms)/1000)
	}
	return fmt.Sprintf("%dms", ms)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
