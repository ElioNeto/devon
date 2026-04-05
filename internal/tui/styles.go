package tui

import "github.com/charmbracelet/lipgloss"

// Palette
const (
	colorBg         = lipgloss.Color("#0d1117")
	colorSurface    = lipgloss.Color("#161b22")
	colorSurface2   = lipgloss.Color("#21262d")
	colorBorder     = lipgloss.Color("#30363d")
	colorBorderFocus = lipgloss.Color("#388bfd")
	colorText       = lipgloss.Color("#e6edf3")
	colorMuted      = lipgloss.Color("#8b949e")
	colorFaint      = lipgloss.Color("#484f58")
	colorPrimary    = lipgloss.Color("#388bfd")
	colorGreen      = lipgloss.Color("#3fb950")
	colorYellow     = lipgloss.Color("#d29922")
	colorRed        = lipgloss.Color("#f85149")
	colorOrange     = lipgloss.Color("#e3b341")
	colorPurple     = lipgloss.Color("#bc8cff")
	colorCyan       = lipgloss.Color("#39d353")
)

type uiStyles struct {
	// Panels
	panelBase    lipgloss.Style
	panelFocused lipgloss.Style
	panelTitle   lipgloss.Style

	// Status bar
	statusBar    lipgloss.Style
	statusKey    lipgloss.Style
	statusVal    lipgloss.Style
	statusSep    lipgloss.Style

	// Left panel items
	itemNormal   lipgloss.Style
	itemSelected lipgloss.Style
	itemSection  lipgloss.Style

	// Tool statuses
	toolRunning  lipgloss.Style
	toolDone     lipgloss.Style
	toolError    lipgloss.Style

	// Chat
	userMsg      lipgloss.Style
	agentMsg     lipgloss.Style
	sysMsg       lipgloss.Style
	errMsg       lipgloss.Style

	// Input
	inputBar     lipgloss.Style
	inputPrompt  lipgloss.Style
	cursorStyle  lipgloss.Style

	// Diff
	diffAdd      lipgloss.Style
	diffDel      lipgloss.Style
	diffHunk     lipgloss.Style

	// Context menu
	menuStyle    lipgloss.Style
	menuItem     lipgloss.Style
	menuSelected lipgloss.Style

	// Chart
	chartBar     lipgloss.Style
	chartLabel   lipgloss.Style
	chartValue   lipgloss.Style

	// Misc
	helpStyle    lipgloss.Style
	keyStyle     lipgloss.Style
	badge        lipgloss.Style
}

func newUIStyles() uiStyles {
	s := uiStyles{}

	s.panelBase = lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(colorBorder)

	s.panelFocused = lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(colorPrimary)

	s.panelTitle = lipgloss.NewStyle().
		Foreground(colorMuted).
		PaddingLeft(1)

	s.statusBar = lipgloss.NewStyle().
		Background(colorSurface).
		Foreground(colorText).
		PaddingLeft(1).PaddingRight(1)

	s.statusKey = lipgloss.NewStyle().
		Foreground(colorMuted)

	s.statusVal = lipgloss.NewStyle().
		Foreground(colorText)

	s.statusSep = lipgloss.NewStyle().
		Foreground(colorFaint)

	s.itemNormal = lipgloss.NewStyle().
		Foreground(colorMuted).
		PaddingLeft(1)

	s.itemSelected = lipgloss.NewStyle().
		Foreground(colorText).
		Background(colorSurface2).
		PaddingLeft(1)

	s.itemSection = lipgloss.NewStyle().
		Foreground(colorFaint).
		PaddingLeft(1).
		MarginTop(1)

	s.toolRunning = lipgloss.NewStyle().Foreground(colorYellow)
	s.toolDone = lipgloss.NewStyle().Foreground(colorGreen)
	s.toolError = lipgloss.NewStyle().Foreground(colorRed)

	s.userMsg = lipgloss.NewStyle().Foreground(colorPrimary).PaddingLeft(1)
	s.agentMsg = lipgloss.NewStyle().Foreground(colorText).PaddingLeft(1)
	s.sysMsg = lipgloss.NewStyle().Foreground(colorMuted).Italic(true).PaddingLeft(1)
	s.errMsg = lipgloss.NewStyle().Foreground(colorRed).PaddingLeft(1)

	s.inputBar = lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderTop(true).
		BorderForeground(colorBorder)

	s.inputPrompt = lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
	s.cursorStyle = lipgloss.NewStyle().Foreground(colorPrimary)

	s.diffAdd = lipgloss.NewStyle().Foreground(colorGreen)
	s.diffDel = lipgloss.NewStyle().Foreground(colorRed)
	s.diffHunk = lipgloss.NewStyle().Foreground(colorCyan)

	s.menuStyle = lipgloss.NewStyle().
		Background(colorSurface2).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(colorPrimary).
		Padding(0, 1)

	s.menuItem = lipgloss.NewStyle().Foreground(colorMuted).PaddingLeft(1)
	s.menuSelected = lipgloss.NewStyle().Foreground(colorText).Background(colorSurface).PaddingLeft(1)

	s.chartBar = lipgloss.NewStyle().Foreground(colorPrimary)
	s.chartLabel = lipgloss.NewStyle().Foreground(colorMuted)
	s.chartValue = lipgloss.NewStyle().Foreground(colorText)

	s.helpStyle = lipgloss.NewStyle().Foreground(colorFaint)
	s.keyStyle = lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)

	s.badge = lipgloss.NewStyle().
		Foreground(colorBg).
		Background(colorPrimary).
		PaddingLeft(1).PaddingRight(1)

	return s
}
