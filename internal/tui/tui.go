// Package tui implementa a interface de usuário com Bubble Tea.
package tui

import (
	"fmt"

	"github.com/ElioNeto/devon/internal/config"
)

// Run inicia a TUI e bloqueia até o usuário sair.
// TODO(issue #4): implementar TUI completa com Bubble Tea.
func Run(cfg *config.Config) error {
	fmt.Printf("Devon — %s @ %s\n", cfg.Model, cfg.BaseURL)
	fmt.Printf("Modo: %s | WorkDir: %s\n", cfg.Mode, cfg.WorkDir)
	if cfg.ContextDoc != "" {
		fmt.Printf("DEVON.md: %s\n", cfg.ContextDoc)
	}
	fmt.Println("TUI em construção — veja issue #4")
	return nil
}
