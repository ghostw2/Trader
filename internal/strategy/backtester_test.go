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

func TestBacktestBuyThenSell(t *testing.T) {
	bt := strategy.NewBacktester()
	// 50 prices at 10 -> 50 at 200 -> 50 at 10
	// BUY fires when fast crosses above slow (at first 200 price)
	// SELL fires when fast crosses below slow (at first 10 price in final phase)
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
