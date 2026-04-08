package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// Event representa um evento publicado pelo subscriber.
type Event struct {
	Type      string    `json:"type"`
	SessionID string    `json:"session_id,omitempty"`
	AgentID   string    `json:"agent_id,omitempty"`
	Payload   any       `json:"payload,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// Message representa uma mensagem no chat.
type Message struct {
	ID        int64     `json:"id"`
	SessionID string    `json:"session_id"`
	AgentID   string    `json:"agent_id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// AgentState representa o snapshot do estado de um agente.
type AgentState struct {
	AgentID    string    `json:"agent_id"`
	SessionID  string    `json:"session_id"`
	Snapshot   string    `json:"snapshot"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ToolCall representa uma chamada de ferramenta.
type ToolCall struct {
	ID        int64     `json:"id"`
	AgentID   string    `json:"agent_id"`
	SessionID string    `json:"session_id"`
	ToolName  string    `json:"tool_name"`
	Arguments string    `json:"arguments"`
	Status    string    `json:"status"`
	Result    string    `json:"result,omitempty"`
	Error     string    `json:"error,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// CostSummary representa o resumo de custos.
type CostSummary struct {
	TotalCost   float64            `json:"total_cost"`
	TokenUsage  map[string]int     `json:"token_usage"`
	CreatedAt   time.Time          `json:"created_at"`
	AgentCosts  map[string]float64 `json:"agent_costs"`
}

// Store é a interface para armazenamento persistente.
type Store interface {
	// Sessions
	CreateSession(ctx context.Context, id string) error
	GetSession(ctx context.Context, id string) (bool, error)
	ListSessions(ctx context.Context, limit int) ([]string, error)

	// Hot path - Messages
	PutMessage(ctx context.Context, agentID, sessionID, role, content string) error
	GetMessages(ctx context.Context, agentID, sessionID string, limit int) ([]Message, error)
	SlidingWindow(ctx context.Context, agentID, sessionID string, windowSize int) error

	// Hot path - Agent State
	PutAgentState(ctx context.Context, agentID, sessionID, snapshot string) error
	GetAgentState(ctx context.Context, agentID string) (*AgentState, error)

	// Hot path - Tool Calls
	PutToolCall(ctx context.Context, agentID, sessionID, toolName, arguments, status, result, err string) (int64, error)
	GetToolCalls(ctx context.Context, sessionID string) ([]ToolCall, error)

	// Cold path - Session History
	ArchiveMessages(ctx context.Context, agentID, sessionID string) error
	GetSessionHistory(ctx context.Context, sessionID string, limit int) ([]Message, error)

	// Cold path - Artifacts
	PutArtifact(ctx context.Context, key, sessionID string, data []byte) error
	GetArtifact(ctx context.Context, key string) ([]byte, error)

	// Cold path - Cost Summary
	GetCostSummary(ctx context.Context, sessionID string) (*CostSummary, error)
	UpdateCostSummary(ctx context.Context, sessionID string, cost float64, tokens map[string]int) error

	// Pub/Sub
	Subscribe(ctx context.Context, topic string) (<-chan Event, error)
	Publish(ctx context.Context, topic string, payload any) error

	// Close
	Close() error
}

// SQLiteStore é a implementação Store usando modernc.org/sqlite.
type SQLiteStore struct {
	db       *sql.DB
	mu       sync.RWMutex
	pubsub   map[string]chan Event
	muPubsub sync.RWMutex
}

// New cria uma nova instância de SQLiteStore.
func New(dbPath string) (Store, error) {
	dsn := dbPath + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(1 * time.Hour)

	if err := execSchema(db); err != nil {
		db.Close()
		return nil, err
	}

	store := &SQLiteStore{
		db:       db,
		pubsub:   make(map[string]chan Event),
	}

	return store, nil
}

func execSchema(db *sql.DB) error {
	stmts := splitSchema(schema)
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func splitSchema(s string) []string {
	var stmts []string
	var current string
	inString := false
	quote := byte(0)

	for i := 0; i < len(s); i++ {
		c := s[i]

		if !inString {
			if c == '"' || c == '\'' {
				inString = true
				quote = c
			} else if c == ';' {
				trimmed := strings.TrimSpace(current)
				if trimmed != "" {
					stmts = append(stmts, trimmed)
				}
				current = ""
				continue
			}
		} else {
			if c == quote && (i == 0 || s[i-1] != '\\') {
				inString = false
			}
		}
		current += string(c)
	}

	trimmed := strings.TrimSpace(current)
	if trimmed != "" {
		stmts = append(stmts, trimmed)
	}

	return stmts
}

// Sessions
func (s *SQLiteStore) CreateSession(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT OR REPLACE INTO sessions (id) VALUES (?)", id)
	return err
}

func (s *SQLiteStore) GetSession(ctx context.Context, id string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions WHERE id=?", id).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *SQLiteStore) ListSessions(ctx context.Context, limit int) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id FROM sessions ORDER BY created_at DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// Hot path - Messages
func (s *SQLiteStore) PutMessage(ctx context.Context, agentID, sessionID, role, content string) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO messages (agent_id, session_id, role, content) VALUES (?, ?, ?, ?)",
		agentID, sessionID, role, content)
	return err
}

func (s *SQLiteStore) GetMessages(ctx context.Context, agentID, sessionID string, limit int) ([]Message, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, agent_id, session_id, role, content, timestamp FROM messages WHERE agent_id=? AND session_id=? ORDER BY timestamp DESC LIMIT ?",
		agentID, sessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.AgentID, &m.SessionID, &m.Role, &m.Content, &m.Timestamp); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

func (s *SQLiteStore) SlidingWindow(ctx context.Context, agentID, sessionID string, windowSize int) error {
	// Conta mensagens do agente
	var total int
	if err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM messages WHERE agent_id=?", agentID).Scan(&total); err != nil {
		return err
	}

	if total <= windowSize {
		return nil
	}

	// Mover mensagens antigas para cold storage
	toArchive := total - windowSize
	rows, err := s.db.QueryContext(ctx,
		`INSERT INTO session_history (session_id, agent_id, role, content, archived_at)
		 SELECT session_id, agent_id, role, content, datetime('now')
		 FROM messages WHERE agent_id=?
		 ORDER BY timestamp ASC
		 LIMIT ?`, agentID, toArchive)
	if err != nil {
		return err
	}
	rows.Close()

	// Deletar do hot path
	_, err = s.db.ExecContext(ctx,
		`DELETE FROM messages WHERE agent_id=?
		 AND id IN (
			 SELECT id FROM messages WHERE agent_id=? ORDER BY timestamp ASC LIMIT ?
		 )`, agentID, agentID, toArchive)
	return err
}

// Hot path - Agent State
func (s *SQLiteStore) PutAgentState(ctx context.Context, agentID, sessionID, snapshot string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO agent_states (agent_id, session_id, snapshot, updated_at)
		 VALUES (?, ?, ?, datetime('now'))
		 ON CONFLICT(agent_id) DO UPDATE SET
		 session_id=?, snapshot=?, updated_at=datetime('now')`,
		agentID, sessionID, snapshot, sessionID, snapshot)
	return err
}

func (s *SQLiteStore) GetAgentState(ctx context.Context, agentID string) (*AgentState, error) {
	var state AgentState
	err := s.db.QueryRowContext(ctx,
		`SELECT agent_id, session_id, snapshot, updated_at FROM agent_states WHERE agent_id=?`, agentID).
		Scan(&state.AgentID, &state.SessionID, &state.Snapshot, &state.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &state, nil
}

// Hot path - Tool Calls
func (s *SQLiteStore) PutToolCall(ctx context.Context, agentID, sessionID, toolName, arguments, status, result, errStr string) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO tool_calls (agent_id, session_id, tool_name, arguments, status, result, error)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		agentID, sessionID, toolName, arguments, status, result, errStr)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *SQLiteStore) GetToolCalls(ctx context.Context, sessionID string) ([]ToolCall, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, agent_id, session_id, tool_name, arguments, status, result, error, timestamp
		 FROM tool_calls WHERE session_id=? ORDER BY timestamp ASC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var calls []ToolCall
	for rows.Next() {
		var tc ToolCall
		if err := rows.Scan(&tc.ID, &tc.AgentID, &tc.SessionID, &tc.ToolName,
			&tc.Arguments, &tc.Status, &tc.Result, &tc.Error, &tc.Timestamp); err != nil {
			return nil, err
		}
		calls = append(calls, tc)
	}
	return calls, rows.Err()
}

// Cold path - Session History
func (s *SQLiteStore) ArchiveMessages(ctx context.Context, agentID, sessionID string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO session_history (session_id, agent_id, role, content, archived_at)
		 SELECT session_id, agent_id, role, content, datetime('now')
		 FROM messages WHERE agent_id=?`, agentID)
	return err
}

func (s *SQLiteStore) GetSessionHistory(ctx context.Context, sessionID string, limit int) ([]Message, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, agent_id, session_id, role, content, archived_at as timestamp
		 FROM session_history WHERE session_id=? ORDER BY timestamp DESC LIMIT ?`, sessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.AgentID, &m.SessionID, &m.Role, &m.Content, &m.Timestamp); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

// Cold path - Artifacts
func (s *SQLiteStore) PutArtifact(ctx context.Context, key, sessionID string, data []byte) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO artifacts (key, session_id, data) VALUES (?, ?, ?)`,
		key, sessionID, data)
	return err
}

func (s *SQLiteStore) GetArtifact(ctx context.Context, key string) ([]byte, error) {
	var data []byte
	err := s.db.QueryRowContext(ctx, "SELECT data FROM artifacts WHERE key=?", key).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return data, err
}

// Cold path - Cost Summary
func (s *SQLiteStore) GetCostSummary(ctx context.Context, sessionID string) (*CostSummary, error) {
	var cs CostSummary
	var tokenUsageStr string
	err := s.db.QueryRowContext(ctx,
		"SELECT total_cost, token_usage, created_at FROM cost_summary WHERE session_id=?", sessionID).
		Scan(&cs.TotalCost, &tokenUsageStr, &cs.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(tokenUsageStr), &cs.TokenUsage)
	return &cs, nil
}

func (s *SQLiteStore) UpdateCostSummary(ctx context.Context, sessionID string, cost float64, tokens map[string]int) error {
	tokensJSON, err := json.Marshal(tokens)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO cost_summary (session_id, total_cost, token_usage)
		 VALUES (?, ?, ?)`, sessionID, cost, string(tokensJSON))
	return err
}

// Pub/Sub
func (s *SQLiteStore) Subscribe(ctx context.Context, topic string) (<-chan Event, error) {
	s.muPubsub.Lock()
	defer s.muPubsub.Unlock()

	ch, exists := s.pubsub[topic]
	if !exists {
		ch = make(chan Event, 64)
		s.pubsub[topic] = ch
	}
	return ch, nil
}

func (s *SQLiteStore) Publish(ctx context.Context, topic string, payload any) error {
	s.muPubsub.RLock()
	ch, exists := s.pubsub[topic]
	if !exists {
		s.muPubsub.RUnlock()
		return nil
	}

	event := Event{
		Type:      topic,
		Payload:   payload,
		Timestamp: time.Now(),
	}

	select {
	case ch <- event:
		s.muPubsub.RUnlock()
	case <-ctx.Done():
		s.muPubsub.RUnlock()
		return ctx.Err()
	}
	return nil
}

// Close fecha o database.
func (s *SQLiteStore) Close() error {
	s.muPubsub.Lock()
	for _, ch := range s.pubsub {
		close(ch)
	}
	s.muPubsub.Unlock()
	return s.db.Close()
}

