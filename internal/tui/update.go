// Package tui — Update loop and top-level key dispatch.
package tui

import (
	"github.com/ElioNeto/devon/internal/agent"
	"github.com/ElioNeto/devon/internal/cost"
	"github.com/ElioNeto/devon/internal/history"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
)

// ── Update ────────────────────────────────────────────────────────────────────

func (m *appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		if m.ctxMenu.visible && key.Type == tea.KeyEscape {
			m.ctxMenu.close()
			return m, nil
		}
		if m.popup != "" {
			m.popup = ""
			return m, nil
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.layout = calcLayout(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		if m.showCmdMenu {
			switch msg.String() {
			case "esc", "!":
				m.showCmdMenu = false
				m.cmdMenuCursor = 0
			case "enter":
				m.execCmdMenuAction()
			case "up":
				if m.cmdMenuCursor > 0 {
					m.cmdMenuCursor--
				}
			case "down":
				if m.cmdMenuCursor < len(cmdMenuActions)-1 {
					m.cmdMenuCursor++
				}
			}
			return m, nil
		}
		if m.ctxMenu.visible {
			return m.handleCtxMenuKey(msg)
		}
		if m.showMenu {
			return m.updateMenu(msg)
		}
		if m.showHelp {
			m.showHelp = false
			return m, nil
		}
		return m.handleKey(msg)

	case agentEventMsg:
		m.processAgentEvent(agent.Event(msg))
		return m, m.spinner.Tick

	case agentResult:
		for _, ev := range msg.events {
			m.processAgentEvent(ev)
		}
		if m.session != nil {
			_ = history.SaveMessages(m.cfg.WorkDir, m.session.ID, m.agentMessages(), &m.session.Usage)
		}
		total := m.tracker.TotalInputTokens + m.tracker.TotalOutputTokens
		m.tokenPerTurn = append(m.tokenPerTurn, total)
		if m.cancel != nil {
			m.cancel()
			m.cancel = nil
		}
		m.running = false
		m.toolRuns = nil
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

// ── Key dispatch ──────────────────────────────────────────────────────────────

func (m *appModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		if m.running && m.cancel != nil {
			m.cancel()
			m.running = false
			m.toolRuns = nil
			m.appendLog("system", "Agente interrompido.", "")
			return m, nil
		}
		return m, tea.Quit

	case "ctrl+l":
		m.messages = nil
		m.toolRuns = nil
		m.logEvents = nil
		m.scroll = 0
		return m, nil

	case "ctrl+k":
		m.messages = nil
		m.toolRuns = nil
		m.logEvents = nil
		m.fileChanges = nil
		m.scroll = 0
		m.tracker = cost.NewSession(m.cfg.Model)
		m.tokenPerTurn = nil
		var err error
		m.session, err = history.CreateSession(m.cfg.WorkDir)
		if err != nil {
			m.messages = append(m.messages, chatMessage{Sender: "system", Content: "Erro: " + err.Error()})
			m.appendLog("system", "Erro ao criar sessão: "+err.Error(), "")
		} else {
			m.messages = append(m.messages, chatMessage{Sender: "system", Content: "Nova sessão " + m.session.ID})
			m.appendLog("system", "Nova sessão "+m.session.ID, "")
		}
		return m, nil

	case "tab":
		m.cycleSection()
		return m, nil

	case "shift+tab":
		m.cycleSectionBack()
		return m, nil

	case "up":
		if m.leftFocus {
			m.navigateLeft(-1)
		} else if m.rightScroll > 0 {
			m.rightScroll--
		}
		return m, nil

	case "down":
		if m.leftFocus {
			m.navigateLeft(1)
		} else {
			m.rightScroll++
		}
		return m, nil

	case "left":
		if len([]rune(m.input)) > 0 && m.cursor > 0 {
			m.cursor--
		} else if m.leftFocus {
			m.navigateLeft(-1)
		} else {
			m.leftFocus = true
		}
		return m, nil

	case "right":
		if m.leftFocus {
			m.leftFocus = false
		} else {
			if m.cursor < len([]rune(m.input)) {
				m.cursor++
			}
		}
		return m, nil

	case "pgup":
		if m.rightScroll > 0 {
			m.rightScroll -= 5
			if m.rightScroll < 0 {
				m.rightScroll = 0
			}
		}
		return m, nil

	case "pgdown":
		m.rightScroll += 5
		return m, nil

	case "!":
		m.toggleCmdMenu()
		return m, nil

	case "enter":
		if m.showCmdMenu {
			m.execCmdMenuAction()
			return m, nil
		}
		if m.leftFocus {
			m.selectLeftItem()
			return m, nil
		}
		return m.sendInput()

	case KeyExpand: // Ctrl+E — toggle expanded
		m.expandedView = !m.expandedView
		return m, nil

	case KeySession2: // Ctrl+2 — workspace 1
		m.switchWorkspace(0)
		return m, nil
	case KeySession4: // Ctrl+4 — workspace 2
		m.switchWorkspace(1)
		return m, nil
	case KeySession5: // Ctrl+5 — workspace 3
		m.switchWorkspace(2)
		return m, nil

	case "?", "ctrl+h":
		m.showHelp = true
		return m, nil

	case "esc":
		m.showMenu = false
		m.popup = ""
		m.showHelp = false
		m.showCmdMenu = false
		m.cmdMenuCursor = 0
		return m, nil

	case "backspace":
		if m.cursor > 0 {
			m.deleteCharBefore()
		}
		return m, nil

	case "ctrl+u":
		m.input = ""
		m.cursor = 0
		return m, nil

	case "ctrl+w":
		m.deleteWord()
		return m, nil

	case " ":
		m.insertRune(' ')
		return m, nil

	case "home":
		m.cursor = 0
		return m, nil

	case "end":
		m.cursor = len([]rune(m.input))
		return m, nil

	default:
		if msg.Type == tea.KeyRunes {
			for _, r := range msg.Runes {
				m.insertRune(r)
			}
		}
	}
	return m, nil
}

// ── Menu update ───────────────────────────────────────────────────────────────

func (m *appModel) updateMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	actions := contextMenuFor(m)
	switch msg.String() {
	case "up":
		if m.menuCursor > 0 {
			m.menuCursor--
		}
	case "down":
		if m.menuCursor < len(actions)-1 {
			m.menuCursor++
		}
	case "enter":
		if m.menuCursor < len(actions) {
			actions[m.menuCursor].Action(m)
		}
		m.showMenu = false
	case "esc", "q", "x":
		m.showMenu = false
	default:
		for _, a := range actions {
			if a.Key == msg.String() {
				a.Action(m)
				m.showMenu = false
				break
			}
		}
	}
	return m, nil
}

// ── View ──────────────────────────────────────────────────────────────────────

func (m appModel) View() string {
	if m.height == 0 || m.width == 0 {
		return "Iniciando Devon..."
	}

	l := m.layout
	if l.width == 0 {
		l = calcLayout(m.width, m.height)
	}

	statusBarH := 1
	inputH := 3
	panelH := m.height - statusBarH - inputH
	if panelH < 5 {
		panelH = 5
	}

	leftW := l.leftPanelW
	if leftW <= 0 {
		leftW = m.width / 3
	}
	if leftW < 24 {
		leftW = 24
	}
	rightW := m.width - leftW
	if rightW < 20 {
		rightW = 20
	}

	leftPanel := renderLeftPanel(&m, leftW, panelH, m.leftFocus)
	rightPanel := renderRightPanel(&m, rightW, panelH, !m.leftFocus)
	panels := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)

	inputBar := renderInputBar(&m, m.width)
	statusBar := renderStatusBar(&m, m.width)

	view := lipgloss.JoinVertical(lipgloss.Left, panels, inputBar, statusBar)

	if m.showHelp {
		view = overlayCenter(view, renderHelp(&m, m.width), m.width, m.height)
	}
	if m.popup != "" {
		popupView := m.styles.menuStyle.Width(min(m.width-8, 70)).Render(m.popup)
		view = overlayCenter(view, popupView, m.width, m.height)
	}
	if m.ctxMenu.visible {
		view += "\n" + m.ctxMenu.render(m.width)
	}
	if m.showCmdMenu {
		cmdView := renderCmdMenuOverlay(&m, m.width)
		view = overlayCenter(view, cmdView, m.width, m.height)
	}

	return view
}

func overlayCenter(base, overlay string, _, _ int) string {
	return base + "\n\n" + overlay
}
