// Package memory contém as ferramentas de memória semântica e seu Manager.
package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ElioNeto/devon/internal/permissions"
)

// RememberTool is a tool that saves a fact to the semantic memory store.
// Name: "remember"
// Permission: PermRead (it only writes to the database, not to the filesystem).
type RememberTool struct {
	Manager   *Manager
	ProjectID string
}

// Name returns the tool name: "remember".
func (t *RememberTool) Name() string { return "remember" }

// Description returns the purpose and parameter schema of the remember tool.
func (t *RememberTool) Description() string {
	return `Salva um fato sobre o projeto para memória semântica.

Args: {
  "category": "string",     // categoria (ex: "convention", "architecture", "error")
  "content": "string"       // o fato em si
}`
}

// Schema returns the JSON Schema for the remember tool parameters.
func (t *RememberTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"category":{"type":"string","description":"categoria do fato"},"content":{"type":"string","description":"conteúdo do fato a ser salvo"}},"required":["category","content"]}`)
}

// Execute runs the remember tool, saving a fact via Manager.Remember.
// It expects "category" and "content" in the JSON params.
func (t *RememberTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var req struct {
		Category string `json:"category"`
		Content  string `json:"content"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return "", fmt.Errorf("remember: invalid params: %w", err)
	}
	if req.Category == "" || req.Content == "" {
		return "", fmt.Errorf("remember: category and content are required")
	}
	if err := t.Manager.Remember(ctx, t.ProjectID, req.Category, req.Content); err != nil {
		return "", fmt.Errorf("remember: %w", err)
	}
	return fmt.Sprintf("Saved fact [%s]: %s", req.Category, req.Content), nil
}

// Permission returns the permission level required to use this tool.
func (t *RememberTool) Permission() permissions.PermissionLevel {
	return permissions.PermRead
}

// RecallTool is a tool that retrieves facts from semantic memory.
// Name: "recall"
// Permission: PermRead.
type RecallTool struct {
	Manager   *Manager
	ProjectID string
}

// Name returns the tool name: "recall".
func (t *RecallTool) Name() string { return "recall" }

// Description returns the purpose and parameter schema of the recall tool.
func (t *RecallTool) Description() string {
	return `Retrieves facts from semantic memory.

Args: {
  "category": "string",  // optional: filter by category
  "keyword": "string"    // optional: search by keyword in content
}`
}

// Schema returns the JSON Schema for the recall tool parameters.
func (t *RecallTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"category":{"type":"string","description":"filter by category"},"keyword":{"type":"string","description":"search keyword"}}}`)
}

// Execute runs the recall tool, fetching facts via Manager.Recall.
// It accepts optional "category" and "keyword" params. Returns a formatted string of facts.
func (t *RecallTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var req struct {
		Category string `json:"category"`
		Keyword  string `json:"keyword"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return "", fmt.Errorf("recall: invalid params: %w", err)
	}
	facts, err := t.Manager.Recall(ctx, t.ProjectID, req.Category, req.Keyword)
	if err != nil {
		return "", fmt.Errorf("recall: %w", err)
	}
	if len(facts) == 0 {
		if req.Category != "" && req.Keyword != "" {
			return fmt.Sprintf("No facts found for category=%q and keyword=%q", req.Category, req.Keyword), nil
		}
		if req.Category != "" {
			return fmt.Sprintf("No facts found for category=%q", req.Category), nil
		}
		if req.Keyword != "" {
			return fmt.Sprintf("No facts found with keyword=%q", req.Keyword), nil
		}
		return "No facts found.", nil
	}
	var b strings.Builder
	b.WriteString("Facts:\n")
	for _, f := range facts {
		b.WriteString(fmt.Sprintf("- [%s] %s\n", f.Category, f.Content))
	}
	return strings.TrimSuffix(b.String(), "\n"), nil
}

// Permission returns the permission level required to use this tool.
func (t *RecallTool) Permission() permissions.PermissionLevel {
	return permissions.PermRead
}
