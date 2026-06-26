package strategy_test

import (
	"testing"

	"github.com/menribardhi/trader/internal/strategy"
)

func TestBacktestNoTrades(t *testing.T) {
	bt := strategy.NewBacktester()
	closes := make([]float64, 100)
	for i := range closes {
		closes[i] = 50000.0
	}
	result := bt.Run(closes)
	if result.TotalTrades != 0 {
		t.Errorf("expected 0 trades on flat prices, got %d", result.TotalTrades)
	}
	if result.FinalValue != 10000.0 {
		t.Errorf("final value: got %v, want 10000.0", result.FinalValue)
	}
	if len(result.Trades) != 0 {
		t.Errorf("trades slice should be empty, got %d elements", len(result.Trades))
	}
}

// TestBacktestFilterBlocksCrossover verifies that the old spike pattern now
// produces 0 trades because RSI=100 blocks the BUY crossover.
func TestBacktestFilterBlocksCrossover(t *testing.T) {
	bt := strategy.NewBacktester()
	closes := make([]float64, 150)
	for i := 0; i < 50; i++ {
		closes[i] = 10.0
	}
	for i := 50; i < 100; i++ {
		closes[i] = 200.0
	}
	for i := 100; i < 150; i++ {
		closes[i] = 10.0
	}
	result := bt.Run(closes)
	if result.TotalTrades != 0 {
		t.Errorf("expected 0 trades when filter blocks spike crossover, got %d", result.TotalTrades)
	}
}

// TestBacktestBuyThenSell uses a 74-price sequence where both the BUY and SELL
// crossovers satisfy the RSI and EMA filters.
// BUY at index 68 ($200): RSI≈66.7<70, price>EMA≈174
// SELL at index 73 ($100): RSI≈38.8>30, price<EMA≈158
func TestBacktestBuyThenSell(t *testing.T) {
	bt := strategy.NewBacktester()
	closes := make([]float64, 74)
	for i := 0; i < 50; i++ {
		closes[i] = 200.0
	}
	for i := 50; i < 60; i++ {
		closes[i] = 100.0
	}
	for i := 60; i < 69; i++ {
		closes[i] = 200.0
	}
	closes[69] = 300.0
	for i := 70; i < 74; i++ {
		closes[i] = 100.0
	}

	result := bt.Run(closes)
	if result.TotalTrades < 2 {
		t.Fatalf("expected at least 2 trades (BUY+SELL), got %d", result.TotalTrades)
	}
	if result.Trades[0].Side != "BUY" {
		t.Errorf("first trade should be BUY, got %s", result.Trades[0].Side)
	}
	hasSell := false
	for _, tr := range result.Trades {
		if tr.Side == "SELL" {
			hasSell = true
			break
		}
	}
	if !hasSell {
		t.Error("expected at least one SELL trade")
	}
}
