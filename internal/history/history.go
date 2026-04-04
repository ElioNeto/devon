// Package history persiste sessões de conversa por projeto.
// TODO(issue #5): implementar persistência completa.
package history

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
)

// sessionDir retorna o diretório onde as sessões do projeto são salvas.
// Usa um hash do WorkDir para isolar projetos entre si.
func sessionDir(workDir string) (string, error) {
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(workDir)))[:12]
	dir := filepath.Join(os.Getenv("HOME"), ".devon", "sessions", hash)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("history: criar diretório de sessão: %w", err)
	}
	return dir, nil
}

// TODO: implementar Load, Save, List conforme issue #5.
// O formato de persistência será JSONL (uma mensagem por linha).

// SessionDir expõe o diretório para uso externo (diagnósticos, etc.).
func SessionDir(workDir string) (string, error) {
	return sessionDir(workDir)
}
