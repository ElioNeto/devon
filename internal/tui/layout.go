package tui

import "fmt"

// layout dimensions calculadas dinamicamente.
type layout struct {
	width        int
	height       int
	headerH      int
	separatorH   int
	inputH       int
	statusBarH   int
	chatH        int // altura da área de chat

	leftPanelW  int
	rightPanelW int
	panelSepH   int
}

func calcLayout(w, h int) layout {
	l := layout{width: w, height: h}
	l.headerH = 1
	l.separatorH = 1
	l.inputH = 1
	l.statusBarH = 1
	l.chatH = h - l.headerH - l.separatorH - l.inputH - l.statusBarH
	if l.chatH < 1 {
		l.chatH = 1
	}

	l.leftPanelW = w / 3
	if l.leftPanelW < 25 {
		l.leftPanelW = 25
	}
	if l.leftPanelW > 45 {
		l.leftPanelW = 45
	}
	l.rightPanelW = w - l.leftPanelW
	l.panelSepH = h - l.inputH - l.statusBarH
	return l
}

// BarChartASCII desenha linhas de barra simples.
func (l layout) BarChartASCII(rows []BarRow, maxWidth int) string {
	if len(rows) == 0 {
		return "(sem dados)"
	}
	maxVal := 1
	for _, r := range rows {
		if r.Value > maxVal {
			maxVal = r.Value
		}
	}
	labelW := 0
	for _, r := range rows {
		if len(r.Label) > labelW {
			labelW = len(r.Label)
		}
	}
	if labelW < 6 {
		labelW = 6
	}

	var lines []string
	for _, r := range rows {
		barW := 0
		if maxVal > 0 {
			barW = int(float64(r.Value) / float64(maxVal) * float64(maxWidth))
		}
		if barW < 1 && r.Value > 0 {
			barW = 1
		}
		bar := ""
		for i := 0; i < barW; i++ {
			bar += "█"
		}
		line := fmt.Sprintf("%-*s %s  %s", labelW, r.Label, bar, r.Meta)
		lines = append(lines, line)
	}
	s := ""
	for i, line := range lines {
		if i > 0 {
			s += "\n"
		}
		s += line
	}
	return s
}

type BarRow struct {
	Label string
	Value int
	Meta  string
}

// ProgressBar returns a simple text progress bar.
func (l layout) ProgressBar(pct float64, width int) string {
	if width < 2 {
		return ""
	}
	filled := int(float64(width-2) * pct)
	if filled < 0 {
		filled = 0
	}
	if filled > width-2 {
		filled = width - 2
	}
	bar := "["
	for i := 0; i < filled; i++ {
		bar += "█"
	}
	for i := filled; i < width-2; i++ {
		bar += "░"
	}
	bar += "]"
	return bar
}
