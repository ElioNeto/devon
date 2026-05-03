// Package main implements a mock MCP server for testing stdio transport.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type JsonRpcRequest struct {
	JsonRpc string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      interface{}     `json:"id,omitempty"`
}

type JsonRpcResponse struct {
	JsonRpc string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JsonRpcError   `json:"error,omitempty"`
	ID      interface{}     `json:"id,omitempty"`
}

type JsonRpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type ListToolsResult struct {
	Tools []Tool `json:"tools"`
}

type CallToolParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type CallToolResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

var initialized = false

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var req JsonRpcRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			sendError(req.ID, -32700, "Parse error")
			continue
		}

		switch req.Method {
		case "initialize":
			initialized = true
			result, _ := json.Marshal(map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"serverInfo": map[string]string{
					"name":    "mock-mcp-server",
					"version": "1.0.0",
				},
			})
			sendResponse(req.ID, result)

		case "notifications/initialized":
			// Acknowledge

		case "tools/list":
			if !initialized {
				sendError(req.ID, -32002, "not initialized")
				continue
			}
			tools := []Tool{
				{
					Name:        "echo",
					Description: "Echoes the input text",
					InputSchema: json.RawMessage(`{"type":"object","properties":{"text":{"type":"string"}},"required":["text"]}`),
				},
				{
					Name:        "test_tool",
					Description: "A test tool",
					InputSchema: json.RawMessage(`{"type":"object","properties":{"input":{"type":"string"}},"required":["input"]}`),
				},
			}
			result, _ := json.Marshal(ListToolsResult{Tools: tools})
			sendResponse(req.ID, result)

		case "tools/call":
			if !initialized {
				sendError(req.ID, -32002, "not initialized")
				continue
			}
			var params CallToolParams
			if err := json.Unmarshal(req.Params, &params); err != nil {
				sendError(req.ID, -32600, "Invalid params")
				continue
			}
			var args map[string]interface{}
			json.Unmarshal(params.Arguments, &args)
			text := fmt.Sprintf("tool %s called", params.Name)
			if s, ok := args["text"].(string); ok {
				text = s
			} else if s, ok := args["input"].(string); ok {
				text = s
			}
			result, _ := json.Marshal(CallToolResult{
				Content: []ContentBlock{{Type: "text", Text: text}},
			})
			sendResponse(req.ID, result)

		default:
			sendError(req.ID, -32601, "Method not found")
		}
	}
}

func sendResponse(id interface{}, result json.RawMessage) {
	resp := JsonRpcResponse{
		JsonRpc: "2.0",
		Result:  result,
		ID:      id,
	}
	data, _ := json.Marshal(resp)
	fmt.Println(string(data))
}

func sendError(id interface{}, code int, message string) {
	resp := JsonRpcResponse{
		JsonRpc: "2.0",
		Error:   &JsonRpcError{Code: code, Message: message},
		ID:      id,
	}
	data, _ := json.Marshal(resp)
	fmt.Println(string(data))
}
