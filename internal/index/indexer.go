package index

import (
	"bufio"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"modernc.org/sqlite"
	"modernc.org/sqlite/sql"
)

// Config holds the indexer configuration.
type Config struct {
	// Extensions are the file extensions to index (without dots).
	// Default: [".go", ".md", ".txt", ".json", ".yaml", ".yml", ".toml"]
	Extensions []string

	// Excludes are paths/patterns to exclude from indexing.
	// Supports glob patterns and directory prefixes.
	// Default: [vendor/, node_modules/, .git/, dist/, build/, *.pb.go]
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

// DefaultConfig returns the default indexer configuration.
func DefaultConfig() Config {
	return Config{
		Extensions:    []string{".go", ".md", ".txt", ".json", ".yaml", ".yml", ".toml", ".js", ".ts", ".py", ".rb"},
		Excludes:      []string{"vendor/", "node_modules/", ".git/", "dist/", "build/", "*.pb.go", "third_party/"},
		MaxFileSizeKB: 500,
		TopK:          5,
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
	if c.CacheDir == "" {
		c.CacheDir = DefaultConfig().CacheDir
	}
}

// shouldExclude checks if a path should be excluded based on config.
func (c *Config) shouldExclude(path string) bool {
	for _, excl := range c.Excludes {
		if strings.HasSuffix(excl, "/") {
			// Directory prefix match
			if strings.HasPrefix(path, excl) {
				return true
			}
		} else if strings.Contains(excl, "*") {
			// Glob pattern
			pattern, err := filepath.Match(excl, filepath.Base(path))
			if err == nil && pattern {
				return true
			}
			// Also check full path
			pattern, err = filepath.Match(excl, path)
			if err == nil && pattern {
				return true
			}
		} else {
			// Exact match or substring
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
		if strings.HasPrefix(allowed, ".") {
			if ext == allowed {
				return true
			}
		} else {
			if ext == "."+allowed {
				return true
			}
		}
	}
	return false
}

// parseGitignore loads and parses .gitignore patterns.
func parseGitignore(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var patterns []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Remove trailing slashes for directory patterns
		line = strings.TrimSuffix(line, "/")
		patterns = append(patterns, line)
	}

	return patterns, scanner.Err()
}

// GitignoreMatcher matches paths against .gitignore patterns.
type GitignoreMatcher struct {
	patterns []string
}

// NewGitignoreMatcher creates a matcher from gitignore patterns.
func NewGitignoreMatcher(patterns []string) *GitignoreMatcher {
	return &GitignoreMatcher{patterns: patterns}
}

// ShouldIgnore checks if a path should be ignored.
func (m *GitignoreMatcher) ShouldIgnore(relPath string) bool {
	for _, pattern := range m.patterns {
		if shouldIgnorePath(pattern, relPath) {
			return true
		}
	}
	return false
}

func shouldIgnorePath(pattern, path string) bool {
	// Handle directory patterns (ending with /)
	if strings.HasSuffix(pattern, "/") {
		pattern = strings.TrimSuffix(pattern, "/")
		// Match as prefix
		if path == pattern || strings.HasPrefix(path, pattern+"/") {
			return true
		}
		// Match any directory named pattern
		if strings.Contains(path, "/"+pattern+"/") || strings.HasSuffix(path, "/"+pattern) {
			return true
		}
		return false
	}

	// Handle ** patterns (match any path segment)
	if strings.Contains(pattern, "**") {
		parts := strings.Split(pattern, "**")
		if len(parts) == 2 {
			start := strings.Trim(parts[0], "/")
			end := strings.Trim(parts[1], "/")
			if start != "" && end != "" {
				return strings.HasPrefix(path, start) && strings.HasSuffix(path, end)
			} else if start != "" {
				return strings.HasPrefix(path, start)
			} else if end != "" {
				return strings.HasSuffix(path, end)
			}
		}
	}

	// Handle simple patterns
	basename := filepath.Base(path)
	if pattern[0] == '^' {
		return pattern[1:] == basename
	}
	if strings.Contains(pattern, "*") {
		p, _ := filepath.Match(pattern, basename)
		return p
	}
	return pattern == basename || strings.Contains(path, pattern)
}

// Indexer handles file indexing with persistence.
type Indexer struct {
	config    Config
	dbPath    string
	db        *sql.DB
	indexPath *Index
}

// NewIndexer creates a new indexer with the given config.
func NewIndexer(cacheDir string, projectID string) (*Indexer, error) {
	config := DefaultConfig()

	// Ensure cache directory exists
	if cacheDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("cannot determine home directory: %w", err)
		}
		cacheDir = filepath.Join(home, ".devon", "index")
	}

	projectHash := projectHash(projectID)
	dbPath := filepath.Join(cacheDir, fmt.Sprintf("%s.db", projectHash))

	return &Indexer{
		config:    config,
		dbPath:    dbPath,
		indexPath: NewIndex(),
	}, nil
}

// projectHash generates a hash from the project path.
func projectHash(path string) string {
	h := sha256.Sum256([]byte(path))
	return fmt.Sprintf("%x", h[:16]) // Truncate to 16 hex chars
}

// Load loads an existing index from disk, or creates a new one.
func (i *Indexer) Load() error {
	if _, err := os.Stat(i.dbPath); os.IsNotExist(err) {
		return i.createSchema()
	}

	db, err := i.openDB()
	if err != nil {
		return err
	}

	i.db = db
	return i.loadFromDB()
}

// openDB opens an existing SQLite database.
func (i *Indexer) openDB() (*sql.DB, error) {
	db, err := sql.OpenFile(
		i.dbPath,
		sql.WithAutoprocess(true),
		sql.WithBusyTimeout(5000),
		sql.WithJournalMode(sql.JournalModeWal),
	)
	if err != nil {
		return nil, fmt.Errorf("cannot open index db: %w", err)
	}

	return db, nil
}

// loadFromDB loads index data from the database.
func (i *Indexer) loadFromDB() error {
	if i.db == nil {
		return fmt.Errorf("database not opened")
	}

	// Load documents
	rows, err := i.db.Query("SELECT id, path, modified, word_len FROM index_docs")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var doc Document
		var modified int64
		var wordLen sql.NullFloat64

		if err := rows.Scan(&doc.ID, &doc.Path, &modified, &wordLen); err != nil {
			return err
		}

		doc.Modified = modified
		if wordLen.Valid {
			doc.WordLen = wordLen.Float64
		}

		// Load tokens for this document
		tokens, err := i.loadTokens(doc.ID)
		if err != nil {
			return err
		}
		doc.Tokens = tokens
		doc.Length = len(tokens)

		i.indexedDocuments[doc.ID] = &doc
		i.docIDByPath[doc.Path] = doc.ID

		if wordLen.Valid {
			i.corpusLen += wordLen.Float64
		}
	}

	// Load term frequencies
	rows, err = i.db.Query(`
		SELECT doc_id, term, freq
		FROM index_term_freq
		ORDER BY doc_id
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var docID, term string
		var freq int

		if err := rows.Scan(&docID, &term, &freq); err != nil {
			return err
		}

		if i.termFreqs[docID] == nil {
			i.termFreqs[docID] = make(map[string]int)
		}
		i.termFreqs[docID][term]++
		i.docFreqs[term]++
	}

	i.totalDocs = i.indexedDocuments.Len()
	return nil
}

// loadTokens loads tokens for a document.
func (i *Indexer) loadTokens(docID string) ([]Token, error) {
	tokens := make([]Token, 0)

	rows, err := i.db.Query(`
		SELECT text, offset, position
		FROM index_tokens
		WHERE doc_id = ?
		ORDER BY position
	`, docID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var tok Token
		if err := rows.Scan(&tok.Text, &tok.Offset, &tok.Position); err != nil {
			return nil, err
		}
		tokens = append(tokens, tok)
	}

	return tokens, rows.Err()
}

// createSchema creates the database schema.
func (i *Indexer) createSchema() error {
	db, err := sql.Open(":memory:",
		sql.WithAutoprocess(true),
	)
	if err != nil {
		return err
	}

	schema := `
		CREATE TABLE index_docs (
			id TEXT PRIMARY KEY,
			path TEXT UNIQUE NOT NULL,
			modified INTEGER NOT NULL,
			word_len REAL
		);

		CREATE TABLE index_tokens (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			doc_id TEXT NOT NULL,
			text TEXT NOT NULL,
			offset INTEGER NOT NULL,
			position INTEGER NOT NULL,
			FOREIGN KEY (doc_id) REFERENCES index_docs(id) ON DELETE CASCADE
		);

		CREATE TABLE index_term_freq (
			doc_id TEXT NOT NULL,
			term TEXT NOT NULL,
			freq INTEGER NOT NULL,
			PRIMARY KEY (doc_id, term),
			FOREIGN KEY (doc_id) REFERENCES index_docs(id) ON DELETE CASCADE
		);

		CREATE TABLE index_metadata (
			key TEXT PRIMARY KEY,
			value TEXT
		);

		CREATE INDEX idx_tokens_doc ON index_tokens(doc_id);
		CREATE INDEX idx_term_freq_term ON index_term_freq(term);
		CREATE INDEX idx_docs_path ON index_docs(path);
	`

	_, err = db.Exec(schema)
	if err != nil {
		db.Close()
		return fmt.Errorf("create schema: %w", err)
	}

	db.Close()

	// Now create the actual file
	if err := os.MkdirAll(filepath.Dir(i.dbPath), 0755); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}

	i.db, err = sql.OpenFile(
		i.dbPath,
		sql.WithAutoprocess(true),
		sql.WithBusyTimeout(5000),
		sql.WithJournalMode(sql.JournalModeWal),
	)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}

	// Execute schema
	execSQL, err := i.db.Prepare("EXEC " + strings.ReplaceAll(schema, ";", "; EXEC "))
	if err != nil {
		return fmt.Errorf("prepare schema: %w", err)
	}

	stmts := strings.Split(schema, ";")
	for _, stmt := range stmts {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		// Replace CREATE TABLE... AS with CREATE TABLE AS
		stmt = regexp.MustCompile(`\s+AS\s+`).ReplaceAllString(stmt, " AS ")
		if _, err := i.db.Exec(stmt); err != nil {
			i.db.Close()
			return fmt.Errorf("exec schema: %w", err)
		}
	}

	return nil
}

// Save persists the index to disk.
func (i *Indexer) Save() error {
	if i.db == nil {
		return fmt.Errorf("database not initialized")
	}

	// Start transaction
	_, err := i.db.Exec("BEGIN TRANSACTION")
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	// Save documents
	for docID, doc := range i.indexedDocuments {
		_, err = i.db.ExecContext(`
			INSERT OR REPLACE INTO index_docs (id, path, modified, word_len)
			VALUES (?, ?, ?, ?)
		`, docID, doc.Path, doc.Modified, doc.WordLen)
		if err != nil {
			i.db.Exec("ROLLBACK")
			return fmt.Errorf("save doc: %w", err)
		}

		// Delete existing tokens
		_, err = i.db.ExecContext("DELETE FROM index_tokens WHERE doc_id = ?", docID)
		if err != nil {
			i.db.Exec("ROLLBACK")
			return fmt.Errorf("clear tokens: %w", err)
		}

		// Insert tokens (batch via multiple inserts)
		for _, tok := range doc.Tokens {
			_, err = i.db.ExecContext(`
				INSERT INTO index_tokens (doc_id, text, offset, position)
				VALUES (?, ?, ?, ?)
			`, docID, tok.Text, tok.Offset, tok.Position)
			if err != nil {
				i.db.Exec("ROLLBACK")
				return fmt.Errorf("save token: %w", err)
			}
		}
	}

	// Clear and re-insert term frequencies
	_, err = i.db.Exec("DELETE FROM index_term_freq")
	if err != nil {
		i.db.Exec("ROLLBACK")
		return fmt.Errorf("clear term freq: %w", err)
	}

	for docID, terms := range i.termFreqs {
		for term, freq := range terms {
			_, err = i.db.ExecContext(`
				INSERT INTO index_term_freq (doc_id, term, freq)
				VALUES (?, ?, ?)
			`, docID, term, freq)
			if err != nil {
				i.db.Exec("ROLLBACK")
				return fmt.Errorf("save term freq: %w", err)
			}
		}
	}

	// Commit
	if _, err := i.db.Exec("COMMIT"); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	// Vacuum to reduce file size
	if _, err := i.db.Exec("VACUUM"); err != nil {
		// Don't fail on vacuum
	}

	return nil
}

// AddFile indexes a single file. Returns true if the file was indexed.
func (i *Indexer) AddFile(ctx context.Context, path string, content string) (bool, error) {
	if i.shouldIndex(path) && !i.shouldExclude(path) {
		// Check file size
		if len(content) > i.config.MaxFileSizeKB*1024 {
			return false, nil
		}

		doc := &Document{
			ID:   path,
			Path: path,
		}

		// Tokenize content
		tokenizer := NewTokenizer()
		doc.Tokens = tokenizer.Tokenize(content)
		if len(doc.Tokens) == 0 {
			return false, nil
		}

		// Calculate modified time
		info, err := os.Stat(path)
		if err != nil {
			doc.Modified = time.Now().Unix()
		} else {
			doc.Modified = info.ModTime().Unix()
		}

		// Index the document
		i.indexedDocuments[path] = doc
		i.docIDByPath[path] = path

		for _, tok := range doc.Tokens {
			if !i.isStopword(tok.Text) {
				if i.termFreqs[path] == nil {
					i.termFreqs[path] = make(map[string]int)
				}
				i.termFreqs[path][tok.Text]++
				i.docFreqs[tok.Text]++
			}
		}

		i.totalDocs++
		i.corpusLen += doc.WordLen

		// Auto-save after batch
		const batchSizes = 100
		if i.totalDocs%batchSizes == 0 {
			_ = i.Save()
		}

		return true, nil
	}

	return false, nil
}

// RemoveFile removes a file from the index.
func (i *Indexer) RemoveFile(path string) {
	if docID, exists := i.docIDByPath[path]; exists {
		i.removeDocument(docID)
		delete(i.docIDByPath, path)
	}
}

// removeDocument removes a document from the internal index.
func (i *Indexer) removeDocument(docID string) {
	if doc, exists := i.indexedDocuments[docID]; exists {
		// Remove from term frequencies
		for term := range i.termFreqs[docID] {
			i.docFreqs[term]--
			if i.docFreqs[term] == 0 {
				delete(i.docFreqs, term)
			}
		}

		delete(i.termFreqs, docID)
		delete(i.indexedDocuments, docID)
		i.totalDocs--
		i.corpusLen -= doc.WordLen
	}
}

// Rebuild completely rebuilds the index from scratch.
func (i *Indexer) Rebuild(ctx context.Context, workdir string) error {
	// Close existing database
	if i.db != nil {
		i.db.Close()
		i.db = nil
	}

	// Clear in-memory index
	i.indexedDocuments = make(map[string]*Document)
	i.docIDByPath = make(map[string]string)
	i.termFreqs = make(map[string]map[string]int)
	i.docFreqs = make(map[string]int)
	i.totalDocs = 0
	i.corpusLen = 0

	// Create fresh database
	if err := i.createSchema(); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}

	// Walk directory and index files
	return i.walkDir(ctx, workdir, workdir)
}

// walkDir recursively walks a directory and indexes files.
func (i *Indexer) walkDir(ctx context.Context, workdir string, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil // Skip unreadable directories
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

		// Check exclusions
		if i.shouldExclude(relPath) {
			continue
		}

		if entry.IsDir() {
			// Check for .gitignore in subdirectory
			gitignorePath := filepath.Join(path, ".gitignore")
			if _, err := os.Stat(gitignorePath); err == nil {
				patterns, err := parseGitignore(gitignorePath)
				if err == nil {
					matcher := NewGitignoreMatcher(patterns)
					// Add this matcher to a global list (simplified)
				}
			}
			continue
		}

		// Index file
		if _, err := os.Stat(path); err != nil {
			continue
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Continue
		}
	}

	return nil
}

// IndexDir indexes all files in a directory.
func (i *Indexer) IndexDir(ctx context.Context, workdir string) error {
	return i.indexDirRecursive(ctx, workdir, workdir, make(map[string]bool))
}

func (i *Indexer) indexDirRecursive(ctx context.Context, workdir, dir string, visited map[string]bool) error {
	// Avoid infinite loops on symlinks
	if realPath, err := filepath.EvalSymlinks(dir); err == nil {
		if visited[realPath] {
			return nil
		}
		visited[realPath] = true
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		if ctx.Err() != nil {
			return ctxErr()
		}

		path := filepath.Join(dir, entry.Name())
		relPath := filepath.ToSlash(filepath.Rel(workdir, path))

		// Check exclusions
		if i.shouldExclude(relPath) {
			if entry.IsDir() {
				continue
			}
		}

		if entry.IsDir() {
			_ = filepath.WalkDir(path, func(walkPath string, d fs.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				if d.IsDir() {
					return nil
				}
				rel, _ := filepath.Rel(workdir, walkPath)
				if i.shouldExclude(rel) {
					return nil
				}
				return indexSingleFile(ctx, i, walkPath, rel)
			})
			continue
		}

		_ = indexSingleFile(ctx, i, path, relPath)
	}

	return nil
}

func indexSingleFile(ctx context.Context, indexer *Indexer, path, relPath string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		if len(content) > int(indexer.config.MaxFileSizeKB*1024) {
			return nil
		}

		_, _ = indexer.AddFile(ctx, relPath, string(content))
		_ = indexer.Save()
		return nil
	}
}

// Close closes the database connection.
func (i *Indexer) Close() error {
	if i.db != nil {
		return i.db.Close()
	}
	return nil
}

// GetStats returns indexer statistics.
func (i *Indexer) GetStats() Stats {
	return Stats{
		TotalDocs:   i.totalDocs,
		TermCount:   len(i.docFreqs),
		DBPath:      i.dbPath,
		IsIndexed:   i.db != nil,
	}
}

// Stats holds indexer statistics.
type Stats struct {
	TotalDocs int    `json:"total_docs"`
	TermCount int    `json:"term_count"`
	DBPath    string `json:"db_path"`
	IsIndexed bool   `json:"is_indexed"`
}
