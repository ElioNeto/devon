// Package mcp implements Model Context Protocol client and transport layers.
package mcp

import (
	"context"
	"encoding/json"
	"testing"
)

func TestMCPTool_Name(t *testing.T) {
	tool := &MCPTool{
		name:        "test_tool",
		description: "A test tool",
		schema:      json.RawMessage("{}"),
		client:      nil,
	}
	
	if tool.Name() != "test_tool" {
		t.Errorf("expected 'test_tool', got %s", tool.Name())
	}
}

func TestMCPTool_Description(t *testing.T) {
	tool := &MCPTool{
		name:        "test_tool",
		description: "A test tool",
		schema:      json.RawMessage("{}"),
		client:      nil,
	}
	
	if tool.Description() != "A test tool" {
		t.Errorf("expected 'A test tool', got %s", tool.Description())
	}
}

func TestMCPTool_Schema(t *testing.T) {
	schema := json.RawMessage(`{"type": "object"}`)
	tool := &MCPTool{
		name:        "test_tool",
		description: "A test tool",
		schema:      schema,
		client:      nil,
	}
	
	if string(tool.Schema()) != string(schema) {
		t.Errorf("schema mismatch")
	}
}

func TestMCPTool_Execute(t *testing.T) {
	server := newMockMCPServer()
	ts := startHTTPMock(t, server)
	defer ts.Close()
	
	transport := &httpTransport{url: ts.URL}
	client := NewClient(transport)
	
	if err := client.Connect(context.Background()); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	
	tool := NewMCPTool(Tool{
		Name:        "echo_tool",
		Description: "Echoes the input",
		InputSchema: json.RawMessage(`{"type": "object"}`),
	}, client)
	
	args := json.RawMessage(`{"message": "hello"}`)
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	
	if result != "hello" {
		t.Errorf("expected 'hello', got %s", result)
	}
}

func TestMCPTool_ExecuteError(t *testing.T) {
	server := newMockMCPServer()
	server.setCallError("test_tool", json.Unmarshal([]byte(""), &struct{}{}))
	
	ts := startHTTPMock(t, server)
	defer ts.Close()
	
	transport := &httpTransport{url: ts.URL}
	client := NewClient(transport)
	
	if err := client.Connect(context.Background()); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	
	tool := NewMCPTool(Tool{
		Name:        "test_tool",
		Description: "A test tool",
		InputSchema: json.RawMessage("{}"),
	}, client)
	
	args := json.RawMessage(`{"input": "test"}`)
	_, err := tool.Execute(context.Background(), args)
	if err == nil {
		t.Error("expected error")
	}
}

func TestMCPTool_Permission(t *testing.T) {
	tool := &MCPTool{
		name:        "test_tool",
		description: "A test tool",
		schema:      json.RawMessage("{}"),
		client:      nil,
	}
	
	// MCP tools default to PermWrite
	perm := tool.Permission()
	// Permission() returns permissions.PermissionLevel, not a pointer
	// Just verify it's not the zero value
	if perm == "" {
		t.Error("expected permission level")
	}
}

func TestMCPTool_ToToolDef(t *testing.T) {
	schema := json.RawMessage(`{"type": "object", "properties": {"input": {"type": "string"}}}`)
	tool := NewMCPTool(Tool{
		Name:        "test_tool",
		Description: "A test tool",
		InputSchema: schema,
	}, nil)
	
	toolDef := tool.ToToolDef()
	if toolDef.Type != "function" {
		t.Errorf("expected type 'function', got %s", toolDef.Type)
	}
	if toolDef.Function.Name != "test_tool" {
		t.Errorf("expected name 'test_tool', got %s", toolDef.Function.Name)
	}
	if toolDef.Function.Description != "A test tool" {
		t.Errorf("expected description 'A test tool', got %s", toolDef.Function.Description)
	}
}

func TestNewMCPTool(t *testing.T) {
	server := newMockMCPServer()
	ts := startHTTPMock(t, server)
	defer ts.Close()
	
	transport := &httpTransport{url: ts.URL}
	client := NewClient(transport)
	
	if err := client.Connect(context.Background()); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	
	mcpTool := Tool{
		Name:        "test_tool",
		Description: "A test tool",
		InputSchema: json.RawMessage(`{"type": "object"}`),
	}
	
	tool := NewMCPTool(mcpTool, client)
	if tool == nil {
		t.Fatal("NewMCPTool returned nil")
	}
	if tool.Name() != "test_tool" {
		t.Errorf("expected name 'test_tool', got %s", tool.Name())
	}
}
