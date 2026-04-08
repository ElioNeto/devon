// Package prompts provê o system prompt embarcado no binário.
package prompts

import _ "embed"

//go:embed system.md
var systemPrompt string

// GetSystemPrompt retorna o system prompt do Devon.
func GetSystemPrompt() string {
	return systemPrompt
}
