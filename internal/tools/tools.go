// Package tools define a interface Tool e o registro central de ferramentas.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/ElioNeto/devon/internal/llm"
	"github.com/ElioNeto/devon/internal/permissions"
)

// Tool é a interface que toda ferramenta do Devon deve implementar.
type Tool interface {
	// Name retorna o nome da ferramenta (usado pelo LLM).
	Name() string
	// Description descreve o propósito da ferramenta para o LLM.
	Description() string
	// Schema retorna o JSON Schema dos parâmetros aceitos.
	Schema() json.RawMessage
	// Execute executa a ferramenta com os parâmetros fornecidos.
	// Retorna o resultado como string (enviado de volta ao LLM).
	Execute(ctx context.Context, params json.RawMessage) (string, error)
	// Permission retorna o nivel de permissao da ferramenta.
	Permission() permissions.PermissionLevel
}

// Registry mantém o conjunto de ferramentas disponíveis.
type Registry struct {
	tools map[string]Tool
}

// NewRegistry cria um registro vazio.
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

// Register adiciona uma ferramenta ao registro.
// Entra em pânico se o nome já estiver registrado (erro de programação).
func (r *Registry) Register(t Tool) {
	if _, dup := r.tools[t.Name()]; dup {
		panic(fmt.Sprintf("tools: ferramenta duplicada: %q", t.Name()))
	}
	r.tools[t.Name()] = t
}

// Get retorna uma ferramenta pelo nome.
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// List returns the names of all registered tools.
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Defs retorna as definições no formato esperado pelo LLM.
func (r *Registry) Defs() []llm.ToolDef {
	defs := make([]llm.ToolDef, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, llm.ToolDef{
			Type: "function",
			Function: llm.ToolDefFunc{
				Name:        t.Name(),
				Description: t.Description(),
				Parameters:  t.Schema(),
			},
		})
	}
	return defs
}
