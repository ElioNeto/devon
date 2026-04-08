package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	agentpkg "github.com/ElioNeto/devon/internal/agent"
	"github.com/ElioNeto/devon/internal/config"
	"github.com/ElioNeto/devon/internal/llm"
	"github.com/ElioNeto/devon/internal/tools"
	"github.com/ElioNeto/devon/internal/tui"
	"github.com/spf13/cobra"
)

// applyProfileFlags resolves and applies a profile and optional model override.
func applyProfileFlags(cmd *cobra.Command, cfg *config.Config) error {
	profileName, _ := cmd.Flags().GetString("profile")
	modelOverride, _ := cmd.Flags().GetString("model")

	if profileName != "" {
		tc, err := config.LoadToml()
		if err != nil {
			return fmt.Errorf("falha ao carregar devon.toml: %w", err)
		}
		p, err := config.ResolveProfile(tc, profileName)
		if err != nil {
			return err
		}
		if err := config.ApplyProfile(cfg, p); err != nil {
			return err
		}
	}

	if modelOverride != "" {
		cfg.Model = modelOverride
	}

	return nil
}

// exitCoder is used to pass exit codes through Cobra's error handling.
type exitCoder struct {
	err  error
	code int
}

func (e *exitCoder) Error() string { return e.err.Error() }
func (e *exitCoder) ExitCode() int { return e.code }

func configError(err error) *exitCoder { return &exitCoder{err: err, code: 2} }
func exitError(err error) *exitCoder   { return &exitCoder{err: err, code: 1} }

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
	root.PersistentFlags().String("model", "", "Sobrescreve o modelo do perfil ativo")
	root.PersistentFlags().String("env", ".env", "Caminho para o arquivo .env")
	root.PersistentFlags().StringP("profile", "p", "", "Perfil de provider definido em devon.toml")

	// Subcomando doctor
	doctor := &cobra.Command{
		Use:   "doctor",
		Short: "Valida configuração e testa conexão com o provider",
		RunE:  runDoctor,
	}
	root.AddCommand(doctor)

	// Subcomando profiles
	root.AddCommand(newProfilesCommand())

	// Subcomando run (one-shot non-interactive)
	runCmd := &cobra.Command{
		Use:   "run <tarefa>",
		Short: "Executa uma tarefa de forma não-interativa",
		Long: `Executa uma tarefa sem abrir a TUI, retornando o resultado via stdout.

Exemplos:
  devon run "crie a função main.go"
  echo "refatore auth.go" | devon run
  devon run "adicione testes ao read.go" --mode yolo

Exit codes:
  0   Sucesso
  1   Erro na execução
  2   Erro de configuração
  130 Cancelado (SIGINT/Ctrl+C)`,
		Args: cobra.MinimumNArgs(1),
		RunE: runTask,
	}
	runCmd.Flags().String("mode", "auto", "Modo de permissão: auto | safe | yolo")
	root.AddCommand(runCmd)

	return root
}

func runAgent(cmd *cobra.Command, _ []string) error {
	envFile, _ := cmd.Flags().GetString("env")
	cfg, err := config.Load(envFile)
	if err != nil {
		return fmt.Errorf("falha ao carregar configuração: %w", err)
	}

	if err := applyProfileFlags(cmd, cfg); err != nil {
		return err
	}

	mode, _ := cmd.Flags().GetString("mode")
	cfg.Mode = config.ParseMode(mode)

	return tui.Run(cfg)
}

func runTask(cmd *cobra.Command, args []string) error {
	task := strings.Join(args, " ")

	// Se stdin é um pipe, lê conteúdo e anexa à tarefa
	if hasStdinPipe() {
		stdinContent, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("falha ao ler stdin: %w", err)
		}
		if s := strings.TrimSpace(string(stdinContent)); s != "" {
			task = task + "\n\n" + s
		}
	}

	envFile, _ := cmd.Flags().GetString("env")
	cfg, err := config.Load(envFile)
	if err != nil {
		return configError(fmt.Errorf("falha ao carregar configuração: %w", err))
	}

	if err := applyProfileFlags(cmd, cfg); err != nil {
		return configError(err)
	}

	mode, _ := cmd.Flags().GetString("mode")
	cfg.Mode = config.ParseMode(mode)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	result, err := runOneShot(ctx, cfg, task)
	if err != nil {
		return exitError(err)
	}
	fmt.Println(result)
	return nil
}

func hasStdinPipe() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeNamedPipe != 0 || info.Size() > 0
}

func runOneShot(ctx context.Context, cfg *config.Config, task string) (string, error) {
	client := llm.New(cfg.APIKey, cfg.BaseURL, cfg.Model, cfg.Timeout)
	registry := tools.NewRegistry()

	// Create a simple in-memory store for run-only mode
	fakeDB := &fakeDB{}
	agent := agentpkg.New(cfg, client, registry, fakeDB, "default-agent")

	events := agent.Run(ctx, task)

	var text strings.Builder
	for ev := range events {
		switch ev.Type {
		case "text":
			text.WriteString(ev.Text)
		case "tool_start":
			fmt.Fprintf(os.Stderr, "tool: %s\n", ev.Tool)
		case "tool_done":
			fmt.Fprintf(os.Stderr, "tool: %s done\n", ev.Tool)
		case "tool_error":
			fmt.Fprintf(os.Stderr, "tool: %s error: %v\n", ev.Tool, ev.Err)
		case "error":
			return "", fmt.Errorf("agente: %w", ev.Err)
		}
	}
	return strings.TrimSpace(text.String()), nil
}

func runDoctor(cmd *cobra.Command, _ []string) error {
	envFile, _ := cmd.Flags().GetString("env")
	cfg, err := config.Load(envFile)
	if err != nil {
		return fmt.Errorf("falha ao carregar configuração: %w", err)
	}
	return cfg.Doctor(cmd.Context())
}

// fakeDB is a simple in-memory store for one-shot mode
type fakeDB struct{}

func (f *fakeDB) CreateSession(ctx context.Context, id string) error { return nil }
func (f *fakeDB) GetSession(ctx context.Context, id string) (bool, error) { return false, nil }
func (f *fakeDB) ListSessions(ctx context.Context, limit int) ([]db.Message, error) { return nil, nil }
func (f *fakeDB) PutMessage(ctx context.Context, agentID, sessionID, role, content string) error { return nil }
func (f *fakeDB) GetMessages(ctx context.Context, agentID, sessionID string, limit int) ([]db.Message, error) { return nil, nil }
func (f *fakeDB) SlidingWindow(ctx context.Context, agentID, sessionID string, windowSize int) error { return nil }
func (f *fakeDB) PutAgentState(ctx context.Context, agentID, sessionID, snapshot string) error { return nil }
func (f *fakeDB) GetAgentState(ctx context.Context, agentID string) (*db.AgentState, error) { return nil, nil }
func (f *fakeDB) PutToolCall(ctx context.Context, agentID, sessionID, toolName, arguments, status, result, err string) (int64, error) { return 0, nil }
func (f *fakeDB) GetToolCalls(ctx context.Context, sessionID string) ([]db.ToolCall, error) { return nil, nil }
func (f *fakeDB) ArchiveMessages(ctx context.Context, agentID, sessionID string) error { return nil }
func (f *fakeDB) GetSessionHistory(ctx context.Context, sessionID string, limit int) ([]db.Message, error) { return nil, nil }
func (f *fakeDB) PutArtifact(ctx context.Context, key, sessionID string, data []byte) error { return nil }
func (f *fakeDB) GetArtifact(ctx context.Context, key string) ([]byte, error) { return nil, nil }
func (f *fakeDB) GetCostSummary(ctx context.Context, sessionID string) (*db.CostSummary, error) { return nil, nil }
func (f *fakeDB) UpdateCostSummary(ctx context.Context, sessionID string, cost float64, tokens map[string]int) error { return nil }
func (f *fakeDB) Subscribe(ctx context.Context, topic string) (<-chan db.Event, error) { return nil, nil }
func (f *fakeDB) Publish(ctx context.Context, topic string, payload interface{}) error { return nil }
func (f *fakeDB) Close() error { return nil }
