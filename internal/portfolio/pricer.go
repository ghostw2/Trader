package portfolio

import (
	"context"
	"strconv"
	"sync"

	"github.com/menribardhi/trader/internal/hub"
	"github.com/menribardhi/trader/internal/models"
)

type PriceFeed struct {
	hub      *hub.Hub
	mu       sync.RWMutex
	price    float64
	hasPrice bool
}

func NewPriceFeed(h *hub.Hub) *PriceFeed {
	return &PriceFeed{hub: h}
}

func (p *PriceFeed) Run(ctx context.Context) {
	sub := p.hub.Subscribe()
	defer p.hub.Unsubscribe(sub)
	for {
		select {
		case tick, ok := <-sub:
			if !ok {
				return
			}
			p.update(tick)
		case <-ctx.Done():
			return
		}
	}
}

func (p *PriceFeed) update(tick models.Tick) {
	price, err := strconv.ParseFloat(tick.Price, 64)
	if err != nil {
		return
	}
	p.mu.Lock()
	p.price = price
	p.hasPrice = true
	p.mu.Unlock()
}

func (p *PriceFeed) Latest() (float64, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.price, p.hasPrice
}
