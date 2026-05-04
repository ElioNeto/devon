// Package mcp implements Model Context Protocol client and transport layers.
package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"
)

// stdioTransport implements Transport using stdio (subprocess).
type stdioTransport struct {
	command string
	args    []string
	env     map[string]string

	cmd   *exec.Cmd
	stdin io.WriteCloser
	stdout io.ReadCloser
	mu     sync.Mutex
}

// Connect starts the subprocess and sets up stdio communication.
func (t *stdioTransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.cmd != nil {
		return fmt.Errorf("stdio transport already connected")
	}

	cmd := exec.CommandContext(ctx, t.command, t.args...)

	// Set environment variables
	if len(t.env) > 0 {
		env := make([]string, 0, len(t.env))
		for k, v := range t.env {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		cmd.Env = append(cmd.Env, env...)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	t.cmd = cmd
	t.stdin = stdin
	t.stdout = stdout

	return nil
}

// Close terminates the subprocess gracefully.
func (t *stdioTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.cmd == nil {
		return nil
	}

	// Close stdin to signal EOF to the subprocess
	if t.stdin != nil {
		if err := t.stdin.Close(); err != nil {
			// Log but continue with shutdown
			fmt.Printf("warning: failed to close stdin: %v\n", err)
		}
		t.stdin = nil
	}

	// Close stdout
	if t.stdout != nil {
		if err := t.stdout.Close(); err != nil {
			fmt.Printf("warning: failed to close stdout: %v\n", err)
		}
		t.stdout = nil
	}

	// Wait for process to exit with timeout
	if t.cmd.Process != nil {
		done := make(chan error, 1)
		go func() {
			done <- t.cmd.Wait()
		}()

		select {
		case err := <-done:
			t.cmd = nil
			return err
		case <-time.After(5 * time.Second):
			// Process didn't exit gracefully, kill it
			if t.cmd.Process != nil {
				if err := t.cmd.Process.Kill(); err != nil {
					return fmt.Errorf("failed to kill process: %w", err)
				}
			}
			// Wait for cleanup after kill
			err := t.cmd.Wait()
			t.cmd = nil
			return err
		}
	}

	return nil
}

// Send sends a JSON-RPC request and returns the response.
func (t *stdioTransport) Send(ctx context.Context, req JsonRpcRequest) (*JsonRpcResponse, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.stdin == nil || t.stdout == nil {
		return nil, fmt.Errorf("stdio transport not connected")
	}

	// Marshal request
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Write request with newline delimiter
	if _, err := fmt.Fprintf(t.stdin, "%s\n", data); err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	// Read response
	reader := bufio.NewReader(t.stdout)
	responseChan := make(chan []byte, 1)
	errChan := make(chan error, 1)

	go func() {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			errChan <- err
			return
		}
		responseChan <- bytes.TrimSpace(line)
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-errChan:
		return nil, fmt.Errorf("failed to read response: %w", err)
	case data := <-responseChan:
		var resp JsonRpcResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			return nil, fmt.Errorf("failed to unmarshal response: %w", err)
		}
		return &resp, nil
	}
}
