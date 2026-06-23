package hub

import (
	"context"
	"sync"

	"github.com/menribardhi/trader/internal/models"
)

// Hub fans price ticks from one source channel to many subscriber channels.
type Hub struct {
	mu      sync.RWMutex
	clients map[chan models.Tick]struct{}
	ticks   chan models.Tick
}

// New creates a Hub that reads from ticks.
func New(ticks chan models.Tick) *Hub {
	return &Hub{
		clients: make(map[chan models.Tick]struct{}),
		ticks:   ticks,
	}
}

// Subscribe registers a new subscriber and returns its buffered channel.
func (h *Hub) Subscribe() chan models.Tick {
	ch := make(chan models.Tick, 16)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

// Unsubscribe removes the subscriber and closes its channel.
func (h *Hub) Unsubscribe(ch chan models.Tick) {
	h.mu.Lock()
	delete(h.clients, ch)
	h.mu.Unlock()
	close(ch)
}

// Run reads ticks and broadcasts each to all subscribers.
// Slow subscribers are dropped (non-blocking send).
func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case tick, ok := <-h.ticks:
			if !ok {
				return
			}
			h.mu.RLock()
			for ch := range h.clients {
				select {
				case ch <- tick:
				default:
				}
			}
			h.mu.RUnlock()
		case <-ctx.Done():
			return
		}
	}
}
