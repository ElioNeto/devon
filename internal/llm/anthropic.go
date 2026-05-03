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

const (
	anthropicBaseURL    = "https://api.anthropic.com/v1"
	anthropicAPIVersion = "2023-06-01"
	anthropicMaxTokens  = 8192
)

// AnthropicProvider implements Provider using Anthropic Messages API.
type AnthropicProvider struct {
	apiKey  string
	baseURL string
	model   string
	timeout time.Duration
	info    ModelInfo
}

// NewAnthropicProvider creates a new Anthropic provider.
func NewAnthropicProvider(cfg ProviderConfig) *AnthropicProvider {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = anthropicBaseURL
	}

	return &AnthropicProvider{
		apiKey:  cfg.APIKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   cfg.Model,
		info: ModelInfo{
			Name:           cfg.Name,
			Provider:       "anthropic",
			SupportsTools:  true,
			SupportsVision: strings.HasPrefix(cfg.Model, "claude-3") || strings.HasPrefix(cfg.Model, "claude-sonnet-4"),
		},
	}
}

func (p *AnthropicProvider) Name() string    { return p.info.Name }
func (p *AnthropicProvider) Info() ModelInfo { return p.info }

// anthropicContentBlock is a content block in an Anthropic message.
type anthropicContentBlock struct {
	Type   string `json:"type"`
	Text   string `json:"text,omitempty"`
	Source *struct {
		Type      string `json:"type"`
		MediaType string `json:"media_type"`
		Data      string `json:"data"`
	} `json:"source,omitempty"`
}

type anthropicMsg struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"` // string or []anthropicContentBlock
}

type anthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type anthropicBody struct {
	Model     string          `json:"model"`
	System    string          `json:"system,omitempty"`
	Messages  []anthropicMsg  `json:"messages"`
	Tools     []anthropicTool `json:"tools,omitempty"`
	MaxTokens int             `json:"max_tokens"`
	Stream    bool            `json:"stream"`
}

func (p *AnthropicProvider) Stream(ctx context.Context, messages []Message, tools []ToolDef) (<-chan Delta, error) {
	body, err := p.buildBody(messages, tools)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("anthropic: failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", anthropicAPIVersion)

	client := &http.Client{}
	if p.timeout > 0 {
		ctx2, cancel := context.WithTimeout(ctx, p.timeout)
		defer cancel()
		req = req.WithContext(ctx2)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		resp.Body.Close()
		return nil, errorFromResponse(resp)
	}

	deltaCh := make(chan Delta, 32)
	go func() {
		defer close(deltaCh)
		p.parseSSE(ctx, resp.Body, deltaCh)
	}()

	return deltaCh, nil
}

func (p *AnthropicProvider) buildBody(messages []Message, tools []ToolDef) ([]byte, error) {
	var systemMsg string
	var userMessages []anthropicMsg

	for _, m := range messages {
		switch m.Role {
		case RoleSystem:
			text := ""
			if m.Content != nil {
				text = *m.Content
			}
			if systemMsg != "" {
				systemMsg += "\n"
			}
			systemMsg += text
		case RoleUser:
			am, err := m.toAnthropicMsg("user")
			if err != nil {
				return nil, err
			}
			userMessages = append(userMessages, am)
		case RoleAssistant:
			am, err := m.toAnthropicMsg("assistant")
			if err != nil {
				return nil, err
			}
			userMessages = append(userMessages, am)
		case RoleTool:
			am, err := m.toAnthropicMsg("user")
			if err != nil {
				return nil, err
			}
			// Wrap tool result content
			if len(m.ContentParts) == 0 && m.Content != nil {
				content := fmt.Sprintf("[tool_result id=%s] %s", m.ToolCallID, *m.Content)
				am = anthropicMsg{
					Role:    "user",
					Content: mustRawMessage(json.Marshal(content)),
				}
			}
			userMessages = append(userMessages, am)
		}
	}

	var aTools []anthropicTool
	for _, t := range tools {
		aTools = append(aTools, anthropicTool{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			InputSchema: t.Function.Parameters,
		})
	}

	reqBody := anthropicBody{
		Model:     p.model,
		System:    systemMsg,
		Messages:  userMessages,
		MaxTokens: anthropicMaxTokens,
		Stream:    true,
	}
	if len(aTools) > 0 {
		reqBody.Tools = aTools
	}

	return json.Marshal(reqBody)
}

func errorFromResponse(resp *http.Response) error {
	defer resp.Body.Close()
	limited := io.LimitReader(resp.Body, maxBodyBytes)
	bodyBytes, _ := io.ReadAll(limited)
	msg := extractErrorMessage(bodyBytes)
	return fmt.Errorf("anthropic: HTTP %d: %s", resp.StatusCode, msg)
}

// toAnthropicMsg converts a Message to an anthropicMsg, handling both
// legacy string content and multimodal ContentParts (images).
func (m Message) toAnthropicMsg(role string) (anthropicMsg, error) {
	am := anthropicMsg{Role: role}

	if len(m.ContentParts) > 0 {
		// Multimodal: build content blocks
		var blocks []anthropicContentBlock
		for _, part := range m.ContentParts {
			switch part.Type {
			case TypeText:
				blocks = append(blocks, anthropicContentBlock{Type: "text", Text: part.Text})
			case TypeImageURL:
				if part.ImageURL != nil {
					mediaType, data := parseDataURI(part.ImageURL.URL)
					blocks = append(blocks, anthropicContentBlock{
						Type: "image",
						Source: &struct {
							Type      string `json:"type"`
							MediaType string `json:"media_type"`
							Data      string `json:"data"`
						}{
							Type:      "base64",
							MediaType: mediaType,
							Data:      data,
						},
					})
				}
			}
		}
		raw, err := json.Marshal(blocks)
		if err != nil {
			return am, fmt.Errorf("anthropic: failed to marshal content blocks: %w", err)
		}
		am.Content = raw
	} else if m.Content != nil {
		// Legacy string content
		am.Content = mustRawMessage(json.Marshal(*m.Content))
	} else {
		// Empty content
		am.Content = mustRawMessage(json.Marshal(""))
	}

	return am, nil
}

// parseDataURI extracts media_type and base64 data from a data URI.
// Expected format: "data:image/png;base64,<base64data>"
func parseDataURI(uri string) (mediaType, data string) {
	if len(uri) < 5 || uri[:5] != "data:" {
		return "image/png", uri // fallback
	}
	rest := uri[5:]
	commaIdx := -1
	for i, c := range rest {
		if c == ',' {
			commaIdx = i
			break
		}
	}
	if commaIdx < 0 {
		return "image/png", uri
	}
	mediaPart := rest[:commaIdx]
	data = rest[commaIdx+1:]
	// Extract media_type from ";base64" prefix
	// Format: "image/png;base64"
	parts := strings.SplitN(mediaPart, ";", 2)
	mediaType = parts[0]
	return mediaType, data
}

// mustRawMessage wraps the result of json.Marshal into a json.RawMessage.
func mustRawMessage(b []byte, err error) json.RawMessage {
	if err != nil {
		// This should never happen in practice for the simple types we marshal.
		panic(err)
	}
	return json.RawMessage(b)
}

func (p *AnthropicProvider) parseSSE(ctx context.Context, body io.ReadCloser, deltaCh chan<- Delta) {
	defer body.Close()

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, bufio.MaxScanTokenSize), 1024*1024)

	toolNames := map[int]string{} // index → name (from content_block_start)
	toolIDs := map[int]string{}   // index → id
	toolArgBuf := map[int]*strings.Builder{}

	send := func(d Delta) {
		select {
		case deltaCh <- d:
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
		if data == "" || data == "[DONE]" {
			continue
		}

		var eventBase struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal([]byte(data), &eventBase); err != nil {
			continue
		}

		switch eventBase.Type {
		case "content_block_delta":
			// Try text_delta first
			var textEv struct {
				Delta struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"delta"`
			}
			if err := json.Unmarshal([]byte(data), &textEv); err == nil {
				if textEv.Delta.Type == "text_delta" && textEv.Delta.Text != "" {
					send(Text(textEv.Delta.Text))
					continue
				}
			}

			// Try input_json_delta
			var toolEv struct {
				Index int `json:"index"`
				Delta struct {
					Type        string `json:"type"`
					PartialJSON string `json:"partial_json"`
				} `json:"delta"`
			}
			if err := json.Unmarshal([]byte(data), &toolEv); err == nil {
				if toolEv.Delta.Type == "input_json_delta" && toolEv.Delta.PartialJSON != "" {
					if toolArgBuf[toolEv.Index] == nil {
						toolArgBuf[toolEv.Index] = &strings.Builder{}
					}
					toolArgBuf[toolEv.Index].WriteString(toolEv.Delta.PartialJSON)
				}
			}

		case "content_block_start":
			var blockEv struct {
				Index        int `json:"index"`
				ContentBlock struct {
					Type  string          `json:"type"`
					ID    string          `json:"id"`
					Name  string          `json:"name"`
					Input json.RawMessage `json:"input"`
				} `json:"content_block"`
			}
			if err := json.Unmarshal([]byte(data), &blockEv); err == nil {
				if blockEv.ContentBlock.Type == "tool_use" {
					toolNames[blockEv.Index] = blockEv.ContentBlock.Name
					toolIDs[blockEv.Index] = blockEv.ContentBlock.ID
					if toolArgBuf[blockEv.Index] == nil {
						toolArgBuf[blockEv.Index] = &strings.Builder{}
					}
				}
			}

		case "content_block_stop":
			var stopEv struct {
				Index int `json:"index"`
			}
			if err := json.Unmarshal([]byte(data), &stopEv); err == nil {
				if name := toolNames[stopEv.Index]; name != "" {
					args := ""
					if b := toolArgBuf[stopEv.Index]; b != nil {
						args = b.String()
					}
					id := toolIDs[stopEv.Index]
					tc := ToolCall{
						ID:   id,
						Type: "function",
						Function: ToolCallFunction{
							Name:      name,
							Arguments: args,
						},
					}
					send(ToolCallDelta(tc))
				}
			}

		case "message_delta":
			var usageEv struct {
				Usage struct {
					OutputTokens int `json:"output_tokens"`
				} `json:"usage"`
			}
			if err := json.Unmarshal([]byte(data), &usageEv); err == nil && usageEv.Usage.OutputTokens > 0 {
				u := &Usage{CompletionTokens: usageEv.Usage.OutputTokens}
				send(DoneDelta(u))
			}
		}
	}

	if err := scanner.Err(); err != nil {
		send(ErrorDelta(fmt.Errorf("anthropic: stream error: %w", err)))
	}
}
