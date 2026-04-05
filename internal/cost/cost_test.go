package cost

import (
	"strings"
	"testing"
)

func TestEstimateCost_Gpt4o(t *testing.T) {
	// gpt-4o: $2.50/1M input, $10/1M output
	input := 10_000  // 10k input = $0.025
	output := 5_000  // 5k output = $0.05
	cost := EstimateCost("gpt-4o", input, output)
	expected := 0.025 + 0.05 // $0.075
	if cost != expected {
		t.Errorf("expected cost $%.4f, got $%.4f", expected, cost)
	}
}

func TestEstimateCost_FreeModel(t *testing.T) {
	cost := EstimateCost("mistralai/devstral-2512:free", 1000, 500)
	if cost != 0 {
		t.Errorf("expected cost $0 for free model, got $%.4f", cost)
	}
}

func TestEstimateCost_UnknownModel(t *testing.T) {
	cost := EstimateCost("unknown-custom-model-xyz", 1000, 500)
	if cost != 0 {
		t.Errorf("expected cost $0 for unknown model, got $%.4f", cost)
	}
}

func TestEstimateCost_MatchModelPrefix(t *testing.T) {
	// "gpt-4o" should match "gpt-4o-2024-05-13" (longest match)
	cost := EstimateCost("gpt-4o-2024-05-13", 1_000_000, 0)
	expected := 2.50 // full 1M input tokens
	if cost != expected {
		t.Errorf("expected cost $%.2f, got $%.4f", expected, cost)
	}
}

func TestEstimateCost_MatchModelNoMatch(t *testing.T) {
	cost := EstimateCost("some-random-model", 1000, 500)
	if cost != 0 {
		t.Errorf("expected cost $0 for unmatched model, got $%.4f", cost)
	}
}

func TestSession_AddTokens(t *testing.T) {
	s := NewSession("gpt-4o")
	s.AddTokens(1000, 500)
	s.AddTokens(2000, 1000)

	if s.TotalInputTokens != 3000 {
		t.Errorf("expected 3000 input tokens, got %d", s.TotalInputTokens)
	}
	if s.TotalOutputTokens != 1500 {
		t.Errorf("expected 1500 output tokens, got %d", s.TotalOutputTokens)
	}
	if s.TotalRequests != 2 {
		t.Errorf("expected 2 requests, got %d", s.TotalRequests)
	}
	if s.TotalCostUSD == 0 {
		t.Error("expected non-zero cost")
	}
}

func TestSession_AddTokens_ZeroModel(t *testing.T) {
	s := NewSession("unknown-model")
	s.AddTokens(1000, 500)

	if s.TotalInputTokens != 1000 {
		t.Errorf("expected 1000 input tokens, got %d", s.TotalInputTokens)
	}
	if s.TotalCostUSD != 0 {
		t.Errorf("expected $0 cost for unknown model, got $%.4f", s.TotalCostUSD)
	}
}

func TestSession_Format(t *testing.T) {
	s := NewSession("gpt-4o")
	s.AddTokens(1000, 500)

	out := s.Format()
	if !strings.Contains(out, "gpt-4o") {
		t.Error("format should contain model name")
	}
	if !strings.Contains(out, "Requests: 1") {
		t.Error("format should contain request count")
	}
	if !strings.Contains(out, "Cost: $") {
		t.Error("format should contain cost")
	}
}

func TestFormatCost(t *testing.T) {
	tests := []struct {
		cost float64
		want string
	}{
		{0, "$0.00"},
		{0.0255, "$0.0255"},
		{1.50, "$1.5000"},
		{0.0001, "$0.0001"},
	}
	for _, tt := range tests {
		got := FormatCost(tt.cost)
		if got != tt.want {
			t.Errorf("FormatCost(%.4f) = %q, want %q", tt.cost, got, tt.want)
		}
	}
}

func TestNewSession(t *testing.T) {
	s := NewSession("gemini-2.5-flash")
	if s.Model != "gemini-2.5-flash" {
		t.Errorf("expected model 'gemini-2.5-flash', got %q", s.Model)
	}
	if s.TotalCostUSD != 0 {
		t.Error("new session should have zero cost")
	}
	if s.TotalInputTokens != 0 {
		t.Error("new session should have zero input tokens")
	}
	if s.TotalRequests != 0 {
		t.Error("new session should have zero requests")
	}
}

func TestEstimateCost_GeminiFlash(t *testing.T) {
	// gemini-2.5-flash: $0.10/1M input, $0.40/1M output
	cost := EstimateCost("gemini-2.5-flash", 1_000_000, 1_000_000)
	expected := 0.10 + 0.40 // $0.50
	if cost != expected {
		t.Errorf("expected cost $%.2f, got $%.4f", expected, cost)
	}
}

func TestEstimateCost_Gpt4(t *testing.T) {
	// gpt-4: $30.00/1M input, $60/1M output
	// Should NOT match gpt-4o since that's longer
	cost := EstimateCost("gpt-4", 1_000_000, 0)
	expected := 30.00
	if cost != expected {
		t.Errorf("expected cost $%.2f, got $%.4f", expected, cost)
	}
}

func TestEstimateCost_ClaudeSonnet(t *testing.T) {
	cost := EstimateCost("claude-sonnet-4-6", 1_000_000, 0)
	expected := 3.00
	if cost != expected {
		t.Errorf("expected cost $%.2f, got $%.4f", expected, cost)
	}
}

func TestEstimateCost_ClaudeHaiku(t *testing.T) {
	cost := EstimateCost("claude-haiku-4-5-20251001", 1_000_000, 0)
	expected := 0.80
	if cost < expected*0.99 || cost > expected*1.01 {
		t.Errorf("expected cost ~$%.2f, got $%.4f", expected, cost)
	}
}

func TestCostEstimate_ZeroTokens(t *testing.T) {
	cost := EstimateCost("gpt-4o", 0, 0)
	if cost != 0 {
		t.Errorf("expected $0 for zero tokens, got $%.4f", cost)
	}
}

func TestCostEstimate_LargeNumbers(t *testing.T) {
	cost := EstimateCost("gpt-4o", 10_000_000, 5_000_000)
	expected := 10_000_000.0/1_000_000.0*2.50 + 5_000_000.0/1_000_000.0*10.00 // $25 + $50 = $75
	if cost != expected {
		t.Errorf("expected cost $%.2f, got $%.4f", expected, cost)
	}
}

func TestSession_MultipleAddTokensAccumulates(t *testing.T) {
	s := NewSession("gemini-2.5-flash")
	s.AddTokens(500_000, 250_000)
	s.AddTokens(500_000, 250_000)

	// Total: 1M input + 500k output = $0.10 + $0.20 = $0.30
	expectedCost := 0.10 + 0.20
	if s.TotalCostUSD < expectedCost*0.99 || s.TotalCostUSD > expectedCost*1.01 {
		t.Errorf("expected cost ~$%.2f, got $%.4f", expectedCost, s.TotalCostUSD)
	}
	if s.TotalRequests != 2 {
		t.Errorf("expected 2 requests, got %d", s.TotalRequests)
	}
}

func TestRoundTo6(t *testing.T) {
	// Test the rounding function indirectly via EstimateCost
	// 1 token of gpt-4o input: 1/1,000,000 * 2.50 = 0.0000025
	cost := EstimateCost("gpt-4o", 1, 0)
	if cost <= 0 {
		t.Errorf("cost for 1 token should be positive, got $%.6f", cost)
	}
}
