package llm

import (
	"context"
)

// OpenAIProvider wraps the existing Client for OpenAI-compatible APIs.
type OpenAIProvider struct {
	client *Client
	info   ModelInfo
}

// NewOpenAIProvider creates a provider using the existing OpenAI-compatible Client.
func NewOpenAIProvider(baseURL, model string, cfg ProviderConfig) *OpenAIProvider {
	client := New(cfg.APIKey, baseURL, model, cfg.Timeout)
	return &OpenAIProvider{
		client: client,
		info: ModelInfo{
			Name:           cfg.Name,
			Provider:       "openai",
			InputCost:      0,
			OutputCost:     0,
			MaxTokens:      0,
			SupportsTools:  true,
			SupportsVision: true,
		},
	}
}

func (p *OpenAIProvider) Name() string    { return p.info.Name }
func (p *OpenAIProvider) Info() ModelInfo { return p.info }

func (p *OpenAIProvider) Stream(ctx context.Context, messages []Message, tools []ToolDef) (<-chan Delta, error) {
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
