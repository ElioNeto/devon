// Package mcp implements Model Context Protocol client and transport layers.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// Client represents an MCP client connection to a server.
type Client struct {
	transport Transport
	serverInfo ServerInfo
	mu         sync.Mutex
	initialized bool
}

// NewClient creates a new MCP client with the given transport.
func NewClient(transport Transport) *Client {
	return &Client{
		transport: transport,
	}
}

// Connect initializes the connection to the MCP server.
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.transport.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect transport: %w", err)
	}

	// Initialize the protocol
	params := InitializeParams{
		ProtocolVersion: "2024-11-05",
		ClientInfo: ClientInfo{
			Name:    "devon",
			Version: "1.0.0",
		},
	}

	resp, err := sendRequest(ctx, c.transport, "initialize", params, 1)
	if err != nil {
		return fmt.Errorf("initialize request failed: %w", err)
	}

	if resp.Error != nil {
		return fmt.Errorf("initialize error: code=%d message=%s", resp.Error.Code, resp.Error.Message)
	}

	var result InitializeResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("failed to unmarshal initialize result: %w", err)
	}

	c.serverInfo = result.ServerInfo
	c.initialized = true

	// Send initialized notification
	notif := JsonRpcRequest{
		JsonRpc: "2.0",
		Method:  "notifications/initialized",
	}
	if _, err := c.transport.Send(ctx, notif); err != nil {
		// Log warning but don't fail - notification is not critical
		fmt.Printf("warning: failed to send initialized notification: %v\n", err)
	}

	return nil
}

// ListTools lists all available tools from the MCP server.
func (c *Client) ListTools(ctx context.Context) ([]Tool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.initialized {
		return nil, fmt.Errorf("client not initialized")
	}

	resp, err := sendRequest(ctx, c.transport, "tools/list", nil, 2)
	if err != nil {
		return nil, fmt.Errorf("tools/list request failed: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("tools/list error: code=%d message=%s", resp.Error.Code, resp.Error.Message)
	}

	var result ListToolsResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tools/list result: %w", err)
	}

	return result.Tools, nil
}

// CallTool calls a tool on the MCP server with the given arguments.
func (c *Client) CallTool(ctx context.Context, name string, arguments json.RawMessage) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.initialized {
		return "", fmt.Errorf("client not initialized")
	}

	params := CallToolParams{
		Name:      name,
		Arguments: arguments,
	}

	resp, err := sendRequest(ctx, c.transport, "tools/call", params, 3)
	if err != nil {
		return "", fmt.Errorf("tools/call request failed: %w", err)
	}

	if resp.Error != nil {
		return "", fmt.Errorf("tools/call error: code=%d message=%s", resp.Error.Code, resp.Error.Message)
	}

	var result CallToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", fmt.Errorf("failed to unmarshal tools/call result: %w", err)
	}

	if result.IsError {
		// Concatenate error text from content blocks
		var errText string
		for _, block := range result.Content {
			if block.Type == "text" {
				errText += block.Text
			}
		}
		return "", fmt.Errorf("tool error: %s", errText)
	}

	// Concatenate text from content blocks
	var text string
	for _, block := range result.Content {
		if block.Type == "text" {
			text += block.Text
		}
	}

	return text, nil
}

// Close closes the connection to the MCP server.
func (c *Client) Close() error {
	return c.transport.Close()
}

// ServerInfo returns the server information from initialization.
func (c *Client) ServerInfo() ServerInfo {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.serverInfo
}
