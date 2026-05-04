// Package mcp implements Model Context Protocol client and transport layers.
package mcp

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ElioNeto/devon/internal/config"
	"github.com/ElioNeto/devon/internal/tools"
)

// RegistryHelper provides helper functions to initialize MCP servers from config.
type RegistryHelper struct {
	logger *slog.Logger
}

// NewRegistryHelper creates a new RegistryHelper.
func NewRegistryHelper(logger *slog.Logger) *RegistryHelper {
	if logger == nil {
		logger = slog.Default()
	}
	return &RegistryHelper{logger: logger}
}

// InitMCPServersFromConfig initializes MCP servers from configuration and registers their tools.
// If a server fails to connect, it logs a warning and continues without that server's tools.
func (h *RegistryHelper) InitMCPServersFromConfig(ctx context.Context, configs []config.MCPServerConfig, registry *tools.Registry) {
	for _, cfg := range configs {
		if !cfg.Enabled {
			h.logger.Info("MCP server disabled, skipping", "name", cfg.Name)
			continue
		}

		h.logger.Info("Initializing MCP server", "name", cfg.Name, "type", cfg.Type)

		tools, err := h.initServer(ctx, cfg)
		if err != nil {
			h.logger.Warn("Failed to initialize MCP server, continuing without it",
				"name", cfg.Name,
				"error", err)
			continue
		}

		for _, tool := range tools {
			registry.Register(tool)
			h.logger.Info("Registered MCP tool", "server", cfg.Name, "tool", tool.Name())
		}
	}
}

// initServer initializes a single MCP server and returns its tools.
func (h *RegistryHelper) initServer(ctx context.Context, cfg config.MCPServerConfig) ([]*MCPTool, error) {
	transportConfig := ToTransportConfig(cfg)

	transport, err := NewTransport(transportConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %w", err)
	}

	client := NewClient(transport)

	if err := client.Connect(ctx); err != nil {
		transport.Close()
		return nil, fmt.Errorf("failed to connect to MCP server: %w", err)
	}

	// List tools from the server
	mcpTools, err := client.ListTools(ctx)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}

	// Convert MCP tools to MCPTool instances
	tools := make([]*MCPTool, 0, len(mcpTools))
	for _, mcpTool := range mcpTools {
		tools = append(tools, NewMCPTool(mcpTool, client))
	}

	return tools, nil
}

// InitMCPServersFromConfigFunc returns a function that can be called to initialize MCP servers.
// This is useful for dependency injection patterns.
func InitMCPServersFromConfigFunc(configs []config.MCPServerConfig, logger *slog.Logger) func(context.Context, *tools.Registry) {
	helper := NewRegistryHelper(logger)
	return func(ctx context.Context, registry *tools.Registry) {
		helper.InitMCPServersFromConfig(ctx, configs, registry)
	}
}
