package strategy_test

import (
	"math"
	"testing"

	"github.com/menribardhi/trader/internal/strategy"
)

func TestSMA(t *testing.T) {
	prices := []float64{10, 20, 30, 40, 50}
	got := strategy.SMA(prices, 3)
	if got != 40.0 {
		t.Errorf("SMA: got %v, want 40.0", got)
	}
}

func TestSMAInsufficientData(t *testing.T) {
	got := strategy.SMA([]float64{1, 2}, 5)
	if got != 0 {
		t.Errorf("SMA insufficient: got %v, want 0", got)
	}
}

func TestEMA(t *testing.T) {
	// seed = (10+20+30)/3 = 20; k=0.5; EMA = 40*0.5 + 20*0.5 = 30
	prices := []float64{10, 20, 30, 40}
	got := strategy.EMA(prices, 3)
	if got != 30.0 {
		t.Errorf("EMA: got %v, want 30.0", got)
	}
}

func TestEMAInsufficientData(t *testing.T) {
	got := strategy.EMA([]float64{1, 2}, 5)
	if got != 0 {
		t.Errorf("EMA insufficient: got %v, want 0", got)
	}
}

func TestRSIAllGains(t *testing.T) {
	// All gains -> RSI = 100
	prices := []float64{100, 110, 120}
	got := strategy.RSI(prices, 1)
	if got != 100.0 {
		t.Errorf("RSI all gains: got %v, want 100.0", got)
	}
}

func TestRSIAllLosses(t *testing.T) {
	// All losses -> RSI = 0
	prices := []float64{120, 110, 100}
	got := strategy.RSI(prices, 1)
	if got != 0.0 {
		t.Errorf("RSI all losses: got %v, want 0.0", got)
	}
}

func TestRSIMixed(t *testing.T) {
	// [100,110,100], period=2: avgGain=(10+0)/2=5, avgLoss=(0+10)/2=5 -> RS=1 -> RSI=50
	prices := []float64{100, 110, 100}
	got := strategy.RSI(prices, 2)
	if math.Abs(got-50.0) > 1e-9 {
		t.Errorf("RSI mixed: got %v, want 50.0", got)
	}
}

func TestRSIInsufficientData(t *testing.T) {
	got := strategy.RSI([]float64{100}, 2) // need period+1=3 prices
	if got != 0 {
		t.Errorf("RSI insufficient: got %v, want 0", got)
	}
}
