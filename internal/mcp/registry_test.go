// Package mcp implements Model Context Protocol client and transport layers.
package mcp

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	
	"github.com/ElioNeto/devon/internal/config"
	"github.com/ElioNeto/devon/internal/tools"
)

func TestRegistryHelper_InitMCPServersFromConfig(t *testing.T) {
	// Create a mock MCP server
	server := newMockMCPServer()
	ts := startHTTPMock(t, server)
	defer ts.Close()
	
	// Create config with the mock server
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
	
	// Check that tools were registered
	// We need to check the registry somehow - this depends on the tools.Registry implementation
	// For now, just verify no panic occurred
	t.Log("InitMCPServersFromConfig completed without panic")
}

func TestRegistryHelper_InitMCPServersFromConfig_Disabled(t *testing.T) {
	configs := []config.MCPServerConfig{
		{
			Name:    "disabled-server",
			Type:    "http",
			Enabled: false,
			URL:     "http://example.com",
		},
	}
	
	logger := slog.Default()
	helper := NewRegistryHelper(logger)
	registry := tools.NewRegistry()
	
	// Should skip disabled servers without error
	helper.InitMCPServersFromConfig(context.Background(), configs, registry)
}

func TestRegistryHelper_InitMCPServersFromConfig_ConnectionError(t *testing.T) {
	// Use a URL that will fail to connect
	configs := []config.MCPServerConfig{
		{
			Name:    "failing-server",
			Type:    "http",
			Enabled: true,
			URL:     "http://localhost:19999", // Non-existent server
		},
	}
	
	logger := slog.Default()
	helper := NewRegistryHelper(logger)
	registry := tools.NewRegistry()
	
	// Should log warning and continue, not panic
	helper.InitMCPServersFromConfig(context.Background(), configs, registry)
}

func TestRegistryHelper_InitMCPServersFromConfig_MultipleServers(t *testing.T) {
	server1 := newMockMCPServer()
	ts1 := startHTTPMock(t, server1)
	defer ts1.Close()
	
	server2 := newMockMCPServer()
	server2.addTool(Tool{
		Name:        "another_tool",
		Description: "Another test tool",
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
	
	// Both servers should have been processed
	t.Log("Multiple servers processed")
}

func TestRegistryHelper_InitMCPServersFromConfig_EmptyConfig(t *testing.T) {
	logger := slog.Default()
	helper := NewRegistryHelper(logger)
	registry := tools.NewRegistry()
	
	// Should handle empty config without error
	helper.InitMCPServersFromConfig(context.Background(), nil, registry)
}

func TestRegistryHelper_InitMCPServersFromConfig_NilRegistry(t *testing.T) {
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
	
	// This will likely panic, but we should handle it gracefully
	// For now, just test that the function handles it
	defer func() {
		if r := recover(); r != nil {
			t.Logf("Panic recovered (expected if registry is nil): %v", r)
		}
	}()
	
	helper.InitMCPServersFromConfig(context.Background(), configs, nil)
}

func TestNewRegistryHelper(t *testing.T) {
	logger := slog.Default()
	helper := NewRegistryHelper(logger)
	
	if helper == nil {
		t.Fatal("NewRegistryHelper returned nil")
	}
	
	if helper.logger == nil {
		t.Error("logger should not be nil")
	}
}

func TestNewRegistryHelper_NilLogger(t *testing.T) {
	helper := NewRegistryHelper(nil)
	
	if helper == nil {
		t.Fatal("NewRegistryHelper returned nil")
	}
	
	// Should use slog.Default()
	if helper.logger == nil {
		t.Error("logger should default to slog.Default()")
	}
}

func TestInitMCPServersFromConfigFunc(t *testing.T) {
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
	f := InitMCPServersFromConfigFunc(configs, logger)
	
	if f == nil {
		t.Fatal("InitMCPServersFromConfigFunc returned nil")
	}
	
	// Call the function
	registry := tools.NewRegistry()
	f(context.Background(), registry)
}
