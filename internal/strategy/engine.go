package strategy

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/menribardhi/trader/internal/hub"
	"github.com/menribardhi/trader/internal/models"
)

const (
	fastPeriod = 10
	slowPeriod = 50
	emaPeriod  = 20
	rsiPeriod  = 14
)

type Engine struct {
	hub      *hub.Hub
	mu       sync.Mutex
	window   []float64
	fastPrev float64
	slowPrev float64
	clientMu sync.RWMutex
	clients  map[chan models.Signal]struct{}
}

func NewEngine(h *hub.Hub) *Engine {
	return &Engine{
		hub:     h,
		clients: make(map[chan models.Signal]struct{}),
	}
}

func (e *Engine) Subscribe() chan models.Signal {
	ch := make(chan models.Signal, 4)
	e.clientMu.Lock()
	e.clients[ch] = struct{}{}
	e.clientMu.Unlock()
	return ch
}

func (e *Engine) Unsubscribe(ch chan models.Signal) {
	e.clientMu.Lock()
	delete(e.clients, ch)
	e.clientMu.Unlock()
	close(ch)
}

// Run subscribes to the hub and processes ticks until ctx is cancelled.
// Safe to call with a nil hub (blocks on ctx.Done only).
func (e *Engine) Run(ctx context.Context) {
	if e.hub == nil {
		<-ctx.Done()
		return
	}
	sub := e.hub.Subscribe()
	defer e.hub.Unsubscribe(sub)
	for {
		select {
		case tick, ok := <-sub:
			if !ok {
				return
			}
			price, err := strconv.ParseFloat(tick.Price, 64)
			if err != nil {
				continue
			}
			e.ProcessPrice(price)
		case <-ctx.Done():
			return
		}
	}
}

// ProcessPrice appends price to the rolling window, computes indicators,
// detects SMA crossovers, and broadcasts a Signal to all subscribers.
// Exported for use in tests and the SSE handler test helper.
func (e *Engine) ProcessPrice(price float64) {
	e.mu.Lock()
	e.window = append(e.window, price)
	if len(e.window) > slowPeriod {
		e.window = e.window[len(e.window)-slowPeriod:]
	}
	window := make([]float64, len(e.window))
	copy(window, e.window)
	fastPrev := e.fastPrev
	slowPrev := e.slowPrev
	e.mu.Unlock()

	fastNow := SMA(window, fastPeriod)
	slowNow := SMA(window, slowPeriod)

	sig := models.Signal{
		Price:     price,
		Timestamp: time.Now().UnixMilli(),
		SMAFast:   fastNow,
		SMASlow:   slowNow,
		EMA:       EMA(window, emaPeriod),
		RSI:       RSI(window, rsiPeriod),
	}

	if len(window) < slowPeriod {
		sig.Side = "HOLD"
	} else {
		switch {
		case fastPrev <= slowPrev && fastNow > slowNow:
			sig.Side = "BUY"
		case fastPrev >= slowPrev && fastNow < slowNow:
			sig.Side = "SELL"
		default:
			sig.Side = "HOLD"
		}
		e.mu.Lock()
		e.fastPrev = fastNow
		e.slowPrev = slowNow
		e.mu.Unlock()
	}

	e.clientMu.RLock()
	for ch := range e.clients {
		select {
		case ch <- sig:
		default:
		}
	}
	e.clientMu.RUnlock()
}
