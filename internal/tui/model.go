// Package tui implementa a interface com Bubble Tea para o Devon.
package tui

import (
	"context"

	"path/filepath"

	"github.com/ElioNeto/devon/internal/agent"
	"github.com/ElioNeto/devon/internal/config"
	"github.com/ElioNeto/devon/internal/cost"
	"github.com/ElioNeto/devon/internal/db"
	"github.com/ElioNeto/devon/internal/history"
	"github.com/ElioNeto/devon/internal/llm"
	"github.com/ElioNeto/devon/internal/memory"
	sessionpkg "github.com/ElioNeto/devon/internal/session"
	"github.com/ElioNeto/devon/internal/tools"
	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// appModel é o modelo principal do Bubble Tea.
type appModel struct {
	width  int
	height int

	cfg     *config.Config
	agent   *agent.Agent
	session *history.Session
	tracker *cost.Session
	styles  uiStyles
	spinner spinner.Model

	// Left panel
	leftItems  []leftItem
	leftCursor int
	leftFocus  bool

	// Right panel
	rightView    rightView
	rightScroll  int
	expandedView bool

	// Log stream (right panel — Logs tab)
	logEvents []logEvent

	// Diff content (right panel — Diff tab)
	lastDiff string

	// Messages (Steps tab fallback)
	messages []chatMessage
	toolRuns []toolRun
	running  bool
	cancel   context.CancelFunc
	showHelp bool
	popup    string
	layout   layout

	// Current task label (shown in Tasks while running)
	currentTask string

	// Pending tasks (queue)
	pendingTasks []pendingTask

	// Navigation
	leftItemCount   int
	selectedTurnIdx int
	selectedTool    *toolRun

	// History turns
	historyTurns []historyTurn

	// Tool stats
	toolStats map[string]*toolStat

	// Memory facts
	memoryFacts []memoryFact

	// File changes
	fileChanges []fileChange

	// Context
	maxContextTokens int

	// Context menu
	ctxMenu    ctxMenuState
	showMenu   bool
	menuCursor int

	// Session slots (Ctrl+1..5 — like Linux workspaces / tmux windows)
	workspaceSlots  [5]workspaceSlot
	activeWorkspace int // 0..4

	// Command menu (! — shows available commands in a sidebar)
	showCmdMenu   bool
	cmdMenuCursor int
	cmdMenuFilter string

	// Confirmation prompt (inline permission check)
	confirm confirmState

	// Input
	input     string
	cursor    int
	scroll    int
	statusMsg string
	inputHist inputHistory

	// Multi-line input bar
	multilineRows int // number of visible rows the input bar currently has

	// Token history per turn
	tokenPerTurn []int

	// Attachments (multimodal input)
	attachments   []Attachment
	showFilePicker bool
	fp             filepicker.Model

	// DB store and session manager (for persistence and picker)
	dbStore   db.Store
	sessionMgr *sessionpkg.Manager

	// Session picker overlay
	picker sessionPickerState
}

type chatMessage struct {
	Sender  string
	Content string
	IsError bool
}

type toolRun struct {
	Name       string
	Args       string
	Result     string
	Status     string // "running"|"done"|"error"
	DurationMs int
	StartedAt  int64
}

type toolStat struct {
	Calls int
	AvgMs int64
	MaxMs int64
}

type historyTurn struct {
	UserPrompt       string
	AgentReply       string
	ToolSummary      string
	Timestamp        string
	Elapsed          int64
	PromptTokens     int
	CompletionTokens int
}

type memoryFact struct {
	Category string
	Key      string
	Value    string
}

type pendingTask struct {
	Label  string
	Status string // "waiting"|"pending"|"error"
	Meta   string
}

// ── Initialization ────────────────────────────────────────────────────────────

func newModel(cfg *config.Config, registry *tools.Registry) appModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(colorPrimary)

	// If registry is nil, create a new one (fallback)
	if registry == nil {
		registry = tools.NewRegistry()
	}

	// Open DB store for session persistence and picker
	var store db.Store
	if cfg.DBPath != "" {
		dbPath := filepath.Join(cfg.WorkDir, cfg.DBPath)
		s, err := db.New(dbPath)
		if err == nil {
			store = s
		}
	}

	client := llm.New(cfg.APIKey, cfg.BaseURL, cfg.Model, cfg.Timeout)
	mem := memory.New(nil, cfg.WorkDir) // nil store para memória em memória apenas
	agt := agent.New(cfg, client, registry, store, "", mem, cfg.WorkDir)
	tracker := cost.NewSession(cfg.Model)

	maxCtx := 32000

	session, err := history.LoadLastSession(cfg.WorkDir)
	if err != nil {
		session = nil
	}

	// Initialize file picker
	fp := filepicker.New()
	fp.AllowedTypes = allowedImageExts
	fp.CurrentDirectory = cfg.WorkDir
	fp.FileAllowed = true
	fp.DirAllowed = false

	m := appModel{
		cfg:              cfg,
		agent:            agt,
		session:          session,
		tracker:          tracker,
		spinner:          s,
		styles:           newUIStyles(),
		layout:           calcLayout(0, 0),
		rightView:        viewLogs,
		toolStats:        make(map[string]*toolStat),
		selectedTurnIdx:  -1,
		leftFocus:        true,
		maxContextTokens: maxCtx,
		activeWorkspace:  0,
		fp:               fp,
		attachments:      []Attachment{},
		dbStore:          store,
	}

	if store != nil {
		mgr := sessionpkg.NewManager(store)
		m.sessionMgr = mgr
		m.initSessionPicker(mgr)
		// Show picker on startup if there are sessions
		if len(m.picker.sessions) > 0 {
			m.picker.visible = true
		}
	}

	return m
}

// ── Init ──────────────────────────────────────────────────────────────────────

func (m appModel) Init() tea.Cmd {
	welcome := "Devon pronto. ↑↓ navegar · Enter selecionar · Ctrl+X menu · ? ajuda"
	if m.session != nil {
		welcome = "Sessão " + m.session.ID + " carregada."
	}
	return tea.Sequence(
		m.spinner.Tick,
		func() tea.Msg {
			return agentEventMsg(agent.Event{Type: "system", Text: welcome})
		},
	)
}
