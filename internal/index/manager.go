package index

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// ManagerConfig holds the configuration for the index Manager.
// It embeds IndexedConfig for indexer-specific settings like extensions and excludes.
type ManagerConfig struct {
	// Enabled controls whether indexing is active. When false,
	// Index, Rebuild, and Search are all no-ops.
	Enabled bool

	// IndexedConfig contains the lower-level indexer settings.
	IndexedConfig
}

// IndexedConfig holds the indexer configuration fields shared between ManagerConfig and Config.
type IndexedConfig struct {
	// Extensions are the file extensions to index (e.g. ".go", ".md", ".py").
	Extensions []string `toml:"extensions"`

	// Excludes are path prefixes or glob patterns to skip during indexing.
	Excludes []string `toml:"excludes"`

	// MaxFileSizeKB is the maximum file size to index (in KB). Default: 500.
	MaxFileSizeKB int `toml:"max_file_size_kb"`

	// CacheDir is the directory for storing index data. Default: ~/.devon/index.
	CacheDir string `toml:"cache_dir"`

	// TopK is the default number of search results to return. Default: 5.
	TopK int `toml:"top_k"`
}

// Manager is the high-level coordinator for codebase indexing and search.
// It wraps an Indexer and controls whether indexing is enabled. When disabled,
// all operations (Index, Rebuild, Search) are no-ops returning zero values.
type Manager struct {
	indexer *Indexer
	config  ManagerConfig
}

// NewManager creates a new Manager with the given workDir and config.
// It initialises the cache directory (defaulting to ~/.devon/index) and
// creates the underlying Indexer. Returns an error if the cache directory
// cannot be created.
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

// Enable activates indexing for this manager. Subsequent Index/Rebuild/Search
// calls will perform real work.
func (m *Manager) Enable() {
	m.config.Enabled = true
}

// Disable deactivates indexing. All subsequent Index/Rebuild/Search calls
// become no-ops returning zero values.
func (m *Manager) Disable() {
	m.config.Enabled = false
}

// IsEnabled returns whether indexing is currently enabled.
func (m *Manager) IsEnabled() bool {
	return m.config.Enabled
}

// Index indexes all files in the given work directory. This is an alias for
// Rebuild that preserves the same semantics. If the manager is disabled, it
// returns nil immediately.
func (m *Manager) Index(ctx context.Context, workDir string) error {
	if !m.config.Enabled {
		return nil
	}
	return m.indexer.IndexDir(ctx, workDir)
}

// Rebuild discards the current index and re-indexes all files in workDir.
// If the manager is disabled, it returns nil immediately.
func (m *Manager) Rebuild(ctx context.Context, workDir string) error {
	if !m.config.Enabled {
		return nil
	}
	return m.indexer.Rebuild(ctx, workDir)
}

// Search performs a BM25 semantic search over the indexed codebase.
// Returns matching documents with scores, or nil if disabled.
func (m *Manager) Search(query string, topK int) ([]DocumentWithScore, error) {
	if !m.config.Enabled {
		return nil, nil
	}
	return m.indexer.Search(query, topK), nil
}

// GetStats returns indexer statistics (total docs, term count, indexed status).
func (m *Manager) GetStats() Stats {
	if m.indexer == nil {
		return Stats{}
	}
	return m.indexer.GetStats()
}

// GetIndex returns the underlying Index for direct read access.
// Returns nil if the indexer has not been initialised.
func (m *Manager) GetIndex() *Index {
	if m.indexer == nil {
		return nil
	}
	return m.indexer.GetIndex()
}

// CreateTool returns a SearchCodebaseTool wired to this manager's indexer,
// ready for registration in the agent tool set.
func (m *Manager) CreateTool() *SearchCodebaseTool {
	topK := m.config.TopK
	if topK <= 0 {
		topK = 5
	}
	return NewSearchCodebaseTool(m.indexer, topK)
}

// Close releases the underlying indexer resources.
func (m *Manager) Close() error {
	if m.indexer != nil {
		return m.indexer.Close()
	}
	return nil
}
