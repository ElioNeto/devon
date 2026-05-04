package llm

import (
	"testing"

	"github.com/ElioNeto/devon/internal/config"
)

func TestClassifyTask(t *testing.T) {
	tests := []struct {
		name   string
		prompt string
		want   config.TaskType
	}{
		// ── Explore ──────────────────────────────────────────────
		{
			name:   "explore: 'explore' keyword",
			prompt: "Explore this codebase for bugs",
			want:   config.TaskTypeExplore,
		},
		{
			name:   "explore: 'find' keyword",
			prompt: "Find all TODO comments in the project",
			want:   config.TaskTypeExplore,
		},
		{
			name:   "explore: 'search' keyword",
			prompt: "Search for the function definition",
			want:   config.TaskTypeExplore,
		},
		{
			name:   "explore: 'explain' keyword",
			prompt: "Explain how the routing works",
			want:   config.TaskTypeExplore,
		},
		{
			name:   "explore: 'what is' keyword",
			prompt: "What is the purpose of this package?",
			want:   config.TaskTypeExplore,
		},
		{
			name:   "explore: 'investigate' keyword",
			prompt: "Investigate why the test is failing",
			want:   config.TaskTypeExplore,
		},

		// ── Plan ─────────────────────────────────────────────────
		{
			name:   "plan: 'plan' keyword",
			prompt: "Plan the implementation of user authentication",
			want:   config.TaskTypePlan,
		},
		{
			name:   "plan: 'design' keyword",
			prompt: "Design the database schema for orders",
			want:   config.TaskTypePlan,
		},
		{
			name:   "plan: 'architecture' keyword",
			prompt: "Architecture review of the current system",
			want:   config.TaskTypePlan,
		},
		{
			name:   "plan: 'analyze' keyword",
			prompt: "Analyze the performance bottlenecks",
			want:   config.TaskTypePlan,
		},
		{
			name:   "plan: 'how to' keyword",
			prompt: "How to design the database schema?",
			want:   config.TaskTypePlan,
		},
		{
			name:   "plan: 'compare' keyword",
			prompt: "Compare SQLite vs PostgreSQL for this use case",
			want:   config.TaskTypePlan,
		},

		// ── Code (default) ───────────────────────────────────────
		{
			name:   "code: default for unknown",
			prompt: "Write a function that sorts an array",
			want:   config.TaskTypeCode,
		},
		{
			name:   "code: implementation task",
			prompt: "Add input validation to the form handler",
			want:   config.TaskTypeCode,
		},
		{
			name:   "code: refactoring task",
			prompt: "Refactor the database layer",
			want:   config.TaskTypeCode,
		},
		{
			name:   "code: fix bug",
			prompt: "Fix the null pointer exception in user.go",
			want:   config.TaskTypeCode,
		},
		{
			name:   "code: add test",
			prompt: "Write unit tests for the authentication module",
			want:   config.TaskTypeCode,
		},

		// ── Edge cases ───────────────────────────────────────────
		{
			name:   "empty prompt defaults to code",
			prompt: "",
			want:   config.TaskTypeCode,
		},
		{
			name:   "mixed: explore word in code context defaults to first match",
			prompt: "explore and then implement",
			want:   config.TaskTypeExplore,
		},
		{
			name:   "case insensitive: EXPLORE",
			prompt: "EXPLORE this codebase",
			want:   config.TaskTypeExplore,
		},
		{
			name:   "case insensitive: Plan",
			prompt: "Plan the implementation",
			want:   config.TaskTypePlan,
		},
		{
			name:   "plan prioritized over code with 'design'",
			prompt: "Design and implement a caching layer",
			want:   config.TaskTypePlan,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyTask(tc.prompt)
			if got != tc.want {
				t.Errorf("ClassifyTask(%q) = %v, want %v", tc.prompt, got, tc.want)
			}
		})
	}
}
