// Package tui implementa a interface de usuÃ¡rio com Bubble Tea.
package tui

import (
	"fmt"

	"github.com/ElioNeto/devon/internal/config"
	"github.com/ElioNeto/devon/internal/tools"
	tea "github.com/charmbracelet/bubbletea"
)

// Run inicia a TUI e bloqueia atÃ© o usuÃ¡rio sair.
func Run(cfg *config.Config, registry *tools.Registry) error {
	fmt.Printf("Devon â€” %s @ %s\n", cfg.Model, cfg.BaseURL)
	fmt.Printf("Modo: %s | WorkDir: %s\n", cfg.Mode, cfg.WorkDir)
	if cfg.ContextDoc != "" {
		fmt.Printf("DEVON.md: %s\n", cfg.ContextDoc)
	}

	m := newModel(cfg, registry)
	p := tea.NewProgram(&m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("tui: %w", err)
	}
	return nil
}
