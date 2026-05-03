package rpc

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"

	"github.com/ElioNeto/devon/internal/db"
)

// StreamManager subscribes to agent/db events and broadcasts them to all
// connected RPC clients as JSON events.
type StreamManager struct {
	srv     *Server
	store   db.Store
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// NewStreamManager creates a new stream manager.
func NewStreamManager(srv *Server, store db.Store) *StreamManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &StreamManager{
		srv:    srv,
		store:  store,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start begins subscribing to DB events and forwarding them to clients.
func (sm *StreamManager) Start() error {
	ch, err := sm.store.Subscribe(sm.ctx, "agent.events")
	if err != nil {
		return err
	}

	sm.wg.Add(1)
	go sm.loop(ch)
	slog.Info("rpc: stream manager started")
	return nil
}

// Stop halts the stream manager.
func (sm *StreamManager) Stop() {
	sm.cancel()
	sm.wg.Wait()
	slog.Info("rpc: stream manager stopped")
}

// loop reads events from the channel and broadcasts them.
func (sm *StreamManager) loop(ch <-chan db.Event) {
	defer sm.wg.Done()
	for {
		select {
		case <-sm.ctx.Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			payload, err := json.Marshal(ev.Payload)
			if err != nil {
				slog.Warn("rpc: stream marshal error", "err", err)
				continue
			}
			event := Event{
				Type:    ev.Type,
				Payload: payload,
			}
			data, err := json.Marshal(event)
			if err != nil {
				slog.Warn("rpc: stream event marshal error", "err", err)
				continue
			}
			sm.srv.Broadcast(data)
		}
	}
}
