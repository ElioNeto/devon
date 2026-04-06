// Package tui — input history for navigating previous messages.
package tui

// inputHistory stores sent messages for recall via up/down arrows.
type inputHistory struct {
	entries []string
	cursor  int
	draft   string
}

// push adds a new entry to the history and resets the cursor.
func (h *inputHistory) push(s string) {
	if s == "" {
		return
	}
	if len(h.entries) > 0 && h.entries[len(h.entries)-1] == s {
		return
	}
	h.entries = append(h.entries, s)
	h.cursor = 0
	h.draft = ""
}

// navigateUp returns the message before the current position, saving the
// current draft when first entering history.
func (h *inputHistory) navigateUp() string {
	if len(h.entries) == 0 {
		return ""
	}
	if h.cursor == 0 {
		h.draft = h.entries[0] // placeholder, overwritten by caller
	}
	if h.cursor < len(h.entries) {
		h.cursor++
	}
	return h.entries[len(h.entries)-h.cursor]
}

// navigateDown returns the next message toward the draft.
func (h *inputHistory) navigateDown() string {
	if h.cursor <= 0 {
		return ""
	}
	h.cursor--
	if h.cursor == 0 {
		return h.draft
	}
	if h.cursor < len(h.entries) {
		return h.entries[len(h.entries)-h.cursor]
	}
	return ""
}

// reset clears the history navigator back to the present.
func (h *inputHistory) reset() {
	h.cursor = 0
	h.draft = ""
}
