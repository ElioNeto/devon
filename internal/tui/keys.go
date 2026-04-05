package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up          key.Binding
	Down        key.Binding
	Left        key.Binding
	Right       key.Binding
	Enter       key.Binding
	Tab         key.Binding
	ContextMenu key.Binding
	Expand      key.Binding
	Help        key.Binding
	Quit        key.Binding
	Interrupt   key.Binding
	ClearChat   key.Binding
	NewSession  key.Binding
	PgUp        key.Binding
	PgDown      key.Binding
}

var defaultKeys = keyMap{
	Up:          key.NewBinding(key.WithKeys("up"),        key.WithHelp("↑", "acima")),
	Down:        key.NewBinding(key.WithKeys("down"),      key.WithHelp("↓", "abaixo")),
	Left:        key.NewBinding(key.WithKeys("left"),      key.WithHelp("←", "painel esq")),
	Right:       key.NewBinding(key.WithKeys("right"),     key.WithHelp("→", "painel dir")),
	Enter:       key.NewBinding(key.WithKeys("enter"),     key.WithHelp("enter", "selecionar/enviar")),
	Tab:         key.NewBinding(key.WithKeys("tab"),       key.WithHelp("tab", "trocar seção")),
	ContextMenu: key.NewBinding(key.WithKeys("x"),         key.WithHelp("x", "menu de contexto")),
	Expand:      key.NewBinding(key.WithKeys("e"),         key.WithHelp("e", "expandir/colapsar")),
	Help:        key.NewBinding(key.WithKeys("?"),         key.WithHelp("?", "ajuda")),
	Quit:        key.NewBinding(key.WithKeys("q", "esc"),  key.WithHelp("q/esc", "fechar/voltar")),
	Interrupt:   key.NewBinding(key.WithKeys("ctrl+c"),    key.WithHelp("ctrl+c", "interromper")),
	ClearChat:   key.NewBinding(key.WithKeys("ctrl+l"),    key.WithHelp("ctrl+l", "limpar chat")),
	NewSession:  key.NewBinding(key.WithKeys("ctrl+k"),    key.WithHelp("ctrl+k", "nova sessão")),
	PgUp:        key.NewBinding(key.WithKeys("pgup"),      key.WithHelp("pgup", "rolar cima")),
	PgDown:      key.NewBinding(key.WithKeys("pgdown"),    key.WithHelp("pgdn", "rolar baixo")),
}
