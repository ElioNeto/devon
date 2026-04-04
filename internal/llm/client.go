// Package llm fornece um cliente HTTP para qualquer API OpenAI-compatible.
// Suporta streaming SSE e function calling (tool_calls).
package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// --- Tipos de mensagem ---

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message representa uma mensagem no histórico da conversa.
type Message struct {
	Role       Role        `json:"role"`
	Content    string      `json:"content,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
}

// ToolCall representa uma chamada de ferramenta solicitada pelo modelo.
type ToolCall struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"`
	Function ToolCallFunction `json:"function"`
}

type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// ToolDef define uma ferramenta disponível para o modelo.
type ToolDef struct {
	Type     string       `json:"type"` // sempre "function"
	Function ToolDefFunc  `json:"function"`
}

type ToolDefFunc struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"` // JSON Schema
}

// --- Request / Response ---

type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Tools    []ToolDef `json:"tools,omitempty"`
	Stream   bool      `json:"stream"`
}

// StreamEvent é emitido durante o streaming.
type StreamEvent struct {
	Type    string    // "text" | "tool_call" | "done" | "error"
	Text    string    // para Type=="text"
	Tool    *ToolCall // para Type=="tool_call"
	Err     error     // para Type=="error"
	Usage   *Usage    // para Type=="done"
}

// Usage reporta tokens consumidos.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// --- Cliente ---

// Client é um cliente HTTP para APIs OpenAI-compatible.
type Client struct {
	apiKey  string
	baseURL string
	model   string
	http    *http.Client
}

// New cria um novo Client.
func New(apiKey, baseURL, model string, timeout time.Duration) *Client {
	return &Client{
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		http:    &http.Client{Timeout: timeout},
	}
}

// Stream envia uma requisição de chat e retorna um canal de StreamEvent.
// O canal é fechado quando o stream termina ou um erro ocorre.
func (c *Client) Stream(
	ctx context.Context,
	messages []Message,
	tools []ToolDef,
) (<-chan StreamEvent, error) {
	body, err := json.Marshal(ChatRequest{
		Model:    c.model,
		Messages: messages,
		Tools:    tools,
		Stream:   true,
	})
	if err != nil {
		return nil, fmt.Errorf("llm: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost,
		c.baseURL+"/chat/completions",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("llm: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("llm: request failed: %w", err)
	}
	if resp.StatusCode >= 400 {
		resp.Body.Close()
		return nil, fmt.Errorf("llm: provider returned HTTP %d", resp.StatusCode)
	}

	ch := make(chan StreamEvent, 32)
	go parseSSE(ctx, resp.Body, ch)
	return ch, nil
}

// parseSSE lê o stream SSE e emite eventos no canal.
func parseSSE(ctx context.Context, body io.ReadCloser, ch chan<- StreamEvent) {
	defer body.Close()
	defer close(ch)

	scanner := bufio.NewScanner(body)

	// buffers para tool_calls que chegam fragmentados no stream
	toolBuf := map[int]*ToolCall{}
	nameBuf := map[int]string{}
	argBuf  := map[int]string{}

	send := func(e StreamEvent) {
		select {
		case ch <- e:
		case <-ctx.Done():
		}
	}

	for scanner.Scan() {
		if ctx.Err() != nil {
			return
		}
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			// Emite tool_calls acumulados antes de finalizar
			for idx, tc := range toolBuf {
				tc.Function.Name = nameBuf[idx]
				tc.Function.Arguments = argBuf[idx]
				send(StreamEvent{Type: "tool_call", Tool: tc})
			}
			send(StreamEvent{Type: "done"})
			return
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content   string     `json:"content"`
					ToolCalls []struct {
						Index    int    `json:"index"`
						ID       string `json:"id"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
			} `json:"choices"`
			Usage *Usage `json:"usage"`
		}{}

		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if chunk.Usage != nil {
			send(StreamEvent{Type: "done", Usage: chunk.Usage})
			return
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		delta := chunk.Choices[0].Delta

		if delta.Content != "" {
			send(StreamEvent{Type: "text", Text: delta.Content})
		}

		for _, tc := range delta.ToolCalls {
			idx := tc.Index
			if _, ok := toolBuf[idx]; !ok {
				toolBuf[idx] = &ToolCall{ID: tc.ID, Type: "function"}
			}
			if tc.ID != "" {
				toolBuf[idx].ID = tc.ID
			}
			nameBuf[idx] += tc.Function.Name
			argBuf[idx] += tc.Function.Arguments
		}
	}

	if err := scanner.Err(); err != nil {
		send(StreamEvent{Type: "error", Err: fmt.Errorf("llm: stream read error: %w", err)})
	}
}
