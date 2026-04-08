// Package llm fornece um cliente HTTP para qualquer API OpenAI-compatible.
// Suporta streaming SSE e function calling (tool_calls).
package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
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

// TextContent returns a pointer to s for use in Message.Content.
func TextContent(s string) *string { return &s }

// Message representa uma mensagem no histórico da conversa.
type Message struct {
	Role       Role       `json:"role"`
	Content    *string    `json:"content,omitempty"` // nil → omitido no JSON
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

// ToolCall representa uma chamada de ferramenta solicitada pelo modelo.
type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// ToolDef define uma ferramenta disponível para o modelo.
type ToolDef struct {
	Type     string      `json:"type"` // sempre "function"
	Function ToolDefFunc `json:"function"`
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
	Type  string    // "text" | "tool_call" | "done" | "error"
	Text  string    // para Type=="text"
	Tool  *ToolCall // para Type=="tool_call"
	Err   error     // para Type=="error"
	Usage *Usage    // para Type=="done"
}

// Usage reporta tokens consumidos.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// --- Cliente ---

// Streamer é a interface para envio de requisições com streaming.
// Permite mockar o cliente LLM em testes.
type Streamer interface {
	Stream(ctx context.Context, messages []Message, tools []ToolDef) (<-chan StreamEvent, error)
}

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

// --- Retry configuration ---

const (
	// maxRetries é o número máximo de tentativas de requisição HTTP.
	maxRetries = 5
	// baseDelay é o delay inicial para backoff exponencial (Alibaba upstream é lento).
	baseDelay = 5 * time.Second
	// maxDelay é o cap do backoff exponencial.
	maxDelay = 60 * time.Second
	// maxRetries5xx é o número máximo de retries para erros 5xx.
	maxRetries5xx = 2
	// maxBodyBytes é o limite de leitura do body para extrair mensagens de erro.
	maxBodyBytes = 4096
)

// Stream envia uma requisição de chat e retorna um canal de StreamEvent.
// O canal é fechado quando o stream termina ou um erro ocorre.
// Em caso de 429 (rate limit) o client faz retry com backoff exponencial.
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
		return nil, fmt.Errorf("llm: nao foi possivel marshal a requisição: %w", err)
	}

	maxAttempts := maxRetries

	for attempt := 0; attempt < maxAttempts; attempt++ {
		resp, err := c.doRequest(ctx, body)
		if err != nil {
			// Se o tipo de erro já indica 4xx não-retryable (exceto 429), para imediatamente
			var httpErr *httpStatusError
			if errors.As(err, &httpErr) {
				if !isRetryableStatus(httpErr.StatusCode) {
					return nil, fmt.Errorf("llm: %w", err)
				}
				// Limita retries de 5xx
				if httpErr.StatusCode >= 500 && attempt+1 >= maxRetries5xx {
					return nil, fmt.Errorf("llm: máximo de %d tentativas atingido", maxRetries)
				}
			}
			if attempt+1 >= maxAttempts {
				return nil, fmt.Errorf("llm: máximo de %d tentativas atingido", maxRetries)
			}
			delay := retryDelay(resp, attempt, baseDelay, maxDelay)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			continue
		}

		ch := make(chan StreamEvent, 32)
		go parseSSE(ctx, resp.Body, ch)
		return ch, nil
	}
	return nil, fmt.Errorf("llm: máximo de %d tentativas atingido", maxRetries)
}

// doRequest cria e envia uma requisição HTTP para o endpoint de chat.
// Retorna *httpStatusError para status >= 400 com o body capturado.
func (c *Client) doRequest(ctx context.Context, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost,
		c.baseURL+"/chat/completions",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("llm: nao foi possivel criar nova requisição: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("llm: falha na requisição: %w", err)
	}
	if resp.StatusCode >= 400 {
		// Lê o body para extrair a mensagem de erro
		return resp, readAndReturnHTTPErr(resp)
	}

	return resp, nil
}

// readAndReturnHTTPErr lê o body de uma resposta de erro e retorna um erro estruturado.
func readAndReturnHTTPErr(resp *http.Response) error {
	defer resp.Body.Close()
	limitedReader := io.LimitReader(resp.Body, maxBodyBytes)
	bodyBytes, _ := io.ReadAll(limitedReader)
	msg := extractErrorMessage(bodyBytes)
	return &httpStatusError{
		StatusCode: resp.StatusCode,
		Message:    msg,
		Response:   resp,
	}
}

// httpStatusError carrega o código HTTP e a mensagem de erro extraída do body.
type httpStatusError struct {
	StatusCode int
	Message    string
	Response   *http.Response
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("provedor retornou HTTP %d: %s", e.StatusCode, e.Message)
}

// isRetryableStatus indica se um status code deve ser retentado.
// 429 → retry (rate limit). 5xx → retry simples.
// Outros 4xx (400, 401, 403, 404) → falha imediata.
func isRetryableStatus(code int) bool {
	return code == http.StatusTooManyRequests || code >= 500
}

// retryDelay calcula o tempo de espera antes da próxima tentativa.
// Se o header Retry-After estiver presente (segundos inteiros), usa-o + 500ms.
// Caso contrário, usa backoff exponencial com base em baseDelay.
func retryDelay(resp *http.Response, attempt int, base, max time.Duration) time.Duration {
	if resp != nil {
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if secs, err := strconv.Atoi(ra); err == nil && secs > 0 {
				return time.Duration(secs)*time.Second + 500*time.Millisecond
			}
		}
	}
	// Backoff exponencial
	delay := base * (1 << attempt)
	if delay > max {
		delay = max
	}
	return delay
}

// extractErrorMessage parseia o JSON de erro do OpenRouter.
// Precedência: .error.metadata.raw > .error.message > body string.
func extractErrorMessage(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var errResp struct {
		Error struct {
			Message  string `json:"message"`
			Metadata struct {
				Raw string `json:"raw"`
			} `json:"metadata"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &errResp); err == nil {
		if errResp.Error.Metadata.Raw != "" {
			return errResp.Error.Metadata.Raw
		}
		if errResp.Error.Message != "" {
			return errResp.Error.Message
		}
	}
	return string(body)
}

// parseSSE lê o stream SSE e emite eventos no canal.
func parseSSE(ctx context.Context, body io.ReadCloser, ch chan<- StreamEvent) {
	defer body.Close()
	defer close(ch)

	scanner := bufio.NewScanner(body)

	// buffers para tool_calls que chegam fragmentados no stream
	toolBuf := map[int]*ToolCall{}
	nameBuf := map[int]string{}
	argBuf := map[int]string{}

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
					Content   string `json:"content"`
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
		}

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
		send(StreamEvent{Type: "error", Err: fmt.Errorf("llm: erro na leitura do stream: %w", err)})
	}
}
