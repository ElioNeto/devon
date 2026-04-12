package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ElioNeto/devon/internal/db"
	"github.com/ElioNeto/devon/internal/agent/prompts"
	"github.com/ElioNeto/devon/internal/config"
	"github.com/ElioNeto/devon/internal/llm"
	"github.com/ElioNeto/devon/internal/memory"
	"github.com/ElioNeto/devon/internal/permissions"
	"github.com/ElioNeto/devon/internal/tools"
)

// isRateLimited detects rate-limit errors.
func isRateLimited(err error) bool {
	s := err.Error()
	return strings.Contains(s, "429") || strings.Contains(s, "rate")
}

// Event é emitido pelo agente para a TUI.
type Event struct {
	Type   string // "text" | "tool_start" | "tool_done" | "tool_error" | "rate_limited" | "turn_done" | "error" | "confirm_request"
	Text   string
	Tool   string
	Args   string
	Result string
	Err    error
}

// ConfirmReply is sent back from TUI.
type ConfirmReply struct {
	Level int // 0=no, 1=yes, 2=always
}

// Agent executa o loop de raciocínio do Devon.
type Agent struct {
	id          string
	cfg         *config.Config
	client      llm.Streamer
	registry    *tools.Registry
	checker     *permissions.Checker
	db          db.Store
	history     []llm.Message
	ReplyCh     chan ConfirmReply
	mu          chan Event
	mem         *memory.Manager
	projectID   string
}

// New cria um novo Agent com DB injection.
func New(cfg *config.Config, client llm.Streamer, registry *tools.Registry, db db.Store, agentID string, mem *memory.Manager, projectID string) *Agent {
	tools.RegisterBuiltin(registry, cfg.WorkDir, cfg.Timeout, cfg.Sandbox)
	tools.RegisterMemoryTools(registry, mem, projectID)

	blocklist := permissions.DefaultBlocklist

	a := &Agent{
		id:          agentID,
		cfg:         cfg,
		client:      client,
		registry:    registry,
		db:          db,
		mem:         mem,
		projectID:   projectID,
		checker: &permissions.Checker{
			Mode:      cfg.Mode,
			Session:   make(map[string]bool),
			Blocklist: blocklist,
		},
		ReplyCh: make(chan ConfirmReply, 1),
	}
	a.history = a.buildSystemMessages()

	// Persiste mensagem inicial no DB
	if db != nil {
		db.CreateSession(context.Background(), agentID)
	}

	return a
}

// AgentID retorna o ID único deste agente.
func (a *Agent) AgentID() string {
	return a.id
}

// Run processa um turno do usuário.
func (a *Agent) Run(ctx context.Context, userInput string) <-chan Event {
	a.mu = make(chan Event, 64)
	go a.run(ctx, userInput, a.mu)
	return a.mu
}

func (a *Agent) run(ctx context.Context, userInput string, ch chan<- Event) {
	defer close(ch)

	a.history = append(a.history, llm.Message{
		Role:    llm.RoleUser,
		Content: llm.TextContent(userInput),
	})

	// Persiste mensagem no DB
	if a.db != nil {
		a.db.PutMessage(ctx, a.id, a.id, "user", userInput)
	}

	for turn := 0; turn < a.cfg.MaxTurns; turn++ {
		if ctx.Err() != nil {
			return
		}

		if turn > 0 && a.cfg.TurnDelay > 0 {
			select {
			case <-time.After(a.cfg.TurnDelay):
			case <-ctx.Done():
				return
			}
		}

		// Auto-compact context
		used := estimateTokens(a.history)
		if compacted, ok := compactIfNeeded(a.history, a.cfg.Model, used); ok && len(compacted) < len(a.history) {
			removed := len(a.history) - len(compacted)
			a.history = compacted
			ch <- Event{Type: "system", Text: fmt.Sprintf("Contexto compactado: removidas %d mensagens", removed)}
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

		assistantMsg := llm.Message{Role: llm.RoleAssistant}
		if textBuf.Len() > 0 {
			assistantMsg.Content = llm.TextContent(textBuf.String())
		}
		if len(pendingTools) > 0 {
			assistantMsg.ToolCalls = pendingTools
		}
		a.history = append(a.history, assistantMsg)

		if len(pendingTools) == 0 {
			ch <- Event{Type: "turn_done"}

			// Persiste snapshot do estado
			if a.db != nil {
				snapshot, _ := json.Marshal(a.history)
				a.db.PutAgentState(ctx, a.id, a.id, string(snapshot))
			}

			return
		}

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
				Content:    llm.TextContent(result),
			})

			// Persistir tool call no DB
			if a.db != nil {
				_, _ = a.db.PutToolCall(ctx, a.id, a.id, tc.Function.Name, tc.Function.Arguments, "completed", result, "")
			}
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
		return fmt.Sprintf("ferramenta %q bloqueada", tc.Function.Name), fmt.Errorf("ferramenta bloqueada")
	}

	if needsConfirm {
		a.ReplyCh = make(chan ConfirmReply, 1)
		ch <- Event{Type: "confirm_request", Tool: tc.Function.Name, Args: tc.Function.Arguments}

		select {
		case reply := <-a.ReplyCh:
			a.ReplyCh = nil
			switch reply.Level {
			case 0:
				return fmt.Sprintf("ferramenta %q recusada", tc.Function.Name), fmt.Errorf("recusado")
			case 2:
				a.checker.Approve(tc.Function.Name)
			}
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	return t.Execute(ctx, json.RawMessage(tc.Function.Arguments))
}

func (a *Agent) buildSystemMessages() []llm.Message {
	system := prompts.GetSystemPrompt()

	if a.cfg.ContextDoc != "" {
		system += "\n\n# Contexto do Projeto (DEVON.md)\n"
		system += a.cfg.ContextDoc
	}

	// Append semantic memory context if manager is available
	if a.mem != nil {
		memCtx, _ := a.mem.ContextFor(context.Background(), a.projectID, "")
		if memCtx != "" {
			system += "\n\n" + memCtx
		}
	}

	return []llm.Message{
		{Role: llm.RoleSystem, Content: llm.TextContent(system)},
	}
}
