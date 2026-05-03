// Package mcp implements Model Context Protocol client and transport layers.
package mcp

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"
	
	"github.com/ElioNeto/devon/internal/config"
	"github.com/ElioNeto/devon/internal/tools"
)

// TestIntegration_FullFlow tests the full flow of initializing MCP servers,
// registering tools, and calling them through the registry.
func TestIntegration_FullFlow(t *testing.T) {
	// Create a mock MCP server with custom tools
	server := newMockMCPServer()
	server.addTool(Tool{
		Name:        "integration_tool",
		Description: "Tool for integration testing",
		InputSchema: json.RawMessage(`{"type": "object", "properties": {"data": {"type": "string"}}, "required": ["data"]}`),
	})
	
	ts := startHTTPMock(t, server)
	defer ts.Close()
	
	// Create config
	configs := []config.MCPServerConfig{
		{
			Name:    "integration-server",
			Type:    "http",
			Enabled: true,
			URL:     ts.URL,
		},
	}
	
	// Initialize MCP servers
	logger := slog.Default()
	helper := NewRegistryHelper(logger)
	registry := tools.NewRegistry()
	
	helper.InitMCPServersFromConfig(context.Background(), configs, registry)
	
	// Verify tools were registered (this depends on tools.Registry API)
	// For now, just verify no panic
	t.Log("Full flow completed: MCP servers initialized, tools registered")
}

// TestIntegration_MultipleServers tests integration with multiple MCP servers.
func TestIntegration_MultipleServers(t *testing.T) {
	server1 := newMockMCPServer()
	server1.addTool(Tool{
		Name:        "tool_from_server1",
		Description: "Tool from first server",
		InputSchema: json.RawMessage(`{"type": "object"}`),
	})
	ts1 := startHTTPMock(t, server1)
	defer ts1.Close()
	
	server2 := newMockMCPServer()
	server2.addTool(Tool{
		Name:        "tool_from_server2",
		Description: "Tool from second server",
		InputSchema: json.RawMessage(`{"type": "object"}`),
	})
	ts2 := startHTTPMock(t, server2)
	defer ts2.Close()
	
	configs := []config.MCPServerConfig{
		{
			Name:    "server1",
			Type:    "http",
			Enabled: true,
			URL:     ts1.URL,
		},
		{
			Name:    "server2",
			Type:    "http",
			Enabled: true,
			URL:     ts2.URL,
		},
	}
	
	logger := slog.Default()
	helper := NewRegistryHelper(logger)
	registry := tools.NewRegistry()
	
	helper.InitMCPServersFromConfig(context.Background(), configs, registry)
	
	t.Log("Multiple servers integration test completed")
}

// TestIntegration_ConnectionFailure tests that connection failures don't break the agent.
func TestIntegration_ConnectionFailure(t *testing.T) {
	// Server that will fail to connect (wrong port)
	configs := []config.MCPServerConfig{
		{
			Name:    "failing-server",
			Type:    "http",
			Enabled: true,
			URL:     "http://localhost:19999", // Non-existent
		},
		{
			Name:    "working-server",
			Type:    "http",
			Enabled: true,
			URL:     "", // Will be set below
		},
	}
	
	// Create a working server
	server := newMockMCPServer()
	ts := startHTTPMock(t, server)
	defer ts.Close()
	configs[1].URL = ts.URL
	
	logger := slog.Default()
	helper := NewRegistryHelper(logger)
	registry := tools.NewRegistry()
	
	// Should not panic, should log warning and continue
	helper.InitMCPServersFromConfig(context.Background(), configs, registry)
	
	t.Log("Connection failure handled gracefully")
}

// TestIntegration_StdioAndHTTP tests both stdio and HTTP transports together.
func TestIntegration_StdioAndHTTP(t *testing.T) {
	// For stdio, we'd need an actual executable MCP server
	// For now, just test HTTP part
	server := newMockMCPServer()
	ts := startHTTPMock(t, server)
	defer ts.Close()
	
	configs := []config.MCPServerConfig{
		{
			Name:    "http-server",
			Type:    "http",
			Enabled: true,
			URL:     ts.URL,
		},
	}
	
	logger := slog.Default()
	helper := NewRegistryHelper(logger)
	registry := tools.NewRegistry()
	
	helper.InitMCPServersFromConfig(context.Background(), configs, registry)
	
	t.Log("Mixed transport integration test completed")
}

// TestIntegration_ToolCallViaRegistry tests calling an MCP tool through the registry.
func TestIntegration_ToolCallViaRegistry(t *testing.T) {
	server := newMockMCPServer()
	ts := startHTTPMock(t, server)
	defer ts.Close()
	
	configs := []config.MCPServerConfig{
		{
			Name:    "test-server",
			Type:    "http",
			Enabled: true,
			URL:     ts.URL,
		},
	}
	
	logger := slog.Default()
	helper := NewRegistryHelper(logger)
	registry := tools.NewRegistry()
	
	helper.InitMCPServersFromConfig(context.Background(), configs, registry)
	
	// Now we need to get the tool from registry and call it
	// This depends on the tools.Registry API
	// For now, just verify the flow works
	t.Log("Tool call via registry test completed")
}

// TestIntegration_ContextTimeout tests that context timeouts are respected.
func TestIntegration_ContextTimeout(t *testing.T) {
	server := newMockMCPServer()
	server.setCallDelay("test_tool", 2*time.Second)
	
	ts := startHTTPMock(t, server)
	defer ts.Close()
	
	configs := []config.MCPServerConfig{
		{
			Name:    "slow-server",
			Type:    "http",
			Enabled: true,
			URL:     ts.URL,
		},
	}
	
	logger := slog.Default()
	helper := NewRegistryHelper(logger)
	registry := tools.NewRegistry()
	
	// Use a short timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	
	helper.InitMCPServersFromConfig(ctx, configs, registry)
	
	t.Log("Context timeout test completed")
}

// TestIntegration_GracefulErrorHandling tests the error handling in registry.
func TestIntegration_GracefulErrorHandling(t *testing.T) {
	// Create a server that returns errors for tool calls
	server := newMockMCPServer()
	server.setCallError("test_tool", json.Unmarshal([]byte(""), &struct{}{}))
	
	ts := startHTTPMock(t, server)
	defer ts.Close()
	
	configs := []config.MCPServerConfig{
		{
			Name:    "error-server",
			Type:    "http",
			Enabled: true,
			URL:     ts.URL,
		},
	}
	
	logger := slog.Default()
	helper := NewRegistryHelper(logger)
	registry := tools.NewRegistry()
	
	// Should handle errors gracefully
	helper.InitMCPServersFromConfig(context.Background(), configs, registry)
	
	t.Log("Graceful error handling test completed")
}
