// Package mcp implements Model Context Protocol client and transport layers.
package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// httpTransport implements Transport using HTTP with SSE support.
type httpTransport struct {
	url     string
	headers map[string]string
	client  *http.Client
	mu      sync.Mutex
}

// Connect initializes the HTTP transport by creating an HTTP client.
func (t *httpTransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.client == nil {
		t.client = &http.Client{
			Timeout: 30 * time.Second,
		}
	}

	return nil
}

// Close is a no-op for HTTP transport.
func (t *httpTransport) Close() error {
	return nil
}

// Send sends a JSON-RPC request via HTTP POST and returns the response.
// Supports both simple POST responses and SSE streams per MCP specification.
func (t *httpTransport) Send(ctx context.Context, req JsonRpcRequest) (*JsonRpcResponse, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.client == nil {
		return nil, fmt.Errorf("http transport not connected")
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, t.url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream, application/json")
	for k, v := range t.headers {
		httpReq.Header.Set(k, v)
	}

	resp, err := t.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP request failed with status %d: %s", resp.StatusCode, string(body))
	}

	contentType := resp.Header.Get("Content-Type")

	// Check if response is SSE
	if strings.Contains(contentType, "text/event-stream") {
		return t.readSSEResponse(resp.Body)
	}

	// Fall back to simple JSON response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var jsonResp JsonRpcResponse
	if err := json.Unmarshal(body, &jsonResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &jsonResp, nil
}

// readSSEResponse reads a JSON-RPC response from an SSE stream.
// SSE format: "data: {...}\n\n"
func (t *httpTransport) readSSEResponse(body io.Reader) (*JsonRpcResponse, error) {
	scanner := bufio.NewScanner(body)
	var dataBuffer bytes.Buffer
	foundData := false

	for scanner.Scan() {
		line := scanner.Text()

		// Empty line indicates end of event
		if line == "" {
			if foundData {
				break
			}
			continue
		}

		// Parse SSE event
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			dataBuffer.Reset()
			dataBuffer.WriteString(data)
			foundData = true
		}
		// Ignore other SSE fields (event:, id:, retry:)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("SSE stream read error: %w", err)
	}

	if !foundData {
		return nil, fmt.Errorf("no SSE data received")
	}

	var jsonResp JsonRpcResponse
	if err := json.Unmarshal(dataBuffer.Bytes(), &jsonResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal SSE data: %w", err)
	}

	return &jsonResp, nil
}
