// Package mcp implements Model Context Protocol client and transport layers.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
)

// Transport defines the interface for MCP transport layers.
type Transport interface {
	// Connect establishes the connection to the MCP server.
	Connect(ctx context.Context) error
	// Close terminates the connection.
	Close() error
	// Send sends a JSON-RPC request and returns the response.
	Send(ctx context.Context, req JsonRpcRequest) (*JsonRpcResponse, error)
}

// TransportConfig holds configuration for creating transports.
type TransportConfig struct {
	Type    string            `json:"type"` // "stdio" or "http"
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

// NewTransport creates a new transport based on the configuration.
func NewTransport(cfg TransportConfig) (Transport, error) {
	switch cfg.Type {
	case "stdio":
		if cfg.Command == "" {
			return nil, fmt.Errorf("stdio transport requires command")
		}
		return &stdioTransport{
			command: cfg.Command,
			args:    cfg.Args,
			env:     cfg.Env,
		}, nil
	case "http":
		if cfg.URL == "" {
			return nil, fmt.Errorf("http transport requires url")
		}
		return &httpTransport{
			url:     cfg.URL,
			headers: cfg.Headers,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported transport type: %s", cfg.Type)
	}
}

// sendRequest is a helper to marshal and send a request via a transport.
func sendRequest(ctx context.Context, t Transport, method string, params interface{}, id interface{}) (*JsonRpcResponse, error) {
	paramsData, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params: %w", err)
	}

	req := JsonRpcRequest{
		JsonRpc: "2.0",
		Method:  method,
		Params:  paramsData,
		ID:      id,
	}

	return t.Send(ctx, req)
}
