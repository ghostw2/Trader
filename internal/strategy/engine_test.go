package strategy_test

import (
	"testing"

	"github.com/menribardhi/trader/internal/strategy"
)

func TestNoSignalBeforeWindow(t *testing.T) {
	eng := strategy.NewEngine(nil)
	ch := eng.Subscribe()
	defer eng.Unsubscribe(ch)

	for i := 0; i < 49; i++ {
		eng.ProcessPrice(100.0)
		sig := <-ch
		if sig.Side != "HOLD" {
			t.Fatalf("price %d: expected HOLD before window full, got %s", i+1, sig.Side)
		}
	}
}

// TestBuySignalOnCrossoverUp uses a three-phase price sequence that produces a
// BUY crossover with RSI≈66.7 (< 70) and price (200) above EMA (≈174):
//   - 50×200: fills window (initialized guard fires, all HOLD)
//   - 10×100: fast SMA(10) falls below slow SMA(50); prevFast=100, prevSlow=180
//   - 8×200: partial recovery; at the 8th price fast==slow==180, no crossover
//   - 9th×200: fast(190) crosses above slow(180) → BUY if filters pass
func TestBuySignalOnCrossoverUp(t *testing.T) {
	eng := strategy.NewEngine(nil)
	ch := eng.Subscribe()
	defer eng.Unsubscribe(ch)

	for i := 0; i < 50; i++ {
		eng.ProcessPrice(200.0)
		<-ch
	}
	for i := 0; i < 10; i++ {
		eng.ProcessPrice(100.0)
		<-ch
	}
	for i := 0; i < 8; i++ {
		eng.ProcessPrice(200.0)
		<-ch
	}
	eng.ProcessPrice(200.0)
	sig := <-ch
	if sig.Side != "BUY" {
		t.Errorf("expected BUY on filtered crossover up, got %s (RSI=%.1f EMA=%.1f Price=%.1f)",
			sig.Side, sig.RSI, sig.EMA, sig.Price)
	}
}

// TestSellSignalOnCrossoverDown uses a three-phase sequence that produces a
// SELL crossover with RSI≈30.3 (> 30) and price (90) below EMA (≈122):
//   - 50×100: fills window
//   - 10×200: fast rises above slow; prevFast=200, prevSlow=120
//   - 7×90: partial drop; at 7th price fast(123)>slow(118.6), no crossover yet
//   - 8th×90: fast(112) crosses below slow(118.4) → SELL if filters pass
func TestSellSignalOnCrossoverDown(t *testing.T) {
	eng := strategy.NewEngine(nil)
	ch := eng.Subscribe()
	defer eng.Unsubscribe(ch)

	for i := 0; i < 50; i++ {
		eng.ProcessPrice(100.0)
		<-ch
	}
	for i := 0; i < 10; i++ {
		eng.ProcessPrice(200.0)
		<-ch
	}
	for i := 0; i < 7; i++ {
		eng.ProcessPrice(90.0)
		<-ch
	}
	eng.ProcessPrice(90.0)
	sig := <-ch
	if sig.Side != "SELL" {
		t.Errorf("expected SELL on filtered crossover down, got %s (RSI=%.1f EMA=%.1f Price=%.1f)",
			sig.Side, sig.RSI, sig.EMA, sig.Price)
	}
}

// TestBuyBlockedByFilter reuses the old spike pattern (50×10 then 200).
// The crossover fires (fast=29 > slow=13.8) but RSI=100 blocks the BUY.
func TestBuyBlockedByFilter(t *testing.T) {
	eng := strategy.NewEngine(nil)
	ch := eng.Subscribe()
	defer eng.Unsubscribe(ch)

	for i := 0; i < 50; i++ {
		eng.ProcessPrice(10.0)
		<-ch
	}
	eng.ProcessPrice(200.0)
	sig := <-ch
	if sig.Side != "HOLD" {
		t.Errorf("expected HOLD when RSI=100 blocks BUY crossover, got %s (RSI=%.1f)", sig.Side, sig.RSI)
	}
}
