package tui

import (
	"fmt"
	"strings"
)

const (
	barFull  = "█"
	barEmpty = "░"
	barSpark = "▁▂▃▄▅▆▇█"
)

// HorizBar renders a horizontal bar: label [████░░░░] value
func HorizBar(label string, value, max int, width int, labelW int) string {
	if max == 0 {
		max = 1
	}
	availW := width - labelW - 12
	if availW < 4 {
		availW = 4
	}
	filled := int(float64(availW) * float64(value) / float64(max))
	if filled > availW {
		filled = availW
	}
	bar := strings.Repeat(barFull, filled) + strings.Repeat(barEmpty, availW-filled)
	padded := fmt.Sprintf("%-*s", labelW, label)
	return fmt.Sprintf("%s %s %s", padded, bar, formatShort(value))
}

// VertBars renders a vertical bar chart for a slice of (label, value) pairs.
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
		for i, v := range values {
			cell := strings.Repeat(" ", colW)
			if v >= threshold {
				cell = centerStr(barFull, colW)
			}
			_ = i
			line.WriteString(cell)
		}
		rows[row] = line.String()
	}

	// Labels row
	var labelLine strings.Builder
	for _, lbl := range labels {
		labelLine.WriteString(centerStr(truncate(lbl, colW), colW))
	}
	rows[height] = labelLine.String()

	return rows
}

// Sparkline renders a compact single-line bar from values.
func Sparkline(values []int, width int) string {
	if len(values) == 0 {
		return ""
	}
	maxV := 1
	for _, v := range values {
		if v > maxV {
			maxV = v
		}
	}
	runes := []rune(barSpark)
	n := len(runes)
	var sb strings.Builder
	for i, v := range values {
		if i >= width {
			break
		}
		idx := int(float64(v)/float64(maxV)*float64(n-1) + 0.5)
		if idx >= n {
			idx = n - 1
		}
		sb.WriteRune(runes[idx])
	}
	return sb.String()
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

func formatShort(n int) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 1 {
		return "."
	}
	return s[:max-1] + "…"
}
