package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// leftSection identifica as seções do painel esquerdo.
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

// leftItem representa um item navegável no painel esquerdo.
type leftItem struct {
	Section  leftSection
	Label    string
	SubLabel string // ex: nome da ferramenta ou ID do turno
	Index    int    // índice dentro da seção
	Icon     string
}

// buildLeftItems constrói a lista flat de itens navegáveis a partir do estado.
func buildLeftItems(m *appModel) []leftItem {
	var items []leftItem

	// ── Sessão atual ──────────────────────────────────────────
	items = append(items, leftItem{Section: secTurno, Label: sectionLabels[secTurno], Icon: "─"})

	if m.running {
		items = append(items, leftItem{Section: secTurno, Label: "▶ Turno ativo", Icon: "⏳", Index: 0})
		for i, tr := range m.toolRuns {
			icon := "⏳"
			if tr.Status == "done" {
				icon = "✔"
			} else if tr.Status == "error" {
				icon = "✘"
			}
			items = append(items, leftItem{
				Section:  secTurno,
				Label:    "  " + tr.Name,
				SubLabel: shortenArgs(tr.Args),
				Icon:     icon,
				Index:    i + 1,
			})
		}
	} else if len(m.messages) > 0 {
		items = append(items, leftItem{Section: secTurno, Label: "Turno concluído", Icon: "✔", Index: 0})
	}

	// ── Histórico ─────────────────────────────────────────────
	items = append(items, leftItem{Section: secHistorico, Label: sectionLabels[secHistorico], Icon: "─"})
	for i, hm := range m.historyTurns {
		lbl := truncate(hm.UserPrompt, 20)
		items = append(items, leftItem{
			Section:  secHistorico,
			Label:    fmt.Sprintf("  Turno %d", i+1),
			SubLabel: lbl,
			Icon:     "○",
			Index:    i,
		})
	}

	// ── Ferramentas ───────────────────────────────────────────
	items = append(items, leftItem{Section: secFerramentas, Label: sectionLabels[secFerramentas], Icon: "─"})
	for name, stat := range m.toolStats {
		items = append(items, leftItem{
			Section:  secFerramentas,
			Label:    fmt.Sprintf("  %-12s", name),
			SubLabel: fmt.Sprintf("%dx %s", stat.Calls, formatDuration(stat.AvgMs)),
			Icon:     "◆",
			Index:    0,
		})
	}

	// ── Memória ───────────────────────────────────────────────
	items = append(items, leftItem{Section: secMemoria, Label: sectionLabels[secMemoria], Icon: "─"})
	items = append(items, leftItem{Section: secMemoria, Label: "  Fatos do projeto", Icon: "◈", Index: 0})

	// ── Tokens ────────────────────────────────────────────────
	items = append(items, leftItem{Section: secTokens, Label: sectionLabels[secTokens], Icon: "─"})
	items = append(items, leftItem{Section: secTokens, Label: "  Consumo por turno", Icon: "▦", Index: 0})

	return items
}

// renderLeftPanel renderiza o painel esquerdo com lipgloss.
func renderLeftPanel(m *appModel, width, height int, focused bool) string {
	items := buildLeftItems(m)
	m.leftItems = items

	s := m.styles
	var lines []string

	for i, item := range items {
		// Cabeçalhos de seção
		if item.Icon == "─" {
			lbl := s.itemSection.Width(width - 2).Render(
				"── " + strings.ToUpper(item.Label) + " " + strings.Repeat("─", max(0, width-7-len(item.Label))),
			)
			lines = append(lines, lbl)
			continue
		}

		isSelected := i == m.leftCursor
		lineStr := item.Icon + " " + item.Label
		if item.SubLabel != "" {
			lineStr += "  " + item.SubLabel
		}
		lineStr = truncate(lineStr, width-3)

		var rendered string
		if isSelected {
			if focused {
				rendered = lipgloss.NewStyle().
					Foreground(colorText).
					Background(colorSurface2).
					Width(width - 2).
					Render(lineStr)
			} else {
				rendered = s.itemSelected.Width(width - 2).Render(lineStr)
			}
		} else {
			var style lipgloss.Style
			switch item.Icon {
			case "⏳":
				style = s.toolRunning
			case "✔":
				style = s.toolDone
			case "✘":
				style = s.toolError
			default:
				style = s.itemNormal
			}
			rendered = style.Width(width - 2).Render(lineStr)
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
