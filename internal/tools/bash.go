package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// BashTool executa comandos shell.
type BashTool struct {
	Dir     string
	Timeout time.Duration
}

type bashParams struct {
	Command string `json:"command"`
}

func (t *BashTool) Name() string        { return "bash" }
func (t *BashTool) Description() string { return "Execute a shell command and return its output. Use this for build, test, git operations, or any other command-line task." }
func (t *BashTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {
				"type": "string",
				"description": "The shell command to execute"
			}
		},
		"required": ["command"]
	}`)
}

func (t *BashTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p bashParams
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("bash: invalid params: %w", err)
	}
	if p.Command == "" {
		return "", fmt.Errorf("bash: command cannot be empty")
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
			return fmt.Sprintf("Command timed out after %v: %s", timeout, out), fmt.Errorf("bash: command timed out: %v", timeout)
		}
		return out, fmt.Errorf("bash: exit error: %w", err)
	}

	if out == "" {
		return "(no output)", nil
	}
	return out, nil
}

// sanitizeOutput trunca output excessivo para evitar sobrecarga do contexto.
func sanitizeOutput(s string) string {
	const maxLen = 32 * 1024 // 32 KB
	if len(s) > maxLen {
		return s[:maxLen] + "\n... [output truncated: exceeded 32 KB limit]"
	}
	return s
}
