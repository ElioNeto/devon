package permissions

import (
	"fmt"
	"os"
	"path/filepath"
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

// Close fecha o arquivo de auditoria.
func (a *AuditLogger) Close() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.file != nil {
		_ = a.file.Close()
		a.file = nil
	}
}
