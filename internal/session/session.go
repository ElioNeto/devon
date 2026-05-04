// Package session provides the domain model and manager for sessions.
//
// It wraps the db.Store to provide session CRUD, listing, statistics,
// and export functionality on top of the SQLite-backed persistence layer.
package session

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/ElioNeto/devon/internal/db"
)

// Session represents a full session domain object with related data loaded from the DB.
type Session struct {
	ID           string          `json:"id"`
	Task         string          `json:"task,omitempty"`
	Model        string          `json:"model,omitempty"`
	Status       string          `json:"status"`
	Duration     time.Duration   `json:"duration"`
	LastActivity time.Time       `json:"last_activity"`
	CreatedAt    time.Time       `json:"created_at"`
	Messages     []db.Message    `json:"messages,omitempty"`
	ToolCalls    []db.ToolCall   `json:"tool_calls,omitempty"`
	Cost         *db.CostSummary `json:"cost,omitempty"`
	FileAccess   []db.FileAccess `json:"file_access,omitempty"`
}

// ShortID generates an 8-character hex string from 4 random bytes.
// It uses crypto/rand for uniqueness suitable for session identifiers.
func ShortID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
