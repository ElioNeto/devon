package rpc

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestServer_ListenAndServe(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	sockPath := filepath.Join(dir, "test-rpc.sock")

	srv := NewServer()
	srv.sockPath = sockPath

	// Register a simple test handler
	srv.Register("ping", func(ctx context.Context, req *Request) Response {
		return NewResponse(req.ID, map[string]string{"pong": "ok"})
	})

	err := srv.Listen(dir)
	if err != nil {
		t.Fatalf("Listen() error: %v", err)
	}
	defer srv.Close()

	go func() {
		if err := srv.Serve(); err != nil {
			t.Logf("Serve() returned: %v", err)
		}
	}()

	// Give the server a moment to start
	time.Sleep(50 * time.Millisecond)

	// Connect and send a request
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("net.Dial error: %v", err)
	}
	defer conn.Close()

	reqID := int64(1)
	req := Request{
		JSONRPC: Version,
		ID:      &reqID,
		Method:  "ping",
	}
	reqData, _ := json.Marshal(req)
	_, err = fmt.Fprintf(conn, "%s\n", reqData)
	if err != nil {
		t.Fatalf("write error: %v", err)
	}

	// Read response
	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		t.Fatal("no response received")
	}
	var resp Response
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("unexpected error response: %+v", resp.Error)
	}
	if resp.ID == nil || *resp.ID != reqID {
		t.Errorf("response ID = %v, want %d", resp.ID, reqID)
	}
}

func TestServer_MethodNotFound(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	sockPath := filepath.Join(dir, "test-rpc2.sock")

	srv := NewServer()
	srv.sockPath = sockPath

	err := srv.Listen(dir)
	if err != nil {
		t.Fatalf("Listen() error: %v", err)
	}
	defer srv.Close()

	go func() {
		_ = srv.Serve()
	}()

	time.Sleep(50 * time.Millisecond)

	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("net.Dial error: %v", err)
	}
	defer conn.Close()

	reqID := int64(42)
	req := Request{
		JSONRPC: Version,
		ID:      &reqID,
		Method:  "nonexistent",
	}
	reqData, _ := json.Marshal(req)
	_, _ = fmt.Fprintf(conn, "%s\n", reqData)

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		t.Fatal("no response received")
	}
	var resp Response
	json.Unmarshal(scanner.Bytes(), &resp)

	if resp.Error == nil {
		t.Fatal("expected error response for unknown method")
	}
	if resp.Error.Code != ErrMethodNotFound {
		t.Errorf("error code = %d, want %d", resp.Error.Code, ErrMethodNotFound)
	}
}

func TestServer_InvalidJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	sockPath := filepath.Join(dir, "test-rpc3.sock")

	srv := NewServer()
	srv.sockPath = sockPath

	err := srv.Listen(dir)
	if err != nil {
		t.Fatalf("Listen() error: %v", err)
	}
	defer srv.Close()

	go func() {
		_ = srv.Serve()
	}()

	time.Sleep(50 * time.Millisecond)

	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("net.Dial error: %v", err)
	}
	defer conn.Close()

	_, _ = fmt.Fprintf(conn, "{invalid json}\n")

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		t.Fatal("no response received")
	}
	var resp Response
	json.Unmarshal(scanner.Bytes(), &resp)

	if resp.Error == nil {
		t.Fatal("expected error response for invalid JSON")
	}
	if resp.Error.Code != ErrParse {
		t.Errorf("error code = %d, want %d", resp.Error.Code, ErrParse)
	}
}

func TestServer_ConcurrentClients(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	sockPath := filepath.Join(dir, "test-rpc-concurrent.sock")

	srv := NewServer()
	srv.sockPath = sockPath

	srv.Register("echo", func(ctx context.Context, req *Request) Response {
		// Echo back the params
		return NewResponse(req.ID, string(req.Params))
	})

	err := srv.Listen(dir)
	if err != nil {
		t.Fatalf("Listen() error: %v", err)
	}
	defer srv.Close()

	go func() {
		_ = srv.Serve()
	}()

	time.Sleep(50 * time.Millisecond)

	// Start 5 concurrent clients
	const numClients = 5
	errs := make(chan error, numClients)
	for i := 0; i < numClients; i++ {
		go func(idx int) {
			conn, dialErr := net.Dial("unix", sockPath)
			if dialErr != nil {
				errs <- fmt.Errorf("client %d dial: %w", idx, dialErr)
				return
			}
			defer conn.Close()

			reqID := int64(idx)
			req := Request{
				JSONRPC: Version,
				ID:      &reqID,
				Method:  "echo",
			}
			reqData, _ := json.Marshal(req)
			_, writeErr := fmt.Fprintf(conn, "%s\n", reqData)
			if writeErr != nil {
				errs <- fmt.Errorf("client %d write: %w", idx, writeErr)
				return
			}

			scanner := bufio.NewScanner(conn)
			if !scanner.Scan() {
				errs <- fmt.Errorf("client %d no response", idx)
				return
			}
			var resp Response
			if umErr := json.Unmarshal(scanner.Bytes(), &resp); umErr != nil {
				errs <- fmt.Errorf("client %d unmarshal: %w", idx, umErr)
				return
			}
			if resp.Error != nil {
				errs <- fmt.Errorf("client %d error: %+v", idx, resp.Error)
				return
			}
			errs <- nil
		}(i)
	}

	for i := 0; i < numClients; i++ {
		select {
		case err := <-errs:
			if err != nil {
				t.Errorf("concurrent client error: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for concurrent clients")
		}
	}
}

func TestServer_HandlerReturnsError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	sockPath := filepath.Join(dir, "test-rpc-error.sock")

	srv := NewServer()
	srv.sockPath = sockPath

	srv.Register("fail", func(ctx context.Context, req *Request) Response {
		return NewErrorResponse(req.ID, ErrServer, "something went wrong", nil)
	})

	err := srv.Listen(dir)
	if err != nil {
		t.Fatalf("Listen() error: %v", err)
	}
	defer srv.Close()

	go func() {
		_ = srv.Serve()
	}()

	time.Sleep(50 * time.Millisecond)

	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("net.Dial error: %v", err)
	}
	defer conn.Close()

	reqID := int64(1)
	req := Request{
		JSONRPC: Version,
		ID:      &reqID,
		Method:  "fail",
	}
	reqData, _ := json.Marshal(req)
	_, _ = fmt.Fprintf(conn, "%s\n", reqData)

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		t.Fatal("no response received")
	}
	var resp Response
	json.Unmarshal(scanner.Bytes(), &resp)

	if resp.Error == nil {
		t.Fatal("expected error response")
	}
	if resp.Error.Code != ErrServer {
		t.Errorf("error code = %d, want %d", resp.Error.Code, ErrServer)
	}
	if resp.Error.Message != "something went wrong" {
		t.Errorf("error message = %q, want %q", resp.Error.Message, "something went wrong")
	}
}

func TestServer_Broadcast(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	sockPath := filepath.Join(dir, "test-rpc-broadcast.sock")

	srv := NewServer()
	srv.sockPath = sockPath

	err := srv.Listen(dir)
	if err != nil {
		t.Fatalf("Listen() error: %v", err)
	}
	defer srv.Close()

	go func() {
		_ = srv.Serve()
	}()

	time.Sleep(50 * time.Millisecond)

	// Connect a client
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("net.Dial error: %v", err)
	}
	defer conn.Close()

	// Send a harmless request first to wait for the connection to be registered
	reqID := int64(1)
	req := Request{
		JSONRPC: Version,
		ID:      &reqID,
		Method:  "ping",
	}
	srv.Register("ping", func(ctx context.Context, req *Request) Response {
		return NewResponse(req.ID, "pong")
	})
	reqData, _ := json.Marshal(req)
	_, _ = fmt.Fprintf(conn, "%s\n", reqData)

	// Wait for the response to ensure the client is registered
	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		t.Fatal("no response received")
	}

	// Broadcast a message
	broadcastData := []byte(`{"type":"test","payload":"hello"}`)
	srv.Broadcast(broadcastData)

	// Read broadcast message
	if !scanner.Scan() {
		t.Fatal("no broadcast received")
	}
	var evt Event
	if err := json.Unmarshal(scanner.Bytes(), &evt); err != nil {
		t.Fatalf("unmarshal event error: %v", err)
	}
	if evt.Type != "test" {
		t.Errorf("event type = %q, want %q", evt.Type, "test")
	}
}

func TestServer_Close(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	sockPath := filepath.Join(dir, "test-rpc-close.sock")

	srv := NewServer()
	srv.sockPath = sockPath

	err := srv.Listen(dir)
	if err != nil {
		t.Fatalf("Listen() error: %v", err)
	}

	// Close immediately
	if err := srv.Close(); err != nil {
		t.Errorf("Close() error: %v", err)
	}

	// Socket should be removed
	if _, err := os.Stat(sockPath); !os.IsNotExist(err) {
		t.Errorf("expected socket file to be removed, stat error: %v", err)
	}
}

func TestResolveSocketPath(t *testing.T) {
	tests := []struct {
		name     string
		workDir  string
		sockPath string
		want     string
	}{
		{
			name:     "relative path",
			workDir:  "/home/user/project",
			sockPath: ".devon/rpc.sock",
			want:     "/home/user/project/.devon/rpc.sock",
		},
		{
			name:     "absolute path",
			workDir:  "/home/user/project",
			sockPath: "/var/run/devon.sock",
			want:     "/var/run/devon.sock",
		},
		{
			name:     "default path",
			workDir:  "/tmp",
			sockPath: DefaultSocketPath,
			want:     "/tmp/.devon/rpc.sock",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveSocketPath(tt.workDir, tt.sockPath)
			if got != tt.want {
				t.Errorf("resolveSocketPath(%q, %q) = %q, want %q", tt.workDir, tt.sockPath, got, tt.want)
			}
		})
	}
}
