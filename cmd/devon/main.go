package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"

	agentpkg "github.com/ElioNeto/devon/internal/agent"
	"github.com/ElioNeto/devon/internal/config"
	"github.com/ElioNeto/devon/internal/db"
	"github.com/ElioNeto/devon/internal/index"
	initpkg "github.com/ElioNeto/devon/internal/init"
	"github.com/ElioNeto/devon/internal/llm"
	"github.com/ElioNeto/devon/internal/mcp"
	"github.com/ElioNeto/devon/internal/memory"
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
	root.PersistentFlags().Bool("index", false, "Ativa indexação semântica do codebase")
	root.PersistentFlags().Bool("no-index", false, "Desativa indexação semântica (força contexto completo)")

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

	// Subcomando index
	indexCmd := &cobra.Command{
		Use:   "index",
		Short: "Gerencia o índice semântico do codebase",
	}
	indexRebuildCmd := &cobra.Command{
		Use:   "rebuild",
		Short: "Reconstrói o índice semântico do zero",
		RunE: func(cmd *cobra.Command, args []string) error {
			envFile, _ := cmd.Flags().GetString("env")
			cfg, err := config.Load(envFile)
			if err != nil {
				return fmt.Errorf("falha ao carregar configuração: %w", err)
			}

			mgr, err := index.NewManager(cfg.WorkDir, index.ManagerConfig{
				Enabled: true,
				IndexedConfig: index.IndexedConfig{
					Extensions:    cfg.Index.Extensions,
					Excludes:      cfg.Index.Exclude,
					MaxFileSizeKB: cfg.Index.MaxFileSizeKB,
					TopK:          cfg.Index.TopK,
				},
			})
			if err != nil {
				return fmt.Errorf("falha ao criar indexer: %w", err)
			}
			defer mgr.Close()

			fmt.Fprintf(os.Stdout, "Reconstruindo índice em %s...\n", cfg.WorkDir)
			if err := mgr.Rebuild(cmd.Context(), cfg.WorkDir); err != nil {
				return fmt.Errorf("falha ao reindexar: %w", err)
			}

			stats := mgr.GetStats()
			fmt.Fprintf(os.Stdout, "Índice reconstruído: %d arquivos, %d termos\n",
				stats.TotalDocs, stats.TermCount)
			return nil
		},
	}
	indexCmd.AddCommand(indexRebuildCmd)
	root.AddCommand(indexCmd)

	// Subcomando init
	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Configura o projeto criando um DEVON.md",
		Long: `O wizard detecta automaticamente a linguagem, framework e comandos do projeto,
depois faz perguntas para montar um DEVON.md completo.

Flags:
  --yes    Aceita todos os valores detectados sem perguntar
  --force  Sobrescreve DEVON.md existente sem confirmar

Exemplos:
  devon init                    → wizard interativo
  devon init --yes              → modo não-interativo (CI)
  devon init --force            → sobrescreve DEVON.md
`,
		RunE: runInit,
	}
	initCmd.Flags().Bool("yes", false, "Aceita padrões detectados sem perguntar (modo CI)")
	initCmd.Flags().Bool("force", false, "Sobrescreve DEVON.md existente sem confirmar")
	root.AddCommand(initCmd)

	// Subcomando memory
	memoryCmd := &cobra.Command{
		Use:   "memory",
		Short: "Gerencia a memória semântica do projeto",
	}
	memoryClearCmd := &cobra.Command{
		Use:   "clear",
		Short: "Limpa todos os fatos da memória semântica do projeto atual",
		RunE:  runMemoryClear,
	}
	memoryCmd.AddCommand(memoryClearCmd)
	root.AddCommand(memoryCmd)

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

	if v, _ := cmd.Flags().GetBool("index"); v {
		cfg.Index.Enabled = true
	}
	if v, _ := cmd.Flags().GetBool("no-index"); v {
		cfg.Index.Enabled = false
	}

	// Initialize MCP servers and get registry with all tools
	registry := initMCPTools(cmd.Context(), cfg, slog.Default())

	return tui.Run(cfg, registry)
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

	if v, _ := cmd.Flags().GetBool("index"); v {
		cfg.Index.Enabled = true
	}
	if v, _ := cmd.Flags().GetBool("no-index"); v {
		cfg.Index.Enabled = false
	}

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

	// Initialize MCP servers and get registry with all tools
	registry := initMCPTools(ctx, cfg, slog.Default())

	// Create a simple in-memory store for run-only mode
	fakeDB := &fakeDB{}
	agent := agentpkg.New(cfg, client, registry, fakeDB, "default-agent", nil, "")

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

// initMCPTools initializes MCP servers and registers their tools.
// Returns a registry with all tools (built-in + MCP) registered.
// Used by both TUI mode (via tui.Run) and one-shot mode.
func initMCPTools(ctx context.Context, cfg *config.Config, logger *slog.Logger) *tools.Registry {
	registry := tools.NewRegistry()

	tc, err := config.LoadToml()
	if err != nil || tc == nil || len(tc.MCPServers) == 0 {
		return registry
	}

	mcpHelper := mcp.NewRegistryHelper(logger)
	mcpHelper.InitMCPServersFromConfig(ctx, tc.MCPServers, registry)

	return registry
}

func runDoctor(cmd *cobra.Command, _ []string) error {
	envFile, _ := cmd.Flags().GetString("env")
	cfg, err := config.Load(envFile)
	if err != nil {
		return fmt.Errorf("falha ao carregar configuração: %w", err)
	}
	return cfg.Doctor(cmd.Context())
}

func runInit(cmd *cobra.Command, args []string) error {
	yesFlag, _ := cmd.Flags().GetBool("yes")
	forceFlag, _ := cmd.Flags().GetBool("force")

	// Get current working directory
	worksDir, err1 := os.Getwd()
	if err1 != nil {
		return fmt.Errorf("não foi possível obter diretório atual: %w", err1)
	}

	detector := initpkg.NewDetector(worksDir)
	wizard := initpkg.NewWizard(detector)

	// Try to detect project name from git remote
	if devonName, err3 := initpkg.DetectFromGitRemote(worksDir); err3 == nil && devonName != "" {
		fmt.Fprintf(os.Stdout, "Detetado git remote: %s\n", devonName)
	}

	fmt.Printf("\nDevon — configurar projeto\n")
	fmt.Printf("\nDiretório: %s\n", worksDir)

	var info initpkg.ProjectInfo

	if yesFlag {
		// Non-interactive mode (CI)
		fmt.Fprintf(os.Stdout, "Modo não-interativo (--yes)\n")
		info, err1 = wizard.RunNonInteractive()
	} else {
		// Interactive mode
		info, err1 = wizard.Run()
	}

	if err1 != nil {
		return fmt.Errorf("falha ao executar wizard: %w", err1)
	}

	// Print detected info summary
	initpkg.PrintSummary(info)

	// Generate DEVON.md content
	devonContent := info.GenerateDEVONmd()

	// Get output path
	devonPath := filepath.Join(worksDir, "DEVON.md")

	// Ask to open in editor if file exists and --force not set
	if !forceFlag {
		if _, err2 := os.Stat(devonPath); err2 == nil {
			fmt.Fprintf(os.Stderr, "\n%s já existe. Abrir no editor ou sobrescrever?\n", devonPath)
			fmt.Fprintf(os.Stderr, "  [1] Abrir no $EDITOR (padrão)\n")
			fmt.Fprintf(os.Stderr, "  [2] Sobrescrever\n")
			fmt.Print("  Escolha: ")

			reader := bufio.NewReader(os.Stdin)
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)

			if input != "1" && input != "" {
				forceFlag = true
			}
		}
	}

	if !forceFlag {
		// Ask to open in editor
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "nano"
		}
		fmt.Fprintf(os.Stderr, "Abrindo %s no editor...\n", devonPath)
		editorCmd := exec.Command(editor, devonPath)
		editorCmd.Stdin = os.Stdin
		editorCmd.Stdout = os.Stdout
		editorCmd.Stderr = os.Stderr
		return editorCmd.Run()
	}

	// Atomic write
	tmpFile, err4 := os.CreateTemp(worksDir, ".tmp-devon-")
	if err4 != nil {
		return fmt.Errorf("criar arquivo temporário: %w", err4)
	}
	tmpPath := tmpFile.Name()

	_, err4 = tmpFile.WriteString(devonContent)
	if err4 != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("escrever arquivo temporário: %w", err4)
	}

	if err4 := tmpFile.Close(); err4 != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("fechar arquivo temporário: %w", err4)
	}

	if err4 := os.Rename(tmpPath, devonPath); err4 != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renomear para destino: %w", err4)
	}

	fmt.Fprintf(os.Stdout, "Criando DEVON.md... ✔\n")
	fmt.Fprintf(os.Stdout, "Arquivo: %s\n", devonPath)
	fmt.Fprintf(os.Stdout, "Tamanho: %d bytes\n", len(devonContent))

	return nil
}

func runMemoryClear(cmd *cobra.Command, args []string) error {
	envFile, _ := cmd.Flags().GetString("env")
	cfg, err := config.Load(envFile)
	if err != nil {
		return fmt.Errorf("falha ao carregar configuração: %w", err)
	}

	dbPath := filepath.Join(cfg.WorkDir, ".devon", "devon.db")
	store, err := db.New(dbPath)
	if err != nil {
		return fmt.Errorf("falha ao abrir banco: %w", err)
	}
	defer store.Close()

	projectID := memory.ProjectIDFromWorkDir(cfg.WorkDir)
	mem := memory.New(store, projectID)

	if err := mem.Clear(cmd.Context(), projectID); err != nil {
		return fmt.Errorf("falha ao limpar memória: %w", err)
	}

	fmt.Fprintf(os.Stdout, "Memória do projeto limpa (WorkDir: %s)\n", cfg.WorkDir)
	return nil
}

// fakeDB is a simple in-memory store for one-shot mode
type fakeDB struct{}

func (f *fakeDB) CreateSession(ctx context.Context, id string) error            { return nil }
func (f *fakeDB) GetSession(ctx context.Context, id string) (bool, error)       { return false, nil }
func (f *fakeDB) ListSessions(ctx context.Context, limit int) ([]string, error) { return nil, nil }
func (f *fakeDB) PutMessage(ctx context.Context, agentID, sessionID, role, content string) error {
	return nil
}
func (f *fakeDB) GetMessages(ctx context.Context, agentID, sessionID string, limit int) ([]db.Message, error) {
	return nil, nil
}
func (f *fakeDB) SlidingWindow(ctx context.Context, agentID, sessionID string, windowSize int) error {
	return nil
}
func (f *fakeDB) PutAgentState(ctx context.Context, agentID, sessionID, snapshot string) error {
	return nil
}
func (f *fakeDB) GetAgentState(ctx context.Context, agentID string) (*db.AgentState, error) {
	return nil, nil
}
func (f *fakeDB) PutToolCall(ctx context.Context, agentID, sessionID, toolName, arguments, status, result, err string) (int64, error) {
	return 0, nil
}
func (f *fakeDB) GetToolCalls(ctx context.Context, sessionID string) ([]db.ToolCall, error) {
	return nil, nil
}
func (f *fakeDB) ArchiveMessages(ctx context.Context, agentID, sessionID string) error { return nil }
func (f *fakeDB) GetSessionHistory(ctx context.Context, sessionID string, limit int) ([]db.Message, error) {
	return nil, nil
}
func (f *fakeDB) PutArtifact(ctx context.Context, key, sessionID string, data []byte) error {
	return nil
}
func (f *fakeDB) GetArtifact(ctx context.Context, key string) ([]byte, error) { return nil, nil }
func (f *fakeDB) GetCostSummary(ctx context.Context, sessionID string) (*db.CostSummary, error) {
	return nil, nil
}
func (f *fakeDB) UpdateCostSummary(ctx context.Context, sessionID string, cost float64, tokens map[string]int) error {
	return nil
}
func (f *fakeDB) PutFact(ctx context.Context, projectID, category, content, context string) error {
	return nil
}
func (f *fakeDB) GetFacts(ctx context.Context, projectID, category string, limit int) ([]db.Fact, error) {
	return nil, nil
}
func (f *fakeDB) ListFacts(ctx context.Context, projectID string) ([]db.Fact, error) { return nil, nil }
func (f *fakeDB) DeleteFacts(ctx context.Context, projectID string) error            { return nil }
func (f *fakeDB) RecordFileAccess(ctx context.Context, sessionID, filePath, accessType string) error {
	return nil
}
func (f *fakeDB) GetFileAccess(ctx context.Context, sessionID string, limit int) ([]db.FileAccess, error) {
	return nil, nil
}
func (f *fakeDB) PutErrorPattern(ctx context.Context, projectID, pattern, context string) error {
	return nil
}
func (f *fakeDB) IncrementErrorPattern(ctx context.Context, projectID, pattern string) error {
	return nil
}
func (f *fakeDB) GetErrorPatterns(ctx context.Context, projectID string, limit int) ([]db.ErrorPattern, error) {
	return nil, nil
}
func (f *fakeDB) QueryFacts(ctx context.Context, projectID, keyword string, limit int) ([]db.FactRow, error) {
	return nil, nil
}
func (f *fakeDB) Subscribe(ctx context.Context, topic string) (<-chan db.Event, error) {
	return nil, nil
}
func (f *fakeDB) Publish(ctx context.Context, topic string, payload interface{}) error { return nil }
func (f *fakeDB) Close() error                                                         { return nil }
