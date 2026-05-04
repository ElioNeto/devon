package session

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ElioNeto/devon/internal/db"
)

// ExportData holds all data that can be exported for a session.
type ExportData struct {
	Session    db.SessionDetail `json:"session"`
	Messages   []db.Message     `json:"messages,omitempty"`
	ToolCalls  []db.ToolCall    `json:"tool_calls,omitempty"`
	FileAccess []db.FileAccess  `json:"file_access,omitempty"`
	Cost       *db.CostSummary  `json:"cost,omitempty"`
}

// ExportMarkdown renders the session as a Markdown document.
func ExportMarkdown(data *ExportData) (string, error) {
	if data == nil {
		return "", fmt.Errorf("export: no data to export")
	}

	var b strings.Builder

	// Header
	b.WriteString(fmt.Sprintf("# Session: %s\n\n", data.Session.ID))
	b.WriteString(fmt.Sprintf("**Status:** %s  \n", data.Session.Status))
	b.WriteString(fmt.Sprintf("**Model:** %s  \n", data.Session.Model))
	b.WriteString(fmt.Sprintf("**Task:** %s  \n", data.Session.Task))
	b.WriteString(fmt.Sprintf("**Created:** %s  \n", data.Session.CreatedAt.Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("**Last Activity:** %s  \n", data.Session.LastActivity.Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("**Duration:** %d ms  \n", data.Session.Duration))
	if data.Cost != nil {
		b.WriteString(fmt.Sprintf("**Total Cost:** $%.6f  \n", data.Cost.TotalCost))
	}
	b.WriteString("\n---\n\n")

	// Messages
	if len(data.Messages) > 0 {
		b.WriteString("## Messages\n\n")
		for _, msg := range data.Messages {
			role := strings.ToUpper(msg.Role)
			ts := msg.Timestamp.Format("15:04:05")
			b.WriteString(fmt.Sprintf("### [%s] %s (%s)\n\n", ts, role, msg.AgentID))
			b.WriteString(msg.Content)
			b.WriteString("\n\n")
		}
	}

	// Tool calls
	if len(data.ToolCalls) > 0 {
		b.WriteString("## Tool Calls\n\n")
		b.WriteString("| Time | Tool | Status | Args |\n")
		b.WriteString("|------|------|--------|------|\n")
		for _, tc := range data.ToolCalls {
			ts := tc.Timestamp.Format("15:04:05")
			args := tc.Arguments
			if len(args) > 80 {
				args = args[:80] + "..."
			}
			b.WriteString(fmt.Sprintf("| %s | %s | %s | `%s` |\n", ts, tc.ToolName, tc.Status, args))
		}
		b.WriteString("\n")
	}

	// File access
	if len(data.FileAccess) > 0 {
		b.WriteString("## File Access\n\n")
		b.WriteString("| Time | File | Access Type |\n")
		b.WriteString("|------|------|-------------|\n")
		for _, fa := range data.FileAccess {
			ts := fa.Timestamp.Format("15:04:05")
			b.WriteString(fmt.Sprintf("| %s | %s | %s |\n", ts, fa.FilePath, fa.AccessType))
		}
		b.WriteString("\n")
	}

	return b.String(), nil
}

// ExportJSON returns the session data as indented JSON.
func ExportJSON(data *ExportData) ([]byte, error) {
	if data == nil {
		return nil, fmt.Errorf("export: no data to export")
	}
	return json.MarshalIndent(data, "", "  ")
}
