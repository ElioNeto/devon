package tui

import "fmt"

// leftPanelSection define as seções do painel esquerdo.
type leftPanelSection int

const (
	SectionSession leftPanelSection = iota
	SectionHistory
	SectionTools
	SectionTokens
)

func (s leftPanelSection) String() string {
	return [...]string{"Sessao", "Historico", "Ferramentas", "Tokens"}[s]
}

// Layout dimensions calculadas dinamicamente.
type layout struct {
	width        int
	height       int
	headerH      int
	separatorH   int
	inputH       int
	statusBarH   int
	chatH        int // altura da área de chat

	// Painéis multi-painel
	leftPanelW  int
	rightPanelW int
	panelSepH   int

	// Foco
	leftFocus  leftPanelSection // aba esquerda focada
	rightFocus rightPanel       // painel direito ativo
	panelFocus panelFocus       // qual painel tem foco
}

type panelFocus int

const (
	FocusLeft panelFocus = iota
	FocusRight
	FocusInput
)

type rightPanel = string

const (
	RightChat    rightPanel = "chat"
	RightSession rightPanel = "session"
	RightHistory rightPanel = "history"
	RightTools   rightPanel = "tools"
	RightTokens  rightPanel = "tokens"
	RightMemory  rightPanel = "memory"
	RightContext rightPanel = "context" // contexto do projeto
)

func calcLayout(w, h int) layout {
	l := layout{width: w, height: h}
	l.headerH = 1
	l.separatorH = 1
	l.inputH = 1
	l.statusBarH = 1

	// Altura do chat = altura total - header - separator - input
	l.chatH = h - l.headerH - l.separatorH - l.inputH - l.statusBarH
	if l.chatH < 1 {
		l.chatH = 1
	}

	l.leftPanelW = w / 4
	if l.leftPanelW < 20 {
		l.leftPanelW = 20
	}
	if l.leftPanelW > 40 {
		l.leftPanelW = 40
	}
	l.rightPanelW = w - l.leftPanelW
	l.panelSepH = h - l.inputH - l.statusBarH

	l.leftFocus = SectionSession
	l.rightFocus = RightChat

	return l
}

func (l layout) HeaderBar() string {
	return "" // delega ao renderHeader
}

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

func (l layout) Separator(w int) string {
	sep := "─"
	if w > 0 {
		return sep + "─" + sep
	}
	return sep
}

// BarChartASCII desenha um gráfico simples de barras horizontais.
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
		n := len(r.Label)
		if n > labelW {
			labelW = n
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
	return formatList(lines)
}

type BarRow struct {
	Label string
	Value int // tamanho da barra
	Meta  string // info extra à direita
}

func formatList(lines []string) string {
	s := ""
	for i, line := range lines {
		if i > 0 {
			s += "\n"
		}
		s += line
	}
	return s
}

// Sparkline retorna uma mini linha de sparkline ASCII a partir de valores.
func Sparkline(values []int, width int) string {
	if len(values) == 0 || width < 2 {
		return ""
	}
	max := 1
	for _, v := range values {
		if v > max {
			max = v
		}
	}
	steps := []string{" ", "▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}
	result := ""
	step := float64(len(values)) / float64(width)
	for i := 0; i < width; i++ {
		idx := int(float64(i) * step)
		if idx >= len(values) {
			idx = len(values) - 1
		}
		ratio := float64(values[idx]) / float64(max)
		stepIdx := int(ratio * float64(len(steps)-1))
		if stepIdx >= len(steps) {
			stepIdx = len(steps) - 1
		}
		result += steps[stepIdx]
	}
	return result
}
