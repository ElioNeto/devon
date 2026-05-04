package cache

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/ElioNeto/devon/internal/config"
	"github.com/ElioNeto/devon/internal/llm"
)

// Cache gerencia o cache de respostas do LLM.
// Usa SQLite como armazenamento interno e aplica regras de TTL e
// invalidacao por modificacao de arquivos monitorados.
type Cache struct {
	store *SQLiteCacheStore
	cfg   config.CacheConfig
}

// New cria uma nova instancia de Cache.
// Recebe uma conexao SQLite ja aberta e as configuracoes de cache.
func New(db *sql.DB, cfg config.CacheConfig) (*Cache, error) {
	store, err := NewCacheStore(db)
	if err != nil {
		return nil, fmt.Errorf("cache: falha ao criar store: %w", err)
	}
	return &Cache{store: store, cfg: cfg}, nil
}

// GetResult é o resultado de uma consulta ao cache.
type GetResult struct {
	Response    string
	TokensSaved int
	Hit         bool
}

// Get consulta o cache por uma resposta previamente armazenada.
// Retorna um hit apenas se a entrada existir, nao estiver expirada,
// e os arquivos monitorados nao tiverem sido modificados.
func (c *Cache) Get(ctx context.Context, model string, messages []llm.Message, files []string) (*GetResult, error) {
	hashKey := HashKey(model, messages)

	entry, err := c.store.GetEntry(ctx, hashKey)
	if err != nil {
		return nil, fmt.Errorf("cache: erro ao buscar entrada: %w", err)
	}
	if entry == nil {
		return &GetResult{Hit: false}, nil
	}

	// Verifica TTL
	if entry.ExpiresAt != nil && time.Now().UTC().After(entry.ExpiresAt.UTC()) {
		return &GetResult{Hit: false}, nil
	}

	// Verifica modificacao de arquivos monitorados
	if len(files) > 0 && entry.FileHashes != "" {
		changed, err := filesChanged(entry.FileHashes, files)
		if err != nil {
			return nil, fmt.Errorf("cache: erro ao verificar arquivos: %w", err)
		}
		if changed {
			return &GetResult{Hit: false}, nil
		}
	}

	return &GetResult{
		Response:    entry.Response,
		TokensSaved: entry.TokensSaved,
		Hit:         true,
	}, nil
}

// Set armazena uma resposta no cache.
// files sao caminhos de arquivos cuja modificacao deve invalidar este cache.
// tokensSaved indica quantos tokens foram economizados (tipicamente os tokens
// de prompt da requisicao).
func (c *Cache) Set(ctx context.Context, model string, messages []llm.Message, response string, tokensSaved int, files []string) error {
	hashKey := HashKey(model, messages)

	var fileHashes string
	if len(files) > 0 {
		fh, err := computeFileHashes(files)
		if err != nil {
			return fmt.Errorf("cache: erro ao computar hashes de arquivos: %w", err)
		}
		fileHashes = fh
	}

	var expiresAt *time.Time
	if c.cfg.TTL != "" {
		d, err := time.ParseDuration(c.cfg.TTL)
		if err == nil && d > 0 {
			t := time.Now().UTC().Add(d)
			expiresAt = &t
		}
	}

	entry := &CacheEntry{
		HashKey:     hashKey,
		Model:       model,
		Response:    response,
		TokensSaved: tokensSaved,
		FileHashes:  fileHashes,
		ExpiresAt:   expiresAt,
	}

	if err := c.store.SetEntry(ctx, entry); err != nil {
		return fmt.Errorf("cache: erro ao salvar entrada: %w", err)
	}
	return nil
}

// Stats retorna estatisticas do cache.
func (c *Cache) Stats(ctx context.Context) (*CacheStats, error) {
	return c.store.GetStats(ctx)
}

// Clear remove todas as entradas do cache.
func (c *Cache) Clear(ctx context.Context) error {
	return c.store.Clear(ctx)
}

// DeleteExpired remove todas as entradas expiradas.
func (c *Cache) DeleteExpired(ctx context.Context) error {
	return c.store.DeleteExpired(ctx)
}

// filesChanged verifica se algum arquivo monitorado foi modificado
// comparando com os hashes armazenados.
func filesChanged(storedHashesJSON string, files []string) (bool, error) {
	var storedHashes map[string]string
	if err := json.Unmarshal([]byte(storedHashesJSON), &storedHashes); err != nil {
		return true, nil // Em caso de erro, assume que mudou
	}

	for _, path := range files {
		storedHash, ok := storedHashes[path]
		if !ok {
			return true, nil // Arquivo novo nao monitorado antes
		}
		currentHash, err := fileHash(path)
		if err != nil {
			return true, nil // Erro ao ler arquivo, assume mudanca
		}
		if currentHash != storedHash {
			return true, nil // Arquivo modificado
		}
	}
	return false, nil
}

// computeFileHashes computa hashes SHA-256 de uma lista de arquivos
// e retorna como JSON map[path]hash.
func computeFileHashes(files []string) (string, error) {
	hashes := make(map[string]string, len(files))
	for _, path := range files {
		h, err := fileHash(path)
		if err != nil {
			return "", err
		}
		hashes[path] = h
	}
	data, err := json.Marshal(hashes)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// fileHash computa o hash SHA-256 do conteudo de um arquivo.
func fileHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum), nil
}
