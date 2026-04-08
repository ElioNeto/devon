// Package history persiste e gerencia sessoes de conversa por projeto.
package history

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ElioNeto/devon/internal/llm"
)

// Session represents a single conversation session.
type Session struct {
	ID        string        `json:"id"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
	Messages  []llm.Message `json:"messages"`
	Usage     UsageSummary  `json:"usage"`
}

// UsageSummary tracks token consumption for a session.
type UsageSummary struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
	Requests         int `json:"requests"`
}

// AddUsage accumulates token usage from a single request.
func (u *UsageSummary) AddUsage(usage *llm.Usage) {
	if usage == nil {
		return
	}
	u.PromptTokens += usage.PromptTokens
	u.CompletionTokens += usage.CompletionTokens
	u.TotalTokens += usage.TotalTokens
	u.Requests++
}

// sessionDir returns the directory where session data is stored for a project.
func sessionDir(workDir string) (string, error) {
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(workDir)))[:12]
	dir := filepath.Join(os.Getenv("HOME"), ".devon", "sessions", hash)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("history: nao foi possivel criar diretorio da sessao: %w", err)
	}
	return dir, nil
}

// SessionDir exposes the session directory path for a project.
func SessionDir(workDir string) (string, error) {
	return sessionDir(workDir)
}

// sessionFile returns the path to a session file by ID.
func sessionFile(dir, id string) string {
	return filepath.Join(dir, id+".json")
}

// CreateSession creates a new session with a unique ID and returns it.
func CreateSession(workDir string) (*Session, error) {
	dir, err := sessionDir(workDir)
	if err != nil {
		return nil, err
	}
	return createSession(dir)
}

// createSession creates a new session with a unique ID and returns it.
func createSession(dir string) (*Session, error) {
	id := time.Now().UTC().Format("20060102T150405.000Z")
	now := time.Now().UTC()

	s := &Session{
		ID:        id,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Save initial session data
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("history: nao foi possivel marshal a sessao: %w", err)
	}

	if err := os.WriteFile(sessionFile(dir, id), data, 0o600); err != nil {
		return nil, fmt.Errorf("history: nao foi possivel escrever arquivo da sessao: %w", err)
	}

	return s, nil
}

// LoadLastSession loads the most recent session for the project.
// If no sessions exist, creates a new one.
func LoadLastSession(workDir string) (*Session, error) {
	dir, err := sessionDir(workDir)
	if err != nil {
		return nil, err
	}

	sessions, err := listSessions(dir)
	if err != nil {
		return nil, err
	}

	if len(sessions) == 0 {
		return createSession(dir)
	}

	// Load the most recent session (sorted by ID descending)
	recent := sessions[len(sessions)-1]
	sessionPath := sessionFile(dir, recent)

	data, err := os.ReadFile(sessionPath)
	if err != nil {
		return nil, fmt.Errorf("history: nao foi possivel ler sessao %q: %w", recent, err)
	}

	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("history: nao foi possivel unmarshal da sessao %q: %w", recent, err)
	}

	return &s, nil
}

// Save persists the session state to disk.
func Save(workDir string, session *Session) error {
	dir, err := sessionDir(workDir)
	if err != nil {
		return err
	}

	session.UpdatedAt = time.Now().UTC()

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("history: nao foi possivel marshal a sessao: %w", err)
	}

	if err := os.WriteFile(sessionFile(dir, session.ID), data, 0o600); err != nil {
		return fmt.Errorf("history: nao foi possivel escrever arquivo da sessao: %w", err)
	}

	return nil
}

// ListSessions returns all session IDs for the project, sorted chronologically.
func ListSessions(workDir string) ([]string, error) {
	dir, err := sessionDir(workDir)
	if err != nil {
		return nil, err
	}
	return listSessions(dir)
}

// listSessions reads the session directory and returns sorted session IDs.
func listSessions(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("history: nao foi possivel ler diretorio da sessao: %w", err)
	}

	var ids []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		ids = append(ids, strings.TrimSuffix(e.Name(), ".json"))
	}

	sort.Strings(ids)
	return ids, nil
}

// LoadSession loads a specific session by ID.
func LoadSession(workDir string, id string) (*Session, error) {
	dir, err := sessionDir(workDir)
	if err != nil {
		return nil, err
	}

	sessionPath := sessionFile(dir, id)
	data, err := os.ReadFile(sessionPath)
	if err != nil {
		return nil, fmt.Errorf("history: nao foi possivel ler sessao %q: %w", id, err)
	}

	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("history: nao foi possivel unmarshal da sessao %q: %w", id, err)
	}

	return &s, nil
}

// ClearSession removes the current session file.
func ClearSession(workDir string, id string) error {
	dir, err := sessionDir(workDir)
	if err != nil {
		return err
	}

	path := sessionFile(dir, id)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("history: nao foi possivel remover sessao %q: %w", id, err)
	}

	return nil
}

// LoadMessages loads the messages from a session file.
func LoadMessages(workDir string, id string) ([]llm.Message, error) {
	s, err := LoadSession(workDir, id)
	if err != nil {
		return nil, err
	}
	return s.Messages, nil
}

// SaveMessages saves messages to the session, updating usage.
func SaveMessages(workDir string, id string, messages []llm.Message, usage *UsageSummary) error {
	dir, err := sessionDir(workDir)
	if err != nil {
		return err
	}

	sessionPath := sessionFile(dir, id)
	var s Session

	if data, err := os.ReadFile(sessionPath); err == nil {
		if err := json.Unmarshal(data, &s); err != nil {
			return fmt.Errorf("history: nao foi possivel unmarshal da sessao: %w", err)
		}
	} else {
		s = Session{
			ID:        id,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
	}

	s.Messages = messages
	if usage != nil {
		s.Usage = *usage
	}
	s.UpdatedAt = time.Now().UTC()

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("history: nao foi possivel marshal a sessao: %w", err)
	}

	return os.WriteFile(sessionPath, data, 0o600)
}

// AppendMessage appends a message to the session and persists it.
func AppendMessage(workDir string, id string, msg llm.Message, usage *UsageSummary) error {
	dir, err := sessionDir(workDir)
	if err != nil {
		return err
	}

	sessionPath := sessionFile(dir, id)
	var s Session

	if data, err := os.ReadFile(sessionPath); err == nil {
		if err := json.Unmarshal(data, &s); err != nil {
			return fmt.Errorf("history: nao foi possivel unmarshal da sessao: %w", err)
		}
	}

	if s.ID == "" {
		s = Session{
			ID:        id,
			CreatedAt: time.Now().UTC(),
		}
	}

	s.Messages = append(s.Messages, msg)
	if usage != nil {
		s.Usage.PromptTokens += usage.PromptTokens
		s.Usage.CompletionTokens += usage.CompletionTokens
		s.Usage.TotalTokens += usage.TotalTokens
		s.Usage.Requests++
	}
	s.UpdatedAt = time.Now().UTC()

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("history: nao foi possivel marshal a sessao: %w", err)
	}

	return os.WriteFile(sessionPath, data, 0o600)
}

// GetSessionInfo returns basic info about the most recent session.
func GetSessionInfo(workDir string) (*Session, error) {
	return LoadLastSession(workDir)
}

// SaveMessagesJSONL appends a message to a JSONL file for the session.
// This is the primary write method for live streaming.
func SaveMessagesJSONL(workDir string, id string, msg llm.Message) error {
	dir, err := sessionDir(workDir)
	if err != nil {
		return err
	}

	jsonlPath := filepath.Join(dir, id+".jsonl")
	f, err := os.OpenFile(jsonlPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("history: nao foi possivel abrir arquivo jsonl: %w", err)
	}
	defer f.Close()

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("history: nao foi possivel marshal a mensagem: %w", err)
	}

	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("history: nao foi possivel escrever mensagem: %w", err)
	}

	return nil
}

// LoadMessagesJSONL loads messages from a JSONL file for a session.
func LoadMessagesJSONL(workDir string, id string) ([]llm.Message, error) {
	dir, err := sessionDir(workDir)
	if err != nil {
		return nil, err
	}

	jsonlPath := filepath.Join(dir, id+".jsonl")
	f, err := os.Open(jsonlPath)
	if err != nil {
		return nil, fmt.Errorf("history: nao foi possivel abrir arquivo jsonl: %w", err)
	}
	defer f.Close()

	var messages []llm.Message
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var msg llm.Message
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			continue // skip malformed lines
		}
		messages = append(messages, msg)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("history: nao foi possivel ler jsonl: %w", err)
	}

	return messages, nil
}
