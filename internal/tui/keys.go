package tui

// Key bindings centralizados da TUI.
const (
	KeyQuit         = "ctrl+c"
	KeySend         = "enter"
	KeyClear        = "ctrl+l"
	KeyNewSession   = "ctrl+k"
	KeyClearInput   = "ctrl+u"
	KeyDeleteWord   = "ctrl+w"
	KeyDeleteWordBS = "ctrl+backspace"

	// Navegação de painéis (Tab para painel, setas para cursor)
	KeyTabNext = "tab"
	KeyTabPrev = "shift+tab"

	// Navegação de itens
	KeyUp    = "up"
	KeyDown  = "down"
	KeyEnter = "enter"

	// Scroll
	KeyPageUp    = "pgup"
	KeyPageDown  = "pgdown"
	KeyBackspace   = "backspace"
	KeyDelete    = "delete"
	KeyLeft      = "left"
	KeyRight     = "right"

	// UI — all single-letter shortcuts moved to Ctrl+ combos to
	// avoid conflicts with free-text input (issue #27).
	KeyHelp   = "?"
	KeyEscape = "esc"
	KeyExpand = "ctrl+e"
	KeyCtxCmd = "!"

	// Session slots — Ctrl+2..5 switch between independent sessions
	// (similar to Linux workspace tabs / tmux windows).
	// Note: Ctrl+1 = \x01 = Ctrl+A (ambiguous), Ctrl+3 is terminal interrupt.
	KeySession2 = "\x02" // Ctrl+2 → workspace 1
	KeySession4 = "\x04" // Ctrl+4 → workspace 2
	KeySession5 = "\x05" // Ctrl+5 → workspace 3
)

// KeyHint descreve um atalho para exibição na ajuda.
type KeyHint struct {
	Keys   string
	Action string
}

// AllHints retorna todos os atalhos para exibição.
func AllHints() []KeyHint {
	return []KeyHint{
		{"Enter", "enviar mensagem"},
		{"!", "painel de comandos"},
		{"Ctrl+C", "interromper / sair"},
		{"Ctrl+L", "limpar chat"},
		{"Ctrl+K", "nova sessão"},
		{"Ctrl+2..5", "trocar de sessão"},
		{"Ctrl+E", "expandir/colapsar"},
		{"Tab", "ciclo de aba esquerda"},
		{"← / →", "mudar foco painel"},
		{"↑ / ↓", "navegar itens"},
		{"PgUp/Down", "scroll"},
		{"?", "ajuda completa"},
		{"Esc", "fechar menu"},
	}
}
