package memory

import (
	"context"
	"strings"

	"github.com/ElioNeto/devon/internal/db"
)

// ContextFor recupera fatos relevantes e injeta no system prompt.
func ContextFor(ctx context.Context, store db.Store, projectID string, userPrompt string, limit int) string {
	if limit <= 0 {
		limit = 15
	}

	allFacts, err := store.ListFacts(ctx, projectID)
	if err != nil {
		return ""
	}

	if len(allFacts) == 0 {
		return ""
	}

	// Classifica fatos por relevância com a requisição atual
	relevances := scoreRelevance(allFacts, userPrompt)

	// Ordena por score (highest first)
	topFacts := relevances[:min(limit, len(relevances))]

	if len(topFacts) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n=== CONTEXTO DO PROJETO (MEMÓRIA) ===\n")

	for _, r := range topFacts[:limit] {
		sb.WriteString("- ")
		if r.Relevance > 0.7 {
			sb.WriteString("**")
		}
		sb.WriteString(r.Fact.Content)
		if r.Relevance > 0.7 {
			sb.WriteString("**")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("=== FIM CONTEXTO ===\n")

	return sb.String()
}

type relevantFact struct {
	Fact        db.Fact
	Relevance   float64
	MatchType   string // "keyword", "category", "semantic"
}

func scoreRelevance(facts []db.Fact, prompt string) []relevantFact {
	result := make([]relevantFact, len(facts))

	promptLower := strings.ToLower(prompt)
	words := tokenize(promptLower)

	for i, f := range facts {
		contentLower := strings.ToLower(f.Content)
		_ = strings.ToLower(f.Context)
		categoryLower := strings.ToLower(f.Category)

		score := 0.0
		matchType := "low"

		// Keyword matching
		for _, word := range words {
			if len(word) < 3 {
				continue
			}
			if strings.Contains(contentLower, word) {
				score += 0.3
				matchType = "keyword"
			}
			if strings.Contains(categoryLower, word) {
				score += 0.2
				matchType = "category"
			}
		}

		// Category importance
		if strings.Contains(promptLower, f.Category) {
			score += 0.4
			matchType = "category"
		}

		// Boost high confidence facts
		if f.Confidence >= 0.9 {
			score += 0.1
		}

		// Clamp score between 0 and 1
		if score > 1.0 {
			score = 1.0
		}

		result[i] = relevantFact{
			Fact:      f,
			Relevance: score,
			MatchType: matchType,
		}
	}

	// Bubble sort by relevance (simple for small arrays)
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].Relevance > result[i].Relevance {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result
}

func tokenize(s string) []string {
	words := strings.Fields(s)
	// Remove punctuation
	for i, w := range words {
		w = strings.Trim(w, ".,!?;:\"'()-")
		if w != "" {
			words[i] = w
		}
	}
	return words
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
