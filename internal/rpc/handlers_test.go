package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/ElioNeto/devon/internal/agent"
	"github.com/ElioNeto/devon/internal/db"
)

// fakeStore implements db.Store for testing.
type fakeStore struct {
	createSessionWithMetaFn func(ctx context.Context, id, task, model, status string) error
	listSessionsDetailFn    func(ctx context.Context, limit int) ([]db.SessionDetail, error)
	getSessionDetailFn      func(ctx context.Context, id string) (*db.SessionDetail, error)
	subscribeFn             func(ctx context.Context, topic string) (<-chan db.Event, error)
}

func (f *fakeStore) CreateSession(ctx context.Context, id string) error {
	return nil
}
func (f *fakeStore) CreateSessionWithMeta(ctx context.Context, id, task, model, status string) error {
	if f.createSessionWithMetaFn != nil {
		return f.createSessionWithMetaFn(ctx, id, task, model, status)
	}
	return nil
}
func (f *fakeStore) GetSession(ctx context.Context, id string) (bool, error) {
	return false, nil
}
func (f *fakeStore) ListSessions(ctx context.Context, limit int) ([]string, error) {
	return nil, nil
}
func (f *fakeStore) GetSessionDetail(ctx context.Context, id string) (*db.SessionDetail, error) {
	if f.getSessionDetailFn != nil {
		return f.getSessionDetailFn(ctx, id)
	}
	return nil, nil
}
func (f *fakeStore) ListSessionsDetail(ctx context.Context, limit int) ([]db.SessionDetail, error) {
	if f.listSessionsDetailFn != nil {
		return f.listSessionsDetailFn(ctx, limit)
	}
	return nil, nil
}
func (f *fakeStore) UpdateSession(ctx context.Context, id, task, model, status string) error {
	return nil
}
func (f *fakeStore) DeleteSession(ctx context.Context, id string) error {
	return nil
}
func (f *fakeStore) PutMessage(ctx context.Context, agentID, sessionID, role, content string) error {
	return nil
}
func (f *fakeStore) GetMessages(ctx context.Context, agentID, sessionID string, limit int) ([]db.Message, error) {
	return nil, nil
}
func (f *fakeStore) SlidingWindow(ctx context.Context, agentID, sessionID string, windowSize int) error {
	return nil
}
func (f *fakeStore) PutAgentState(ctx context.Context, agentID, sessionID, snapshot string) error {
	return nil
}
func (f *fakeStore) GetAgentState(ctx context.Context, agentID string) (*db.AgentState, error) {
	return nil, nil
}
func (f *fakeStore) PutToolCall(ctx context.Context, agentID, sessionID, toolName, arguments, status, result, errStr string) (int64, error) {
	return 0, nil
}
func (f *fakeStore) GetToolCalls(ctx context.Context, sessionID string) ([]db.ToolCall, error) {
	return nil, nil
}
func (f *fakeStore) ArchiveMessages(ctx context.Context, agentID, sessionID string) error {
	return nil
}
func (f *fakeStore) GetSessionHistory(ctx context.Context, sessionID string, limit int) ([]db.Message, error) {
	return nil, nil
}
func (f *fakeStore) PutArtifact(ctx context.Context, key, sessionID string, data []byte) error {
	return nil
}
func (f *fakeStore) GetArtifact(ctx context.Context, key string) ([]byte, error) {
	return nil, nil
}
func (f *fakeStore) GetCostSummary(ctx context.Context, sessionID string) (*db.CostSummary, error) {
	return nil, nil
}
func (f *fakeStore) UpdateCostSummary(ctx context.Context, sessionID string, cost float64, tokens map[string]int) error {
	return nil
}
func (f *fakeStore) PutFact(ctx context.Context, projectID, category, content, context string) error {
	return nil
}
func (f *fakeStore) GetFacts(ctx context.Context, projectID, category string, limit int) ([]db.Fact, error) {
	return nil, nil
}
func (f *fakeStore) ListFacts(ctx context.Context, projectID string) ([]db.Fact, error) {
	return nil, nil
}
func (f *fakeStore) DeleteFacts(ctx context.Context, projectID string) error {
	return nil
}
func (f *fakeStore) RecordFileAccess(ctx context.Context, sessionID, filePath, accessType string) error {
	return nil
}
func (f *fakeStore) GetFileAccess(ctx context.Context, sessionID string, limit int) ([]db.FileAccess, error) {
	return nil, nil
}
func (f *fakeStore) PutErrorPattern(ctx context.Context, projectID, pattern, context string) error {
	return nil
}
func (f *fakeStore) IncrementErrorPattern(ctx context.Context, projectID, pattern string) error {
	return nil
}
func (f *fakeStore) GetErrorPatterns(ctx context.Context, projectID string, limit int) ([]db.ErrorPattern, error) {
	return nil, nil
}
func (f *fakeStore) QueryFacts(ctx context.Context, projectID, keyword string, limit int) ([]db.FactRow, error) {
	return nil, nil
}
func (f *fakeStore) Subscribe(ctx context.Context, topic string) (<-chan db.Event, error) {
	if f.subscribeFn != nil {
		return f.subscribeFn(ctx, topic)
	}
	return nil, nil
}
func (f *fakeStore) Publish(ctx context.Context, topic string, payload any) error {
	return nil
}
func (f *fakeStore) Close() error { return nil }

func TestHandlerManager_GetStatus_NoAgent(t *testing.T) {
	srv := NewServer()
	hm := NewHandlerManager(nil, &fakeStore{}, srv)
	req := &Request{
		JSONRPC: Version,
		Method:  "getStatus",
	}
	resp := hm.handleGetStatus(context.Background(), req)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	var status StatusInfo
	if err := json.Unmarshal(resp.Result, &status); err != nil {
		t.Fatalf("unmarshal status: %v", err)
	}
	if status.Running {
		t.Error("expected Running=false when agent is nil")
	}
}

func TestHandlerManager_SendPrompt_EmptyPrompt(t *testing.T) {
	srv := NewServer()
	hm := NewHandlerManager(nil, &fakeStore{}, srv)
	req := &Request{
		JSONRPC: Version,
		Method:  "sendPrompt",
		Params:  json.RawMessage(`{"prompt":""}`),
	}
	resp := hm.handleSendPrompt(context.Background(), req)

	if resp.Error == nil {
		t.Fatal("expected error for empty prompt")
	}
	if resp.Error.Code != ErrInvalidParams {
		t.Errorf("error code = %d, want %d", resp.Error.Code, ErrInvalidParams)
	}
}

func TestHandlerManager_SendPrompt_NilAgent(t *testing.T) {
	srv := NewServer()
	hm := NewHandlerManager(nil, &fakeStore{}, srv)
	req := &Request{
		JSONRPC: Version,
		Method:  "sendPrompt",
		Params:  json.RawMessage(`{"prompt":"hello"}`),
	}
	resp := hm.handleSendPrompt(context.Background(), req)

	if resp.Error == nil {
		t.Fatal("expected error for nil agent")
	}
	if resp.Error.Code != ErrServer {
		t.Errorf("error code = %d, want %d", resp.Error.Code, ErrServer)
	}
}

func TestHandlerManager_GetSession_InvalidParams(t *testing.T) {
	srv := NewServer()
	hm := NewHandlerManager(nil, &fakeStore{}, srv)

	tests := []struct {
		name    string
		params  string
		wantErr int
	}{
		{
			name:    "missing id",
			params:  `{}`,
			wantErr: ErrInvalidParams,
		},
		{
			name:    "empty id",
			params:  `{"id":""}`,
			wantErr: ErrInvalidParams,
		},
		{
			name:    "invalid JSON",
			params:  `not json`,
			wantErr: ErrInvalidParams,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &Request{
				JSONRPC: Version,
				Method:  "getSession",
				Params:  json.RawMessage(tt.params),
			}
			resp := hm.handleGetSession(context.Background(), req)
			if resp.Error == nil {
				t.Fatal("expected error")
			}
			if resp.Error.Code != tt.wantErr {
				t.Errorf("error code = %d, want %d", resp.Error.Code, tt.wantErr)
			}
		})
	}
}

func TestHandlerManager_GetSession_NotFound(t *testing.T) {
	srv := NewServer()
	store := &fakeStore{}
	hm := NewHandlerManager(nil, store, srv)

	req := &Request{
		JSONRPC: Version,
		Method:  "getSession",
		Params:  json.RawMessage(`{"id":"nonexistent"}`),
	}
	resp := hm.handleGetSession(context.Background(), req)

	// Session not found is not an error (nil session), it returns ErrServer
	if resp.Error == nil {
		t.Fatal("expected error for nonexistent session")
	}
}

func TestHandlerManager_GetSession_Success(t *testing.T) {
	srv := NewServer()
	store := &fakeStore{
		getSessionDetailFn: func(ctx context.Context, id string) (*db.SessionDetail, error) {
			return &db.SessionDetail{
				ID:            id,
				Task:          "test task",
				Model:         "test-model",
				Status:        "active",
				MessageCount:  5,
				ToolCallCount: 3,
				TotalCost:     0.05,
				Duration:      1000,
			}, nil
		},
	}
	hm := NewHandlerManager(nil, store, srv)

	req := &Request{
		JSONRPC: Version,
		Method:  "getSession",
		Params:  json.RawMessage(`{"id":"session-1"}`),
	}
	resp := hm.handleGetSession(context.Background(), req)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	var info SessionInfo
	if err := json.Unmarshal(resp.Result, &info); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if info.ID != "session-1" {
		t.Errorf("ID = %q, want %q", info.ID, "session-1")
	}
	if info.MessageCount != 5 {
		t.Errorf("MessageCount = %d, want %d", info.MessageCount, 5)
	}
	if info.TotalCost != 0.05 {
		t.Errorf("TotalCost = %f, want %f", info.TotalCost, 0.05)
	}
}

func TestHandlerManager_ListSessions(t *testing.T) {
	srv := NewServer()
	store := &fakeStore{
		listSessionsDetailFn: func(ctx context.Context, limit int) ([]db.SessionDetail, error) {
			return []db.SessionDetail{
				{ID: "s1", Status: "active", MessageCount: 2},
				{ID: "s2", Status: "completed", MessageCount: 10},
			}, nil
		},
	}
	hm := NewHandlerManager(nil, store, srv)

	req := &Request{
		JSONRPC: Version,
		Method:  "listSessions",
	}
	resp := hm.handleListSessions(context.Background(), req)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	var infos []SessionInfo
	if err := json.Unmarshal(resp.Result, &infos); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(infos) != 2 {
		t.Fatalf("got %d sessions, want 2", len(infos))
	}
	if infos[0].ID != "s1" {
		t.Errorf("first session ID = %q, want %q", infos[0].ID, "s1")
	}
}

func TestHandlerManager_ListSessions_StoreError(t *testing.T) {
	srv := NewServer()
	store := &fakeStore{
		listSessionsDetailFn: func(ctx context.Context, limit int) ([]db.SessionDetail, error) {
			return nil, errors.New("db error")
		},
	}
	hm := NewHandlerManager(nil, store, srv)

	req := &Request{
		JSONRPC: Version,
		Method:  "listSessions",
	}
	resp := hm.handleListSessions(context.Background(), req)

	if resp.Error == nil {
		t.Fatal("expected error for store error")
	}
}

func TestHandlerManager_Interrupt_NilAgent(t *testing.T) {
	srv := NewServer()
	hm := NewHandlerManager(nil, &fakeStore{}, srv)

	// With nil agent, interrupt still works (no-op)
	req := &Request{
		JSONRPC: Version,
		Method:  "interrupt",
	}
	resp := hm.handleInterrupt(context.Background(), req)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
}

func TestMarshalEventPayload(t *testing.T) {
	tests := []struct {
		name string
		ev   agent.Event
	}{
		{
			name: "text event",
			ev:   agent.Event{Type: "text", Text: "hello"},
		},
		{
			name: "tool start event",
			ev:   agent.Event{Type: "tool_start", Tool: "read_file", Args: `{"path":"main.go"}`},
		},
		{
			name: "error event",
			ev:   agent.Event{Type: "error", Err: errors.New("something failed")},
		},
		{
			name: "tool done event",
			ev:   agent.Event{Type: "tool_done", Tool: "write_file", Result: "ok"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := marshalEventPayload(tt.ev)
			if payload == nil {
				t.Fatal("marshalEventPayload returned nil")
			}
			// Verify it's valid JSON
			var m map[string]any
			if err := json.Unmarshal(payload, &m); err != nil {
				t.Fatalf("invalid JSON: %v", err)
			}
			if m["type"] != tt.ev.Type {
				t.Errorf("type = %v, want %q", m["type"], tt.ev.Type)
			}
		})
	}
}


