// Package memory fornece uma camada de memória semântica para o agente.
// Persiste fatos sobre o projeto e recupera contexto relevante para o system prompt.
package memory

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ElioNeto/devon/internal/db"
)

// Manager gerencia fatos do projeto: persistência, recuperação e contexto para prompts.
type Manager struct {
	store     db.Store
	projectID string
}

// New cria um novo Manager com o store fornecido.
func New(store db.Store, projectID string) *Manager {
	return &Manager{
		store:     store,
		projectID: projectID,
	}
}

// Fact representa um fato simples para retorno das APIs de memória.
type Fact struct {
	ID      int64
	Category string
	Content string
}

// ContextFor retorna uma string Markdown com os fatos relevantes para
// injetar no system prompt. Busca na tabela facts por project_id filtrando
// por keywords do prompt (tokenização simples por espaço).
//
// projectID deve ser um hash SHA1 truncado de 8 chars do WorkDir.
func (m *Manager) ContextFor(ctx context.Context, projectID, prompt string) (string, error) {
	// Tokenizar prompt por espaço
	words := strings.Fields(strings.ToLower(prompt))
	if len(words) == 0 {
		return "", nil
	}

	// Buscar fatos que contenham alguma das palavras-chave
	var allFacts []db.FactRow
	seen := make(map[int64]bool)

	for _, word := range words {
		if len(word) < 2 {
			continue
		}

		facts, err := m.store.QueryFacts(ctx, projectID, word, 10)
		if err != nil {
			return "", fmt.Errorf("QueryFacts: %w", err)
		}

		for _, f := range facts {
			if !seen[f.ID] {
				seen[f.ID] = true
				allFacts = append(allFacts, f)
			}
		}
	}

	if len(allFacts) == 0 {
		return "", nil
	}

	// Agrupar por categoria
	b := &strings.Builder{}
	b.WriteString("## Memória do projeto\n")

	for _, f := range allFacts {
		b.WriteString(fmt.Sprintf("- %s: %s\n", f.Category, f.Content))
	}

	return b.String(), nil
}

// Remember salva um fato na tabela facts.
func (m *Manager) Remember(ctx context.Context, projectID, category, content string) error {
	if err := m.store.PutFact(ctx, projectID, category, content, ""); err != nil {
		return fmt.Errorf("PutFact: %w", err)
	}
	return nil
}

// Recall busca fatos por categoria e/ou keyword.
func (m *Manager) Recall(ctx context.Context, projectID, category, keyword string) ([]Fact, error) {
	factsRow, err := m.store.QueryFacts(ctx, projectID, keyword, 100)
	if err != nil {
		return nil, fmt.Errorf("QueryFacts: %w", err)
	}

	// Filtrar por categoria se fornecida
	if category != "" {
		var filtered []Fact
		for _, f := range factsRow {
			if f.Category == category {
				filtered = append(filtered, Fact{
					ID:       f.ID,
					Category: f.Category,
					Content:  f.Content,
				})
			}
		}
		return filtered, nil
	}

	var facts []Fact
	for _, f := range factsRow {
		facts = append(facts, Fact{
			ID:       f.ID,
			Category: f.Category,
			Content:  f.Content,
		})
	}

	return facts, nil
}

// Clear remove todos os fatos de um projeto.
func (m *Manager) Clear(ctx context.Context, projectID string) error {
	if err := m.store.DeleteFacts(ctx, projectID); err != nil {
		return fmt.Errorf("DeleteFacts: %w", err)
	}
	return nil
}

// ProjectIDFromWorkDir retorna um hash SHA1 truncado de 8 caracteres do caminho do workdir.
func ProjectIDFromWorkDir(workDir string) string {
	h := sha1.Sum([]byte(workDir))
	return hex.EncodeToString(h[:])[:8]
}
