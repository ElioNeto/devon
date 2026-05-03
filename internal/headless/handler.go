package headless

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"

	agentpkg "github.com/ElioNeto/devon/internal/agent"
	"github.com/ElioNeto/devon/internal/config"
	"github.com/ElioNeto/devon/internal/db"
	"github.com/ElioNeto/devon/internal/llm"
	"github.com/ElioNeto/devon/internal/tools"
)

// agentRun is a helper that creates an agent, runs it with the given prompt,
// and streams events to the provided channel. The events channel is closed
// when the agent finishes.
func agentRun(ctx context.Context, cfg *config.Config, registry *tools.Registry, router *llm.AgentRouter, prompt string, eventsCh chan<- agentEvent) {
	defer close(eventsCh)

	client := llm.New(cfg.APIKey, cfg.BaseURL, cfg.Model, cfg.Timeout)
	agent := agentpkg.New(cfg, client, registry, &noopStore{}, "headless-agent", nil, cfg.WorkDir, router)
	if cfg.ForcedTaskType != "" {
		agent.SetForcedTaskType(cfg.ForcedTaskType)
	}

	agentEvents := agent.Run(ctx, prompt)
	for ev := range agentEvents {
		eventsCh <- agentEvent{source: ev, agent: agent}
	}
}

// agentEvent wraps an agent.Event with a reference to the agent that produced it.
type agentEvent struct {
	source agentpkg.Event
	agent  *agentpkg.Agent
}

// RegisterHandlers registers all HTTP handlers on the server.
func RegisterHandlers(s *Server, cfg *config.Config, registry *tools.Registry, router *llm.AgentRouter) {
	s.mux.HandleFunc("/api/prompt", func(w http.ResponseWriter, r *http.Request) {
		handlePrompt(s, cfg, registry, router, w, r)
	})
	s.mux.HandleFunc("/api/respond", func(w http.ResponseWriter, r *http.Request) {
		handleRespond(s, w, r)
	})
	s.mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		handleHealth(w, r)
	})
	s.mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		handleStatus(w, r)
	})
}

// handlePrompt handles POST /api/prompt — accepts a prompt, runs the agent, and
// streams events back as SSE (Server-Sent Events).
func handlePrompt(s *Server, cfg *config.Config, registry *tools.Registry, router *llm.AgentRouter, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req PromptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	if req.Prompt == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "prompt is required"})
		return
	}

	// Apply mode override from request
	if req.Mode != "" {
		cfg.Mode = config.ParseMode(req.Mode)
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming not supported"})
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	eventsCh := make(chan agentEvent, 64)

	// Start agent in background
	go agentRun(ctx, cfg, registry, router, req.Prompt, eventsCh)

	// Stream events via SSE
	for ev := range eventsCh {
		select {
		case <-ctx.Done():
			return
		default:
		}

		ag := ev.agent
		src := ev.source

		switch src.Type {
		case "text":
			sendSSE(w, flusher, EventTypeTextDelta, TextDeltaPayload{Text: src.Text})

		case "tool_start":
			sendSSE(w, flusher, EventTypeToolStart, ToolStartPayload{Tool: src.Tool, Args: src.Args})

		case "tool_done":
			sendSSE(w, flusher, EventTypeToolDone, ToolDonePayload{Tool: src.Tool, Result: src.Result})

		case "tool_error":
			errMsg := ""
			if src.Err != nil {
				errMsg = src.Err.Error()
			}
			sendSSE(w, flusher, EventTypeToolError, ToolErrorPayload{Tool: src.Tool, Err: errMsg})

		case "turn_done":
			sendSSE(w, flusher, EventTypeTurnDone, TurnDonePayload{})

		case "error":
			errMsg := ""
			if src.Err != nil {
				errMsg = src.Err.Error()
			}
			sendSSE(w, flusher, EventTypeError, ErrorPayload{Message: errMsg})

		case "system":
			sendSSE(w, flusher, EventTypeSystem, SystemPayload{Text: src.Text})

		case "rate_limited":
			errMsg := ""
			if src.Err != nil {
				errMsg = src.Err.Error()
			}
			sendSSE(w, flusher, EventTypeRateLimited, ErrorPayload{Message: errMsg})

		case "confirm_request":
			// Agent is blocking on ReplyCh — we need to ask the client
			ch := make(chan int, 1)
			requestID := s.agents.RegisterPendingRequest(ch)
			sendSSE(w, flusher, EventTypeActionRequired, ActionRequiredPayload{
				Tool:      src.Tool,
				Args:      src.Args,
				RequestID: requestID,
			})

			// Wait for the client to respond via /api/respond
			select {
			case level := <-ch:
				if ag != nil && ag.ReplyCh != nil {
					ag.ReplyCh <- agentpkg.ConfirmReply{Level: level}
				}
			case <-ctx.Done():
				s.agents.RemovePendingRequest(requestID)
				return
			}

		case "file_change":
			sendSSE(w, flusher, EventTypeFileChange, map[string]string{
				"file":   src.Text,
				"change": src.Result,
			})
		}
	}
}

// handleRespond handles POST /api/respond — unblocks an action_required event.
func handleRespond(s *Server, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req ActionRequiredRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	if req.RequestID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "request_id is required"})
		return
	}

	level := 1 // approved
	if !req.Approved {
		level = 0 // denied
	}
	if req.AlwaysAllow {
		level = 2 // always allow
	}

	ok := s.agents.ResolvePendingRequest(req.RequestID, level)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "request_id not found or already resolved"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// handleHealth handles GET /api/health.
func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// agentStatus stores the current agent status (set by the handler).
var agentStatus struct {
	running   atomic.Bool
	sessionID atomic.Value // string
	model     atomic.Value // string
	taskType  atomic.Value // string
}

func init() {
	agentStatus.sessionID.Store("")
	agentStatus.model.Store("")
	agentStatus.taskType.Store("")
}

// handleStatus handles GET /api/status.
func handleStatus(w http.ResponseWriter, r *http.Request) {
	running := agentStatus.running.Load()
	sessionID, _ := agentStatus.sessionID.Load().(string)
	model, _ := agentStatus.model.Load().(string)
	taskType, _ := agentStatus.taskType.Load().(string)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"running":    running,
		"session_id": sessionID,
		"model":      model,
		"task_type":  taskType,
	})
}

// sendSSE writes an SSE event to the response writer and flushes.
func sendSSE(w http.ResponseWriter, flusher http.Flusher, eventType string, payload interface{}) {
	ev := EventResponse{
		Type:    eventType,
		Payload: payload,
	}
	data, err := json.Marshal(ev)
	if err != nil {
		slog.Error("headless: failed to marshal SSE event", "err", err)
		return
	}
	_, err = fmt.Fprintf(w, "data: %s\n\n", string(data))
	if err != nil {
		slog.Warn("headless: failed to write SSE event", "err", err)
		return
	}
	flusher.Flush()
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("headless: failed to write JSON response", "err", err)
	}
}

// noopStore is a no-op implementation of db.Store for headless mode.
type noopStore struct{}

func (n *noopStore) CreateSession(ctx context.Context, id string) error                                         { return nil }
func (n *noopStore) CreateSessionWithMeta(ctx context.Context, id, task, model, status string) error            { return nil }
func (n *noopStore) GetSession(ctx context.Context, id string) (bool, error)                                 { return false, nil }
func (n *noopStore) ListSessions(ctx context.Context, limit int) ([]string, error)                          { return nil, nil }
func (n *noopStore) GetSessionDetail(ctx context.Context, id string) (*db.SessionDetail, error)             { return nil, nil }
func (n *noopStore) ListSessionsDetail(ctx context.Context, limit int) ([]db.SessionDetail, error)          { return nil, nil }
func (n *noopStore) UpdateSession(ctx context.Context, id, task, model, status string) error                { return nil }
func (n *noopStore) DeleteSession(ctx context.Context, id string) error                                     { return nil }
func (n *noopStore) PutMessage(ctx context.Context, agentID, sessionID, role, content string) error          { return nil }
func (n *noopStore) GetMessages(ctx context.Context, agentID, sessionID string, limit int) ([]db.Message, error) {
	return nil, nil
}
func (n *noopStore) SlidingWindow(ctx context.Context, agentID, sessionID string, windowSize int) error     { return nil }
func (n *noopStore) PutAgentState(ctx context.Context, agentID, sessionID, snapshot string) error           { return nil }
func (n *noopStore) GetAgentState(ctx context.Context, agentID string) (*db.AgentState, error)             { return nil, nil }
func (n *noopStore) PutToolCall(ctx context.Context, agentID, sessionID, toolName, arguments, status, result, err string) (int64, error) {
	return 0, nil
}
func (n *noopStore) GetToolCalls(ctx context.Context, sessionID string) ([]db.ToolCall, error)             { return nil, nil }
func (n *noopStore) ArchiveMessages(ctx context.Context, agentID, sessionID string) error                   { return nil }
func (n *noopStore) GetSessionHistory(ctx context.Context, sessionID string, limit int) ([]db.Message, error) {
	return nil, nil
}
func (n *noopStore) PutArtifact(ctx context.Context, key, sessionID string, data []byte) error              { return nil }
func (n *noopStore) GetArtifact(ctx context.Context, key string) ([]byte, error)                            { return nil, nil }
func (n *noopStore) GetCostSummary(ctx context.Context, sessionID string) (*db.CostSummary, error)          { return nil, nil }
func (n *noopStore) UpdateCostSummary(ctx context.Context, sessionID string, cost float64, tokens map[string]int) error {
	return nil
}
func (n *noopStore) Close() error                                                                           { return nil }
func (n *noopStore) PutFact(ctx context.Context, projectID, category, content, context string) error        { return nil }
func (n *noopStore) GetFacts(ctx context.Context, projectID, category string, limit int) ([]db.Fact, error)      { return nil, nil }
func (n *noopStore) ListFacts(ctx context.Context, projectID string) ([]db.Fact, error)                     { return nil, nil }
func (n *noopStore) DeleteFacts(ctx context.Context, projectID string) error                                { return nil }
func (n *noopStore) RecordFileAccess(ctx context.Context, sessionID, filePath, accessType string) error    { return nil }
func (n *noopStore) GetFileAccess(ctx context.Context, sessionID string, limit int) ([]db.FileAccess, error) { return nil, nil }
func (n *noopStore) PutErrorPattern(ctx context.Context, projectID, pattern, context string) error         { return nil }
func (n *noopStore) IncrementErrorPattern(ctx context.Context, projectID, pattern string) error           { return nil }
func (n *noopStore) GetErrorPatterns(ctx context.Context, projectID string, limit int) ([]db.ErrorPattern, error) {
	return nil, nil
}
func (n *noopStore) QueryFacts(ctx context.Context, projectID, keyword string, limit int) ([]db.FactRow, error) {
	return nil, nil
}
func (n *noopStore) Subscribe(ctx context.Context, topic string) (<-chan db.Event, error)                    { return nil, nil }
func (n *noopStore) Publish(ctx context.Context, topic string, payload interface{}) error                     { return nil }
