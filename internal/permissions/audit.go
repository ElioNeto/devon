package permissions

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// AuditLogger registra acoes de ferramentas em um arquivo de auditoria.
type AuditLogger struct {
	mu   sync.Mutex
	path string
	file *os.File
}

// NewAuditLogger cria um logger que escreve em ~/.devon/audit.log.
func NewAuditLogger() (*AuditLogger, error) {
	dir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("audit: nao foi possivel obter home dir: %w", err)
	}
	dir = filepath.Join(dir, ".devon")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("audit: nao foi possivel criar %q: %w", dir, err)
	}
	return &AuditLogger{path: filepath.Join(dir, "audit.log")}, nil
}

// Log registra uma execucao de ferramenta.
func (a *AuditLogger) Log(toolName, args, result string, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.file == nil {
		f, openErr := os.OpenFile(a.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if openErr != nil {
			return // falha silenciosa — auditoria nao deve quebrar o fluxo
		}
		a.file = f
	}

	status := "OK"
	if err != nil {
		status = "ERROR: " + err.Error()
	}

	line := fmt.Sprintf("[%s] tool=%s args=%q status=%s result=%q\n",
		time.Now().Format(time.RFC3339), toolName, args, status, result)

	_, _ = a.file.WriteString(line)
}

// AuditEntry representa uma unica linha no log de auditoria.
type AuditEntry struct {
	Timestamp time.Time
	Tool      string
	Args      string
	Status    string
	Result    string
	Blocked   bool
}

// Entries retorna todas as entradas do log de auditoria.
func (a *AuditLogger) Entries() ([]AuditEntry, error) {
	a.mu.Lock()
	// Flush qualquer buffer pendente
	if a.file != nil {
		_ = a.file.Sync()
	}
	a.mu.Unlock()

	data, err := os.ReadFile(a.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("audit: nao foi possivel ler %q: %w", a.path, err)
	}

	var entries []AuditEntry
	lines := string(data)
	for _, line := range splitLines(lines) {
		if line == "" {
			continue
		}
		e := parseAuditLine(line)
		entries = append(entries, e)
	}
	return entries, nil
}

// Summary gera um resumo das entradas de auditoria.
func (a *AuditLogger) Summary() (string, error) {
	entries, err := a.Entries()
	if err != nil {
		return "", err
	}
	if len(entries) == 0 {
		return "Nenhuma acao registrada nesta sessao.", nil
	}

	var (
		total   int
		ok      int
		errs    int
		blocked int
		byTool  = make(map[string]struct{ ok, fail int })
	)

	for _, e := range entries {
		total++
		isErr := e.Status != "OK"
		if e.Blocked {
			blocked++
		}
		if isErr {
			errs++
		} else {
			ok++
		}
		s := byTool[e.Tool]
		if isErr {
			s.fail++
		} else {
			s.ok++
		}
		byTool[e.Tool] = s
	}

	b := &strings.Builder{}
	b.WriteString(fmt.Sprintf("--- Resumo da Sessao (%d acoes) ---\n", total))
	b.WriteString(fmt.Sprintf("%d OK  %d erros  %d bloqueados\n", ok, errs, blocked))
	b.WriteString("\nPor ferramenta:\n")

	// Order tools alphabetically via sorted keys
	keys := make([]string, 0, len(byTool))
	for k := range byTool {
		keys = append(keys, k)
	}
	// Simple insertion sort (small N)
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}

	for _, k := range keys {
		s := byTool[k]
		b.WriteString(fmt.Sprintf("  %-12s %d OK  %d erros\n", k, s.ok, s.fail))
	}

	return b.String(), nil
}

func parseAuditLine(line string) AuditEntry {
	e := AuditEntry{}
	// Format: [RFC3339] tool=X args="..." status=OK|ERROR result="..."
	if len(line) < 2 || line[0] != '[' {
		return e
	}
	close := -1
	for i := 1; i < len(line); i++ {
		if line[i] == ']' {
			close = i
			break
		}
	}
	if close < 0 {
		return e
	}
	ts := line[1:close]
	e.Timestamp, _ = time.Parse(time.RFC3339, ts)

	rest := line[close+2:] // skip "] "
	for rest != "" {
		rest = consumeField(rest, &e)
	}
	return e
}

func consumeField(s string, e *AuditEntry) string {
	if !hasPrefix(s, "tool=") && !hasPrefix(s, "args=") && !hasPrefix(s, "status=") && !hasPrefix(s, "result=") {
		// skip to next space
		for i := 1; i < len(s); i++ {
			if s[i] == ' ' {
				return s[i+1:]
			}
		}
		return ""
	}

	if hasPrefix(s, "tool=") {
		s = s[5:]
		end := fieldEnd(s)
		e.Tool = s[:end]
		return skipPast(s, end)
	}
	if hasPrefix(s, "args=") {
		s = s[5:]
		end := quotedEnd(s)
		if end >= 0 && end < len(s) {
			e.Args = s[1:end]
		}
		return skipPast(s, end+1)
	}
	if hasPrefix(s, "status=") {
		s = s[7:]
		end := fieldEnd(s)
		e.Status = s[:end]
		if hasPrefix(s, "ERROR") || hasPrefix(s, "blocked") {
			e.Blocked = true
		}
		return skipPast(s, end)
	}
	if hasPrefix(s, "result=") {
		s = s[7:]
		end := quotedEnd(s)
		if end >= 0 && end < len(s) {
			e.Result = s[1:end]
		}
		return skipPast(s, end+1)
	}
	return ""
}

func hasPrefix(s, p string) bool { return len(s) >= len(p) && s[:len(p)] == p }
func fieldEnd(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' {
			return i
		}
	}
	return len(s)
}
func skipPast(s string, i int) string {
	if i >= len(s) {
		return ""
	}
	for i < len(s) && s[i] == ' ' {
		i++
	}
	if i >= len(s) {
		return ""
	}
	return s[i:]
}
func quotedEnd(s string) int {
	if len(s) < 1 || s[0] != '"' {
		return fieldEnd(s)
	}
	for i := 1; i < len(s); i++ {
		if s[i] == '\\' {
			i++ // skip escaped
			continue
		}
		if s[i] == '"' {
			return i
		}
	}
	return len(s) - 1
}
func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

// Close fecha o arquivo de auditoria.
func (a *AuditLogger) Close() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.file != nil {
		_ = a.file.Close()
		a.file = nil
	}
}
