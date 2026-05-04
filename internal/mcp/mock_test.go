// Package mcp implements Model Context Protocol client and transport layers.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// mockMCPServer implements a simple mock MCP server for testing.
type mockMCPServer struct {
	mu          sync.Mutex
	initialized bool
	tools       []Tool
	callCount   map[string]int
	callDelay   map[string]time.Duration
	callError   map[string]error
	initDelay   time.Duration
	listDelay   time.Duration
	initError   error
	listError   error
}

// newMockMCPServer creates a new mock MCP server with default tools.
func newMockMCPServer() *mockMCPServer {
	return &mockMCPServer{
		tools: []Tool{
			{
				Name:        "test_tool",
				Description: "A test tool",
				InputSchema: json.RawMessage(`{"type": "object", "properties": {"input": {"type": "string"}}, "required": ["input"]}`),
			},
			{
				Name:        "echo_tool",
				Description: "Echoes the input",
				InputSchema: json.RawMessage(`{"type": "object", "properties": {"message": {"type": "string"}}, "required": ["message"]}`),
			},
		},
		callCount: make(map[string]int),
		callDelay: make(map[string]time.Duration),
		callError: make(map[string]error),
	}
}

// handleRequest processes a JSON-RPC request and returns a response.
func (m *mockMCPServer) handleRequest(ctx context.Context, req JsonRpcRequest) (JsonRpcResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch req.Method {
	case "initialize":
		if m.initDelay > 0 {
			time.Sleep(m.initDelay)
		}
		if m.initError != nil {
			return JsonRpcResponse{
				JsonRpc: "2.0",
				ID:      req.ID,
				Error: &JsonRpcError{
					Code:    -32603,
					Message: m.initError.Error(),
				},
			}, nil
		}
		m.initialized = true
		result, _ := json.Marshal(InitializeResult{
			ProtocolVersion: "2024-11-05",
			ServerInfo: ServerInfo{
				Name:    "mock-mcp-server",
				Version: "1.0.0",
			},
		})
		return JsonRpcResponse{
			JsonRpc: "2.0",
			ID:      req.ID,
			Result:  result,
		}, nil

	case "notifications/initialized":
		return JsonRpcResponse{}, nil

	case "tools/list":
		if !m.initialized {
			return JsonRpcResponse{
				JsonRpc: "2.0",
				ID:      req.ID,
				Error:   &JsonRpcError{Code: -32002, Message: "not initialized"},
			}, nil
		}
		if m.listDelay > 0 {
			time.Sleep(m.listDelay)
		}
		if m.listError != nil {
			return JsonRpcResponse{
				JsonRpc: "2.0",
				ID:      req.ID,
				Error: &JsonRpcError{
					Code:    -32603,
					Message: m.listError.Error(),
				},
			}, nil
		}
		result, _ := json.Marshal(ListToolsResult{Tools: m.tools})
		return JsonRpcResponse{
			JsonRpc: "2.0",
			ID:      req.ID,
			Result:  result,
		}, nil

	case "tools/call":
		if !m.initialized {
			return JsonRpcResponse{
				JsonRpc: "2.0",
				ID:      req.ID,
				Error:   &JsonRpcError{Code: -32002, Message: "not initialized"},
			}, nil
		}

		var params CallToolParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return JsonRpcResponse{
				JsonRpc: "2.0",
				ID:      req.ID,
				Error:   &JsonRpcError{Code: -32700, Message: "parse error"},
			}, nil
		}

		m.callCount[params.Name]++

		if delay, ok := m.callDelay[params.Name]; ok && delay > 0 {
			time.Sleep(delay)
		}
		if err, ok := m.callError[params.Name]; ok && err != nil {
			result, _ := json.Marshal(CallToolResult{
				IsError: true,
				Content: []ContentBlock{{Type: "text", Text: err.Error()}},
			})
			return JsonRpcResponse{
				JsonRpc: "2.0",
				ID:      req.ID,
				Result:  result,
			}, nil
		}

		var args map[string]interface{}
		json.Unmarshal(params.Arguments, &args)

		resultText := fmt.Sprintf("tool %s called", params.Name)
		if msg, ok := args["message"].(string); ok {
			resultText = msg
		} else if input, ok := args["input"].(string); ok {
			resultText = input
		}

		result, _ := json.Marshal(CallToolResult{
			Content: []ContentBlock{{Type: "text", Text: resultText}},
		})
		return JsonRpcResponse{
			JsonRpc: "2.0",
			ID:      req.ID,
			Result:  result,
		}, nil

	default:
		return JsonRpcResponse{
			JsonRpc: "2.0",
			ID:      req.ID,
			Error:   &JsonRpcError{Code: -32601, Message: "method not found"},
		}, nil
	}
}

// startHTTPMock starts a mock MCP server that communicates over HTTP.
func startHTTPMock(t *testing.T, server *mockMCPServer) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("failed to read body"))
			return
		}

		var req JsonRpcRequest
		if err := json.Unmarshal(body, &req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("invalid JSON"))
			return
		}

		resp, err := server.handleRequest(r.Context(), req)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	return httptest.NewServer(mux)
}

// addTool adds a tool to the mock server.
func (m *mockMCPServer) addTool(tool Tool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tools = append(m.tools, tool)
}

// setCallError sets an error for a specific tool call.
func (m *mockMCPServer) setCallError(toolName string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callError[toolName] = err
}

// setCallDelay sets a delay for a specific tool call.
func (m *mockMCPServer) setCallDelay(toolName string, delay time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callDelay[toolName] = delay
}

// callCountFor returns the number of times a tool was called.
func (m *mockMCPServer) callCountFor(toolName string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount[toolName]
}
