package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// MCPServerConfig represents the configuration for an MCP server.
type MCPServerConfig struct {
	Name        string            `toml:"name"`
	Type        string            `toml:"type"` // "stdio" or "http"
	Command     string            `toml:"command,omitempty"`
	Args        []string          `toml:"args,omitempty"`
	Env         map[string]string `toml:"env,omitempty"`
	URL         string            `toml:"url,omitempty"`
	Headers     map[string]string `toml:"headers,omitempty"`
	Enabled     bool              `toml:"enabled"`
	Description string            `toml:"description,omitempty"`
}

// Profile defines a named provider configuration.
type Profile struct {
	Name      string   `toml:"name"`
	APIKeyEnv string   `toml:"api_key_env"`
	BaseURL   string   `toml:"base_url"`
	Model     string   `toml:"model"`
	Provider  string   `toml:"provider"`
	Fallback  []string `toml:"fallback"`
}

// IndexConfig configura a indexação semântica do codebase.
type IndexConfig struct {
	Enabled       bool     `toml:"enabled"`
	Extensions    []string `toml:"extensions"`
	Exclude       []string `toml:"exclude"`
	MaxFileSizeKB int      `toml:"max_file_size_kb"`
	TopK          int      `toml:"top_k"`
}

// CacheConfig configura o cache de respostas do LLM.
type CacheConfig struct {
	Enabled     bool   `toml:"enabled"`
	TTL         string `toml:"ttl"`
	MaxSizeMB   int    `toml:"max_size_mb"`
	OnlyOneShot bool   `toml:"only_one_shot"`
}

// AttachmentsConfig configura o suporte a anexos de imagem.
type AttachmentsConfig struct {
	MaxSizeMB int `toml:"max_size_mb"`
}

// TaskType categorizes the user prompt for agent routing.
type TaskType string

const (
	TaskTypeExplore TaskType = "explore"
	TaskTypePlan    TaskType = "plan"
	TaskTypeCode    TaskType = "code"
)

// AgentRoutingConfig maps task types to profile names in devon.toml.
type AgentRoutingConfig struct {
	ExploreProfile string `toml:"explore_profile"`
	PlanProfile    string `toml:"plan_profile"`
	CodeProfile    string `toml:"code_profile"`
}

// TomlConfig represents the structure of devon.toml.
type TomlConfig struct {
	Defaults struct {
		Profile string `toml:"profile"`
		Mode    string `toml:"mode"`
	} `toml:"defaults"`
	Profiles      []Profile           `toml:"profiles"`
	Sandbox       *SandboxConfig      `toml:"sandbox"`
	Index         *IndexConfig        `toml:"index"`
	Cache         *CacheConfig        `toml:"cache"`
	Attachments   *AttachmentsConfig  `toml:"attachments"`
	MCPServers    []MCPServerConfig   `toml:"mcp_servers"`
	AgentRouting  *AgentRoutingConfig `toml:"agent_routing"`
}

// ResolveAgentRouting resolves profile names in AgentRoutingConfig to actual Profiles.
// Returns a map of TaskType → Profile, and nil if no routing is configured.
// Returns an error if a referenced profile does not exist.
func ResolveAgentRouting(tc *TomlConfig) (map[TaskType]*Profile, error) {
	if tc == nil || tc.AgentRouting == nil {
		return nil, nil
	}

	routing := make(map[TaskType]*Profile)
	entries := map[TaskType]string{
		TaskTypeExplore: tc.AgentRouting.ExploreProfile,
		TaskTypePlan:    tc.AgentRouting.PlanProfile,
		TaskTypeCode:    tc.AgentRouting.CodeProfile,
	}

	for tt, profileName := range entries {
		if profileName == "" {
			continue
		}
		p, err := ResolveProfile(tc, profileName)
		if err != nil {
			return nil, fmt.Errorf("agent_routing: perfil %q referenced by %q: %w", profileName, tt, err)
		}
		routing[tt] = p
	}

	return routing, nil
}

// LoadToml carrega devon.toml do diretório atual ou home (~/.devon.toml).
// Retorna nil sem erro se nenhum arquivo for encontrado.
func LoadToml() (*TomlConfig, error) {
	// Try current directory
	if data, err := os.ReadFile("devon.toml"); err == nil {
		return parseToml(data)
	}

	// Try home directory
	if home, err := os.UserHomeDir(); err == nil {
		path := filepath.Join(home, ".devon.toml")
		if data, err2 := os.ReadFile(path); err2 == nil {
			return parseToml(data)
		}
	}

	return nil, nil
}

func parseToml(data []byte) (*TomlConfig, error) {
	var tc TomlConfig
	if _, err := toml.Decode(string(data), &tc); err != nil {
		return nil, fmt.Errorf("falha ao parsear devon.toml: %w", err)
	}
	return &tc, nil
}

// ResolveProfile retorna o Profile pelo nome.
func ResolveProfile(tc *TomlConfig, name string) (*Profile, error) {
	if tc == nil {
		return nil, errors.New("nenhum perfil encontrado (toml não carregado)")
	}

	for _, p := range tc.Profiles {
		if p.Name == name {
			cp := p
			return &cp, nil
		}
	}
	return nil, fmt.Errorf("perfil %q não encontrado", name)
}

// ResolveAPIKey lê o valor da env var referenciada em APIKeyEnv.
func (p *Profile) ResolveAPIKey() string { return os.Getenv(p.APIKeyEnv) }

// ValidateProfile verifica BaseURL e Model obrigatórios.
func ValidateProfile(p *Profile) error {
	if p.Model == "" {
		return fmt.Errorf("perfil %q: model é obrigatório", p.Name)
	}
	if p.BaseURL == "" {
		return fmt.Errorf("perfil %q: base_url é obrigatório", p.Name)
	}
	return nil
}

// ApplyProfile aplica os valores do Profile ao Config existente.
func ApplyProfile(cfg *Config, p *Profile) error {
	cfg.BaseURL = p.BaseURL
	cfg.Model = p.Model
	cfg.APIKey = p.ResolveAPIKey()
	return nil
}
