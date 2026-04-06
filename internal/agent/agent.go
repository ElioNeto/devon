// Package agent implementa o loop principal do Devon:
// prompt → LLM → tool calls → resultado → LLM → ...
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ElioNeto/devon/internal/config"
	"github.com/ElioNeto/devon/internal/llm"
	"github.com/ElioNeto/devon/internal/permissions"
	"github.com/ElioNeto/devon/internal/tools"
)

// isRateLimited detects rate-limit errors from the LLM provider.
func isRateLimited(err error) bool {
	s := err.Error()
	return strings.Contains(s, "429") || strings.Contains(s, "rate")
}

// Event é emitido pelo agente para a TUI durante o processamento.
type Event struct {
	Type string // "text" | "tool_start" | "tool_done" | "tool_error" | "rate_limited" | "turn_done" | "error" | "confirm_request"
	Text string // fragmento de texto (streaming)
	Tool string // nome da ferramenta
	Args string // argumentos JSON da ferramenta
	Result string // resultado da ferramenta
	Err  error
}

// ConfirmReply is sent back from TUI when user responds to confirm_request.
type ConfirmReply struct {
	Level int // 0=no, 1=yes, 2=always
}

// Agent executa o loop de raciocínio do Devon.
type Agent struct {
	cfg      *config.Config
	client   llm.Streamer
	registry *tools.Registry
	checker  *permissions.Checker
	history  []llm.Message
	ReplyCh  chan ConfirmReply // receives user response to confirm_request
}

// New cria um novo Agent com todas as ferramentas nativas registradas.
func New(cfg *config.Config, client llm.Streamer, registry *tools.Registry) *Agent {
	tools.RegisterBuiltin(registry, cfg.WorkDir, cfg.Timeout)

	blocklist := permissions.DefaultBlocklist

	a := &Agent{
		cfg:      cfg,
		client:   client,
		registry: registry,
		checker: &permissions.Checker{
			Mode:      cfg.Mode,
			Session:   make(map[string]bool),
			Blocklist: blocklist,
		},
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

		// Throttle between turns (skip on first turn)
		if turn > 0 && a.cfg.TurnDelay > 0 {
			select {
			case <-time.After(a.cfg.TurnDelay):
			case <-ctx.Done():
				return
			}
		}

		// Auto-compact context if approaching token limit
		used := estimateTokens(a.history)
		if compacted, ok := compactIfNeeded(a.history, a.cfg.Model, used); ok && len(compacted) < len(a.history) {
			removed := len(a.history) - len(compacted)
			a.history = compacted
			ch <- Event{Type: "system", Text: fmt.Sprintf("Contexto compactado: removidas %d mensagens antigas", removed)}
		}

		stream, err := a.client.Stream(ctx, a.history, a.registry.Defs())
		if err != nil {
			if isRateLimited(err) {
				ch <- Event{Type: "rate_limited", Err: err}
			}
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

			result, toolErr := a.executeToolWithPermission(ctx, tc, ch)
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

func (a *Agent) executeToolWithPermission(ctx context.Context, tc llm.ToolCall, ch chan<- Event) (string, error) {
	t, ok := a.registry.Get(tc.Function.Name)
	if !ok {
		return "", fmt.Errorf("ferramenta desconhecida: %q", tc.Function.Name)
	}

	blocked, needsConfirm := a.checker.Requires(t)
	if blocked {
		return fmt.Sprintf("ferramenta %q bloqueada pela blocklist", tc.Function.Name), fmt.Errorf("ferramenta bloqueada")
	}

	if needsConfirm {
		a.ReplyCh = make(chan ConfirmReply, 1)
		ch <- Event{Type: "confirm_request", Tool: tc.Function.Name, Args: tc.Function.Arguments}

		select {
		case reply := <-a.ReplyCh:
			a.ReplyCh = nil
			switch reply.Level {
			case 0: // no
				return fmt.Sprintf("ferramenta %q recusada pelo usuário", tc.Function.Name), fmt.Errorf("recusado pelo usuário")
			case 2: // always
				a.checker.Approve(tc.Function.Name)
			}
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	return t.Execute(ctx, json.RawMessage(tc.Function.Arguments))
}

func (a *Agent) buildSystemMessages() []llm.Message {
	system := strings.Builder{}
	system.WriteString("Você é Devon, um agente de engenharia de software. ")
	system.WriteString("Você tem acesso a ferramentas para ler/escrever arquivos, executar comandos e navegar em código. ")
	system.WriteString("Seu objetivo é completar a tarefa solicitada e entregar o artefato pedido. ")
	system.WriteString("Trabalhe de forma incremental: leia antes de escrever, execute testes após mudanças, ")
	system.WriteString("prefira edições cirúrgicas a rewrites completos. ")
	system.WriteString("Testes passando é um passo intermediário, não o objetivo final. ")
	system.WriteString("Ao finalizar, liste os arquivos criados ou modificados. ")
	system.WriteString("Seja direto: aja, não apenas planeje.")

	projectCtx := BuildProjectContext(a.cfg.WorkDir)
	if projectCtx != "" {
		system.WriteString("\n\n")
		system.WriteString(projectCtx)
	}

	if a.cfg.ContextDoc != "" {
		system.WriteString("\n\n# Contexto do Projeto (DEVON.md)\n")
		system.WriteString(a.cfg.ContextDoc)
	}

	return []llm.Message{
		{Role: llm.RoleSystem, Content: system.String()},
	}
}
