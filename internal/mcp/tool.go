// Package mcp implements Model Context Protocol client and transport layers.
package mcp

import (
	"context"
	"encoding/json"

	"github.com/ElioNeto/devon/internal/llm"
	"github.com/ElioNeto/devon/internal/permissions"
	"github.com/ElioNeto/devon/internal/tools"
)

// MCPTool implements the tools.Tool interface for MCP tools.
type MCPTool struct {
	name        string
	description string
	schema      json.RawMessage
	client      *Client
}

// NewMCPTool creates a new MCPTool from an MCP Tool definition.
func NewMCPTool(tool Tool, client *Client) *MCPTool {
	return &MCPTool{
		name:        tool.Name,
		description: tool.Description,
		schema:      tool.InputSchema,
		client:      client,
	}
}

// Name returns the tool name.
func (t *MCPTool) Name() string {
	return t.name
}

// Description returns the tool description.
func (t *MCPTool) Description() string {
	return t.description
}

// Schema returns the JSON Schema for the tool parameters.
func (t *MCPTool) Schema() json.RawMessage {
	return t.schema
}

// Execute executes the tool with the given parameters.
func (t *MCPTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	return t.client.CallTool(ctx, t.name, params)
}

// Permission returns the permission level for the tool.
func (t *MCPTool) Permission() permissions.PermissionLevel {
	// MCP tools default to write mode (require confirmation for destructive operations)
	return permissions.PermWrite
}

// ToToolDef converts the MCPTool to an llm.ToolDef for the LLM.
func (t *MCPTool) ToToolDef() llm.ToolDef {
	return llm.ToolDef{
		Type: "function",
		Function: llm.ToolDefFunc{
			Name:        t.name,
			Description: t.description,
			Parameters:  t.schema,
		},
	}
}

// Ensure MCPTool implements tools.Tool interface.
var _ tools.Tool = (*MCPTool)(nil)
