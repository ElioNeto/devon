package index

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Config holds the indexer configuration.
type Config struct {
	// Extensions are the file extensions to index (with dots).
	// Example: [".go", ".md", ".txt"]
	Extensions []string

	// Excludes are paths/patterns to exclude from indexing.
	// Example: ["vendor/", "node_modules/", "*.pb.go"]
	Excludes []string

	// MaxFileSizeKB is the maximum file size to index (in KB).
	// Files larger than this are skipped. Default: 500 KB.
	MaxFileSizeKB int

	// CacheDir is the directory where the index is stored.
	// Default: ~/.devon/index
	CacheDir string

	// TopK is the default number of results to return for search queries.
	// Default: 5
	TopK int
}

// DefaultConfig returns a Config populated with sensible defaults:
//   - Extensions: .go, .md, .txt, .json, .yaml, .yml, .toml, .js, .ts, .py, .rb
//   - Excludes: vendor/, node_modules/, .git/, dist/, build/, *.pb.go, third_party/
//   - MaxFileSizeKB: 500
//   - TopK: 5
func DefaultConfig() Config {
	return Config{
		Extensions:    []string{".go", ".md", ".txt", ".json", ".yaml", ".yml", ".toml", ".js", ".ts", ".py", ".rb"},
		Excludes:      []string{"vendor/", "node_modules/", ".git/", "dist/", "build/", "*.pb.go", "third_party/"},
		MaxFileSizeKB: 500,
		TopK:          5,
		CacheDir:      "", // Will default to ~/.devon/index
	}
}

// WithDefaults applies the default configuration values to c.
func (c *Config) WithDefaults() {
	if len(c.Extensions) == 0 {
		c.Extensions = DefaultConfig().Extensions
	}
	if len(c.Excludes) == 0 {
		c.Excludes = DefaultConfig().Excludes
	}
	if c.MaxFileSizeKB == 0 {
		c.MaxFileSizeKB = DefaultConfig().MaxFileSizeKB
	}
	if c.TopK == 0 {
		c.TopK = DefaultConfig().TopK
	}
}

// shouldExclude checks if a path should be excluded based on config.
func (c *Config) shouldExclude(path string) bool {
	for _, excl := range c.Excludes {
		if strings.HasSuffix(excl, "/") {
			if strings.HasPrefix(path, excl) {
				return true
			}
		} else if strings.Contains(excl, "*") {
			pattern := strings.ReplaceAll(excl, "*", ".*")
			pattern = "^" + pattern + "$"
			if matched, _ := regexp.MatchString(pattern, filepath.Base(path)); matched {
				return true
			}
			if matched, _ := regexp.MatchString(pattern, path); matched {
				return true
			}
		} else {
			if strings.Contains(path, excl) {
				return true
			}
		}
	}
	return false
}

// shouldIndex checks if a file extension should be indexed.
func (c *Config) shouldIndex(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	for _, allowed := range c.Extensions {
		if ext == strings.ToLower(allowed) {
			return true
		}
	}
	return false
}

// Indexer handles file indexing with persistence.
type Indexer struct {
	config    Config
	indexPath *Index
}

// NewIndexer creates a new indexer for the given work directory and config.
func NewIndexer(workDir string, config Config) (*Indexer, error) {
	config.WithDefaults()

	return &Indexer{
		config:    config,
		indexPath: NewIndex(),
	}, nil
}

// AddFile indexes a single file. Returns true if the file was indexed.
func (i *Indexer) AddFile(ctx context.Context, path string, content string) (bool, error) {
	if i.config.shouldIndex(path) && !i.config.shouldExclude(path) {
		if len(content) > i.config.MaxFileSizeKB*1024 {
			return false, nil
		}

		doc := &Document{
			ID:   path,
			Path: path,
		}

		tokenizer := NewTokenizer()
		doc.Tokens = tokenizer.Tokenize(content)
		if len(doc.Tokens) == 0 {
			return false, nil
		}

		if info, err := os.Stat(path); err == nil {
			doc.Modified = info.ModTime().Unix()
		} else {
			doc.Modified = time.Now().Unix()
		}

		i.indexPath.Index(doc)

		return true, nil
	}

	return false, nil
}

// RemoveFile removes a file from the index.
func (i *Indexer) RemoveFile(path string) {
	doc := i.indexPath.GetDocument(path)
	if doc != nil {
		i.indexPath.RemoveDocument(doc.ID)
	}
}

// Rebuild completely rebuilds the index from scratch.
func (i *Indexer) Rebuild(ctx context.Context, workdir string) error {
	i.indexPath = NewIndex()
	return i.findAndIndexFiles(ctx, workdir, workdir)
}

// findAndIndexFiles recursively finds and indexes files in a directory.
func (i *Indexer) findAndIndexFiles(ctx context.Context, workdir, dir string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		path := filepath.Join(dir, entry.Name())
		relPath, err := filepath.Rel(workdir, path)
		if err != nil {
			relPath = path
		}
		relPath = filepath.ToSlash(relPath)

		if i.config.shouldExclude(relPath) {
			if entry.IsDir() {
				continue
			}
		}

		if entry.IsDir() {
			_ = i.findAndIndexFiles(ctx, workdir, path)
			continue
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		if len(content) > i.config.MaxFileSizeKB*1024 {
			continue
		}

		_, _ = i.AddFile(ctx, relPath, string(content))
	}

	return nil
}

// IndexDir indexes all indexable files in a directory.
func (i *Indexer) IndexDir(ctx context.Context, workdir string) error {
	return i.Rebuild(ctx, workdir)
}

// Search queries the index and returns top-K results.
func (i *Indexer) Search(query string, topK int) []DocumentWithScore {
	if topK == 0 {
		topK = i.config.TopK
	}
	return i.indexPath.Search(query, topK)
}

// Close closes any resources held by the indexer.
func (i *Indexer) Close() error {
	return nil
}

// GetStats returns indexer statistics.
func (i *Indexer) GetStats() Stats {
	return Stats{
		TotalDocs: i.indexPath.TotalDocs(),
		TermCount: i.indexPath.TermCount(),
		IsIndexed: i.indexPath.TotalDocs() > 0,
	}
}

// Stats holds indexer statistics.
type Stats struct {
	TotalDocs int  `json:"total_docs"`
	TermCount int  `json:"term_count"`
	IsIndexed bool `json:"is_indexed"`
}

// GetIndex returns the underlying index for direct access.
func (i *Indexer) GetIndex() *Index {
	return i.indexPath
}

// SetConfig updates the indexer configuration.
func (i *Indexer) SetConfig(cfg Config) {
	i.config = cfg
}

// GetConfig returns the current configuration.
func (i *Indexer) GetConfig() Config {
	return i.config
}
