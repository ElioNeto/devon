package tui

import (
	"github.com/ElioNeto/devon/internal/cost"
	"github.com/ElioNeto/devon/internal/history"
)

// workspaceSlot holds the state for one workspace (Ctrl+1..5).
type workspaceSlot struct {
	session      *history.Session
	messages     []chatMessage
	toolRuns     []toolRun
	logEvents    []logEvent
	fileChanges  []fileChange
	memoryFacts  []memoryFact
	historyTurns []historyTurn
	tracker      *cost.Session
	tokenPerTurn []int
	currentTask  string
	pendingTasks []pendingTask
	running      bool
}
