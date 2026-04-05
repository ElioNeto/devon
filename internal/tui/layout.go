package tui

import (
	"fmt"
	"strings"
)

// layout dimensions
type layout struct {
	width   int
	height  int
	hdrH    int
	dividerH int
	inputH  int
	footerH int
	panelH  int
	leftW   int
	rightW  int
}

func calcLayout(w, h int) layout {
	l := layout{width: w, height: h}
	l.hdrH = 1
	l.dividerH = 1
	l.inputH = 1
	l.footerH = 1
	l.panelH = h - l.hdrH - l.dividerH - l.inputH - l.footerH
	if l.panelH < 3 {
		l.panelH = 3
	}
	l.leftW = w / 3
	if l.leftW < 20 {
		l.leftW = 20
	}
	if l.leftW > 36 {
		l.leftW = 36
	}
	l.rightW = w - l.leftW - 1
	return l
}

// ═══════════════════ ASCII Charts ═══════════════════════════════════

// HorzBar renders: label [████░░░░] meta
func HorzBar(label string, value, maxV, availW, labelW int) string {
	if maxV <= 0 {
		maxV = 1
	}
	barW := availW - labelW - 8
	if barW < 2 {
		barW = 2
	}
	fill := int(float64(barW) * float64(value) / float64(maxV))
	if fill < 0 {
		fill = 0
	}
	if fill > barW {
		fill = barW
	}
	bar := "[" + strings.Repeat("█", fill) + strings.Repeat("░", barW-fill) + "]"
	return fmt.Sprintf("%-*s %s %s", labelW, label, bar, fmtShort(value))
}

func fmtShort(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// LineChart renders a simple ASCII area chart.
func LineChart(values []int, width, height int, prefix string) []string {
	if len(values) == 0 {
		return []string{prefix + "(sem dados)"}
	}
	maxV := 1
	for _, v := range values {
		if v > maxV {
			maxV = v
		}
	}
	// Sample to fit width
	n := len(values)
	sampled := make([]int, width)
	step := float64(n) / float64(width)
	for x := 0; x < width; x++ {
		idx := int(float64(x) * step)
		if idx >= n {
			idx = n - 1
		}
		sampled[x] = values[idx]
	}

	rows := make([]string, height)
	for y := 0; y < height; y++ {
		thr := int(float64(maxV) * float64(height-y) / float64(height))
		line := make([]rune, 0, len(prefix)+width)
		line = append(line, []rune(prefix)...)
		for _, v := range sampled {
			if v >= thr {
				line = append(line, '█')
			} else if v >= thr-int(float64(maxV)/float64(height)*1) {
				line = append(line, '▓')
			} else if v >= thr-int(float64(maxV)/float64(height)*2) {
				line = append(line, '▒')
			} else {
				line = append(line, '░')
			}
		}
		rows[y] = string(line)
	}
	return rows
}

// Sparkline renders a compact single-line sparkline.
func Sparkline(values []int, width int) string {
	if len(values) == 0 || width < 1 {
		return ""
	}
	maxV := 1
	for _, v := range values {
		if v > maxV {
			maxV = v
		}
	}
	runes := []rune(" ▁▂▃▄▅▆▇█")
	s := len(runes)
	var sb strings.Builder
	step := float64(len(values)) / float64(width)
	for i := 0; i < width; i++ {
		idx := int(float64(i) * step)
		if idx >= len(values) {
			idx = len(values) - 1
		}
		ratio := float64(values[idx]) / float64(maxV)
		rIdx := int(ratio*float64(s-1) + 0.5)
		if rIdx >= s {
			rIdx = s - 1
		}
		sb.WriteRune(runes[rIdx])
	}
	return sb.String()
}

func Trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return "…"
	}
	return s[:n-1] + "…"
}

func joinH(lines []string, h int) string {
	for len(lines) < h {
		lines = append(lines, "")
	}
	if len(lines) > h {
		lines = lines[:h]
	}
	return strings.Join(lines, "\n")
}

func minI(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxI(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
