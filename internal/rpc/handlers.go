package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/ElioNeto/devon/internal/agent"
	"github.com/ElioNeto/devon/internal/db"
	"github.com/ElioNeto/devon/internal/session"
)

// HandlerManager manages RPC method handlers and coordinates with the agent.
type HandlerManager struct {
	agent   *agent.Agent
	store   db.Store
	srv     *Server
	sessMgr *session.Manager
}

// NewHandlerManager creates a new HandlerManager.
func NewHandlerManager(a *agent.Agent, store db.Store, srv *Server) *HandlerManager {
	return &HandlerManager{
		agent:   a,
		store:   store,
		srv:     srv,
		sessMgr: session.NewManager(store),
	}
}

// RegisterAll registers all RPC method handlers on the server.
func (hm *HandlerManager) RegisterAll() {
	hm.srv.Register("sendPrompt", hm.handleSendPrompt)
	hm.srv.Register("getSession", hm.handleGetSession)
	hm.srv.Register("listSessions", hm.handleListSessions)
	hm.srv.Register("interrupt", hm.handleInterrupt)
	hm.srv.Register("getStatus", hm.handleGetStatus)
}

// handleSendPrompt sends a prompt to the agent and returns session info.
func (hm *HandlerManager) handleSendPrompt(ctx context.Context, req *Request) Response {
	var params SendPromptParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrInvalidParams, "Invalid params: "+err.Error(), nil)
	}
	if params.Prompt == "" {
		return NewErrorResponse(req.ID, ErrInvalidParams, "prompt is required", nil)
	}

	if hm.agent == nil {
		return NewErrorResponse(req.ID, ErrServer, "Agent not initialized", nil)
	}

	// Run the agent with the given prompt
	events := hm.agent.Run(ctx, params.Prompt)

	// Consume events in background to keep agent moving
	go func() {
		for ev := range events {
			// Publish events as SSE for connected clients
			eventData, _ := json.Marshal(Event{
				Type:    ev.Type,
				Payload: marshalEventPayload(ev),
			})
			hm.srv.Broadcast(eventData)
			slog.Debug("rpc: event", "type", ev.Type)
		}
	}()

	sessionInfo := SessionInfo{
		ID:     hm.agent.AgentID(),
		Model:  hm.agent.ActiveModel(),
		Status: "active",
	}

	return NewResponse(req.ID, sessionInfo)
}

// handleGetSession returns details for a given session ID.
func (hm *HandlerManager) handleGetSession(ctx context.Context, req *Request) Response {
	var params GetSessionParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrInvalidParams, "Invalid params: "+err.Error(), nil)
	}
	if params.ID == "" {
		return NewErrorResponse(req.ID, ErrInvalidParams, "id is required", nil)
	}

	s, err := hm.sessMgr.Get(ctx, params.ID)
	if err != nil {
		return NewErrorResponse(req.ID, ErrInternal, fmt.Sprintf("Failed to get session: %v", err), nil)
	}
	if s == nil {
		return NewErrorResponse(req.ID, ErrServer, "Session not found", nil)
	}

	info := SessionInfo{
		ID:            s.ID,
		Task:          s.Task,
		Model:         s.Model,
		Status:        s.Status,
		MessageCount:  s.MessageCount,
		ToolCallCount: s.ToolCallCount,
		TotalCost:     s.TotalCost,
		Duration:      s.Duration,
	}
	return NewResponse(req.ID, info)
}

// handleListSessions returns a list of recent sessions.
func (hm *HandlerManager) handleListSessions(ctx context.Context, req *Request) Response {
	var params ListSessionsParams
	if req.Params != nil {
		_ = json.Unmarshal(req.Params, &params)
	}
	if params.Limit <= 0 || params.Limit > 100 {
		params.Limit = 50
	}

	sessions, err := hm.sessMgr.List(ctx, params.Limit)
	if err != nil {
		return NewErrorResponse(req.ID, ErrInternal, fmt.Sprintf("Failed to list sessions: %v", err), nil)
	}

	infos := make([]SessionInfo, 0, len(sessions))
	for _, s := range sessions {
		infos = append(infos, SessionInfo{
			ID:            s.ID,
			Task:          s.Task,
			Model:         s.Model,
			Status:        s.Status,
			MessageCount:  s.MessageCount,
			ToolCallCount: s.ToolCallCount,
			TotalCost:     s.TotalCost,
			Duration:      s.Duration,
		})
	}
	return NewResponse(req.ID, infos)
}

// handleInterrupt interrupts the current agent execution.
// If no agent is running, this is a successful no-op.
func (hm *HandlerManager) handleInterrupt(ctx context.Context, req *Request) Response {
	if hm.agent != nil && hm.agent.ReplyCh != nil {
		hm.agent.ReplyCh <- agent.ConfirmReply{Level: 0}
	}

	return NewResponse(req.ID, map[string]string{"status": "interrupted"})
}

// handleGetStatus returns the current agent status.
func (hm *HandlerManager) handleGetStatus(ctx context.Context, req *Request) Response {
	status := StatusInfo{
		Running:  hm.agent != nil,
		Model:    "",
		TaskType: "",
	}
	if hm.agent != nil {
		status.SessionID = hm.agent.AgentID()
		status.Model = hm.agent.ActiveModel()
		status.TaskType = string(hm.agent.ActiveTaskType())
	}
	return NewResponse(req.ID, status)
}

// marshalEventPayload converts an agent.Event payload to json.RawMessage.
func marshalEventPayload(ev agent.Event) json.RawMessage {
	m := map[string]any{
		"type":   ev.Type,
		"text":   ev.Text,
		"tool":   ev.Tool,
		"args":   ev.Args,
		"result": ev.Result,
	}
	if ev.Err != nil {
		m["error"] = ev.Err.Error()
	}
	data, _ := json.Marshal(m)
	return data
}

