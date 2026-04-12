package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ElioNeto/devon/internal/db"
	"github.com/ElioNeto/devon/internal/permissions"
	"github.com/ElioNeto/devon/internal/tools"
)

// RememberTool salva fatos sobre o projeto na memória semântica.
type RememberTool struct {
	DB db.Store
}

func (t *RememberTool) Name() string { return "remember" }
func (t *RememberTool) Permission() permissions.PermissionLevel {
	return permissions.PermWrite
}
func (t *RememberTool) Description() string {
	return `Salva um fato sobre o projeto para futura recuperação. Utiliza para memorizar:
- Estrutura de arquivos e diretórios
- Padrões de arquitetura
- Decisões técnicas importantes
- Convenções de código
- Configurações específicas do projeto

Exemplos:
- "O projeto usa Next.js 13 com App Router"
- "Todos os componentes usam Tailwind CSS para estilização"
- "O banco de dados é PostgreSQL com migrations controladas por Prisma"
`
}

type rememberParams struct {
	Category string `json:"category"`
	Content  string `json:"content"`
	Context  string `json:"context,omitempty"`
}

func (t *RememberTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"category": {
				"type": "string",
				"description": "Categoria do fato (ex: 'arquitetura', 'config', 'conven', 'estrutura')"
			},
			"content": {
				"type": "string",
				"description": "O fato a ser memorizado, em linguagem natural"
			},
			"context": {
				"type": "string",
				"description": "Contexto adicional opcional para o fato"
			}
		},
		"required": ["category", "content"]
	}`)
}

func (t *RememberTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p rememberParams
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("remember: parametros invalidos: %w", err)
	}
	if p.Category == "" {
		return "", fmt.Errorf("remember: categoria nao pode estar vazia")
	}
	if p.Content == "" {
		return "", fmt.Errorf("remember: conteudo nao pode estar vazio")
	}

	if err := t.DB.PutFact(ctx, "default", p.Category, p.Content, p.Context); err != nil {
		return "", fmt.Errorf("remember: nao foi possivel salvar fato: %w", err)
	}

	return fmt.Sprintf("Fato salvo com sucesso na categoria '%s': %s", p.Category, truncate(p.Content, 100)), nil
}

type forgetParams struct {
	Category string `json:"category,omitempty"`
	Content  string `json:"content,omitempty"`
}

// ForgetTool remove fatos salvos.
type ForgetTool struct {
	DB db.Store
}

func (t *ForgetTool) Name() string { return "forget" }
func (t *ForgetTool) Permission() permissions.PermissionLevel {
	return permissions.PermWrite
}
func (t *ForgetTool) Description() string {
	return `Remove fatos salvos da memória semântica.

Exemplos:
- Remove todos os fatos de uma categoria
- Remove um fato específico pelo conteúdo
`
}

func (t *ForgetTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"category": {
				"type": "string",
				"description": "Categoria dos fatos a serem removidos"
			},
			"content": {
				"type": "string",
				"description": "Conteúdo exato do fato a ser removido (parcial é aceito)"
			}
		}
	}`)
}

func (t *ForgetTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p forgetParams
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("forget: parametros invalidos: %w", err)
	}

	if p.Category == "" && p.Content == "" {
		return "", fmt.Errorf("forget: especifique categoria ou conteudo para remover")
	}

	facts, err := t.DB.GetFacts(ctx, "default", "", 1000)
	if err != nil {
		return "", fmt.Errorf("forget: Falha ao buscar fatos: %w", err)
	}

	deleted := 0
	for _, f := range facts {
		match := false
		if p.Category != "" && f.Category == p.Category {
			match = true
		}
		if p.Content != "" && strings.Contains(f.Content, p.Content) {
			match = true
		}
		if match {
			if err := t.DB.PutFact(ctx, f.ProjectID, f.Category, "", ""); err != nil {
				return "", fmt.Errorf("forget: %w", err)
			}
			deleted++
		}
	}

	if deleted == 0 {
		return "Nenhum fato encontrado para remover", nil
	}

	return fmt.Sprintf("%d fato(s) removido(s)", deleted), nil
}

type RecallTool struct {
	DB db.Store
}

func (t *RecallTool) Name() string { return "recall" }
func (t *RecallTool) Permission() permissions.PermissionLevel {
	return permissions.PermRead
}
func (t *RecallTool) Description() string {
	return `Recupera fatos relevantes da memória semântica.

Exemplos:
- recall --category "arquitetura" -> busca fatos sobre arquitetura
- recall --query "next.js" -> busca fatos contendo next.js
- recall --all -> lista todos os fatos
`
}

func (t *RecallTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"category": {
				"type": "string",
				"description": "Categoria específica para buscar"
			},
			"query": {
				"type": "string",
				"description": "Texto para busca parcial no conteúdo"
			},
			"all": {
				"type": "boolean",
				"description": "Se true, retorna todos os fatos"
			},
			"limit": {
				"type": "integer",
				"description": "Número máximo de fatos a retornar (default 20)"
			}
		}
	}`)
}

func (t *RecallTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p struct {
		Category string `json:"category"`
		Query    string `json:"query"`
		All      bool   `json:"all"`
		Limit    int    `json:"limit"`
	}

	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("recall: parametros invalidos: %w", err)
	}

	if p.Limit <= 0 {
		p.Limit = 20
	}

	facts, err := t.DB.GetFacts(ctx, "default", p.Category, p.Limit)
	if err != nil {
		return "", fmt.Errorf("recall: Falha ao buscar fatos: %w", err)
	}

	// Filtra por query se especificada
	if p.Query != "" {
		filtered := make([]db.Fact, 0)
		for _, f := range facts {
			if strings.Contains(f.Content, p.Query) || (f.Context != "" && strings.Contains(f.Context, p.Query)) {
				filtered = append(filtered, f)
			}
		}
		facts = filtered
	}

	if len(facts) == 0 {
		if p.All {
			return "Sem fatos salvos", nil
		}
		msg := "Nenhum fato encontrado"
		if p.Category != "" {
			msg += " para esta categoria"
		} else if p.Query != "" {
			msg += " para sua busca"
		}
		return msg, nil
	}

	var sb strings.Builder
	for i, f := range facts {
		sb.WriteString(fmt.Sprintf("%d. [%s] %s", i+1, f.Category, f.Content))
		if i < len(facts)-1 {
			sb.WriteString("\n")
		}
	}

	return sb.String(), nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func RegisterMemoryTools(r *tools.Registry, store db.Store) {
	r.Register(&RememberTool{DB: store})
	r.Register(&ForgetTool{DB: store})
	r.Register(&RecallTool{DB: store})
}
