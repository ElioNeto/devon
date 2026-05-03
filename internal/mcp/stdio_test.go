// Package mcp implements Model Context Protocol client and transport layers.
package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func buildMockServer(t *testing.T) string {
	t.Helper()
	destPath := filepath.Join("internal/mcp/testdata", "mock_mcp_server")
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Skipf("mock server binary not found at %s; run 'go build -o %s internal/mcp/testdata/mock_mcp_server.go' to build it", destPath, destPath)
	}
	return destPath
}

func TestStdioTransport_Connect(t *testing.T) {
	mockPath := buildMockServer(t)

	transport := &stdioTransport{
		command: mockPath,
		args:    []string{},
		env:     nil,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := transport.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	if transport.cmd == nil {
		t.Error("command should be set after Connect")
	}

	transport.Close()
}

func TestStdioTransport_Send(t *testing.T) {
	mockPath := buildMockServer(t)

	transport := &stdioTransport{
		command: mockPath,
		args:    []string{},
		env:     nil,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer transport.Close()

	// Test initialize request
	req := JsonRpcRequest{
		JsonRpc: "2.0",
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2024-11-05","clientInfo":{"name":"test","version":"1.0.0"}}`),
		ID:      1,
	}

	resp, err := transport.Send(ctx, req)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("initialize returned error: code=%d message=%s", resp.Error.Code, resp.Error.Message)
	}

	if resp.ID != 1 {
		t.Errorf("expected response ID 1, got %v", resp.ID)
	}
}

func TestStdioTransport_Close(t *testing.T) {
	mockPath := buildMockServer(t)

	transport := &stdioTransport{
		command: mockPath,
		args:    []string{},
	}

	ctx := context.Background()
	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	if err := transport.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

func TestStdioTransport_NotConnected(t *testing.T) {
	transport := &stdioTransport{
		command: "echo",
		args:    []string{"test"},
	}

	req := JsonRpcRequest{
		JsonRpc: "2.0",
		Method:  "test",
		ID:      1,
	}

	_, err := transport.Send(context.Background(), req)
	if err == nil {
		t.Error("expected error when transport not connected")
	}
}

// TestStdioTransportWithMock tests the stdio transport with a mock MCP server.
func TestStdioTransportWithMock(t *testing.T) {
	mockPath := buildMockServer(t)

	transport := &stdioTransport{
		command: mockPath,
		args:    []string{},
		env:     nil,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer transport.Close()

	// Initialize
	initReq := JsonRpcRequest{
		JsonRpc: "2.0",
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2024-11-05","clientInfo":{"name":"test","version":"1.0.0"}}`),
		ID:      1,
	}

	_, err := transport.Send(ctx, initReq)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Send initialized notification
	notif := JsonRpcRequest{
		JsonRpc: "2.0",
		Method:  "notifications/initialized",
	}
	_, _ = transport.Send(ctx, notif)

	// List tools
	listReq := JsonRpcRequest{
		JsonRpc: "2.0",
		Method:  "tools/list",
		ID:      2,
	}

	listResp, err := transport.Send(ctx, listReq)
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}

	var listResult ListToolsResult
	if err := json.Unmarshal(listResp.Result, &listResult); err != nil {
		t.Fatalf("Failed to unmarshal list result: %v", err)
	}

	if len(listResult.Tools) == 0 {
		t.Error("expected at least one tool")
	}

	// Call tool
	callReq := JsonRpcRequest{
		JsonRpc: "2.0",
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"echo","arguments":{"text":"hello"}}`),
		ID:      3,
	}

	callResp, err := transport.Send(ctx, callReq)
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}

	var callResult CallToolResult
	if err := json.Unmarshal(callResp.Result, &callResult); err != nil {
		t.Fatalf("Failed to unmarshal call result: %v", err)
	}

	if callResult.IsError {
		t.Error("expected no error from echo tool")
	}

	// Verify the echo result
	found := false
	for _, block := range callResult.Content {
		if block.Type == "text" && block.Text == "hello" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected echo to return 'hello'")
	}
}
