// Package headless implements an HTTP/SSE server for CI/CD and external clients.
//
// Transport: HTTP with Server-Sent Events (no gRPC dependency required).
// Event flow: text_delta → tool_start → tool_done → turn_done (streaming).
// Blocking flow: action_required blocks until client responds via /api/respond.
package headless

// SSE event type constants. These map to the "type" field in SSE data payloads.
const (
	EventTypeTextDelta      = "text_delta"
	EventTypeToolStart      = "tool_start"
	EventTypeToolDone       = "tool_done"
	EventTypeToolError      = "tool_error"
	EventTypeTurnDone       = "turn_done"
	EventTypeActionRequired = "action_required"
	EventTypeError          = "error"
	EventTypeSystem         = "system"
	EventTypeRateLimited    = "rate_limited"
	EventTypeFileChange     = "file_change"
)

// Default server configuration values.
const (
	DefaultHost = "127.0.0.1"
	DefaultPort = 9876
)

// PromptRequest is the JSON body for POST /api/prompt.
type PromptRequest struct {
	Prompt    string `json:"prompt"`
	Mode      string `json:"mode,omitempty"`       // "auto" | "safe" | "yolo"
	SessionID string `json:"session_id,omitempty"` // optional session resume
}

// EventResponse is a single SSE event payload sent to the client.
type EventResponse struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload,omitempty"`
}

// TextDeltaPayload is the payload for text_delta events.
type TextDeltaPayload struct {
	Text string `json:"text"`
}

// ToolStartPayload is the payload for tool_start events.
type ToolStartPayload struct {
	Tool string `json:"tool"`
	Args string `json:"args,omitempty"`
}

// ToolDonePayload is the payload for tool_done events.
type ToolDonePayload struct {
	Tool   string `json:"tool"`
	Result string `json:"result,omitempty"`
}

// ToolErrorPayload is the payload for tool_error events.
type ToolErrorPayload struct {
	Tool string `json:"tool"`
	Err  string `json:"error"`
}

// ActionRequiredPayload is the payload for action_required events.
type ActionRequiredPayload struct {
	Tool      string `json:"tool"`
	Args      string `json:"args,omitempty"`
	RequestID string `json:"request_id"`
}

// ActionRequiredRequest is the JSON body for POST /api/respond.
type ActionRequiredRequest struct {
	RequestID   string `json:"request_id"`
	Approved    bool   `json:"approved"`
	AlwaysAllow bool   `json:"always_allow,omitempty"`
}

// TurnDonePayload is the payload for turn_done events.
type TurnDonePayload struct{}

// ErrorPayload is the payload for error events.
type ErrorPayload struct {
	Message string `json:"message"`
}

// SystemPayload is the payload for system events.
type SystemPayload struct {
	Text string `json:"text"`
}
