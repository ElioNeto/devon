package cache

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// CacheEntry representa uma entrada no cache de respostas.
type CacheEntry struct {
	HashKey    string
	Model      string
	Response   string
	TokensSaved int
	FileHashes string // JSON contendo map[filePath]modTime
	CreatedAt  time.Time
	ExpiresAt  *time.Time
}

// CacheStats contem estatisticas do cache.
type CacheStats struct {
	TotalEntries int
	TotalTokens  int64
	ExpiredCount int
}

// SQLiteCacheStore implementa armazenamento de cache usando SQLite.
type SQLiteCacheStore struct {
	db *sql.DB
}

// NewCacheStore cria uma nova instancia de SQLiteCacheStore.
// O schema da tabela cache_entries e migrado automaticamente.
func NewCacheStore(db *sql.DB) (*SQLiteCacheStore, error) {
	if err := migrateCacheSchema(db); err != nil {
		return nil, err
	}
	return &SQLiteCacheStore{db: db}, nil
}

func migrateCacheSchema(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS cache_entries (
			hash_key TEXT PRIMARY KEY,
			model TEXT NOT NULL,
			response TEXT NOT NULL,
			tokens_saved INTEGER DEFAULT 0,
			file_hashes TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			expires_at DATETIME
		);
		CREATE INDEX IF NOT EXISTS idx_cache_entries_expires ON cache_entries(expires_at);
	`)
	return err
}

// GetEntry busca uma entrada do cache pela hash_key.
// Retorna nil sem erro se a chave nao existir.
func (s *SQLiteCacheStore) GetEntry(ctx context.Context, hashKey string) (*CacheEntry, error) {
	entry := &CacheEntry{}
	var expiresAt sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT hash_key, model, response, tokens_saved, file_hashes, created_at, expires_at
		 FROM cache_entries WHERE hash_key = ?`, hashKey,
	).Scan(&entry.HashKey, &entry.Model, &entry.Response, &entry.TokensSaved,
		&entry.FileHashes, &entry.CreatedAt, &expiresAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if expiresAt.Valid {
		t, err := time.Parse(time.RFC3339Nano, expiresAt.String)
		if err != nil {
			return nil, fmt.Errorf("cache: erro ao fazer parse de expires_at %q: %w", expiresAt.String, err)
		}
		entry.ExpiresAt = &t
	}
	return entry, nil
}

// SetEntry insere ou substitui uma entrada no cache.
func (s *SQLiteCacheStore) SetEntry(ctx context.Context, entry *CacheEntry) error {
	var expiresAt *string
	if entry.ExpiresAt != nil {
		s := entry.ExpiresAt.UTC().Format(time.RFC3339Nano)
		expiresAt = &s
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO cache_entries
		 (hash_key, model, response, tokens_saved, file_hashes, created_at, expires_at)
		 VALUES (?, ?, ?, ?, ?, datetime('now'), ?)`,
		entry.HashKey, entry.Model, entry.Response, entry.TokensSaved,
		entry.FileHashes, expiresAt,
	)
	return err
}

// DeleteExpired remove todas as entradas expiradas do cache.
func (s *SQLiteCacheStore) DeleteExpired(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM cache_entries WHERE expires_at IS NOT NULL AND expires_at <= datetime('now')`)
	return err
}

// GetStats retorna estatisticas do cache.
func (s *SQLiteCacheStore) GetStats(ctx context.Context) (*CacheStats, error) {
	stats := &CacheStats{}

	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*), COALESCE(SUM(tokens_saved), 0) FROM cache_entries`,
	).Scan(&stats.TotalEntries, &stats.TotalTokens)
	if err != nil {
		return nil, err
	}

	err = s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM cache_entries WHERE expires_at IS NOT NULL AND expires_at <= datetime('now')`,
	).Scan(&stats.ExpiredCount)
	if err != nil {
		return nil, err
	}

	return stats, nil
}

// Clear remove todas as entradas do cache.
func (s *SQLiteCacheStore) Clear(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM cache_entries`)
	return err
}
