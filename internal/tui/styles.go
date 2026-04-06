package tui

import "github.com/charmbracelet/lipgloss"

// ── Palette — fiel à imagem de referência ────────────────────────────────────
const (
	colorBg       = lipgloss.Color("#0d1117")
	colorSurface  = lipgloss.Color("#161b22")
	colorSurface2 = lipgloss.Color("#21262d")

	colorBorder      = lipgloss.Color("#30363d")
	colorBorderFocus = lipgloss.Color("#388bfd")
	colorBorderGreen = lipgloss.Color("#2ea043")

	colorText  = lipgloss.Color("#e6edf3")
	colorMuted = lipgloss.Color("#8b949e")
	colorFaint = lipgloss.Color("#484f58")

	colorPrimary = lipgloss.Color("#388bfd")
	colorGreen   = lipgloss.Color("#3fb950")
	colorYellow  = lipgloss.Color("#d29922")
	colorRed     = lipgloss.Color("#f85149")
	colorOrange  = lipgloss.Color("#e3b341")
	colorCyan    = lipgloss.Color("#39d353")
)

type uiStyles struct {
	// Panels
	panelBase    lipgloss.Style
	panelFocused lipgloss.Style
	panelTitle   lipgloss.Style

	// Status bar
	statusBar lipgloss.Style
	statusKey lipgloss.Style
	statusVal lipgloss.Style
	statusSep lipgloss.Style

	// Left panel list
	itemNormal   lipgloss.Style
	itemSelected lipgloss.Style
	itemSection  lipgloss.Style

	// Status badges (left panel)
	statusRunning lipgloss.Style
	statusWaiting lipgloss.Style
	statusDone    lipgloss.Style
	statusPending lipgloss.Style
	statusError   lipgloss.Style
	statusExited  lipgloss.Style
	statusOther   lipgloss.Style

	// Tool statuses
	toolRunning lipgloss.Style
	toolDone    lipgloss.Style
	toolError   lipgloss.Style

	// File change indicators
	fileModified lipgloss.Style
	fileAdded    lipgloss.Style
	fileDeleted  lipgloss.Style
	fileLines    lipgloss.Style

	// Log actors (right panel)
	actorAgent lipgloss.Style
	actorTool  lipgloss.Style
	actorWarn  lipgloss.Style
	actorOk    lipgloss.Style
	actorTs    lipgloss.Style

	// Right panel tabs
	tabActive   lipgloss.Style
	tabInactive lipgloss.Style

	// Chat messages
	userMsg  lipgloss.Style
	agentMsg lipgloss.Style
	sysMsg   lipgloss.Style
	errMsg   lipgloss.Style

	// Input
	inputBar    lipgloss.Style
	inputPrompt lipgloss.Style
	cursorStyle lipgloss.Style

	// Diff
	diffAdd  lipgloss.Style
	diffDel  lipgloss.Style
	diffHunk lipgloss.Style

	// Context menu
	menuStyle    lipgloss.Style
	menuItem     lipgloss.Style
	menuSelected lipgloss.Style

	// Config / table
	configKey   lipgloss.Style
	configVal   lipgloss.Style
	tableHeader lipgloss.Style
	tableRow    lipgloss.Style
	tableRowSel lipgloss.Style
	tableRowMissing lipgloss.Style

	// Progress bar
	progFill  lipgloss.Style
	progEmpty lipgloss.Style

	// Misc
	helpStyle lipgloss.Style
	keyStyle  lipgloss.Style
	badge     lipgloss.Style

	// Confirm badges
	badgeTool    lipgloss.Style
	badgeRead    lipgloss.Style
	badgeWrite   lipgloss.Style
	badgeExecute lipgloss.Style
}

func newUIStyles() uiStyles {
	s := uiStyles{}

	// ── Panels
	s.panelBase = lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(colorBorder)

	s.panelFocused = lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(colorBorderGreen)

	s.panelTitle = lipgloss.NewStyle().
		Foreground(colorMuted).PaddingLeft(1)

	// ── Status bar
	s.statusBar = lipgloss.NewStyle().
		Background(colorSurface).Foreground(colorMuted).
		PaddingLeft(1).PaddingRight(1)

	s.statusKey = lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
	s.statusVal = lipgloss.NewStyle().Foreground(colorMuted)
	s.statusSep = lipgloss.NewStyle().Foreground(colorFaint)

	// ── Left list
	s.itemNormal = lipgloss.NewStyle().Foreground(colorMuted).PaddingLeft(1)
	s.itemSelected = lipgloss.NewStyle().
		Foreground(colorText).Background(colorSurface2).PaddingLeft(1)
	s.itemSection = lipgloss.NewStyle().
		Foreground(colorBorderGreen).Bold(true)

	// ── Status badges
	s.statusRunning = lipgloss.NewStyle().Foreground(colorGreen).Bold(true)
	s.statusWaiting = lipgloss.NewStyle().Foreground(colorMuted)
	s.statusDone    = lipgloss.NewStyle().Foreground(colorFaint)
	s.statusPending = lipgloss.NewStyle().Foreground(colorYellow)
	s.statusError   = lipgloss.NewStyle().Foreground(colorRed).Bold(true)
	s.statusExited  = lipgloss.NewStyle().Foreground(colorRed).Bold(true)
	s.statusOther   = lipgloss.NewStyle().Foreground(colorYellow)

	// ── Tool statuses
	s.toolRunning = lipgloss.NewStyle().Foreground(colorYellow)
	s.toolDone    = lipgloss.NewStyle().Foreground(colorGreen)
	s.toolError   = lipgloss.NewStyle().Foreground(colorRed)

	// ── File change indicators
	s.fileModified = lipgloss.NewStyle().Foreground(colorYellow).Bold(true)
	s.fileAdded    = lipgloss.NewStyle().Foreground(colorGreen).Bold(true)
	s.fileDeleted  = lipgloss.NewStyle().Foreground(colorRed).Bold(true)
	s.fileLines    = lipgloss.NewStyle().Foreground(colorMuted)

	// ── Log actors
	s.actorAgent = lipgloss.NewStyle().Foreground(colorCyan)
	s.actorTool  = lipgloss.NewStyle().Foreground(colorYellow)
	s.actorWarn  = lipgloss.NewStyle().Foreground(colorRed)
	s.actorOk    = lipgloss.NewStyle().Foreground(colorGreen)
	s.actorTs    = lipgloss.NewStyle().Foreground(colorFaint)

	// ── Tabs
	s.tabActive   = lipgloss.NewStyle().Foreground(colorText).Underline(true).Bold(true)
	s.tabInactive = lipgloss.NewStyle().Foreground(colorMuted)

	// ── Chat
	s.userMsg  = lipgloss.NewStyle().Foreground(colorPrimary).PaddingLeft(1)
	s.agentMsg = lipgloss.NewStyle().Foreground(colorText).PaddingLeft(1)
	s.sysMsg   = lipgloss.NewStyle().Foreground(colorMuted).Italic(true).PaddingLeft(1)
	s.errMsg   = lipgloss.NewStyle().Foreground(colorRed).PaddingLeft(1)

	// ── Input
	s.inputBar = lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderTop(true).
		BorderForeground(colorBorder)
	s.inputPrompt = lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
	s.cursorStyle = lipgloss.NewStyle().Foreground(colorPrimary)

	// ── Diff
	s.diffAdd  = lipgloss.NewStyle().Foreground(colorGreen)
	s.diffDel  = lipgloss.NewStyle().Foreground(colorRed)
	s.diffHunk = lipgloss.NewStyle().Foreground(colorCyan)

	// ── Context menu
	s.menuStyle = lipgloss.NewStyle().
		Background(colorSurface2).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(colorPrimary).
		Padding(0, 1)
	s.menuItem     = lipgloss.NewStyle().Foreground(colorMuted).PaddingLeft(1)
	s.menuSelected = lipgloss.NewStyle().Foreground(colorText).Background(colorSurface).PaddingLeft(1)

	// ── Config / table
	s.configKey   = lipgloss.NewStyle().Foreground(colorMuted)
	s.configVal   = lipgloss.NewStyle().Foreground(colorGreen)
	s.tableHeader = lipgloss.NewStyle().Foreground(colorText).Bold(true).PaddingLeft(1)
	s.tableRow    = lipgloss.NewStyle().Foreground(colorMuted).PaddingLeft(1)
	s.tableRowSel = lipgloss.NewStyle().Foreground(colorText).Background(colorSurface2).PaddingLeft(1)
	s.tableRowMissing = lipgloss.NewStyle().Foreground(colorFaint).PaddingLeft(1)

	// ── Progress bar
	s.progFill  = lipgloss.NewStyle().Foreground(colorGreen)
	s.progEmpty = lipgloss.NewStyle().Foreground(colorFaint)

	// ── Misc
	s.helpStyle = lipgloss.NewStyle().Foreground(colorFaint)
	s.keyStyle  = lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
	s.badge = lipgloss.NewStyle().
		Foreground(colorBg).Background(colorPrimary).
		PaddingLeft(1).PaddingRight(1)

	// ── Confirm badges
	s.badgeTool = lipgloss.NewStyle().
		Foreground(colorBg).Background(colorPrimary).
		PaddingLeft(1).PaddingRight(1)
	s.badgeRead = lipgloss.NewStyle().
		Foreground(colorMuted)
	s.badgeWrite = lipgloss.NewStyle().
		Foreground(colorBg).Background(colorYellow).
		PaddingLeft(1).PaddingRight(1)
	s.badgeExecute = lipgloss.NewStyle().
		Foreground(colorBg).Background(colorRed).
		PaddingLeft(1).PaddingRight(1)

	return s
}
