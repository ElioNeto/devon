package testutil

import (
	"context"
	"encoding/json"
	"sync/atomic"

	"github.com/ElioNeto/devon/internal/permissions"
)

// MockTool implements tools.Tool for agent loop tests.
// It provides deterministic behavior: returns a pre-configured Result or Err.
type MockTool struct {
	// Name is the tool name returned by Name().
	NameValue string
	// Description is returned by Description().
	DescriptionValue string
	// Schema is the JSON schema returned by Schema().
	SchemaValue json.RawMessage
	// Result is returned by Execute() when Err is nil.
	Result string
	// Err, if set, is returned by Execute() instead of Result.
	Err error
	// Called tracks how many times Execute() was invoked.
	Called atomic.Int32
	// PermissionLevel is returned by Permission().
	PermissionLevel permissions.PermissionLevel
}

// Name returns the tool name.
func (m *MockTool) Name() string {
	return m.NameValue
}

// Description returns the tool description.
func (m *MockTool) Description() string {
	if m.DescriptionValue != "" {
		return m.DescriptionValue
	}
	return "mock tool for testing"
}

// Schema returns the JSON schema for tool parameters.
func (m *MockTool) Schema() json.RawMessage {
	if len(m.SchemaValue) > 0 {
		return m.SchemaValue
	}
	return json.RawMessage(`{"type":"object","properties":{}}`)
}

// Execute increments Called and returns Result or Err.
// It respects context cancellation.
func (m *MockTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	m.Called.Add(1)

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	if m.Err != nil {
		return "", m.Err
	}
	return m.Result, nil
}

// Permission returns the permission level of this tool.
func (m *MockTool) Permission() permissions.PermissionLevel {
	return m.PermissionLevel
}
