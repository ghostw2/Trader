# RSI+EMA Confirmation Filter Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add RSI<70 and price>EMA guards to BUY, and RSI>30 and price<EMA guards to SELL, so crossovers that fail the filter emit HOLD instead.

**Architecture:** Four lines change in `engine.go` and four lines change in `backtester.go`. Both files already compute `rsi`, `ema`, and `price` before the crossover switch — the change is adding `&& rsi < 70 && price > ema` (BUY) and `&& rsi > 30 && price < ema` (SELL) to the existing case conditions. Test sequences are redesigned because the old spike pattern produces RSI=100 which blocks every BUY.

**Tech Stack:** Go 1.24, `internal/strategy` package, `go test ./internal/strategy/...`

## Global Constraints

- Module: `github.com/menribardhi/trader`
- No new files, no new exported types, no frontend changes, no API changes
- BUY filter: `rsi < 70 && price > ema` — both conditions required
- SELL filter: `rsi > 30 && price < ema` — both conditions required
- A blocked crossover emits HOLD; `fastPrev`/`slowPrev` are still updated normally
- Thresholds (70, 30) are hardcoded inline — no new constants

---

### Task 1: Apply RSI+EMA filter to engine.go and update engine_test.go

**Files:**
- Modify: `internal/strategy/engine.go` (lines 126–130, the BUY/SELL cases)
- Modify: `internal/strategy/engine_test.go` (all three existing tests + one new test)

**Interfaces:**
- Consumes: `rsi`, `ema`, `price` — already computed before the switch in `ProcessPrice`; no new locals needed
- Produces: nothing new; same `models.Signal` with updated `Side` logic

---

- [ ] **Step 1: Write the failing tests**

Replace `internal/strategy/engine_test.go` entirely:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/strategy/... -run "TestBuySignalOnCrossoverUp|TestSellSignalOnCrossoverDown|TestBuyBlockedByFilter" -v
```

Expected: `TestBuySignalOnCrossoverUp` FAIL (sequence produces HOLD, not BUY, because filter not yet applied). `TestSellSignalOnCrossoverDown` FAIL (same reason). `TestBuyBlockedByFilter` FAIL (currently returns BUY, not HOLD).

- [ ] **Step 3: Apply the filter to engine.go**

In `internal/strategy/engine.go`, change the two crossover cases in `ProcessPrice` (around line 126):

```go
	case oldFastPrev <= oldSlowPrev && fastNow > slowNow && rsi < 70 && price > ema:
		sig.Side = "BUY"
	case oldFastPrev >= oldSlowPrev && fastNow < slowNow && rsi > 30 && price < ema:
		sig.Side = "SELL"
```

The variables `rsi`, `ema`, and `price` are already in scope above the switch (computed at lines 95–97 and the function parameter).

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/strategy/... -v
```

Expected: all four engine tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/strategy/engine.go internal/strategy/engine_test.go
git commit -m "feat: add RSI+EMA confirmation filter to strategy engine"
```

---

### Task 2: Apply RSI+EMA filter to backtester.go and update backtester_test.go

**Files:**
- Modify: `internal/strategy/backtester.go` (add EMA/RSI locals + filter conditions)
- Modify: `internal/strategy/backtester_test.go` (redesign BuyThenSell + add FilterBlocks test)

**Interfaces:**
- Consumes: `EMA(window, emaPeriod)`, `RSI(window, rsiPeriod)` — package-level functions and constants already available; `emaPeriod=20`, `rsiPeriod=14`
- Produces: `BacktestSummary` with fewer trades when filter blocks crossovers

**Price sequence math for TestBacktestBuyThenSell (74 prices):**

- Indices 0–49: `200.0` — fills window; initialized guard fires
- Indices 50–59: `100.0` — fast(100) < slow(180); prevFast=100, prevSlow=180
- Indices 60–67: `200.0` — 8 prices; at idx 67: fast=180, slow=180 (tied, no crossover)
- Index 68: `200.0` — 9th×200: fast(190) > slow(180), prev(100)≤prev(180) → **BUY** at $200; RSI≈66.7<70; price>EMA≈174
- Index 69: `300.0` — fast(210)>slow(182); RSI rises; no new crossover
- Indices 70–72: `100.0` — fast drops; at idx 72: fast(180)>slow(176), no crossover
- Index 73: `100.0` — 4th×100: fast(170) < slow(174), prev(180)≥prev(176) → **SELL** at $100; RSI≈38.8>30; price<EMA≈158

---

- [ ] **Step 1: Write the failing tests**

Replace `internal/strategy/backtester_test.go` entirely:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/strategy/... -run "TestBacktestFilterBlocksCrossover|TestBacktestBuyThenSell" -v
```

Expected: `TestBacktestFilterBlocksCrossover` FAIL (old spike sequence currently produces trades). `TestBacktestBuyThenSell` may FAIL because filter not yet applied to backtester.

- [ ] **Step 3: Apply the filter to backtester.go**

In `internal/strategy/backtester.go`, after the `fastNow`/`slowNow` lines (around line 38), add EMA and RSI locals and extend the case conditions:

```go
		fastNow := SMA(window, fastPeriod)
		slowNow := SMA(window, slowPeriod)
		ema := EMA(window, emaPeriod)
		rsi := RSI(window, rsiPeriod)

		if initialized {
			switch {
			case fastPrev <= slowPrev && fastNow > slowNow && cash > 0 && rsi < 70 && price > ema:
				btc = cash / price
				cash = 0
				trades = append(trades, models.BacktestTrade{Side: "BUY", Price: price, Time: int64(i)})
			case fastPrev >= slowPrev && fastNow < slowNow && btc > 0 && rsi > 30 && price < ema:
				cash = btc * price
				btc = 0
				trades = append(trades, models.BacktestTrade{Side: "SELL", Price: price, Time: int64(i)})
			}
		}
```

- [ ] **Step 4: Run all strategy tests**

```bash
go test ./internal/strategy/... -v
```

Expected: all 7 tests PASS:
- `TestNoSignalBeforeWindow`
- `TestBuySignalOnCrossoverUp`
- `TestSellSignalOnCrossoverDown`
- `TestBuyBlockedByFilter`
- `TestBacktestNoTrades`
- `TestBacktestFilterBlocksCrossover`
- `TestBacktestBuyThenSell`

- [ ] **Step 5: Run the full suite to check for regressions**

```bash
go test ./...
```

Expected: all tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/strategy/backtester.go internal/strategy/backtester_test.go
git commit -m "feat: apply RSI+EMA confirmation filter to backtester"
```
