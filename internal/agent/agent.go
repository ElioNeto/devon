// Package agent implementa o loop principal do Devon:
// prompt → LLM → tool calls → resultado → LLM → ...
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ElioNeto/devon/internal/config"
	"github.com/ElioNeto/devon/internal/llm"
	"github.com/ElioNeto/devon/internal/tools"
)

// Event é emitido pelo agente para a TUI durante o processamento.
type Event struct {
	Type string // "text" | "tool_start" | "tool_done" | "tool_error" | "turn_done" | "error"
	Text string // fragmento de texto (streaming)
	Tool string // nome da ferramenta
	Args string // argumentos JSON da ferramenta
	Result string // resultado da ferramenta
	Err  error
}

// Agent executa o loop de raciocínio do Devon.
type Agent struct {
	cfg      *config.Config
	client   *llm.Client
	registry *tools.Registry
	history  []llm.Message
}

// New cria um novo Agent.
func New(cfg *config.Config, client *llm.Client, registry *tools.Registry) *Agent {
	a := &Agent{
		cfg:      cfg,
		client:   client,
		registry: registry,
	}
	a.history = a.buildSystemMessages()
	return a
}

// Run processa um turno do usuário e emite eventos no canal retornado.
// O canal é fechado quando o turno termina.
func (a *Agent) Run(ctx context.Context, userInput string) <-chan Event {
	ch := make(chan Event, 64)
	go a.run(ctx, userInput, ch)
	return ch
}

func (a *Agent) run(ctx context.Context, userInput string, ch chan<- Event) {
	defer close(ch)

	a.history = append(a.history, llm.Message{
		Role:    llm.RoleUser,
		Content: userInput,
	})

	for turn := 0; turn < a.cfg.MaxTurns; turn++ {
		if ctx.Err() != nil {
			return
		}

		stream, err := a.client.Stream(ctx, a.history, a.registry.Defs())
		if err != nil {
			ch <- Event{Type: "error", Err: fmt.Errorf("agent: stream: %w", err)}
			return
		}

		var textBuf strings.Builder
		var pendingTools []llm.ToolCall

		for ev := range stream {
			switch ev.Type {
			case "text":
				textBuf.WriteString(ev.Text)
				ch <- Event{Type: "text", Text: ev.Text}

			case "tool_call":
				pendingTools = append(pendingTools, *ev.Tool)

			case "error":
				ch <- Event{Type: "error", Err: ev.Err}
				return
			}
		}

		// Adiciona resposta do assistente ao histórico
		assistantMsg := llm.Message{Role: llm.RoleAssistant}
		if textBuf.Len() > 0 {
			assistantMsg.Content = textBuf.String()
		}
		if len(pendingTools) > 0 {
			assistantMsg.ToolCalls = pendingTools
		}
		a.history = append(a.history, assistantMsg)

		// Se não há tool calls, o turno terminou
		if len(pendingTools) == 0 {
			ch <- Event{Type: "turn_done"}
			return
		}

		// Executa cada tool call e adiciona resultados ao histórico
		for _, tc := range pendingTools {
			ch <- Event{Type: "tool_start", Tool: tc.Function.Name, Args: tc.Function.Arguments}

			result, toolErr := a.executeTool(ctx, tc)
			if toolErr != nil {
				ch <- Event{Type: "tool_error", Tool: tc.Function.Name, Err: toolErr}
				result = fmt.Sprintf("error: %v", toolErr)
			} else {
				ch <- Event{Type: "tool_done", Tool: tc.Function.Name, Result: result}
			}

			a.history = append(a.history, llm.Message{
				Role:       llm.RoleTool,
				ToolCallID: tc.ID,
				Content:    result,
			})
		}
	}

	ch <- Event{Type: "error", Err: fmt.Errorf("agent: limite de %d turnos atingido", a.cfg.MaxTurns)}
}

func (a *Agent) executeTool(ctx context.Context, tc llm.ToolCall) (string, error) {
	t, ok := a.registry.Get(tc.Function.Name)
	if !ok {
		return "", fmt.Errorf("ferramenta desconhecida: %q", tc.Function.Name)
	}
	return t.Execute(ctx, json.RawMessage(tc.Function.Arguments))
}

func (a *Agent) buildSystemMessages() []llm.Message {
	system := strings.Builder{}
	system.WriteString("Você é Devon, um agente de engenharia de software. ")
	system.WriteString("Você tem acesso a ferramentas para ler/escrever arquivos, executar comandos e navegar em código. ")
	system.WriteString("Trabalhe de forma incremental: leia antes de escrever, execute testes após mudanças, ")
	system.WriteString("prefira edições cirúrgicas a rewrites completos. ")
	system.WriteString("Seja direto: aja, não apenas planeje.")

	if a.cfg.ContextDoc != "" {
		system.WriteString("\n\n# Contexto do Projeto (DEVON.md)\n")
		system.WriteString(a.cfg.ContextDoc)
	}

	return []llm.Message{
		{Role: llm.RoleSystem, Content: system.String()},
	}
}
