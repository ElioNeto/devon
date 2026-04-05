package tui

import (
	"github.com/ElioNeto/devon/internal/cost"
	"github.com/ElioNeto/devon/internal/history"
)

// sectionLabels maps leftSection index → display label.
var sectionLabels = []string{"Turno Ativo", "Histórico", "Ferramentas", "Memória", "Tokens"}

// workspaceSlot holds the state for one workspace (Ctrl+1..5).
type workspaceSlot struct {
	session    *history.Session
	messages   []chatMessage
	toolRuns   []toolRun
	logEvents  []logEvent
	fileChanges []fileChange
	memoryFacts []memoryFact
	historyTurns []historyTurn
	tracker    *cost.Session
	tokenPerTurn []int
	currentTask string
	pendingTasks []pendingTask
	running    bool
}
