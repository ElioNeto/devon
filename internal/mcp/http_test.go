// Package mcp implements Model Context Protocol client and transport layers.
package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPTransport_Connect(t *testing.T) {
	transport := &httpTransport{
		url:     "http://example.com",
		headers: nil,
	}
	
	// HTTP transport Connect is a no-op
	err := transport.Connect(context.Background())
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
}

func TestHTTPTransport_Send(t *testing.T) {
	// Create a mock HTTP server
	server := newMockMCPServer()
	ts := startHTTPMock(t, server)
	defer ts.Close()
	
	transport := &httpTransport{
		url:     ts.URL,
		headers: nil,
	}
	
	if err := transport.Connect(context.Background()); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	
	// Test initialize request
	req := JsonRpcRequest{
		JsonRpc: "2.0",
		Method:  "initialize",
		Params:  mustMarshal(InitializeParams{ProtocolVersion: "2024-11-05"}),
		ID:      1,
	}
	
	resp, err := transport.Send(context.Background(), req)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	
	if resp.Result == nil {
		t.Error("expected result")
	}
}

func TestHTTPTransport_SendError(t *testing.T) {
	// Create a server that returns errors
	server := newMockMCPServer()
	server.initError = json.Unmarshal([]byte(""), &struct{}{}) // This will cause issues
	
	// Actually, let's create a server that returns HTTP error
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer ts.Close()
	
	transport := &httpTransport{
		url: ts.URL,
	}
	
	req := JsonRpcRequest{
		JsonRpc: "2.0",
		Method:  "initialize",
		ID:      1,
	}
	
	_, err := transport.Send(context.Background(), req)
	if err == nil {
		t.Error("expected error")
	}
}

func TestHTTPTransport_Close(t *testing.T) {
	transport := &httpTransport{
		url: "http://example.com",
	}
	
	// Close is a no-op for HTTP transport
	err := transport.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

func TestHTTPTransport_ContextTimeout(t *testing.T) {
	// Create a slow server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	
	transport := &httpTransport{
		url: ts.URL,
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	
	req := JsonRpcRequest{
		JsonRpc: "2.0",
		Method:  "test",
		ID:      1,
	}
	
	_, err := transport.Send(ctx, req)
	if err == nil {
		t.Error("expected timeout error")
	}
}

func TestHTTPTransport_Headers(t *testing.T) {
	var receivedHeaders http.Header
	
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"jsonrpc":"2.0","result":{},"id":1}`))
	}))
	defer ts.Close()
	
	transport := &httpTransport{
		url: ts.URL,
		headers: map[string]string{
			"Authorization": "Bearer test-token",
			"X-Custom":      "custom-value",
		},
	}
	
	req := JsonRpcRequest{
		JsonRpc: "2.0",
		Method:  "test",
		ID:      1,
	}

	if err := transport.Connect(context.Background()); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	_, err := transport.Send(context.Background(), req)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	
	if receivedHeaders.Get("Authorization") != "Bearer test-token" {
		t.Errorf("expected Authorization header, got %v", receivedHeaders)
	}
	if receivedHeaders.Get("X-Custom") != "custom-value" {
		t.Errorf("expected X-Custom header, got %v", receivedHeaders)
	}
}

func mustMarshal(v interface{}) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}
