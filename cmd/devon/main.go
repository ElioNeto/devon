package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	agentpkg "github.com/ElioNeto/devon/internal/agent"
	cachepkg "github.com/ElioNeto/devon/internal/cache"
	"github.com/ElioNeto/devon/internal/config"
	"github.com/ElioNeto/devon/internal/db"
	"github.com/ElioNeto/devon/internal/index"
	initpkg "github.com/ElioNeto/devon/internal/init"
	"github.com/ElioNeto/devon/internal/llm"
	"github.com/ElioNeto/devon/internal/mcp"
	"github.com/ElioNeto/devon/internal/memory"
	rpcpkg "github.com/ElioNeto/devon/internal/rpc"
	"github.com/ElioNeto/devon/internal/session"
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
	root.PersistentFlags().String("task-type", "", "Força o tipo de tarefa: explore | plan | code (desativa classificação automática)")

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
	runCmd.Flags().Bool("no-cache", false, "Ignora o cache de respostas do LLM")
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

	// Subcomando cache
	root.AddCommand(newCacheCommand())

	// Subcomando sessions
	root.AddCommand(newSessionsCommand())

	// Subcomando rpc
	rpcCmd := &cobra.Command{
		Use:   "rpc",
		Short: "Inicia o servidor RPC sobre Unix socket para extensão VSCode",
		Long: `Inicia o servidor JSON-RPC 2.0 sobre Unix socket que permite à extensão
VSCode se comunicar com o agente Devon.

O servidor cria um socket Unix em <workdir>/.devon/rpc.sock e fica
escutando por conexões. Enquanto o servidor estiver rodando, a TUI
também é iniciada para interação local.

Exemplos:
  devon rpc
  devon rpc --mode yolo`,
		RunE: runRPCServer,
	}
	rpcCmd.Flags().String("mode", "auto", "Modo de permissao: auto | safe | yolo")
	root.AddCommand(rpcCmd)

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

	// Build agent router for task-type-based model selection
	router := buildAgentRouter(cfg)

	// Force task type if --task-type flag is set
	if tt, forced := forceTaskType(cmd); forced {
		cfg.ForcedTaskType = tt
	}

	return tui.Run(cfg, registry, "", router)
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

	// Apply forced task type if --task-type flag is set on parent command
	if tt, forced := forceTaskType(cmd.Parent()); forced {
		cfg.ForcedTaskType = tt
	}

	noCache, _ := cmd.Flags().GetBool("no-cache")

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	result, err := runOneShot(ctx, cfg, task, noCache)
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

func runOneShot(ctx context.Context, cfg *config.Config, task string, noCache bool) (string, error) {
	// Integracao com cache de respostas
	useCache := !noCache && cfg.Cache.Enabled

	var cacheInst *cachepkg.Cache
	if useCache {
		dbPath := filepath.Join(cfg.WorkDir, cfg.DBPath)
		sqlDB, err := openSQLite(dbPath)
		if err != nil {
			slog.Warn("cache: nao foi possivel abrir banco, cache desabilitado", "err", err)
			useCache = false
		} else {
			defer sqlDB.Close()
			cacheInst, err = cachepkg.New(sqlDB, cfg.Cache)
			if err != nil {
				slog.Warn("cache: falha ao inicializar, cache desabilitado", "err", err)
				useCache = false
			}
		}
	}

	if useCache && cacheInst != nil {
		// Tenta cache antes de criar o agente
		msg := cachepkg.MessageForTask(task)
		result, err := cacheInst.Get(ctx, cfg.Model, msg, nil)
		if err != nil {
			slog.Warn("cache: erro ao consultar cache", "err", err)
		} else if result.Hit {
			slog.Info("cache: HIT para a tarefa", "tokens_saved", result.TokensSaved)
			return result.Response, nil
		}
	}

	client := llm.New(cfg.APIKey, cfg.BaseURL, cfg.Model, cfg.Timeout)

	// Initialize MCP servers and get registry with all tools
	registry := initMCPTools(ctx, cfg, slog.Default())

	// Build agent router for task-type-based model selection
	router := buildAgentRouter(cfg)

	// Create a simple in-memory store for run-only mode
	fakeDB := &fakeDB{}
	agent := agentpkg.New(cfg, client, registry, fakeDB, "default-agent", nil, "", router)
	if cfg.ForcedTaskType != "" {
		agent.SetForcedTaskType(cfg.ForcedTaskType)
	}

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

	response := strings.TrimSpace(text.String())

	// Salva no cache apos resposta bem-sucedida
	if useCache && cacheInst != nil && response != "" {
		msg := cachepkg.MessageForTask(task)
		tokensSaved := len(task)/4 + len(response)/4 // estimativa simples (~4 chars/token)
		if err := cacheInst.Set(ctx, cfg.Model, msg, response, tokensSaved, nil); err != nil {
			slog.Warn("cache: erro ao salvar resposta", "err", err)
		} else {
			slog.Info("cache: resposta salva no cache")
		}
	}

	return response, nil
}

// openSQLite abre um banco SQLite e retorna a conexao.
func openSQLite(dbPath string) (*sql.DB, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("falha ao criar diretorio do banco: %w", err)
	}
	dsn := dbPath + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	return sql.Open("sqlite", dsn)
}

// buildAgentRouter creates an AgentRouter from the [agent_routing] section in devon.toml.
// Returns nil when no routing is configured (passthrough mode).
func buildAgentRouter(cfg *config.Config) *llm.AgentRouter {
	tc, err := config.LoadToml()
	if err != nil || tc == nil || tc.AgentRouting == nil {
		return nil
	}

	routing, err := config.ResolveAgentRouting(tc)
	if err != nil {
		slog.Warn("agent_routing: erro ao resolver perfis, usando configuração padrão", "err", err)
		return nil
	}

	if len(routing) == 0 {
		return nil
	}

	defaultClient := llm.New(cfg.APIKey, cfg.BaseURL, cfg.Model, cfg.Timeout)
	return llm.NewAgentRouter(routing, defaultClient)
}

// forceTaskType returns the forced task type when --task-type is set, or zero value if not forced.
func forceTaskType(cmd *cobra.Command) (config.TaskType, bool) {
	v, _ := cmd.Flags().GetString("task-type")
	if v == "" {
		return "", false
	}
	return config.ParseTaskType(v), true
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

	// --yes implies --force (skip all prompts)
	if yesFlag {
		forceFlag = true
	}

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
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		info, err1 = wizard.RunNonInteractive(ctx)
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

	// Write DEVON.md using initpkg.WriteFile (handles prompts, editor, and atomic write)
	if err4 := initpkg.WriteFile(devonPath, devonContent, forceFlag); err4 != nil {
		return fmt.Errorf("falha ao escrever DEVON.md: %w", err4)
	}

	// Print success only if the file was actually written (has our content)
	if data, err4 := os.ReadFile(devonPath); err4 == nil && string(data) == devonContent {
		fmt.Fprintf(os.Stdout, "Criando DEVON.md... ✔\n")
		fmt.Fprintf(os.Stdout, "Arquivo: %s\n", devonPath)
		fmt.Fprintf(os.Stdout, "Tamanho: %d bytes\n", len(devonContent))
	}

	return nil
}

func runRPCServer(cmd *cobra.Command, _ []string) error {
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

	// Initialize MCP servers and get registry
	registry := initMCPTools(cmd.Context(), cfg, slog.Default())

	// Build agent router
	router := buildAgentRouter(cfg)

	// Create the RPC server
	srv := rpcpkg.NewServer()
	if err := srv.Listen(cfg.WorkDir); err != nil {
		return fmt.Errorf("falha ao iniciar servidor RPC: %w", err)
	}

	// Create DB store for session management
	dbPath := cfg.DBPath
	if !filepath.IsAbs(dbPath) {
		dbPath = filepath.Join(cfg.WorkDir, dbPath)
	}
	store, err := db.New(dbPath)
	if err != nil {
		slog.Warn("rpc: nao foi possivel abrir banco, usando armazenamento em memoria", "err", err)
		store = &fakeDB{}
	}
	if store != nil {
		defer store.Close()
	}

	// Create agent
	client := llm.New(cfg.APIKey, cfg.BaseURL, cfg.Model, cfg.Timeout)
	agentID := session.ShortID()
	agent := agentpkg.New(cfg, client, registry, store, agentID, nil, cfg.WorkDir, router)
	if cfg.ForcedTaskType != "" {
		agent.SetForcedTaskType(cfg.ForcedTaskType)
	}

	// Register handlers and start stream manager
	hm := rpcpkg.NewHandlerManager(agent, store, srv)
	hm.RegisterAll()

	sm := rpcpkg.NewStreamManager(srv, store)
	if err := sm.Start(); err != nil {
		slog.Warn("rpc: nao foi possivel iniciar stream manager", "err", err)
	}
	defer sm.Stop()

	// Start serving in background
	go func() {
		slog.Info("rpc: servidor iniciado", "socket", srv.SocketPath())
		if err := srv.Serve(); err != nil {
			slog.Error("rpc: servidor encerrou com erro", "err", err)
		}
	}()
	defer srv.Close()

	// Also start the TUI for local interaction
	fmt.Fprintf(os.Stdout, "Servidor RPC rodando em %s\n", srv.SocketPath())
	fmt.Fprintf(os.Stdout, "Pressione Ctrl+C para encerrar.\n")

	return tui.Run(cfg, registry, "", router)
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

// newSessionsCommand cria o subcomando devon sessions com list, resume, export, delete, stats.
func newSessionsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sessions",
		Short: "Gerencia sessoes de conversa",
		Long: `Lista, exporta, retoma e gerencia sessoes de conversa do Devon.

Exemplos:
  devon sessions list
  devon sessions resume <id>
  devon sessions export <id> --format markdown
  devon sessions delete <id>
  devon sessions stats
`,
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "Lista todas as sessoes",
		RunE: func(cmd *cobra.Command, args []string) error {
			envFile, _ := cmd.Flags().GetString("env")
			cfg, err := config.Load(envFile)
			if err != nil {
				return fmt.Errorf("falha ao carregar configuracao: %w", err)
			}

			store, err := db.New(filepath.Join(cfg.WorkDir, cfg.DBPath))
			if err != nil {
				return fmt.Errorf("falha ao abrir banco: %w", err)
			}
			defer store.Close()

			mgr := session.NewManager(store)
			sessions, err := mgr.List(cmd.Context(), 50)
			if err != nil {
				return fmt.Errorf("falha ao listar sessoes: %w", err)
			}

			if len(sessions) == 0 {
				fmt.Fprintf(os.Stdout, "Nenhuma sessao encontrada.\n")
				return nil
			}

			fmt.Fprintf(os.Stdout, "Sessoes (%d):\n\n", len(sessions))
			for _, s := range sessions {
				fmt.Fprintf(os.Stdout, "  %s\n", s.ID)
				if s.Task != "" {
					fmt.Fprintf(os.Stdout, "    Tarefa: %s\n", s.Task)
				}
				fmt.Fprintf(os.Stdout, "    Status: %s | Modelo: %s\n", s.Status, s.Model)
				fmt.Fprintf(os.Stdout, "    Mensagens: %d | Tools: %d | Custo: $%.4f\n",
					s.MessageCount, s.ToolCallCount, s.TotalCost)
				fmt.Fprintf(os.Stdout, "    Ultima atividade: %s\n", s.LastActivity.Format("2006-01-02 15:04:05"))
				fmt.Fprintf(os.Stdout, "    Duracao: %d ms\n", s.Duration)
				fmt.Fprintln(os.Stdout)
			}
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "resume <id>",
		Short: "Retoma uma sessao anterior",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			envFile, _ := cmd.Flags().GetString("env")
			cfg, err := config.Load(envFile)
			if err != nil {
				return fmt.Errorf("falha ao carregar configuracao: %w", err)
			}

			store, err := db.New(filepath.Join(cfg.WorkDir, cfg.DBPath))
			if err != nil {
				return fmt.Errorf("falha ao abrir banco: %w", err)
			}
			defer store.Close()

			mgr := session.NewManager(store)
			s, err := mgr.Get(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("falha ao buscar sessao: %w", err)
			}
			if s == nil {
				return fmt.Errorf("sessao %q nao encontrada", args[0])
			}

			slog.Info("sessao encontrada, iniciando TUI com sessao", "id", s.ID, "task", s.Task, "model", s.Model, "status", s.Status)

			// Initialize MCP tools and start the TUI with the resumed session
			registry := initMCPTools(cmd.Context(), cfg, slog.Default())
			return tui.Run(cfg, registry, s.ID)
		},
	})

	exportCmd := &cobra.Command{
		Use:   "export <id>",
		Short: "Exporta uma sessao como JSON ou Markdown",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			envFile, _ := cmd.Flags().GetString("env")
			format, _ := cmd.Flags().GetString("format")
			cfg, err := config.Load(envFile)
			if err != nil {
				return fmt.Errorf("falha ao carregar configuracao: %w", err)
			}

			store, err := db.New(filepath.Join(cfg.WorkDir, cfg.DBPath))
			if err != nil {
				return fmt.Errorf("falha ao abrir banco: %w", err)
			}
			defer store.Close()

			mgr := session.NewManager(store)
			s, err := mgr.Get(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("falha ao buscar sessao: %w", err)
			}
			if s == nil {
				return fmt.Errorf("sessao %q nao encontrada", args[0])
			}

			data := &session.ExportData{
				Session: *s,
			}
			if format == "json" {
				out, err := session.ExportJSON(data)
				if err != nil {
					return fmt.Errorf("falha ao exportar JSON: %w", err)
				}
				fmt.Fprintln(os.Stdout, string(out))
			} else {
				out, err := session.ExportMarkdown(data)
				if err != nil {
					return fmt.Errorf("falha ao exportar Markdown: %w", err)
				}
				fmt.Fprintln(os.Stdout, out)
			}
			return nil
		},
	}
	exportCmd.Flags().String("format", "markdown", "Formato de exportacao: markdown ou json")
	cmd.AddCommand(exportCmd)

	cmd.AddCommand(&cobra.Command{
		Use:   "delete <id>",
		Short: "Remove uma sessao do banco de dados",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			envFile, _ := cmd.Flags().GetString("env")
			cfg, err := config.Load(envFile)
			if err != nil {
				return fmt.Errorf("falha ao carregar configuracao: %w", err)
			}

			store, err := db.New(filepath.Join(cfg.WorkDir, cfg.DBPath))
			if err != nil {
				return fmt.Errorf("falha ao abrir banco: %w", err)
			}
			defer store.Close()

			mgr := session.NewManager(store)
			if err := mgr.Delete(cmd.Context(), args[0]); err != nil {
				return fmt.Errorf("falha ao deletar sessao: %w", err)
			}
			fmt.Fprintf(os.Stdout, "Sessao %s removida.\n", args[0])
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "stats",
		Short: "Exibe estatisticas agregadas das sessoes",
		RunE: func(cmd *cobra.Command, args []string) error {
			envFile, _ := cmd.Flags().GetString("env")
			cfg, err := config.Load(envFile)
			if err != nil {
				return fmt.Errorf("falha ao carregar configuracao: %w", err)
			}

			store, err := db.New(filepath.Join(cfg.WorkDir, cfg.DBPath))
			if err != nil {
				return fmt.Errorf("falha ao abrir banco: %w", err)
			}
			defer store.Close()

			mgr := session.NewManager(store)
			stats, err := mgr.Stats(cmd.Context())
			if err != nil {
				return fmt.Errorf("falha ao obter estatisticas: %w", err)
			}

			fmt.Fprintf(os.Stdout, "Sessoes:\n")
			fmt.Fprintf(os.Stdout, "  Total:     %d\n", stats.TotalSessions)
			fmt.Fprintf(os.Stdout, "  Ativas:    %d\n", stats.ActiveSessions)
			fmt.Fprintf(os.Stdout, "  Mensagens: %d\n", stats.TotalMessages)
			fmt.Fprintf(os.Stdout, "  Tool Calls: %d\n", stats.TotalToolCalls)
			fmt.Fprintf(os.Stdout, "  Custo:     $%.4f\n", stats.TotalCost)
			fmt.Fprintf(os.Stdout, "  Duracao media: %d ms\n", stats.AvgDurationMs)
			return nil
		},
	})

	return cmd
}

// newCacheCommand cria o subcomando devon cache com stats e clear.
func newCacheCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Gerencia o cache de respostas do LLM",
	}

	statsCmd := &cobra.Command{
		Use:   "stats",
		Short: "Exibe estatisticas do cache de respostas",
		RunE: func(cmd *cobra.Command, args []string) error {
			envFile, _ := cmd.Flags().GetString("env")
			cfg, err := config.Load(envFile)
			if err != nil {
				return fmt.Errorf("falha ao carregar configuracao: %w", err)
			}

			dbPath := filepath.Join(cfg.WorkDir, cfg.DBPath)
			sqlDB, err := openSQLite(dbPath)
			if err != nil {
				return fmt.Errorf("falha ao abrir banco: %w", err)
			}
			defer sqlDB.Close()

			cacheInst, err := cachepkg.New(sqlDB, cfg.Cache)
			if err != nil {
				return fmt.Errorf("falha ao inicializar cache: %w", err)
			}

			stats, err := cacheInst.Stats(cmd.Context())
			if err != nil {
				return fmt.Errorf("falha ao obter estatisticas: %w", err)
			}

			fmt.Fprintf(os.Stdout, "Cache de respostas:\n")
			fmt.Fprintf(os.Stdout, "  Entradas:      %d\n", stats.TotalEntries)
			fmt.Fprintf(os.Stdout, "  Expiradas:     %d\n", stats.ExpiredCount)
			fmt.Fprintf(os.Stdout, "  Tokens salvos: %d\n", stats.TotalTokens)
			return nil
		},
	}

	clearCmd := &cobra.Command{
		Use:   "clear",
		Short: "Limpa todas as entradas do cache de respostas",
		RunE: func(cmd *cobra.Command, args []string) error {
			envFile, _ := cmd.Flags().GetString("env")
			cfg, err := config.Load(envFile)
			if err != nil {
				return fmt.Errorf("falha ao carregar configuracao: %w", err)
			}

			dbPath := filepath.Join(cfg.WorkDir, cfg.DBPath)
			sqlDB, err := openSQLite(dbPath)
			if err != nil {
				return fmt.Errorf("falha ao abrir banco: %w", err)
			}
			defer sqlDB.Close()

			cacheInst, err := cachepkg.New(sqlDB, cfg.Cache)
			if err != nil {
				return fmt.Errorf("falha ao inicializar cache: %w", err)
			}

			if err := cacheInst.Clear(cmd.Context()); err != nil {
				return fmt.Errorf("falha ao limpar cache: %w", err)
			}

			fmt.Fprintf(os.Stdout, "Cache de respostas limpo.\n")
			return nil
		},
	}

	cmd.AddCommand(statsCmd)
	cmd.AddCommand(clearCmd)
	return cmd
}

// fakeDB is a simple no-op store for one-shot mode
type fakeDB struct{}

func (f *fakeDB) CreateSession(ctx context.Context, id string) error                       { return nil }
func (f *fakeDB) CreateSessionWithMeta(ctx context.Context, id, task, model, status string) error { return nil }
func (f *fakeDB) GetSession(ctx context.Context, id string) (bool, error)       { return false, nil }
func (f *fakeDB) ListSessions(ctx context.Context, limit int) ([]string, error) { return nil, nil }
func (f *fakeDB) GetSessionDetail(ctx context.Context, id string) (*db.SessionDetail, error) {
	return nil, nil
}
func (f *fakeDB) ListSessionsDetail(ctx context.Context, limit int) ([]db.SessionDetail, error) {
	return nil, nil
}
func (f *fakeDB) UpdateSession(ctx context.Context, id, task, model, status string) error { return nil }
func (f *fakeDB) DeleteSession(ctx context.Context, id string) error                       { return nil }
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
