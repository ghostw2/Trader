package hub_test

import (
	"context"
	"testing"
	"time"

	"github.com/menribardhi/trader/internal/hub"
	"github.com/menribardhi/trader/internal/models"
)

func TestHubFansOutToMultipleSubscribers(t *testing.T) {
	ticks := make(chan models.Tick, 1)
	h := hub.New(ticks)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go h.Run(ctx)

	sub1 := h.Subscribe()
	sub2 := h.Subscribe()
	defer h.Unsubscribe(sub1)
	defer h.Unsubscribe(sub2)

	want := models.Tick{Symbol: "BTCUSDT", Price: "50000", Timestamp: 123}
	ticks <- want

	for i, sub := range []chan models.Tick{sub1, sub2} {
		select {
		case got := <-sub:
			if got != want {
				t.Errorf("subscriber %d: got %+v want %+v", i, got, want)
			}
		case <-ctx.Done():
			t.Fatalf("subscriber %d: timeout waiting for tick", i)
		}
	}
}

func TestHubDropsSlowConsumer(t *testing.T) {
	ticks := make(chan models.Tick, 16)
	h := hub.New(ticks)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go h.Run(ctx)

	_ = h.Subscribe() // subscribe but never read — should not block hub

	fast := h.Subscribe()
	defer h.Unsubscribe(fast)

	for i := 0; i < 20; i++ {
		ticks <- models.Tick{Symbol: "BTCUSDT", Price: "1", Timestamp: int64(i)}
	}

	select {
	case <-fast:
	case <-ctx.Done():
		t.Fatal("fast subscriber blocked by slow subscriber")
	}
}

func TestHubUnsubscribeClosesChannel(t *testing.T) {
	ticks := make(chan models.Tick, 1)
	h := hub.New(ticks)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go h.Run(ctx)

	sub := h.Subscribe()
	h.Unsubscribe(sub)

	select {
	case _, ok := <-sub:
		if ok {
			t.Error("expected closed channel, got open channel with value")
		}
	case <-ctx.Done():
		t.Fatal("timeout: channel was not closed")
	}
}
