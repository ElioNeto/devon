// Package rpc implements a JSON-RPC 2.0 server over Unix sockets for
// communication between the Devon agent and the VS Code extension.
package rpc

import "encoding/json"

// JSON-RPC 2.0 constants
const (
	Version = "2.0"
)

// Request represents a JSON-RPC 2.0 request.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response represents a JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC 2.0 error object.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Standard JSON-RPC 2.0 error codes.
const (
	ErrParse           = -32700
	ErrInvalidRequest  = -32600
	ErrMethodNotFound  = -32601
	ErrInvalidParams   = -32602
	ErrInternal        = -32603
	ErrServer          = -32000 // Custom server error base
)

// Event represents a server-sent event pushed to connected clients.
type Event struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// SessionInfo represents a session object returned by getSession / listSessions.
type SessionInfo struct {
	ID            string  `json:"id"`
	Task          string  `json:"task,omitempty"`
	Model         string  `json:"model,omitempty"`
	Status        string  `json:"status"`
	MessageCount  int     `json:"message_count"`
	ToolCallCount int     `json:"tool_call_count"`
	TotalCost     float64 `json:"total_cost,omitempty"`
	Duration      int64   `json:"duration_ms,omitempty"`
}

// StatusInfo represents the result of getStatus.
type StatusInfo struct {
	Running   bool   `json:"running"`
	SessionID string `json:"session_id,omitempty"`
	Model     string `json:"model,omitempty"`
	TaskType  string `json:"task_type,omitempty"`
}

// SendPromptParams are the parameters for the sendPrompt method.
type SendPromptParams struct {
	Prompt string `json:"prompt"`
	Mode   string `json:"mode,omitempty"`
}

// InterruptParams are the parameters for the interrupt method.
type InterruptParams struct{}

// GetSessionParams are the parameters for the getSession method.
type GetSessionParams struct {
	ID string `json:"id"`
}

// ListSessionsParams are the parameters for the listSessions method.
type ListSessionsParams struct {
	Limit int `json:"limit,omitempty"`
}

// NewResponse builds a JSON-RPC 2.0 success response.
func NewResponse(id *int64, result any) Response {
	var raw json.RawMessage
	if result != nil {
		raw, _ = json.Marshal(result)
	}
	return Response{
		JSONRPC: Version,
		ID:      id,
		Result:  raw,
	}
}

// NewErrorResponse builds a JSON-RPC 2.0 error response.
func NewErrorResponse(id *int64, code int, msg string, data any) Response {
	return Response{
		JSONRPC: Version,
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: msg,
			Data:    data,
		},
	}
}
