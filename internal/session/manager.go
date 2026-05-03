package session

import (
	"context"

	"github.com/ElioNeto/devon/internal/db"
)

// Manager wraps a db.Store to provide session CRUD and stats operations.
type Manager struct {
	store db.Store
}

// NewManager creates a new session Manager backed by the given store.
func NewManager(store db.Store) *Manager {
	return &Manager{store: store}
}

// SessionStats holds aggregate statistics across all sessions.
type SessionStats struct {
	TotalSessions   int     `json:"total_sessions"`
	ActiveSessions  int     `json:"active_sessions"`
	TotalMessages   int     `json:"total_messages"`
	TotalToolCalls  int     `json:"total_tool_calls"`
	TotalCost       float64 `json:"total_cost"`
	TotalTokens     int     `json:"total_tokens"`
	AvgDurationMs   int64   `json:"avg_duration_ms"`
}

// List returns all sessions with details, ordered by last activity descending.
func (m *Manager) List(ctx context.Context, limit int) ([]db.SessionDetail, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return m.store.ListSessionsDetail(ctx, limit)
}

// Get returns a single session's detail by ID.
func (m *Manager) Get(ctx context.Context, id string) (*db.SessionDetail, error) {
	return m.store.GetSessionDetail(ctx, id)
}

// Create creates a new session with the given id, task, and model.
func (m *Manager) Create(ctx context.Context, id, task, model string) error {
	return m.store.CreateSessionWithMeta(ctx, id, task, model, "active")
}

// Update updates a session's task, model, and/or status. Empty fields are not changed.
func (m *Manager) Update(ctx context.Context, id, task, model, status string) error {
	return m.store.UpdateSession(ctx, id, task, model, status)
}

// Delete removes a session and all its cascade data from the DB.
func (m *Manager) Delete(ctx context.Context, id string) error {
	return m.store.DeleteSession(ctx, id)
}

// Stats computes aggregate statistics across all sessions.
func (m *Manager) Stats(ctx context.Context) (*SessionStats, error) {
	sessions, err := m.store.ListSessionsDetail(ctx, 1000)
	if err != nil {
		return nil, err
	}

	stats := &SessionStats{}
	for _, s := range sessions {
		stats.TotalSessions++
		if s.Status == "active" {
			stats.ActiveSessions++
		}
		stats.TotalMessages += s.MessageCount
		stats.TotalToolCalls += s.ToolCallCount
		stats.TotalCost += s.TotalCost
		stats.TotalTokens += s.TotalTokens
		stats.AvgDurationMs += s.Duration
	}

	if stats.TotalSessions > 0 {
		stats.AvgDurationMs /= int64(stats.TotalSessions)
	}

	return stats, nil
}

// Touch updates the last_activity timestamp for a session.
func (m *Manager) Touch(ctx context.Context, id string) error {
	return m.store.UpdateSession(ctx, id, "", "", "")
}

