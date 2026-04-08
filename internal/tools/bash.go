package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/ElioNeto/devon/internal/permissions"
)

// BashTool executa comandos shell.
type BashTool struct {
	Dir     string
	Timeout time.Duration
}

type bashParams struct {
	Command string `json:"command"`
}

func (t *BashTool) Name() string                            { return "bash" }
func (t *BashTool) Permission() permissions.PermissionLevel { return permissions.PermExecute }
func (t *BashTool) Description() string {
	return "Executa um comando shell e retorna sua saida. Use para construir, testar, operacoes git ou qualquer outra tarefa de linha de comando."
}
func (t *BashTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {
				"type": "string",
				"description": "O comando shell a executar"
			}
		},
		"required": ["command"]
	}`)
}

func (t *BashTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p bashParams
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("bash: parametros invalidos: %w", err)
	}
	if p.Command == "" {
		return "", fmt.Errorf("bash: comando nao pode estar vazio")
	}

	timeout := t.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if t.Dir == "" {
		t.Dir = "."
	}

	cmd := exec.CommandContext(ctx, "bash", "-c", p.Command) //nolint:gosec
	cmd.Dir = t.Dir
	setProcessGroup(cmd)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	out := strings.TrimSpace(stdout.String())
	if stderr.Len() > 0 {
		out = strings.TrimSpace(stderr.String()) + "\n" + out
	}
	out = sanitizeOutput(out)

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			// Mata todo o process group (shell + subprocessos) se possivel
			_ = killProcessGroup(cmd)
			return fmt.Sprintf("Comando excedeu o tempo limite apos %v: %s", timeout, out), fmt.Errorf("bash: comando excedeu o tempo limite: %v", timeout)
		}
		return out, fmt.Errorf("bash: erro de execucao: %w", err)
	}

	if out == "" {
		return "(sem saida)", nil
	}
	return out, nil
}

// sanitizeOutput trunca output excessivo para evitar sobrecarga do contexto.
func sanitizeOutput(s string) string {
	const maxLen = 32 * 1024 // 32 KB
	if len(s) > maxLen {
		return s[:maxLen] + "\n... [saida truncada: excedeu limite de 32 KB]"
	}
	return s
}
