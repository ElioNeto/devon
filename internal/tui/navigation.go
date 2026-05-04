// Package tui — left panel navigation helpers.
package tui

// ── Navigation ────────────────────────────────────────────────────────────────

func (m *appModel) navigateLeft(dir int) {
	items := buildLeftItems(m)
	m.leftItems = items
	if len(items) == 0 {
		return
	}
	next := m.leftCursor + dir
	for next >= 0 && next < len(items) && items[next].StatusKind == "header" {
		next += dir
	}
	if next >= 0 && next < len(items) {
		m.leftCursor = next
		m.syncRightView()
	}
}

func (m *appModel) cycleSection() {
	current := leftSection(-1)
	if m.leftCursor < len(m.leftItems) {
		current = m.leftItems[m.leftCursor].Section
	}
	nextSec := leftSection((int(current) + 1) % (int(secTokens) + 1))
	for i, item := range m.leftItems {
		if item.Section == nextSec && item.StatusKind != "header" {
			m.leftCursor = i
			m.syncRightView()
			return
		}
	}
}

func (m *appModel) cycleSectionBack() {
	current := leftSection(-1)
	if m.leftCursor < len(m.leftItems) {
		current = m.leftItems[m.leftCursor].Section
	}
	nextSec := leftSection((int(current) + int(secTokens)) % (int(secTokens) + 1))
	for i, item := range m.leftItems {
		if item.Section == nextSec && item.StatusKind != "header" {
			m.leftCursor = i
			m.syncRightView()
			return
		}
	}
}

func (m *appModel) selectLeftItem() {
	if m.leftCursor >= len(m.leftItems) {
		return
	}
	item := m.leftItems[m.leftCursor]
	if item.StatusKind == "header" {
		return
	}

	// Toggle tool call collapse when selecting a tool run item
	if item.Section == secFerramentas && item.Index > 0 && item.Index-1 < len(m.toolRuns) {
		tr := &m.toolRuns[item.Index-1]
		tr.Collapsed = !tr.Collapsed
		return
	}

	m.syncRightView()
	m.leftFocus = false
}

func (m *appModel) syncRightView() {
	if m.leftCursor >= len(m.leftItems) {
		return
	}
	item := m.leftItems[m.leftCursor]
	switch item.Section {
	case secTurno:
		if item.Index > 0 && item.Index-1 < len(m.toolRuns) {
			m.selectedTool = &m.toolRuns[item.Index-1]
		}
		m.rightView = viewLogs
	case secFerramentas:
		m.rightView = viewLogs
	case secMemoria:
		m.rightView = viewDiff
	case secTokens:
		m.rightView = viewConfig
	case secHistorico:
		m.selectedTurnIdx = item.Index
		m.rightView = viewSteps
	}
}
