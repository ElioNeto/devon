// Package tui — Update loop and top-level key dispatch.
package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ElioNeto/devon/internal/agent"
	"github.com/ElioNeto/devon/internal/cost"
	"github.com/ElioNeto/devon/internal/history"
	"github.com/ElioNeto/devon/internal/llm"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
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
		if m.showFilePicker {
			var cmd tea.Cmd
			m.fp, cmd = m.fp.Update(msg)
			return m, cmd
		}
		return m, nil

	case tea.KeyMsg:
		if m.picker.visible && !m.picker.done {
			return m.handleSessionPickerKey(msg)
		}
		if m.showFilePicker {
			// Forward key events to file picker
			var cmd tea.Cmd
			m.fp, cmd = m.fp.Update(msg)
			// Check if a file was selected
			if selected, path := m.fp.DidSelectFile(msg); selected && path != "" {
				if err := m.attachFile(path); err != nil {
					m.appendLog("warn", "Erro ao anexar imagem: "+err.Error(), "")
				} else {
					att := m.attachments[len(m.attachments)-1]
					m.appendLog("agent", "Imagem anexada: "+att.Filename, fmt.Sprintf("%dKB", att.SizeKB))
				}
				m.showFilePicker = false
			}
			if selected, path := m.fp.DidSelectDisabledFile(msg); selected && path != "" {
				m.appendLog("warn", "Tipo de arquivo não suportado: "+filepath.Base(path), "permitidos: png, jpg, jpeg, gif, webp")
				m.showFilePicker = false
			}
			return m, cmd
		}
		if m.confirm.visible {
			return m.handleConfirmKey(msg)
		}
		if m.showCmdMenu {
			switch msg.String() {
			case "esc", "!":
				m.showCmdMenu = false
				m.cmdMenuCursor = 0
				m.cmdMenuFilter = ""
			case "enter":
				if filtered := m.filteredCmdMenuActions(); len(filtered) > 0 && m.cmdMenuCursor < len(filtered) {
					filtered[m.cmdMenuCursor].Action(m)
				}
				m.showCmdMenu = false
				m.cmdMenuCursor = 0
				m.cmdMenuFilter = ""
			case "up":
				if m.cmdMenuCursor > 0 {
					m.cmdMenuCursor--
				}
			case "down":
				if m.cmdMenuCursor < len(m.filteredCmdMenuActions())-1 {
					m.cmdMenuCursor++
				}
			case "backspace":
				if len(m.cmdMenuFilter) > 0 {
					runes := []rune(m.cmdMenuFilter)
					m.cmdMenuFilter = string(runes[:len(runes)-1])
					m.cmdMenuCursor = 0
				}
			case "ctrl+u":
				m.cmdMenuFilter = ""
				m.cmdMenuCursor = 0
			default:
				if msg.Type == tea.KeyRunes {
					m.cmdMenuFilter += string(msg.Runes)
					m.cmdMenuCursor = 0
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

	case tea.MouseMsg:
		return m.handleMouse(msg)
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
		} else if m.inputHist.entries != nil {
			if hist := m.inputHist.navigateUp(); hist != "" {
				m.input = hist
				m.cursor = len([]rune(m.input))
			}
		} else if m.rightScroll > 0 {
			m.rightScroll--
		}
		return m, nil

	case "down":
		if m.leftFocus {
			m.navigateLeft(1)
		} else if m.inputHist.entries != nil {
			if hist := m.inputHist.navigateDown(); hist != "" {
				m.input = hist
				m.cursor = len([]rune(m.input))
			}
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

	case "?":
		if m.input == "" {
			m.showHelp = true
			return m, nil
		}
		m.insertRune('?')
		return m, nil

	case "!":
		m.toggleCmdMenu()
		return m, nil

	case "enter":
		if m.showCmdMenu {
			m.execCmdMenuAction()
			return m, nil
		}
		if m.leftFocus && m.input == "" {
			m.selectLeftItem()
			return m, nil
		}
		if strings.HasPrefix(m.input, "/") && !strings.Contains(m.input, "\n") {
			return m.handleSlashCommand(m.input)
		}
		return m.sendInput()

	case "shift+enter":
		m.newLine()
		return m, nil

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

	case "ctrl+h":
		m.showHelp = true
		return m, nil

	case KeyAttachImage:
		if m.running {
			return m, nil
		}
		m.showFilePicker = true
		fp := m.fp
		cmd := fp.Init()
		m.fp = fp
		return m, cmd

	case KeyRemoveAttach:
		if len(m.attachments) > 0 {
			last := len(m.attachments) - 1
			m.attachments[last].Data = nil
			m.attachments = m.attachments[:last]
		}
		return m, nil

	case "esc":
		if m.showFilePicker {
			m.showFilePicker = false
			return m, nil
		}
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

// ── Slash commands ──────────────────────────────────────────────────────────

func (m *appModel) handleSlashCommand(cmd string) (tea.Model, tea.Cmd) {
	m.input = ""
	m.cursor = 0

	switch {
	case cmd == "/history":
		sessions, err := history.ListSessions(m.cfg.WorkDir)
		if err != nil {
			m.messages = append(m.messages, chatMessage{Sender: "system", Content: "Erro: " + err.Error()})
			return m, nil
		}
		if len(sessions) == 0 {
			m.messages = append(m.messages, chatMessage{Sender: "system", Content: "Nenhuma sessão salva."})
			return m, nil
		}
		var sb strings.Builder
		n := len(sessions)
		if n > 10 {
			n = 10
		}
		sb.WriteString("Sessões anteriores (mais recentes primeiro):\n")
		for i := 0; i < n; i++ {
			id := sessions[len(sessions)-1-i]
			mark := ""
			if m.session != nil && m.session.ID == id {
				mark = " (atual)"
			}
			sb.WriteString(fmt.Sprintf("  %d. %s%s\n", i+1, id, mark))
		}
		sb.WriteString("Use /load <id> para retomar.")
		m.messages = append(m.messages, chatMessage{Sender: "system", Content: sb.String()})

	case strings.HasPrefix(cmd, "/load "):
		id := strings.TrimSpace(strings.TrimPrefix(cmd, "/load"))
		if ses, err := history.LoadSession(m.cfg.WorkDir, id); err == nil {
			m.session = ses
			m.messages = nil
			for _, msg := range ses.Messages {
				var sender string
				switch msg.Role {
				case llm.RoleUser:
					sender = "user"
				case llm.RoleAssistant:
					sender = "devon"
				case llm.RoleTool:
					sender = "system"
				default:
					sender = "system"
				}
				content := ""
				if msg.Content != nil {
					content = *msg.Content
				}
				m.messages = append(m.messages, chatMessage{Sender: sender, Content: content})
			}
			m.messages = append(m.messages, chatMessage{
				Sender:  "system",
				Content: fmt.Sprintf("Sessão %s carregada — %d mensagens.", id, len(ses.Messages)),
			})
		} else {
			m.messages = append(m.messages, chatMessage{Sender: "system", Content: "Erro ao carregar sessão: " + err.Error()})
		}

	case cmd == "/clear":
		m.messages = nil
		m.toolRuns = nil
		m.logEvents = nil
		m.scroll = 0
		var err error
		m.session, err = history.CreateSession(m.cfg.WorkDir)
		if err != nil {
			m.messages = append(m.messages, chatMessage{Sender: "system", Content: "Erro: " + err.Error()})
		} else {
			m.messages = append(m.messages, chatMessage{Sender: "system", Content: "Chat limpo. Nova sessão iniciada."})
		}

	default:
		m.messages = append(m.messages, chatMessage{
			Sender:  "system",
			Content: "Comando desconhecido. Disponíveis: /history, /load <id>, /clear",
		})
	}

	return m, nil
}

// ── Menu update ───────────────────────────────────────────────────────────────

func (m *appModel) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.confirm.close()
		return m, nil
	case "up":
		if m.confirm.cursor > 0 {
			m.confirm.cursor--
		}
		return m, nil
	case "down":
		if m.confirm.cursor < len(m.confirm.choices)-1 {
			m.confirm.cursor++
		}
		return m, nil
	case "y":
		m.agent.ReplyCh <- agent.ConfirmReply{Level: 1}
		m.confirm.close()
		return m, nil
	case "n":
		m.agent.ReplyCh <- agent.ConfirmReply{Level: 0}
		m.confirm.close()
		return m, nil
	case "a":
		m.agent.ReplyCh <- agent.ConfirmReply{Level: 2}
		m.confirm.close()
		return m, nil
	case "enter":
		// Map cursor position to reply level: 0=yes, 1=no, 2=always
		level := m.confirm.cursor
		if level > 2 {
			level = 1
		}
		m.agent.ReplyCh <- agent.ConfirmReply{Level: level}
		m.confirm.close()
		return m, nil
	}
	return m, nil
}

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
	if m.confirm.visible {
		confirmView := renderConfirmOverlay(&m, m.width)
		view = overlayCenter(view, confirmView, m.width, m.height)
	}

	if m.picker.visible && !m.picker.done {
		pickerView := m.sessionPickerView(m.width)
		view = overlayCenter(view, pickerView, m.width, m.height)
	}

	return view
}

func overlayCenter(base, overlay string, _, _ int) string {
	return base + "\n\n" + overlay
}

// ── Mouse handling ────────────────────────────────────────────────────────────

func (m *appModel) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch msg.Action {
	case tea.MouseActionPress:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			if m.leftFocus {
				m.navigateLeft(-1)
			} else {
				if m.rightScroll > 2 {
					m.rightScroll -= 2
				} else {
					m.rightScroll = 0
				}
			}
		case tea.MouseButtonWheelDown:
			if m.leftFocus {
				m.navigateLeft(1)
			} else {
				m.rightScroll++
			}
		default:
			// Click on left third of screen → focus left panel
			if msg.X < m.width/3 {
				if !m.leftFocus {
					m.leftFocus = true
				}
			} else {
				// Click on right two-thirds → focus right panel, scroll to position
				if m.leftFocus {
					m.leftFocus = false
				}
				// Map Y to scroll position (approximate: 1 char per line)
				if msg.Y > 0 {
					m.rightScroll = msg.Y - 1
				}
			}
		}
	}
	return m, nil
}
