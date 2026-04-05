package tui

// Key bindings centralizados da TUI.
const (
	KeyQuit      = "ctrl+c"
	KeySend      = "enter"
	KeyClear     = "ctrl+l"
	KeyNewSession = "ctrl+k"
	KeyClearInput = "ctrl+u"
	KeyDeleteWord = "ctrl+w"
	KeyDeleteWordBS = "ctrl+backspace"

	// Navegação de painéis (Tab para painel, setas para cursor)
	KeyTabNext     = "tab"
	KeyTabPrev     = "shift+tab"

	// Navegação de itens
	KeyUp    = "up"
	KeyDown  = "down"
	KeyEnter = "enter"

	// Scroll
	KeyPageUp    = "pgup"
	KeyPageDown  = "pgdown"
	KeyBackspace = "backspace"
	KeyDelete    = "delete"
	KeyLeft      = "left"
	KeyRight     = "right"

	// UI
	KeyHelp     = "?"
	KeyContext  = "x"
	KeyEscape   = "esc"
	KeyExpand   = "e"
)

// KeyHint descreve um atalho para exibição na ajuda.
type KeyHint struct {
	Keys    string
	Action  string
}

// AllHints retorna todos os atalhos para exibição.
func AllHints() []KeyHint {
	return []KeyHint{
		{"Enter", "enviar mensagem"},
		{"Ctrl+C", "interromper / sair"},
		{"Ctrl+L", "limpar chat"},
		{"Ctrl+K", "nova sessão"},
		{"Tab", "ciclo de aba esquerda"},
		{"← / →", "mudar foco painel"},
		{"↑ / ↓", "navegar itens"},
		{"x", "menu de contexto"},
		{"e", "expandir/colapsar"},
		{"PgUp/Down", "scroll"},
		{"?", "ajuda completa"},
		{"Esc", "fechar menu"},
	}
}
