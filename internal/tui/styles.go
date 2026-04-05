package tui

import "github.com/charmbracelet/lipgloss"

// Palette — lazydocker-inspired dark theme
const (
	// Backgrounds
	colorBg       = lipgloss.Color("#0d1117")
	colorSurface  = lipgloss.Color("#161b22")
	colorSurface2 = lipgloss.Color("#21262d")

	// Borders
	colorBorder      = lipgloss.Color("#30363d")
	colorBorderFocus = lipgloss.Color("#388bfd")
	colorBorderGreen = lipgloss.Color("#2ea043")

	// Text
	colorText  = lipgloss.Color("#e6edf3")
	colorMuted = lipgloss.Color("#8b949e")
	colorFaint = lipgloss.Color("#484f58")

	// Accents
	colorPrimary = lipgloss.Color("#388bfd")
	colorGreen   = lipgloss.Color("#3fb950")
	colorYellow  = lipgloss.Color("#d29922")
	colorRed     = lipgloss.Color("#f85149")
	colorOrange  = lipgloss.Color("#e3b341")
	colorCyan    = lipgloss.Color("#39d353")
)

type uiStyles struct {
	// Box panels (NormalBorder)
	panelBase    lipgloss.Style
	panelFocused lipgloss.Style
	panelTitle   lipgloss.Style

	// Status bar (bottom)
	statusBar lipgloss.Style
	statusKey lipgloss.Style
	statusVal lipgloss.Style
	statusSep lipgloss.Style

	// Left panel list items
	itemNormal   lipgloss.Style
	itemSelected lipgloss.Style
	itemSection  lipgloss.Style

	// Service status badges
	statusRunning lipgloss.Style
	statusExited  lipgloss.Style
	statusOther   lipgloss.Style

	// Tool call statuses
	toolRunning lipgloss.Style
	toolDone    lipgloss.Style
	toolError   lipgloss.Style

	// Chat messages
	userMsg  lipgloss.Style
	agentMsg lipgloss.Style
	sysMsg   lipgloss.Style
	errMsg   lipgloss.Style

	// Input area
	inputBar    lipgloss.Style
	inputPrompt lipgloss.Style
	cursorStyle lipgloss.Style

	// Diff viewer
	diffAdd  lipgloss.Style
	diffDel  lipgloss.Style
	diffHunk lipgloss.Style

	// Context menu overlay
	menuStyle    lipgloss.Style
	menuItem     lipgloss.Style
	menuSelected lipgloss.Style

	// Right-panel: config header fields
	configKey lipgloss.Style
	configVal lipgloss.Style

	// Right-panel: layer table header
	tableHeader lipgloss.Style
	tableRow    lipgloss.Style
	tableRowSel lipgloss.Style
	tableRowMissing lipgloss.Style

	// Misc
	helpStyle lipgloss.Style
	keyStyle  lipgloss.Style
	badge     lipgloss.Style
}

func newUIStyles() uiStyles {
	s := uiStyles{}

	// ── Panels ───────────────────────────────────────────────
	s.panelBase = lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(colorBorder)

	s.panelFocused = lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(colorBorderGreen)

	s.panelTitle = lipgloss.NewStyle().
		Foreground(colorMuted).
		PaddingLeft(1)

	// ── Status bar ───────────────────────────────────────────
	s.statusBar = lipgloss.NewStyle().
		Background(colorSurface).
		Foreground(colorText).
		PaddingLeft(1).PaddingRight(1)

	s.statusKey = lipgloss.NewStyle().Foreground(colorMuted)
	s.statusVal = lipgloss.NewStyle().Foreground(colorText)
	s.statusSep = lipgloss.NewStyle().Foreground(colorFaint)

	// ── Left list ────────────────────────────────────────────
	s.itemNormal = lipgloss.NewStyle().
		Foreground(colorMuted).
		PaddingLeft(1)

	s.itemSelected = lipgloss.NewStyle().
		Foreground(colorText).
		Background(colorSurface2).
		PaddingLeft(1)

	s.itemSection = lipgloss.NewStyle().
		Foreground(colorBorderGreen).
		Bold(true).
		PaddingLeft(1)

	// ── Service status ───────────────────────────────────────
	s.statusRunning = lipgloss.NewStyle().Foreground(colorGreen).Bold(true)
	s.statusExited = lipgloss.NewStyle().Foreground(colorRed).Bold(true)
	s.statusOther = lipgloss.NewStyle().Foreground(colorYellow).Bold(true)

	// ── Tool calls ───────────────────────────────────────────
	s.toolRunning = lipgloss.NewStyle().Foreground(colorYellow)
	s.toolDone = lipgloss.NewStyle().Foreground(colorGreen)
	s.toolError = lipgloss.NewStyle().Foreground(colorRed)

	// ── Chat ────────────────────────────────────────────────
	s.userMsg = lipgloss.NewStyle().Foreground(colorPrimary).PaddingLeft(1)
	s.agentMsg = lipgloss.NewStyle().Foreground(colorText).PaddingLeft(1)
	s.sysMsg = lipgloss.NewStyle().Foreground(colorMuted).Italic(true).PaddingLeft(1)
	s.errMsg = lipgloss.NewStyle().Foreground(colorRed).PaddingLeft(1)

	// ── Input ────────────────────────────────────────────────
	s.inputBar = lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderTop(true).
		BorderForeground(colorBorder)

	s.inputPrompt = lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
	s.cursorStyle = lipgloss.NewStyle().Foreground(colorPrimary)

	// ── Diff ────────────────────────────────────────────────
	s.diffAdd = lipgloss.NewStyle().Foreground(colorGreen)
	s.diffDel = lipgloss.NewStyle().Foreground(colorRed)
	s.diffHunk = lipgloss.NewStyle().Foreground(colorCyan)

	// ── Context menu ────────────────────────────────────────
	s.menuStyle = lipgloss.NewStyle().
		Background(colorSurface2).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(colorPrimary).
		Padding(0, 1)

	s.menuItem = lipgloss.NewStyle().Foreground(colorMuted).PaddingLeft(1)
	s.menuSelected = lipgloss.NewStyle().Foreground(colorText).Background(colorSurface).PaddingLeft(1)

	// ── Right panel: Config header ───────────────────────────
	s.configKey = lipgloss.NewStyle().Foreground(colorMuted)
	s.configVal = lipgloss.NewStyle().Foreground(colorGreen)

	// ── Right panel: Layer table ─────────────────────────────
	s.tableHeader = lipgloss.NewStyle().
		Foreground(colorText).
		Bold(true).
		PaddingLeft(1)

	s.tableRow = lipgloss.NewStyle().
		Foreground(colorMuted).
		PaddingLeft(1)

	s.tableRowSel = lipgloss.NewStyle().
		Foreground(colorText).
		Background(colorSurface2).
		PaddingLeft(1)

	s.tableRowMissing = lipgloss.NewStyle().
		Foreground(colorFaint).
		PaddingLeft(1)

	// ── Misc ────────────────────────────────────────────────
	s.helpStyle = lipgloss.NewStyle().Foreground(colorFaint)
	s.keyStyle = lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)

	s.badge = lipgloss.NewStyle().
		Foreground(colorBg).
		Background(colorPrimary).
		PaddingLeft(1).PaddingRight(1)

	return s
}
