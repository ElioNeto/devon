package config

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// ExecutionMode define como os AgentWorkers serão executados.
type ExecutionMode string

const (
	Sequential ExecutionMode = "sequential"
	Parallel   ExecutionMode = "parallel"
	Async      ExecutionMode = "async"
	Pipeline   ExecutionMode = "pipeline"
)

func ParseExecutionMode(s string) ExecutionMode {
	switch strings.ToLower(s) {
	case "parallel":
		return Parallel
	case "async":
		return Async
	case "pipeline":
		return Pipeline
	default:
		return Sequential
	}
}

func (e ExecutionMode) String() string {
	switch e {
	case Parallel:
		return "parallel"
	case Async:
		return "async"
	case Pipeline:
		return "pipeline"
	default:
		return "sequential"
	}
}

// Mode controla o nível de autonomia do agente.
type Mode int

const (
	// ModeAuto pede confirmação apenas para operações destrutivas.
	ModeAuto Mode = iota
	// ModeSafe pede confirmação para qualquer tool call.
	ModeSafe
	// ModeYolo executa tudo sem perguntar.
	ModeYolo
)

func ParseMode(s string) Mode {
	switch strings.ToLower(s) {
	case "safe":
		return ModeSafe
	case "yolo":
		return ModeYolo
	default:
		return ModeAuto
	}
}

func (m Mode) String() string {
	switch m {
	case ModeSafe:
		return "safe"
	case ModeYolo:
		return "yolo"
	default:
		return "auto"
	}
}

// AgentConfig configura um agente especializado.
type AgentConfig struct {
	ID           string   `toml:"id"`
	Model        string   `toml:"model"`
	Role         string   `toml:"role"`
	Tools        []string `toml:"tools"`
	DependsOn    []string `toml:"depends_on"`
	EnabledTools []string `toml:"-"`
}

// Config contém toda a configuração de runtime do Devon.
type Config struct {
	// Provider
	APIKey  string
	BaseURL string
	Model   string

	// Comportamento
	Mode      Mode
	MaxTurns  int
	Timeout   time.Duration
	TurnDelay time.Duration

	// Multi-Agent
	ExecutionMode    ExecutionMode
	MaxAgents        int
	DBPath           string
	ContextWindowSize int

	// Agents configuration
	Agents []AgentConfig
	// Single agent (legacy mode)
	SingleAgentMode bool

	// Projeto
	WorkDir    string
	ContextDoc string
}

// Load carrega a configuração.
func Load(envFile string) (*Config, error) {
	if envFile != "" {
		_ = godotenv.Load(envFile)
	}

	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("não foi possível obter diretório de trabalho: %w", err)
	}

	// Lê DEVON.md se existir
	var contextDoc string
	if content, err := os.ReadFile("DEVON.md"); err == nil {
		contextDoc = string(content)
	}

	cfg := &Config{
		APIKey:            os.Getenv("DEVON_API_KEY"),
		BaseURL:           getEnvDefault("DEVON_BASE_URL", "https://api.openai.com/v1"),
		Model:             os.Getenv("DEVON_MODEL"),
		Mode:              ParseMode(getEnvDefault("DEVON_MODE", "auto")),
		MaxTurns:          getEnvInt("DEVON_MAX_TURNS", 50),
		Timeout:           time.Duration(getEnvInt("DEVON_TIMEOUT", 30)) * time.Second,
		TurnDelay:         parseDuration(getEnvDefault("DEVON_TURN_DELAY", "0")),
		WorkDir:           wd,
		ContextDoc:        contextDoc,
		ExecutionMode:     ParseExecutionMode(getEnvDefault("DEVON_EXECUTION_MODE", "sequential")),
		MaxAgents:         getEnvInt("DEVON_MAX_AGENTS", 4),
		DBPath:            getEnvDefault("DEVON_DB_PATH", ".devon/state.db"),
		ContextWindowSize: getEnvInt("DEVON_CONTEXT_WINDOW_SIZE", 20),
		Agents:            []AgentConfig{},
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// validate verifica campos obrigatórios.
func (c *Config) validate() error {
	if c.Model == "" {
		return errors.New("DEVON_MODEL não definido. Exemplo: DEVON_MODEL=mistralai/devstral-2512:free")
	}
	if c.APIKey == "" && !isLocalURL(c.BaseURL) {
		return errors.New("DEVON_API_KEY não definido")
	}
	return nil
}

// Doctor valida a configuração.
func (c *Config) Doctor(ctx context.Context) error {
	fmt.Printf("[devon doctor]\n")
	fmt.Printf("  Model:           %s\n", c.Model)
	fmt.Printf("  BaseURL:         %s\n", c.BaseURL)
	fmt.Printf("  Mode:            %s\n", c.Mode)
	fmt.Printf("  ExecutionMode:   %s\n", c.ExecutionMode)
	fmt.Printf("  MaxAgents:       %d\n", c.MaxAgents)
	fmt.Printf("  DBPath:          %s\n", c.DBPath)
	fmt.Printf("  WorkDir:         %s\n", c.WorkDir)

	if c.ContextDoc != "" {
		fmt.Printf("  DEVON.md:        encontrado (%d bytes)\n", len(c.ContextDoc))
	}

	modelsURL := strings.TrimRight(c.BaseURL, "/") + "/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsURL, nil)
	if err != nil {
		return fmt.Errorf("erro ao criar request: %w", err)
	}
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("  [FAIL] Provider inacessível: %v\n", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		fmt.Printf("  [FAIL] Provider retornou HTTP %d\n", resp.StatusCode)
		return fmt.Errorf("provider HTTP %d", resp.StatusCode)
	}

	fmt.Printf("  [PASS] Provider acessível (HTTP %d)\n", resp.StatusCode)
	return nil
}

func getEnvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
			return n
		}
	}
	return fallback
}

func parseDuration(s string) time.Duration {
	if v, err := time.ParseDuration(s); err == nil {
		return v
	}
	return 0
}

func isLocalURL(u string) bool {
	return strings.Contains(u, "localhost") ||
		strings.Contains(u, "127.0.0.1") ||
		strings.Contains(u, "10.0.0.")
}
