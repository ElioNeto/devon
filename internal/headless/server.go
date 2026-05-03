package headless

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// Server is an HTTP server that provides SSE-based endpoints for CI/CD and
// external clients to interact with the Devon agent.
type Server struct {
	host   string
	port   int
	server *http.Server
	ln     net.Listener
	agents *agentRegistry
	mux    *http.ServeMux
	mu     sync.Mutex
	wg     sync.WaitGroup
}

// agentRegistry holds references to active agent sessions so that the
// /api/respond endpoint can forward user decisions to blocked agents.
type agentRegistry struct {
	mu          sync.RWMutex
	pendingReqs map[string]chan int // requestID → channel to send confirm level
	reqCounter  atomic.Int64        // monotonic counter for unique request IDs
}

func newAgentRegistry() *agentRegistry {
	return &agentRegistry{
		pendingReqs: make(map[string]chan int),
	}
}

// RegisterPendingRequest stores a channel for a pending action_required request.
// Returns the generated requestID using a monotonic counter to avoid collisions.
func (ar *agentRegistry) RegisterPendingRequest(ch chan int) string {
	ar.mu.Lock()
	defer ar.mu.Unlock()
	id := fmt.Sprintf("req-%d", ar.reqCounter.Add(1))
	ar.pendingReqs[id] = ch
	return id
}

// ResolvePendingRequest looks up a pending request and sends the confirm level.
// Returns false if the request ID is not found.
func (ar *agentRegistry) ResolvePendingRequest(requestID string, level int) bool {
	ar.mu.Lock()
	ch, ok := ar.pendingReqs[requestID]
	if !ok {
		ar.mu.Unlock()
		return false
	}
	delete(ar.pendingReqs, requestID)
	ar.mu.Unlock()

	select {
	case ch <- level:
	default:
		slog.Warn("headless: pending request channel full or closed", "request_id", requestID)
	}
	return true
}

// RemovePendingRequest cleans up a pending request (e.g. on context cancel).
func (ar *agentRegistry) RemovePendingRequest(requestID string) {
	ar.mu.Lock()
	delete(ar.pendingReqs, requestID)
	ar.mu.Unlock()
}

// handler returns the HTTP handler (implements http.Handler).
func (s *Server) handler() http.Handler {
	return s.mux
}

// NewServer creates a new headless HTTP server.
func NewServer(host string, port int) *Server {
	mux := http.NewServeMux()
	return &Server{
		host:   host,
		port:   port,
		agents: newAgentRegistry(),
		mux:    mux,
	}
}

// Listen starts listening on the configured address. Returns an error if the
// address is already in use.
func (s *Server) Listen() error {
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("headless: listen on %s: %w", addr, err)
	}
	s.ln = ln
	slog.Info("headless: listening", "addr", addr)
	return nil
}

// Serve starts serving HTTP requests. Must be called after Listen.
func (s *Server) Serve() error {
	if s.ln == nil {
		return fmt.Errorf("headless: Serve called before Listen")
	}
	s.server = &http.Server{
		Handler: s.handler(),
	}
	return s.server.Serve(s.ln)
}

// ServeHTTP serves HTTP requests on the given listener (alternative to
// Listen+Serve for testing with net/http/httptest).
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// Close gracefully shuts down the server.
func (s *Server) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		_ = s.server.Shutdown(ctx)
	}
	s.wg.Wait()
	slog.Info("headless: server shut down")
	return nil
}

// Addr returns the listening address, or empty string if not listening.
func (s *Server) Addr() string {
	if s.ln == nil {
		return ""
	}
	return s.ln.Addr().String()
}

// shutdownTimeout is the maximum time to wait for active connections to drain.
const shutdownTimeout = 5 * time.Second
