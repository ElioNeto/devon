// Package cost tracks and estimates LLM token costs by model.
package cost

import (
	"fmt"
	"math"
	"strings"
)

// ModelCost holds per-model pricing (per 1M tokens).
type ModelCost struct {
	Model      string
	InputCost  float64 // USD per 1M input tokens
	OutputCost float64 // USD per 1M output tokens
}

// Pricing for common models (USD per 1M tokens).
var pricing = map[string]ModelCost{
	"gpt-4o":                       {InputCost: 2.50, OutputCost: 10.00},
	"gpt-4o-mini":                  {InputCost: 0.15, OutputCost: 0.60},
	"gpt-4":                        {InputCost: 30.00, OutputCost: 60.00},
	"gpt-4-turbo":                  {InputCost: 10.00, OutputCost: 30.00},
	"claude-sonnet-4-6":            {InputCost: 3.00, OutputCost: 15.00},
	"claude-opus-4-6":              {InputCost: 15.00, OutputCost: 75.00},
	"claude-haiku-4-5-20251001":    {InputCost: 0.80, OutputCost: 4.00},
	"gemini-2.5-flash":             {InputCost: 0.10, OutputCost: 0.40},
	"gemini-2.5-pro":               {InputCost: 1.25, OutputCost: 10.00},
	"mistralai/devstral-2512:free": {InputCost: 0, OutputCost: 0},
	"mistral-large":                {InputCost: 2.00, OutputCost: 6.00},
}

// Session tracks cumulative costs for a session.
type Session struct {
	Model             string
	TotalInputTokens  int
	TotalOutputTokens int
	TotalRequests     int
	TotalCostUSD      float64
}

// NewSession creates a new cost tracker for the given model.
func NewSession(model string) *Session {
	return &Session{Model: model}
}

// AddTokens records tokens used from a single request.
func (s *Session) AddTokens(input, output int) {
	s.TotalInputTokens += input
	s.TotalOutputTokens += output
	s.TotalRequests++
	s.TotalCostUSD = EstimateCost(s.Model, s.TotalInputTokens, s.TotalOutputTokens)
}

// EstimateCost returns the estimated USD cost for the given token counts and model.
func EstimateCost(model string, inputTokens, outputTokens int) float64 {
	pc, ok := pricing[model]
	if !ok {
		pc = matchModel(model)
	}
	if pc.InputCost == 0 && pc.OutputCost == 0 {
		return 0 // free model or unknown
	}

	million := 1_000_000.0
	cost := (float64(inputTokens) / million * pc.InputCost) +
		(float64(outputTokens) / million * pc.OutputCost)

	return roundTo6(cost)
}

func matchModel(model string) ModelCost {
	matches := 0
	var best ModelCost
	modelLower := strings.ToLower(model)
	for name, pc := range pricing {
		nameLower := strings.ToLower(name)
		if strings.HasPrefix(modelLower, nameLower) || strings.Contains(modelLower, nameLower) {
			if len(name) > matches {
				matches = len(name)
				best = pc
			}
		}
	}
	return best
}

// FormatCost returns a human-readable cost string like "$0.04".
func FormatCost(costUSD float64) string {
	if costUSD == 0 {
		return "$0.00"
	}
	return fmt.Sprintf("$%.4f", costUSD)
}

// Format returns a summary of the session usage.
func (s *Session) Format() string {
	return fmt.Sprintf(
		"Model: %s | Requests: %d | Input: %d tokens | Output: %d tokens | Total: %d tokens | Cost: %s",
		s.Model, s.TotalRequests, s.TotalInputTokens, s.TotalOutputTokens,
		s.TotalInputTokens+s.TotalOutputTokens, FormatCost(s.TotalCostUSD),
	)
}

func roundTo6(f float64) float64 {
	return math.Round(f*1_000_000) / 1_000_000
}
