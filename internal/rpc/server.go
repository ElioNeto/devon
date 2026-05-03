package rpc

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"
)

// DefaultSocketPath is the default Unix socket path.
const DefaultSocketPath = ".devon/rpc.sock"

// ClientConn represents a single connected client with its send channel.
type ClientConn struct {
	ID   string
	Send chan []byte
}

// HandlerFunc processes a JSON-RPC request and returns a response.
type HandlerFunc func(ctx context.Context, req *Request) Response

// Server is a JSON-RPC 2.0 server over Unix socket.
type Server struct {
	listener net.Listener
	handlers map[string]HandlerFunc
	clients  map[string]*ClientConn
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	sockPath string
}

// NewServer creates a new RPC server.
func NewServer() *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		handlers: make(map[string]HandlerFunc),
		clients:  make(map[string]*ClientConn),
		ctx:      ctx,
		cancel:   cancel,
		sockPath: DefaultSocketPath,
	}
}

// Register registers a handler for the given method.
func (s *Server) Register(method string, handler HandlerFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[method] = handler
}

// resolveSocketPath returns the absolute path for the socket.
func resolveSocketPath(workDir string, sockPath string) string {
	if filepath.IsAbs(sockPath) {
		return sockPath
	}
	return filepath.Join(workDir, sockPath)
}

// Listen starts listening on the Unix socket.
func (s *Server) Listen(workDir string) (err error) {
	s.sockPath = resolveSocketPath(workDir, s.sockPath)

	// Ensure parent directory exists
	dir := filepath.Dir(s.sockPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("rpc: create socket dir: %w", err)
	}

	// Remove stale socket file
	_ = os.Remove(s.sockPath)

	s.listener, err = net.Listen("unix", s.sockPath)
	if err != nil {
		return fmt.Errorf("rpc: listen: %w", err)
	}
	slog.Info("rpc: listening", "socket", s.sockPath)
	return nil
}

// Serve accepts connections and handles them until context cancellation.
func (s *Server) Serve() error {
	if s.listener == nil {
		return fmt.Errorf("rpc: Serve called before Listen")
	}
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return nil
			default:
				slog.Warn("rpc: accept error", "err", err)
				continue
			}
		}
		c := &ClientConn{
			Send: make(chan []byte, 64),
		}
		s.addClient(c) // addClient sets the ID under lock
		s.wg.Add(1)
		go s.handleConnection(conn, c)
	}
}

// handleConnection reads JSON-RPC requests and writes responses.
func (s *Server) handleConnection(conn net.Conn, c *ClientConn) {
	defer s.wg.Done()
	defer conn.Close()
	defer s.removeClient(c)

	slog.Debug("rpc: client connected", "id", c.ID)

	// Start writer goroutine
	go s.writeLoop(conn, c)

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			slog.Warn("rpc: unmarshal error", "err", err)
			resp := NewErrorResponse(nil, ErrParse, "Parse error", nil)
			s.sendResponse(c, resp)
			continue
		}

		// Dispatch
		s.mu.RLock()
		handler, ok := s.handlers[req.Method]
		s.mu.RUnlock()

		if !ok {
			resp := NewErrorResponse(req.ID, ErrMethodNotFound, "Method not found: "+req.Method, nil)
			s.sendResponse(c, resp)
			continue
		}

		resp := handler(s.ctx, &req)
		s.sendResponse(c, resp)
	}

	if err := scanner.Err(); err != nil {
		slog.Warn("rpc: scan error", "id", c.ID, "err", err)
	}
	slog.Debug("rpc: client disconnected", "id", c.ID)
}

// writeLoop sends messages from the client's send channel.
func (s *Server) writeLoop(conn net.Conn, c *ClientConn) {
	for msg := range c.Send {
		if _, err := conn.Write(append(msg, '\n')); err != nil {
			slog.Warn("rpc: write error", "id", c.ID, "err", err)
			return
		}
	}
}

// sendResponse sends a response to a specific client.
func (s *Server) sendResponse(c *ClientConn, resp Response) {
	data, err := json.Marshal(resp)
	if err != nil {
		slog.Error("rpc: marshal response", "err", err)
		return
	}
	select {
	case c.Send <- data:
	default:
		slog.Warn("rpc: client send buffer full", "id", c.ID)
	}
}

// Broadcast sends a message to all connected clients.
func (s *Server) Broadcast(data []byte) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, c := range s.clients {
		select {
		case c.Send <- data:
		default:
			slog.Warn("rpc: broadcast buffer full", "id", c.ID)
		}
	}
}

// addClient registers a connected client and assigns it an ID under lock.
func (s *Server) addClient(c *ClientConn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c.ID = fmt.Sprintf("client-%d", len(s.clients)+1)
	s.clients[c.ID] = c
}

// removeClient unregisters a disconnected client.
func (s *Server) removeClient(c *ClientConn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.clients, c.ID)
	close(c.Send)
}

// Close gracefully shuts down the server.
func (s *Server) Close() error {
	s.cancel()
	if s.listener != nil {
		_ = s.listener.Close()
	}
	s.wg.Wait()
	_ = os.Remove(s.sockPath)
	slog.Info("rpc: server shut down")
	return nil
}

// SocketPath returns the server's socket path.
func (s *Server) SocketPath() string {
	return s.sockPath
}
