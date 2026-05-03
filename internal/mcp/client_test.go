// Package mcp implements Model Context Protocol client and transport layers.
package mcp

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestClient_Connect(t *testing.T) {
	server := newMockMCPServer()
	ts := startHTTPMock(t, server)
	defer ts.Close()
	
	transport := &httpTransport{url: ts.URL}
	client := NewClient(transport)
	
	if err := client.Connect(context.Background()); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	
	if !client.initialized {
		t.Error("client should be initialized")
	}
	
	if client.serverInfo.Name != "mock-mcp-server" {
		t.Errorf("expected server name 'mock-mcp-server', got %s", client.serverInfo.Name)
	}
}

func TestClient_ConnectError(t *testing.T) {
	server := newMockMCPServer()
	server.initError = json.Unmarshal([]byte("invalid"), &struct{}{})
	
	ts := startHTTPMock(t, server)
	defer ts.Close()
	
	transport := &httpTransport{url: ts.URL}
	client := NewClient(transport)
	
	err := client.Connect(context.Background())
	if err == nil {
		t.Error("expected error")
	}
}

func TestClient_ListTools(t *testing.T) {
	server := newMockMCPServer()
	ts := startHTTPMock(t, server)
	defer ts.Close()
	
	transport := &httpTransport{url: ts.URL}
	client := NewClient(transport)
	
	if err := client.Connect(context.Background()); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	
	tools, err := client.ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}
	
	if len(tools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(tools))
	}
	
	if tools[0].Name != "test_tool" {
		t.Errorf("expected first tool name 'test_tool', got %s", tools[0].Name)
	}
}

func TestClient_ListToolsNotInitialized(t *testing.T) {
	transport := &httpTransport{url: "http://example.com"}
	client := NewClient(transport)
	
	_, err := client.ListTools(context.Background())
	if err == nil {
		t.Error("expected error when not initialized")
	}
}

func TestClient_CallTool(t *testing.T) {
	server := newMockMCPServer()
	ts := startHTTPMock(t, server)
	defer ts.Close()
	
	transport := &httpTransport{url: ts.URL}
	client := NewClient(transport)
	
	if err := client.Connect(context.Background()); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	
	args := json.RawMessage(`{"input": "hello world"}`)
	result, err := client.CallTool(context.Background(), "test_tool", args)
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}
	
	if result != "hello world" {
		t.Errorf("expected 'hello world', got %s", result)
	}
}

func TestClient_CallToolError(t *testing.T) {
	server := newMockMCPServer()
	server.setCallError("test_tool", json.Unmarshal([]byte(""), &struct{}{})) // Will cause issues
	
	ts := startHTTPMock(t, server)
	defer ts.Close()
	
	transport := &httpTransport{url: ts.URL}
	client := NewClient(transport)
	
	if err := client.Connect(context.Background()); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	
	args := json.RawMessage(`{"input": "test"}`)
	_, err := client.CallTool(context.Background(), "test_tool", args)
	if err == nil {
		t.Error("expected error")
	}
}

func TestClient_CallToolNotInitialized(t *testing.T) {
	transport := &httpTransport{url: "http://example.com"}
	client := NewClient(transport)
	
	args := json.RawMessage(`{}`)
	_, err := client.CallTool(context.Background(), "test_tool", args)
	if err == nil {
		t.Error("expected error when not initialized")
	}
}

func TestClient_Close(t *testing.T) {
	server := newMockMCPServer()
	ts := startHTTPMock(t, server)
	
	transport := &httpTransport{url: ts.URL}
	client := NewClient(transport)
	
	if err := client.Connect(context.Background()); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	
	// HTTP transport Close is no-op, but shouldn't error
	if err := client.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	
	ts.Close()
}

func TestClient_ServerInfo(t *testing.T) {
	server := newMockMCPServer()
	ts := startHTTPMock(t, server)
	defer ts.Close()
	
	transport := &httpTransport{url: ts.URL}
	client := NewClient(transport)
	
	if err := client.Connect(context.Background()); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	
	info := client.ServerInfo()
	if info.Name != "mock-mcp-server" {
		t.Errorf("expected 'mock-mcp-server', got %s", info.Name)
	}
}

func TestClient_ContextTimeout(t *testing.T) {
	server := newMockMCPServer()
	server.setCallDelay("test_tool", 2*time.Second)
	
	ts := startHTTPMock(t, server)
	defer ts.Close()
	
	transport := &httpTransport{url: ts.URL}
	client := NewClient(transport)
	
	if err := client.Connect(context.Background()); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	
	args := json.RawMessage(`{"input": "test"}`)
	_, err := client.CallTool(ctx, "test_tool", args)
	if err == nil {
		t.Error("expected timeout error")
	}
}
