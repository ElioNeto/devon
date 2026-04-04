package main

import (
	"fmt"

	"github.com/ElioNeto/devon/internal/config"
	"github.com/ElioNeto/devon/internal/tui"
	"github.com/spf13/cobra"
)

var version = "dev"

// main moved to main_nocov.go for coverage purposes

func newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:     "devon",
		Short:   "Agente de código com TUI — conecta a qualquer LLM OpenAI-compatible",
		Version: version,
		RunE:    runAgent,
	}

	root.PersistentFlags().String("mode", "auto", "Modo de permissão: auto | safe | yolo")
	root.PersistentFlags().String("model", "", "Modelo a usar (sobrescreve DEVON_MODEL)")
	root.PersistentFlags().String("env", ".env", "Caminho para o arquivo .env")

	// Subcomando doctor
	doctor := &cobra.Command{
		Use:   "doctor",
		Short: "Valida configuração e testa conexão com o provider",
		RunE:  runDoctor,
	}
	root.AddCommand(doctor)

	return root
}

func runAgent(cmd *cobra.Command, _ []string) error {
	envFile, _ := cmd.Flags().GetString("env")
	cfg, err := config.Load(envFile)
	if err != nil {
		return fmt.Errorf("falha ao carregar configuração: %w", err)
	}

	mode, _ := cmd.Flags().GetString("mode")
	cfg.Mode = config.ParseMode(mode)

	if override, _ := cmd.Flags().GetString("model"); override != "" {
		cfg.Model = override
	}

	return tui.Run(cfg)
}

func runDoctor(cmd *cobra.Command, _ []string) error {
	envFile, _ := cmd.Flags().GetString("env")
	cfg, err := config.Load(envFile)
	if err != nil {
		return fmt.Errorf("falha ao carregar configuração: %w", err)
	}
	return cfg.Doctor(cmd.Context())
}
