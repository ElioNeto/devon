package tui

import (
	"strings"
)

// ═══════════════════ chart.go ═══════════════════════════════
// Standalone chart primitives.
// HorzBar, fmtShort, Sparkline, Trunc → helpers.go

const (
	barFull  = "█"
	barEmpty = "░"
)

// HorizBar is a compat alias for HorzBar (keeps old call sites working).
func HorizBar(label string, value, maxV, width int, labelW int) string {
	return HorzBar(label, value, maxV, width, labelW)
}

// VertBars renders a vertical bar chart as a slice of strings (rows).
func VertBars(labels []string, values []int, width, height int) []string {
	if len(values) == 0 {
		return []string{"(sem dados)"}
	}
	maxV := 1
	for _, v := range values {
		if v > maxV {
			maxV = v
		}
	}
	colW := width / len(values)
	if colW < 3 {
		colW = 3
	}
	rows := make([]string, height+1)
	for row := 0; row < height; row++ {
		threshold := maxV - (row * maxV / height)
		var line strings.Builder
		for _, v := range values {
			cell := strings.Repeat(" ", colW)
			if v >= threshold {
				cell = centerStr(barFull, colW)
			}
			line.WriteString(cell)
		}
		rows[row] = line.String()
	}
	var labelLine strings.Builder
	for _, lbl := range labels {
		labelLine.WriteString(centerStr(Trunc(lbl, colW), colW))
	}
	rows[height] = labelLine.String()
	return rows
}

func centerStr(s string, w int) string {
	if len(s) >= w {
		return s[:w]
	}
	pad := w - len(s)
	left := pad / 2
	right := pad - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}
