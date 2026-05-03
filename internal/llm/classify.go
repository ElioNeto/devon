package llm

import (
	"strings"

	"github.com/ElioNeto/devon/internal/config"
)

// ClassifyTask determines the task type based on keywords in the prompt.
// Returns config.TaskTypeCode as default when no specific keywords are matched.
func ClassifyTask(prompt string) config.TaskType {
	p := strings.ToLower(prompt)

	if matchesExplore(p) {
		return config.TaskTypeExplore
	}

	if matchesPlan(p) {
		return config.TaskTypePlan
	}

	return config.TaskTypeCode
}

// matchesExplore checks if the prompt looks like an exploration/investigation task.
func matchesExplore(p string) bool {
	exploreKeywords := []string{
		"explor", "investigat", "find", "search", "lookup", "discover",
		"what is", "what are", "how does", "tell me about", "explain",
		"documentation", "read", "show me", "list", "browse",
		"which file", "where is", "who wrote", "when was",
	}
	for _, kw := range exploreKeywords {
		if strings.Contains(p, kw) {
			return true
		}
	}
	return false
}

// matchesPlan checks if the prompt looks like a planning/design task.
func matchesPlan(p string) bool {
	planKeywords := []string{
		"plan", "design", "architect", "architectur", "strategy",
		"roadmap", "proposal", "think about", "consider",
		"how to", "what if", "compare", "evaluat", "decide",
		"analyze", "analysis", "break down", "steps to",
		"approach", "schema", "flowchart", "diagram",
	}
	for _, kw := range planKeywords {
		if strings.Contains(p, kw) {
			return true
		}
	}
	return false
}
