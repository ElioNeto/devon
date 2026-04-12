package index

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// ManagerConfig holds manager configuration.
type ManagerConfig struct {
	// Enabled controls whether indexing is active.
	Enabled bool

	// Embed IndexerConfig for other settings
	IndexedConfig
}

// IndexedConfig holds the indexer configuration.
type IndexedConfig struct {
	// Extensions are the file extensions to index (with dots).
	Extensions []string

	// Excludes are paths/patterns to exclude from indexing.
	Excludes []string

	// MaxFileSizeKB is the maximum file size to index (in KB).
	MaxFileSizeKB int

	// CacheDir is the directory where the index is stored.
	CacheDir string

	// TopK is the default number of results to return for search queries.
	TopK int
}

// Manager coordinates indexing and searching for the Devon agent.
type Manager struct {
	indexer *Indexer
	config  ManagerConfig
}

// NewManager creates a new index manager with the given configuration.
func NewManager(workDir string, config ManagerConfig) (*Manager, error) {
	// Get home directory for cache
	cacheDir := config.CacheDir
	if cacheDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("cannot determine home directory: %w", err)
		}
		cacheDir = filepath.Join(home, ".devon", "index")
	}

	// Ensure cache directory exists
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("create cache directory: %w", err)
	}

	indexer, err := NewIndexer(workDir, indexerConfigFromManagerConfig(config))
	if err != nil {
		return nil, fmt.Errorf("create indexer: %w", err)
	}

	return &Manager{
		indexer: indexer,
		config:  config,
	}, nil
}

// indexerConfigFromManagerConfig converts ManagerConfig to Indexer.Config.
func indexerConfigFromManagerConfig(mc ManagerConfig) Config {
	return Config{
		Extensions:    mc.Extensions,
		Excludes:      mc.Excludes,
		MaxFileSizeKB: mc.MaxFileSizeKB,
		CacheDir:      mc.CacheDir,
		TopK:          mc.TopK,
	}
}

// Enable enables indexing for this manager.
func (m *Manager) Enable() {
	m.config.Enabled = true
}

// Disable disables indexing for this manager.
func (m *Manager) Disable() {
	m.config.Enabled = false
}

// IsEnabled returns whether indexing is enabled.
func (m *Manager) IsEnabled() bool {
	return m.config.Enabled
}

// Index indexes all files in the work directory.
func (m *Manager) Index(ctx context.Context, workDir string) error {
	if !m.config.Enabled {
		return nil
	}
	return m.indexer.IndexDir(ctx, workDir)
}

// Rebuild rebuilds the index from scratch.
func (m *Manager) Rebuild(ctx context.Context, workDir string) error {
	if !m.config.Enabled {
		return nil
	}
	return m.indexer.Rebuild(ctx, workDir)
}

// Search performs a semantic search.
func (m *Manager) Search(query string, topK int) ([]DocumentWithScore, error) {
	if !m.config.Enabled {
		return nil, nil
	}
	return m.indexer.Search(query, topK), nil
}

// GetStats returns indexer statistics.
func (m *Manager) GetStats() Stats {
	if m.indexer == nil {
		return Stats{}
	}
	return m.indexer.GetStats()
}

// GetIndex returns the underlying index for direct access.
func (m *Manager) GetIndex() *Index {
	if m.indexer == nil {
		return nil
	}
	return m.indexer.GetIndex()
}

// CreateTool creates a search_codebase tool for registration.
func (m *Manager) CreateTool() *SearchCodebaseTool {
	topK := m.config.TopK
	if topK <= 0 {
		topK = 5
	}
	return NewSearchCodebaseTool(m.indexer, topK)
}

// Close closes the manager and releases resources.
func (m *Manager) Close() error {
	if m.indexer != nil {
		return m.indexer.Close()
	}
	return nil
}
