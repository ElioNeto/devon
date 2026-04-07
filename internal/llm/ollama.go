package llm

import (
	"context"
)

// OllamaProvider wraps the existing Client for Ollama's OpenAI-compatible API.
// Ollama exposes an OpenAI-compatible endpoint at http://localhost:11434/v1.
type OllamaProvider struct {
	client *Client
	info   ModelInfo
}

// NewOllamaProvider creates a provider for Ollama.
// baseURL should point to the Ollama OpenAI-compatible endpoint (e.g. http://localhost:11434/v1).
func NewOllamaProvider(baseURL, model string, cfg ProviderConfig) *OllamaProvider {
	client := New(cfg.APIKey, baseURL, model, cfg.Timeout)
	return &OllamaProvider{
		client: client,
		info: ModelInfo{
			Name:           cfg.Name,
			Provider:       "ollama",
			InputCost:      0,
			OutputCost:     0,
			MaxTokens:      0,
			SupportsTools:  true,
			SupportsVision: false,
		},
	}
}

func (p *OllamaProvider) Name() string { return p.info.Name }
func (p *OllamaProvider) Info() ModelInfo { return p.info }

func (p *OllamaProvider) Stream(ctx context.Context, messages []Message, tools []ToolDef) (<-chan Delta, error) {
	eventCh, err := p.client.Stream(ctx, messages, tools)
	if err != nil {
		return nil, err
	}

	deltaCh := make(chan Delta, 32)
	go func() {
		defer close(deltaCh)
		for ev := range eventCh {
			var d Delta
			switch ev.Type {
			case "text":
				d = Text(ev.Text)
			case "tool_call":
				if ev.Tool != nil {
					d = ToolCallDelta(*ev.Tool)
				}
			case "done":
				d = DoneDelta(ev.Usage)
			case "error":
				d = ErrorDelta(ev.Err)
			default:
				continue
			}
			select {
			case deltaCh <- d:
			case <-ctx.Done():
				return
			}
		}
	}()

	return deltaCh, nil
}
