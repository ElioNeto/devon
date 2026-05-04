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
	"github.com/ElioNeto/devon/internal/index"
	"github.com/ElioNeto/devon/internal/llm"
	"github.com/ElioNeto/devon/internal/memory"
	"github.com/ElioNeto/devon/internal/permissions"
	"github.com/ElioNeto/devon/internal/tools"
)

// TaskType is a convenience alias so agent callers don't need to import config directly.
type TaskType = config.TaskType

const (
	TaskTypeExplore = config.TaskTypeExplore
	TaskTypePlan    = config.TaskTypePlan
	TaskTypeCode    = config.TaskTypeCode
)

// isRateLimited detects rate-limit errors.
func isRateLimited(err error) bool {
	s := err.Error()
	return strings.Contains(s, "429") || strings.Contains(s, "rate")
}

// fileModifyingTools contains the names of tools that modify files and
// should emit file_change events so the VS Code extension can show gutter indicators.
var fileModifyingTools = map[string]bool{
	"write":      true,
	"edit":       true,
	"patch_file": true,
}

// toolPathParams is used to extract the "path" field from tool call arguments.
type toolPathParams struct {
	Path string `json:"path"`
}

// extractFilePath tries to extract a file path from tool call arguments JSON.
// Returns empty string if the path cannot be determined.
func extractFilePath(args string) string {
	var p toolPathParams
	if err := json.Unmarshal([]byte(args), &p); err != nil {
		return ""
	}
	return p.Path
}

// Event é emitido pelo agente para a TUI e retransmitido via RPC para a extensão VS Code.
type Event struct {
	Type   string // "text" | "tool_start" | "tool_done" | "tool_error" | "file_change" | "rate_limited" | "turn_done" | "error" | "confirm_request"
	Text   string
	Tool   string
	Args   string
	Result string
	Err     error
	Summary string // human-readable summary for turn_done events
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
	router      *llm.AgentRouter
	registry    *tools.Registry
	checker     *permissions.Checker
	db          db.Store
	history     []llm.Message
	activeTaskType  config.TaskType
	activeModel     string
	forcedTaskType  config.TaskType // non-zero when --task-type forces classification
	ReplyCh     chan ConfirmReply
	mu              chan Event
	mem             *memory.Manager
	projectID       string
	idxMgr          *index.Manager
	hasExecutedTools bool // tracks whether any tool was executed in the current run
}

// New cria um novo Agent com DB injection.
// The router parameter is optional; pass nil to use only the default client.
func New(cfg *config.Config, client llm.Streamer, registry *tools.Registry, db db.Store, agentID string, mem *memory.Manager, projectID string, router ...*llm.AgentRouter) *Agent {
	tools.RegisterBuiltin(registry, cfg.WorkDir, cfg.Timeout, cfg.Sandbox)
	tools.RegisterMemoryTools(registry, mem, projectID)
	tools.RegisterWebTools(registry, &cfg.Web)

	blocklist := permissions.DefaultBlocklist

	var r *llm.AgentRouter
	if len(router) > 0 {
		r = router[0]
	}

	a := &Agent{
		id:          agentID,
		cfg:         cfg,
		client:      client,
		router:      r,
		registry:    registry,
		db:          db,
		mem:         mem,
		projectID:   projectID,
		activeTaskType: config.TaskTypeCode,
		activeModel:    client.Info().Name,
		checker: &permissions.Checker{
			Mode:      cfg.Mode,
			Session:   make(map[string]bool),
			Blocklist: blocklist,
		},
		ReplyCh: make(chan ConfirmReply, 1),
	}

	// Registrar search_codebase se indexação estiver habilitada
	if cfg.Index.Enabled {
		idxMgr, err := index.NewManager(cfg.WorkDir, index.ManagerConfig{
			Enabled: true,
			IndexedConfig: index.IndexedConfig{
				Extensions:    cfg.Index.Extensions,
				Excludes:      cfg.Index.Exclude,
				MaxFileSizeKB: cfg.Index.MaxFileSizeKB,
				TopK:          cfg.Index.TopK,
			},
		})
		if err == nil {
			_ = idxMgr.Index(context.Background(), cfg.WorkDir)
			a.registry.Register(idxMgr.CreateTool())
			a.idxMgr = idxMgr
		}
	}

	a.history = a.buildSystemMessages("")

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

// ActiveTaskType returns the task type classified for the current/last run.
func (a *Agent) ActiveTaskType() config.TaskType {
	if a == nil {
		return config.TaskTypeCode
	}
	return a.activeTaskType
}

// ActiveModel returns the model name used for the current/last run.
func (a *Agent) ActiveModel() string {
	if a == nil {
		return ""
	}
	return a.activeModel
}

// SetForcedTaskType forces the agent to use a specific task type regardless of
// automatic classification. Set to empty TaskType to disable forcing.
func (a *Agent) SetForcedTaskType(tt config.TaskType) {
	a.forcedTaskType = tt
}

// SetConversation replaces the agent's internal history with the given messages.
// A fresh system message is built from the current config and prepended.
// The caller is responsible for ensuring msgs does not include the system role.
func (a *Agent) SetConversation(msgs []llm.Message) {
	sysMsgs := a.buildSystemMessages("")
	a.history = append(sysMsgs, msgs...)
}

// ResetHistory resets the agent's history back to just the system message.
// This is typically used when clearing the conversation (/clear command).
func (a *Agent) ResetHistory() {
	a.history = a.buildSystemMessages("")
}

// Run processa um turno do usuário (texto simples).
func (a *Agent) Run(ctx context.Context, userInput string) <-chan Event {
	a.mu = make(chan Event, 64)
	go a.run(ctx, userInput, a.mu)
	return a.mu
}

// RunWithMessage processa um turno com uma mensagem pré-construída (ex: multimodal).
func (a *Agent) RunWithMessage(ctx context.Context, msg llm.Message) <-chan Event {
	a.mu = make(chan Event, 64)
	go a.runWithMessage(ctx, msg, a.mu)
	return a.mu
}

func (a *Agent) run(ctx context.Context, userInput string, ch chan<- Event) {
	defer close(ch)

	if ctx.Err() != nil {
		return
	}

	// Classify the task type based on user input (or use forced type)
	a.activeTaskType = a.resolveTaskType(userInput)
	a.selectClientForTask()

	ch <- Event{Type: "system", Text: fmt.Sprintf("Tipo de tarefa: %s → modelo: %s", a.activeTaskType, a.activeModel)}

	a.history = append(a.history, llm.Message{
		Role:    llm.RoleUser,
		Content: llm.TextContent(userInput),
	})

	// Persiste mensagem no DB
	if a.db != nil {
		a.db.PutMessage(ctx, a.id, a.id, "user", userInput)
	}

	// Rebuild system message with actual user prompt to inject relevant files
	if sysMsgs := a.buildSystemMessages(userInput); len(sysMsgs) > 0 {
		a.history[0] = sysMsgs[0]
	}

	a.runLoop(ctx, ch)
}

func (a *Agent) runWithMessage(ctx context.Context, msg llm.Message, ch chan<- Event) {
	defer close(ch)

	if ctx.Err() != nil {
		return
	}

	// For multimodal messages, infer task type from text content
	textContent := ""
	if msg.Content != nil {
		textContent = *msg.Content
	} else if len(msg.ContentParts) > 0 {
		for _, p := range msg.ContentParts {
			if p.Type == llm.TypeText {
				textContent = p.Text
				break
			}
		}
	}
	if textContent != "" {
		a.activeTaskType = a.resolveTaskType(textContent)
	}
	a.selectClientForTask()

	ch <- Event{Type: "system", Text: fmt.Sprintf("Tipo de tarefa: %s → modelo: %s", a.activeTaskType, a.activeModel)}

	// Check vision support — strip images if model doesn't support vision
	if llm.HasVisionContent(msg.ContentParts) {
		info := a.client.Info()
		if !info.SupportsVision {
			// Strip image parts, keep only text parts
			var textParts []llm.ContentPart
			for _, p := range msg.ContentParts {
				if p.Type == llm.TypeText {
					textParts = append(textParts, p)
				}
			}
			msg.ContentParts = textParts
			ch <- Event{Type: "system", Text: "Aviso: modelo não suporta visão. Imagens removidas da requisição."}
		}
	}

	a.history = append(a.history, msg)

	// Persiste mensagem no DB
	if a.db != nil {
		label := "[multimodal message]"
		if msg.Content != nil && *msg.Content != "" {
			label = *msg.Content
		}
		a.db.PutMessage(ctx, a.id, a.id, "user", label)
	}

	// Rebuild system message with actual user prompt to inject relevant files
	if sysMsgs := a.buildSystemMessages(textContent); len(sysMsgs) > 0 {
		a.history[0] = sysMsgs[0]
	}

	a.runLoop(ctx, ch)
}

func (a *Agent) runLoop(ctx context.Context, ch chan<- Event) {
	a.hasExecutedTools = false
	for turn := 0; turn < a.cfg.MaxAgentLoops; turn++ {
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
			summary := "resposta enviada"
			if a.hasExecutedTools {
				summary = "tarefa concluída"
			}
			ch <- Event{Type: "turn_done", Summary: summary}

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
				a.hasExecutedTools = true
				ch <- Event{Type: "tool_done", Tool: tc.Function.Name, Result: result}

				// file_change: notify VS Code extension about file modifications
				// so gutter indicators (A/M/D) can be rendered.
				if fileModifyingTools[tc.Function.Name] {
					if path := extractFilePath(tc.Function.Arguments); path != "" {
						ch <- Event{
							Type:   "file_change",
							Tool:   tc.Function.Name,
							Args:   path,
							Text:   path,
							Result: "modified",
						}
					}
				}
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

	ch <- Event{Type: "error", Err: fmt.Errorf("agent: limite de %d turnos atingido", a.cfg.MaxAgentLoops)}
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

// selectClientForTask uses the AgentRouter (if available) to switch the active
// client based on the classified task type. Falls back to the default client.
func (a *Agent) selectClientForTask() {
	if a.router == nil {
		// No router configured — use the default client
		if a.activeModel == "" {
			a.activeModel = a.client.Info().Name
		}
		return
	}

	routed := a.router.ClientFor(a.activeTaskType)
	if routed != nil {
		a.client = routed
	}
	a.activeModel = a.router.ModelFor(a.activeTaskType)
	if a.activeModel == "" {
		a.activeModel = a.client.Info().Name
	}
}

// resolveTaskType returns the forced task type if set, otherwise classifies the prompt.
func (a *Agent) resolveTaskType(prompt string) config.TaskType {
	if a.forcedTaskType != "" {
		return a.forcedTaskType
	}
	return llm.ClassifyTask(prompt)
}

func (a *Agent) buildSystemMessages(userPrompt string) []llm.Message {
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

	// Append project context (workdir, git branch, detected languages)
	if projCtx := BuildProjectContext(a.cfg.WorkDir); projCtx != "" {
		system += "\n\n# Contexto do Projeto\n" + projCtx
	}

	// Injetar top-K arquivos relevantes quando indexação ativa
	if a.idxMgr != nil && a.idxMgr.IsEnabled() && userPrompt != "" {
		results, _ := a.idxMgr.Search(userPrompt, 0)
		if len(results) > 0 {
			system += "\n\n## Arquivos relevantes para esta tarefa\n"
			for i, r := range results {
				system += fmt.Sprintf("%d. %s (score: %.2f)\n", i+1, r.Path, r.Score)
			}
		}
	}

	return []llm.Message{
		{Role: llm.RoleSystem, Content: llm.TextContent(system)},
	}
}
