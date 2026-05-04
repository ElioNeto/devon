// Package tui implementa a interface de usuÃ¡rio com Bubble Tea.
package tui

import (
	"fmt"

	"github.com/ElioNeto/devon/internal/config"
	"github.com/ElioNeto/devon/internal/llm"
	"github.com/ElioNeto/devon/internal/tools"
	tea "github.com/charmbracelet/bubbletea"
)

// Run inicia a TUI e bloqueia atÃ© o usuÃ¡rio sair.
// resumeSessionID, when non-empty, loads that session's messages from the DB store.
// router is optional; pass nil to use only the default client.
func Run(cfg *config.Config, registry *tools.Registry, resumeSessionID string, router ...*llm.AgentRouter) error {
	fmt.Printf("Devon \u2014 %s @ %s\n", cfg.Model, cfg.BaseURL)
	fmt.Printf("Modo: %s | WorkDir: %s\n", cfg.Mode, cfg.WorkDir)
	if cfg.ContextDoc != "" {
		fmt.Printf("DEVON.md: %s\n", cfg.ContextDoc)
	}

	var r *llm.AgentRouter
	if len(router) > 0 {
		r = router[0]
	}

	m := newModel(cfg, registry, resumeSessionID, r)
	p := tea.NewProgram(&m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("tui: %w", err)
	}
	return nil
}
