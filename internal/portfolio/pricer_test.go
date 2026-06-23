package portfolio_test

import (
	"context"
	"testing"
	"time"

	"github.com/menribardhi/trader/internal/hub"
	"github.com/menribardhi/trader/internal/models"
	"github.com/menribardhi/trader/internal/portfolio"
)

func setupFeedTest(t *testing.T) (*portfolio.PriceFeed, chan models.Tick, context.CancelFunc) {
	t.Helper()
	ticks := make(chan models.Tick, 4)
	h := hub.New(ticks)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	go h.Run(ctx)
	feed := portfolio.NewPriceFeed(h)
	go feed.Run(ctx)
	time.Sleep(10 * time.Millisecond)
	return feed, ticks, cancel
}

func TestPriceFeedNoPrice(t *testing.T) {
	feed, _, cancel := setupFeedTest(t)
	defer cancel()

	_, hasPrice := feed.Latest()
	if hasPrice {
		t.Error("should have no price before any tick")
	}
}

func TestPriceFeedReceivesTick(t *testing.T) {
	feed, ticks, cancel := setupFeedTest(t)
	defer cancel()

	ticks <- models.Tick{Symbol: "BTCUSDT", Price: "62000.00", Timestamp: 1}
	time.Sleep(50 * time.Millisecond)

	price, hasPrice := feed.Latest()
	if !hasPrice {
		t.Error("should have price after tick")
	}
	if price != 62000.0 {
		t.Errorf("price: got %v, want 62000.0", price)
	}
}

func TestPriceFeedUpdatesToLatest(t *testing.T) {
	feed, ticks, cancel := setupFeedTest(t)
	defer cancel()

	ticks <- models.Tick{Symbol: "BTCUSDT", Price: "62000.00", Timestamp: 1}
	time.Sleep(50 * time.Millisecond)
	ticks <- models.Tick{Symbol: "BTCUSDT", Price: "63000.00", Timestamp: 2}
	time.Sleep(50 * time.Millisecond)

	price, _ := feed.Latest()
	if price != 63000.0 {
		t.Errorf("price: got %v, want 63000.0 (should be latest)", price)
	}
}
