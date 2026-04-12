package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

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

// TomlConfig represents the structure of devon.toml.
type TomlConfig struct {
	Defaults struct {
		Profile string `toml:"profile"`
		Mode    string `toml:"mode"`
	} `toml:"defaults"`
	Profiles []Profile      `toml:"profiles"`
	Sandbox  *SandboxConfig `toml:"sandbox"`
	Index    *IndexConfig   `toml:"index"`
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
