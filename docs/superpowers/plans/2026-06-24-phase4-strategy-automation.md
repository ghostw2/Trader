# Phase 4 — Strategy Automation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add SMA crossover signal engine + backtester to the crypto paper trading app, with a live SSE feed of signals and a Strategy tab in the React frontend.

**Architecture:** A shared `internal/strategy/` package provides pure indicator functions (SMA, EMA, RSI) consumed by both a live `Engine` (subscribes to the hub, fans signals to SSE clients) and a `Backtester` (replays historical Binance klines synchronously). Two new REST endpoints surface both capabilities; a new Strategy tab in the React app shows live signals and runs backtests on demand.

**Tech Stack:** Go 1.24, Chi router, `sync.RWMutex`, Server-Sent Events (native browser `EventSource`), Binance public klines REST API, React 19 + TypeScript + Vite.

## Global Constraints

- Module: `github.com/menribardhi/trader`
- Go 1.24; `go test ./...` must pass after every task
- Signal sides are exactly `"BUY"`, `"SELL"`, `"HOLD"` (uppercase strings)
- SMA fast period: `10`, slow period: `50`, EMA period: `20`, RSI period: `14`
- Backtester starts with `$10,000` cash; BUY allocates all cash, SELL liquidates all BTC
- Binance klines URL: `https://api.binance.com/api/v3/klines?symbol=BTCUSDT&interval=5m&limit=8640`
- SSE per-client buffer capacity: `4`; slow clients dropped with non-blocking send
- `api.New` final signature: `(h *hub.Hub, sqldb *sql.DB, feed *portfolio.PriceFeed, eng *strategy.Engine) *Server`
- TypeScript: `npx tsc --noEmit` and `npm run build` must pass after Task 5
- No auto-execution of trades from signals — Strategy tab is display-only

---

### Task 1: Models + Indicators (TDD)

**Files:**
- Modify: `internal/models/types.go`
- Create: `internal/strategy/indicators.go`
- Create: `internal/strategy/indicators_test.go`

**Interfaces:**
- Consumes: nothing from prior phases (pure functions)
- Produces:
  - `models.Signal{Side string; SMAFast, SMASlow, EMA, RSI, Price float64; Timestamp int64}`
  - `models.BacktestTrade{Side string; Price float64; Time int64}`
  - `models.BacktestSummary{Trades []BacktestTrade; TotalTrades int; FinalValue, ReturnPct float64}`
  - `strategy.SMA(prices []float64, period int) float64`
  - `strategy.EMA(prices []float64, period int) float64`
  - `strategy.RSI(prices []float64, period int) float64`

- [ ] **Step 1: Write the failing tests**

```go
// internal/strategy/indicators_test.go
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
```

- [ ] **Step 2: Run tests — verify they fail**

```
go test ./internal/strategy/... -v
```
Expected: FAIL — `strategy` package does not exist yet.

- [ ] **Step 3: Add models to `internal/models/types.go`**

Append to the end of the existing file:

```go
type Signal struct {
	Side      string  `json:"side"`      // "BUY", "SELL", or "HOLD"
	SMAFast   float64 `json:"sma_fast"`  // SMA(10)
	SMASlow   float64 `json:"sma_slow"`  // SMA(50)
	EMA       float64 `json:"ema"`       // EMA(20)
	RSI       float64 `json:"rsi"`       // RSI(14)
	Price     float64 `json:"price"`
	Timestamp int64   `json:"timestamp"` // Unix ms
}

type BacktestTrade struct {
	Side  string  `json:"side"`  // "BUY" or "SELL"
	Price float64 `json:"price"`
	Time  int64   `json:"time"`  // kline index (not real time)
}

type BacktestSummary struct {
	Trades      []BacktestTrade `json:"trades"`
	TotalTrades int             `json:"total_trades"`
	FinalValue  float64         `json:"final_value"` // starting $10,000 +/- simulated P&L
	ReturnPct   float64         `json:"return_pct"`  // (FinalValue-10000)/10000*100
}
```

- [ ] **Step 4: Create `internal/strategy/indicators.go`**

```go
package strategy

// SMA returns the simple moving average of the last period values.
// Returns 0 if len(prices) < period.
func SMA(prices []float64, period int) float64 {
	if len(prices) < period {
		return 0
	}
	window := prices[len(prices)-period:]
	var sum float64
	for _, p := range window {
		sum += p
	}
	return sum / float64(period)
}

// EMA returns the exponential moving average over all prices.
// Seed is the SMA of the first period values; subsequent values apply
// multiplier k = 2/(period+1). Returns 0 if len(prices) < period.
func EMA(prices []float64, period int) float64 {
	if len(prices) < period {
		return 0
	}
	var seed float64
	for i := 0; i < period; i++ {
		seed += prices[i]
	}
	ema := seed / float64(period)
	k := 2.0 / float64(period+1)
	for i := period; i < len(prices); i++ {
		ema = prices[i]*k + ema*(1-k)
	}
	return ema
}

// RSI returns the Relative Strength Index using Wilder's smoothing.
// Returns 0 if len(prices) < period+1.
func RSI(prices []float64, period int) float64 {
	if len(prices) < period+1 {
		return 0
	}
	changes := make([]float64, len(prices)-1)
	for i := 1; i < len(prices); i++ {
		changes[i-1] = prices[i] - prices[i-1]
	}
	if len(changes) < period {
		return 0
	}
	var avgGain, avgLoss float64
	for i := 0; i < period; i++ {
		if changes[i] > 0 {
			avgGain += changes[i]
		} else {
			avgLoss += -changes[i]
		}
	}
	avgGain /= float64(period)
	avgLoss /= float64(period)
	for i := period; i < len(changes); i++ {
		gain, loss := 0.0, 0.0
		if changes[i] > 0 {
			gain = changes[i]
		} else {
			loss = -changes[i]
		}
		avgGain = (avgGain*float64(period-1) + gain) / float64(period)
		avgLoss = (avgLoss*float64(period-1) + loss) / float64(period)
	}
	if avgLoss == 0 {
		return 100
	}
	rs := avgGain / avgLoss
	return 100 - 100/(1+rs)
}
```

- [ ] **Step 5: Run tests — verify they pass**

```
go test ./internal/strategy/... -v
```
Expected: PASS — 8 tests.

- [ ] **Step 6: Run full suite**

```
go test ./...
```
Expected: all packages pass.

- [ ] **Step 7: Commit**

```
git add internal/models/types.go internal/strategy/
git commit -m "feat: add Signal/BacktestSummary models and SMA/EMA/RSI indicators"
```

---

### Task 2: Strategy Engine (TDD)

**Files:**
- Create: `internal/strategy/engine.go`
- Create: `internal/strategy/engine_test.go`

**Interfaces:**
- Consumes:
  - `hub.Hub.Subscribe() chan models.Tick` / `hub.Unsubscribe(ch)`
  - `strategy.SMA`, `strategy.EMA`, `strategy.RSI` (from Task 1)
  - `models.Signal` (from Task 1)
- Produces:
  - `strategy.NewEngine(h *hub.Hub) *Engine`
  - `(*Engine).Run(ctx context.Context)` — blocks until ctx cancelled; nil hub is safe (Run returns immediately)
  - `(*Engine).Subscribe() chan models.Signal` — registers SSE client; buffer 4
  - `(*Engine).Unsubscribe(ch chan models.Signal)` — removes client, closes channel
  - `(*Engine).ProcessPrice(price float64)` — exported for testing; updates window and broadcasts signal

**Crossover logic (constants defined in engine.go):**
```
fastPeriod = 10, slowPeriod = 50, emaPeriod = 20, rsiPeriod = 14
```
- `fastPrev <= slowPrev && fastNow > slowNow` -> `"BUY"`
- `fastPrev >= slowPrev && fastNow < slowNow` -> `"SELL"`
- otherwise or `len(window) < slowPeriod` -> `"HOLD"`

- [ ] **Step 1: Write failing tests**

```go
// internal/strategy/engine_test.go
package strategy_test

import (
	"testing"

	"github.com/menribardhi/trader/internal/strategy"
)

func TestNoSignalBeforeWindow(t *testing.T) {
	eng := strategy.NewEngine(nil)
	ch := eng.Subscribe()
	defer eng.Unsubscribe(ch)

	// 49 prices = one short of slowPeriod; all must be HOLD
	for i := 0; i < 49; i++ {
		eng.ProcessPrice(100.0)
		sig := <-ch
		if sig.Side != "HOLD" {
			t.Fatalf("price %d: expected HOLD before window full, got %s", i+1, sig.Side)
		}
	}
}

func TestBuySignalOnCrossoverUp(t *testing.T) {
	eng := strategy.NewEngine(nil)
	ch := eng.Subscribe()
	defer eng.Unsubscribe(ch)

	// Fill window with 50 flat prices at 10 -> fastPrev=slowPrev=10
	for i := 0; i < 50; i++ {
		eng.ProcessPrice(10.0)
		<-ch // drain
	}
	// Add price at 200:
	//   window=[10x49, 200]; SMA(10)=(10x9+200)/10=29; SMA(50)=(10x49+200)/50=13.8
	//   fastPrev(10) <= slowPrev(10) && fastNow(29) > slowNow(13.8) -> BUY
	eng.ProcessPrice(200.0)
	sig := <-ch
	if sig.Side != "BUY" {
		t.Errorf("expected BUY on crossover up, got %s (SMAFast=%v SMASlow=%v)", sig.Side, sig.SMAFast, sig.SMASlow)
	}
}

func TestSellSignalOnCrossoverDown(t *testing.T) {
	eng := strategy.NewEngine(nil)
	ch := eng.Subscribe()
	defer eng.Unsubscribe(ch)

	// Fill window with 50 flat prices at 200 -> fastPrev=slowPrev=200
	for i := 0; i < 50; i++ {
		eng.ProcessPrice(200.0)
		<-ch // drain
	}
	// Add price at 10:
	//   window=[200x49, 10]; SMA(10)=(200x9+10)/10=181; SMA(50)=(200x49+10)/50=198.2
	//   fastPrev(200) >= slowPrev(200) && fastNow(181) < slowNow(198.2) -> SELL
	eng.ProcessPrice(10.0)
	sig := <-ch
	if sig.Side != "SELL" {
		t.Errorf("expected SELL on crossover down, got %s (SMAFast=%v SMASlow=%v)", sig.Side, sig.SMAFast, sig.SMASlow)
	}
}
```

- [ ] **Step 2: Run tests — verify they fail**

```
go test ./internal/strategy/... -run TestNoSignal -v
go test ./internal/strategy/... -run TestBuySignal -v
go test ./internal/strategy/... -run TestSellSignal -v
```
Expected: FAIL — `NewEngine` undefined.

- [ ] **Step 3: Create `internal/strategy/engine.go`**

```go
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
```

- [ ] **Step 4: Run tests — verify they pass**

```
go test ./internal/strategy/... -v
```
Expected: PASS — 11 tests (8 from Task 1 + 3 new).

- [ ] **Step 5: Run full suite**

```
go test ./...
```
Expected: all packages pass.

- [ ] **Step 6: Commit**

```
git add internal/strategy/engine.go internal/strategy/engine_test.go
git commit -m "feat: add strategy Engine — SMA crossover signals via SSE fan-out"
```

---

### Task 3: Klines Fetcher + Backtester (TDD)

**Files:**
- Create: `internal/binance/klines.go`
- Create: `internal/binance/klines_test.go`
- Create: `internal/strategy/backtester.go`
- Create: `internal/strategy/backtester_test.go`

**Interfaces:**
- Consumes:
  - `strategy.SMA` (from Task 1)
  - `models.BacktestTrade`, `models.BacktestSummary` (from Task 1)
- Produces:
  - `binance.FetchKlines(symbol, interval string, limit int) ([]float64, error)` — calls real Binance
  - `binance.FetchKlinesFromURL(url string) ([]float64, error)` — exported for test injection
  - `strategy.NewBacktester() *Backtester`
  - `(*Backtester).Run(closes []float64) models.BacktestSummary`

**Kline response format:** `[ [openTime, open, high, low, close, ...], ... ]` — index 4 is close price string.

- [ ] **Step 1: Write failing klines test**

```go
// internal/binance/klines_test.go
package binance_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/menribardhi/trader/internal/binance"
)

func TestFetchKlines(t *testing.T) {
	// Binance klines: array of arrays; index 4 is close price string
	raw := [][]any{
		{0, "1", "2", "3", "50000.00", 0, 0, 0, 0, 0, 0, 0},
		{0, "1", "2", "3", "51000.50", 0, 0, 0, 0, 0, 0, 0},
		{0, "1", "2", "3", "49000.25", 0, 0, 0, 0, 0, 0, 0},
	}
	body, _ := json.Marshal(raw)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	defer srv.Close()

	closes, err := binance.FetchKlinesFromURL(srv.URL)
	if err != nil {
		t.Fatalf("FetchKlinesFromURL: %v", err)
	}
	if len(closes) != 3 {
		t.Fatalf("expected 3 closes, got %d", len(closes))
	}
	if closes[0] != 50000.0 {
		t.Errorf("closes[0]: got %v, want 50000.0", closes[0])
	}
	if closes[1] != 51000.5 {
		t.Errorf("closes[1]: got %v, want 51000.5", closes[1])
	}
	if closes[2] != 49000.25 {
		t.Errorf("closes[2]: got %v, want 49000.25", closes[2])
	}
}
```

- [ ] **Step 2: Write failing backtester tests**

```go
// internal/strategy/backtester_test.go
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
```

- [ ] **Step 3: Run tests — verify they fail**

```
go test ./internal/binance/... -run TestFetchKlines -v
go test ./internal/strategy/... -run TestBacktest -v
```
Expected: FAIL — `FetchKlinesFromURL` and `NewBacktester` undefined.

- [ ] **Step 4: Create `internal/binance/klines.go`**

```go
package binance

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

const binanceKlinesBase = "https://api.binance.com/api/v3/klines"

// FetchKlines fetches close prices from the Binance public klines REST API.
// No API key required.
func FetchKlines(symbol, interval string, limit int) ([]float64, error) {
	url := fmt.Sprintf("%s?symbol=%s&interval=%s&limit=%d", binanceKlinesBase, symbol, interval, limit)
	return FetchKlinesFromURL(url)
}

// FetchKlinesFromURL fetches close prices from an arbitrary klines URL.
// Exported for testing with a mock HTTP server.
func FetchKlinesFromURL(url string) ([]float64, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("klines request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("klines: unexpected status %d", resp.StatusCode)
	}
	var raw [][]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("klines decode: %w", err)
	}
	closes := make([]float64, 0, len(raw))
	for _, k := range raw {
		if len(k) < 5 {
			continue
		}
		var s string
		if err := json.Unmarshal(k[4], &s); err != nil {
			continue
		}
		price, err := strconv.ParseFloat(s, 64)
		if err != nil {
			continue
		}
		closes = append(closes, price)
	}
	return closes, nil
}
```

- [ ] **Step 5: Create `internal/strategy/backtester.go`**

```go
package strategy

import "github.com/menribardhi/trader/internal/models"

const startingCash = 10000.0

// Backtester replays a slice of close prices through the SMA crossover strategy
// and returns a simulated P&L summary.
type Backtester struct{}

func NewBacktester() *Backtester { return &Backtester{} }

// Run simulates paper trading on closes. BUY allocates all cash; SELL liquidates
// all BTC. Returns a BacktestSummary with all executed trades and final value.
func (b *Backtester) Run(closes []float64) models.BacktestSummary {
	cash := startingCash
	btc := 0.0
	var trades []models.BacktestTrade
	var fastPrev, slowPrev float64
	initialized := false

	for i, price := range closes {
		window := closes[:i+1]
		if len(window) > slowPeriod {
			window = window[len(window)-slowPeriod:]
		}
		if len(window) < slowPeriod {
			continue
		}

		fastNow := SMA(window, fastPeriod)
		slowNow := SMA(window, slowPeriod)

		if initialized {
			switch {
			case fastPrev <= slowPrev && fastNow > slowNow && cash > 0:
				btc = cash / price
				cash = 0
				trades = append(trades, models.BacktestTrade{Side: "BUY", Price: price, Time: int64(i)})
			case fastPrev >= slowPrev && fastNow < slowNow && btc > 0:
				cash = btc * price
				btc = 0
				trades = append(trades, models.BacktestTrade{Side: "SELL", Price: price, Time: int64(i)})
			}
		}

		fastPrev = fastNow
		slowPrev = slowNow
		initialized = true
	}

	finalValue := cash + btc*closes[len(closes)-1]
	returnPct := (finalValue - startingCash) / startingCash * 100

	if trades == nil {
		trades = []models.BacktestTrade{}
	}
	return models.BacktestSummary{
		Trades:      trades,
		TotalTrades: len(trades),
		FinalValue:  finalValue,
		ReturnPct:   returnPct,
	}
}
```

- [ ] **Step 6: Run tests — verify they pass**

```
go test ./internal/binance/... -run TestFetchKlines -v
go test ./internal/strategy/... -v
```
Expected: PASS — all tests including 2 new backtester tests.

- [ ] **Step 7: Run full suite**

```
go test ./...
```
Expected: all packages pass.

- [ ] **Step 8: Commit**

```
git add internal/binance/klines.go internal/binance/klines_test.go \
        internal/strategy/backtester.go internal/strategy/backtester_test.go
git commit -m "feat: add klines fetcher and backtester — SMA crossover replay on historical data"
```

---

### Task 4: Strategy REST API + Wire main.go (TDD)

**Files:**
- Create: `internal/api/strategy_handler.go`
- Create: `internal/api/strategy_handler_test.go`
- Modify: `internal/api/server.go` — 4-arg `New`, `eng` field, `klinesURL` field, `SetKlinesURL`, 2 new routes
- Modify: `internal/api/ws_handler_test.go:25` — `api.New(h, nil, nil)` -> `api.New(h, nil, nil, nil)`
- Modify: `internal/api/alerts_handler_test.go:25` — `api.New(h, sqldb, nil)` -> `api.New(h, sqldb, nil, nil)`
- Modify: `internal/api/portfolio_handler_test.go:39` — `api.New(h, sqldb, feed)` -> `api.New(h, sqldb, feed, nil)`
- Modify: `cmd/trader/main.go` — add `strategy.NewEngine`, `go eng.Run(ctx)`, 4-arg `api.New`

**Interfaces:**
- Consumes:
  - `strategy.NewEngine`, `(*Engine).Subscribe`, `(*Engine).Unsubscribe` (Task 2)
  - `strategy.NewBacktester`, `(*Backtester).Run` (Task 3)
  - `binance.FetchKlinesFromURL` (Task 3)
  - `models.Signal`, `models.BacktestSummary` (Task 1)
- Produces:
  - `GET /api/strategy/signals` — SSE stream of `Signal` JSON, one `data:` line per tick
  - `POST /api/strategy/backtest` — 200 `BacktestSummary` JSON; 503 if eng nil or fetch fails
  - `(*Server).SetKlinesURL(url string)` — exported for test injection

**SSE format:** Each event is `data: <JSON>\n\n`. Flush after each event. Disconnect detected via `r.Context().Done()`.

- [ ] **Step 1: Write failing strategy handler tests**

```go
// internal/api/strategy_handler_test.go
package api_test

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/menribardhi/trader/internal/api"
	"github.com/menribardhi/trader/internal/models"
	"github.com/menribardhi/trader/internal/strategy"
)

func TestBacktestEndpoint(t *testing.T) {
	// Mock Binance klines: 60 flat prices at 50000 -> no crossover -> 0 trades
	raw := make([][]any, 60)
	for i := range raw {
		raw[i] = []any{0, "1", "2", "3", "50000.00", 0, 0, 0, 0, 0, 0, 0}
	}
	body, _ := json.Marshal(raw)
	klinesSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	defer klinesSrv.Close()

	eng := strategy.NewEngine(nil)
	s := api.New(nil, nil, nil, eng)
	s.SetKlinesURL(klinesSrv.URL)
	srv := httptest.NewServer(s)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/api/strategy/backtest", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	var result models.BacktestSummary
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.TotalTrades != 0 {
		t.Errorf("expected 0 trades on flat prices, got %d", result.TotalTrades)
	}
	if result.FinalValue != 10000.0 {
		t.Errorf("final value: got %v, want 10000.0", result.FinalValue)
	}
}

func TestBacktestEndpointNilEngine(t *testing.T) {
	srv := httptest.NewServer(api.New(nil, nil, nil, nil))
	defer srv.Close()

	resp, _ := http.Post(srv.URL+"/api/strategy/backtest", "", nil)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503 with nil engine, got %d", resp.StatusCode)
	}
}

func TestSignalsSSE(t *testing.T) {
	eng := strategy.NewEngine(nil)
	srv := httptest.NewServer(api.New(nil, nil, nil, eng))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/strategy/signals")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type: got %q, want text/event-stream", ct)
	}

	// Trigger a signal after SSE handler has time to subscribe
	go func() {
		time.Sleep(10 * time.Millisecond)
		eng.ProcessPrice(50000.0)
	}()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			var sig models.Signal
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &sig); err != nil {
				t.Errorf("could not parse SSE data as Signal: %v -- raw: %q", err, line)
			}
			return // success
		}
	}
	t.Error("no SSE data line received before body closed")
}

func TestSignalsSSENilEngine(t *testing.T) {
	srv := httptest.NewServer(api.New(nil, nil, nil, nil))
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/api/strategy/signals")
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503 with nil engine, got %d", resp.StatusCode)
	}
}

// Suppress unused import warning from fmt
var _ = fmt.Sprintf
```

- [ ] **Step 2: Run tests — verify they fail**

```
go test ./internal/api/... -run TestBacktest -v
go test ./internal/api/... -run TestSignals -v
```
Expected: FAIL — `api.New` still takes 3 args; `SetKlinesURL` undefined.

- [ ] **Step 3: Replace `internal/api/server.go`**

```go
package api

import (
	"database/sql"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/menribardhi/trader/internal/hub"
	"github.com/menribardhi/trader/internal/portfolio"
	"github.com/menribardhi/trader/internal/strategy"
)

const defaultKlinesURL = "https://api.binance.com/api/v3/klines?symbol=BTCUSDT&interval=5m&limit=8640"

type Server struct {
	hub       *hub.Hub
	db        *sql.DB
	feed      *portfolio.PriceFeed
	eng       *strategy.Engine
	klinesURL string
	router    chi.Router
}

func New(h *hub.Hub, sqldb *sql.DB, feed *portfolio.PriceFeed, eng *strategy.Engine) *Server {
	s := &Server{
		hub:       h,
		db:        sqldb,
		feed:      feed,
		eng:       eng,
		klinesURL: defaultKlinesURL,
	}
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Get("/ws", s.handleWS)
	r.Route("/api", func(r chi.Router) {
		r.Post("/alerts", s.handleCreateAlert)
		r.Get("/alerts", s.handleListAlerts)
		r.Delete("/alerts/{id}", s.handleDeleteAlert)
		r.Get("/portfolio", s.handleGetPortfolio)
		r.Post("/orders", s.handleCreateOrder)
		r.Get("/trades", s.handleListTrades)
		r.Get("/strategy/signals", s.handleGetSignals)
		r.Post("/strategy/backtest", s.handleRunBacktest)
	})
	s.router = r
	return s
}

// SetKlinesURL overrides the Binance klines URL. Used in tests to inject a mock server.
func (s *Server) SetKlinesURL(url string) {
	s.klinesURL = url
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}
```

- [ ] **Step 4: Create `internal/api/strategy_handler.go`**

```go
package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/menribardhi/trader/internal/binance"
	"github.com/menribardhi/trader/internal/strategy"
)

func (s *Server) handleGetSignals(w http.ResponseWriter, r *http.Request) {
	if s.eng == nil {
		http.Error(w, "not configured", http.StatusServiceUnavailable)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := s.eng.Subscribe()
	defer s.eng.Unsubscribe(ch)

	for {
		select {
		case sig, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(sig)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (s *Server) handleRunBacktest(w http.ResponseWriter, r *http.Request) {
	if s.eng == nil {
		http.Error(w, "not configured", http.StatusServiceUnavailable)
		return
	}
	closes, err := binance.FetchKlinesFromURL(s.klinesURL)
	if err != nil {
		http.Error(w, "failed to fetch historical data", http.StatusServiceUnavailable)
		return
	}
	result := strategy.NewBacktester().Run(closes)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
```

- [ ] **Step 5: Update 3 existing test files (add 4th nil arg)**

In `internal/api/ws_handler_test.go` at the line calling `api.New(h, nil, nil)`, change to:
```go
srv := httptest.NewServer(api.New(h, nil, nil, nil))
```

In `internal/api/alerts_handler_test.go` at the line calling `api.New(h, sqldb, nil)`, change to:
```go
srv := httptest.NewServer(api.New(h, sqldb, nil, nil))
```

In `internal/api/portfolio_handler_test.go` at line 39 calling `api.New(h, sqldb, feed)`, change to:
```go
srv := httptest.NewServer(api.New(h, sqldb, feed, nil))
```

- [ ] **Step 6: Replace `cmd/trader/main.go`**

```go
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/menribardhi/trader/internal/api"
	"github.com/menribardhi/trader/internal/binance"
	dbpkg "github.com/menribardhi/trader/internal/db"
	"github.com/menribardhi/trader/internal/hub"
	"github.com/menribardhi/trader/internal/models"
	"github.com/menribardhi/trader/internal/portfolio"
	"github.com/menribardhi/trader/internal/strategy"
	"github.com/menribardhi/trader/internal/worker"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	sqldb, err := dbpkg.Open("./trader.db")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to open database")
	}
	defer sqldb.Close()

	if err := dbpkg.InitPortfolio(sqldb); err != nil {
		log.Fatal().Err(err).Msg("failed to init portfolio")
	}

	ticks := make(chan models.Tick, 64)
	h := hub.New(ticks)
	client := binance.New("BTCUSDT", ticks)
	feed := portfolio.NewPriceFeed(h)
	eng := strategy.NewEngine(h)

	go client.Run(ctx)
	go h.Run(ctx)
	go feed.Run(ctx)
	go eng.Run(ctx)
	go worker.NewAlertChecker(h, sqldb).Run(ctx)

	httpSrv := &http.Server{Addr: ":8080", Handler: api.New(h, sqldb, feed, eng)}
	go func() {
		<-ctx.Done()
		_ = httpSrv.Shutdown(context.Background())
	}()
	log.Info().Msg("trader listening on :8080")
	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal().Err(err).Msg("server error")
	}
}
```

- [ ] **Step 7: Build and run full suite**

```
go build ./...
go test ./...
```
Expected: build clean; all tests pass (including the 4 new strategy handler tests).

- [ ] **Step 8: Commit**

```
git add internal/api/strategy_handler.go internal/api/strategy_handler_test.go \
        internal/api/server.go \
        internal/api/ws_handler_test.go internal/api/alerts_handler_test.go \
        internal/api/portfolio_handler_test.go \
        cmd/trader/main.go
git commit -m "feat: add strategy REST API (SSE signals + backtest) and wire engine into main"
```

---

### Task 5: Frontend Strategy Page

**Files:**
- Create: `web/src/hooks/useStrategySignals.ts`
- Create: `web/src/pages/Strategy.tsx`
- Modify: `web/src/components/Nav.tsx` — add `'strategy'` tab
- Modify: `web/src/App.tsx` — add `'strategy'` to page union, render `<Strategy />`

**Interfaces:**
- Consumes: `GET /api/strategy/signals` (SSE), `POST /api/strategy/backtest` (JSON)
- Produces: `useStrategySignals()` hook, `<Strategy />` component

**Style guide:** match existing monospace dark theme (`#0f0f1a` bg, `#00d4aa` teal, `#ff4444` red, `#e0e0e0` text, `#1a1a2e` surface, `#2a2a4a` border).

- [ ] **Step 1: Create `web/src/hooks/useStrategySignals.ts`**

```ts
import { useState, useEffect } from 'react'

export interface Signal {
  side: 'BUY' | 'SELL' | 'HOLD'
  sma_fast: number
  sma_slow: number
  ema: number
  rsi: number
  price: number
  timestamp: number
}

export function useStrategySignals(): {
  latestSignal: Signal | null
  recentSignals: Signal[]
  connected: boolean
} {
  const [latestSignal, setLatestSignal] = useState<Signal | null>(null)
  const [recentSignals, setRecentSignals] = useState<Signal[]>([])
  const [connected, setConnected] = useState(false)

  useEffect(() => {
    const es = new EventSource('/api/strategy/signals')

    es.onopen = () => setConnected(true)

    es.onmessage = (e: MessageEvent) => {
      try {
        const sig = JSON.parse(e.data) as Signal
        setLatestSignal(sig)
        setRecentSignals(prev => [sig, ...prev].slice(0, 20))
      } catch {
        // ignore malformed events
      }
    }

    es.onerror = () => setConnected(false)

    return () => {
      es.close()
      setConnected(false)
    }
  }, [])

  return { latestSignal, recentSignals, connected }
}
```

- [ ] **Step 2: Create `web/src/pages/Strategy.tsx`**

```tsx
import { useState } from 'react'
import { useStrategySignals } from '../hooks/useStrategySignals'
import type { Signal } from '../hooks/useStrategySignals'

interface BacktestTrade {
  side: string
  price: number
  time: number
}

interface BacktestSummary {
  trades: BacktestTrade[]
  total_trades: number
  final_value: number
  return_pct: number
}

const fmt = (n: number) =>
  n.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })

function signalColor(side: Signal['side']): string {
  if (side === 'BUY') return '#00d4aa'
  if (side === 'SELL') return '#ff4444'
  return '#e0e0e0'
}

export function Strategy() {
  const { latestSignal, recentSignals, connected } = useStrategySignals()
  const [backtest, setBacktest] = useState<BacktestSummary | null>(null)
  const [loading, setLoading] = useState(false)
  const [backtestError, setBacktestError] = useState<string | null>(null)

  async function runBacktest() {
    setLoading(true)
    setBacktestError(null)
    try {
      const resp = await fetch('/api/strategy/backtest', { method: 'POST' })
      if (!resp.ok) {
        const text = await resp.text()
        throw new Error(text.trim() || 'Backtest failed')
      }
      setBacktest(await resp.json())
    } catch (err) {
      setBacktestError(err instanceof Error ? err.message : 'Backtest failed')
    } finally {
      setLoading(false)
    }
  }

  const returnColor = backtest && backtest.return_pct >= 0 ? '#00d4aa' : '#ff4444'

  return (
    <div>
      {/* Live indicators */}
      <p style={{ color: connected ? '#00d4aa' : '#ff4444', marginBottom: '1rem' }}>
        {connected ? '● Connected' : '○ Disconnected'}
      </p>

      {latestSignal && (
        <div style={{ display: 'flex', gap: '2rem', flexWrap: 'wrap', marginBottom: '2rem' }}>
          <Stat label="Signal" value={latestSignal.side} color={signalColor(latestSignal.side)} />
          <Stat label="Price" value={`$${fmt(latestSignal.price)}`} />
          <Stat label="SMA(10)" value={fmt(latestSignal.sma_fast)} />
          <Stat label="SMA(50)" value={fmt(latestSignal.sma_slow)} />
          <Stat label="EMA(20)" value={fmt(latestSignal.ema)} />
          <Stat label="RSI(14)" value={latestSignal.rsi.toFixed(1)} />
        </div>
      )}

      {/* Recent signals table */}
      {recentSignals.length > 0 && (
        <div style={{ marginBottom: '2rem' }}>
          <h2 style={{ fontSize: '0.9rem', opacity: 0.5, marginBottom: '0.75rem' }}>
            Recent Signals
          </h2>
          <table style={{ width: '100%', borderCollapse: 'collapse', maxWidth: '640px', fontSize: '0.875rem' }}>
            <thead>
              <tr style={{ opacity: 0.5, textAlign: 'left' }}>
                <th style={{ paddingBottom: '0.5rem' }}>Time</th>
                <th style={{ paddingBottom: '0.5rem' }}>Side</th>
                <th style={{ paddingBottom: '0.5rem' }}>Price</th>
                <th style={{ paddingBottom: '0.5rem' }}>SMA(10)</th>
                <th style={{ paddingBottom: '0.5rem' }}>SMA(50)</th>
              </tr>
            </thead>
            <tbody>
              {recentSignals.map((s, i) => (
                <tr key={i} style={{ borderTop: '1px solid #2a2a4a' }}>
                  <td style={{ padding: '0.35rem 0.5rem 0.35rem 0', opacity: 0.5 }}>
                    {new Date(s.timestamp).toLocaleTimeString()}
                  </td>
                  <td style={{ padding: '0.35rem 0.5rem', color: signalColor(s.side), fontWeight: 'bold' }}>
                    {s.side}
                  </td>
                  <td style={{ padding: '0.35rem 0.5rem' }}>${fmt(s.price)}</td>
                  <td style={{ padding: '0.35rem 0.5rem' }}>{fmt(s.sma_fast)}</td>
                  <td style={{ padding: '0.35rem 0.5rem' }}>{fmt(s.sma_slow)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Backtest panel */}
      <div>
        <h2 style={{ fontSize: '0.9rem', opacity: 0.5, marginBottom: '0.75rem' }}>Backtest</h2>
        <p style={{ fontSize: '0.8rem', opacity: 0.4, marginBottom: '0.75rem' }}>
          30 days of 5-minute BTCUSDT candles, SMA(10/50) crossover strategy
        </p>
        <button
          onClick={runBacktest}
          disabled={loading}
          style={{
            background: loading ? '#2a2a4a' : '#00d4aa',
            color: loading ? '#e0e0e0' : '#0f0f1a',
            border: 'none',
            padding: '0.4rem 1rem',
            cursor: loading ? 'default' : 'pointer',
            fontFamily: 'monospace',
            fontWeight: 'bold',
            marginBottom: '1rem',
          }}
        >
          {loading ? 'Running...' : 'Run Backtest'}
        </button>

        {backtestError && (
          <p style={{ color: '#ff4444', fontSize: '0.875rem', marginBottom: '1rem' }}>
            {backtestError}
          </p>
        )}

        {backtest && (
          <>
            <div style={{ display: 'flex', gap: '2rem', flexWrap: 'wrap', marginBottom: '1.5rem' }}>
              <Stat label="Total Trades" value={String(backtest.total_trades)} />
              <Stat label="Final Value" value={`$${fmt(backtest.final_value)}`} />
              <Stat
                label="Return"
                value={`${backtest.return_pct >= 0 ? '+' : ''}${backtest.return_pct.toFixed(2)}%`}
                color={returnColor}
              />
            </div>

            {backtest.trades.length > 0 && (
              <div style={{ maxHeight: '300px', overflowY: 'auto' }}>
                <table style={{ width: '100%', borderCollapse: 'collapse', maxWidth: '480px', fontSize: '0.875rem' }}>
                  <thead>
                    <tr style={{ opacity: 0.5, textAlign: 'left' }}>
                      <th style={{ paddingBottom: '0.5rem' }}>Index</th>
                      <th style={{ paddingBottom: '0.5rem' }}>Side</th>
                      <th style={{ paddingBottom: '0.5rem' }}>Price</th>
                    </tr>
                  </thead>
                  <tbody>
                    {backtest.trades.map((tr, i) => (
                      <tr key={i} style={{ borderTop: '1px solid #2a2a4a' }}>
                        <td style={{ padding: '0.35rem 0.5rem 0.35rem 0', opacity: 0.5 }}>{tr.time}</td>
                        <td style={{ padding: '0.35rem 0.5rem', color: tr.side === 'BUY' ? '#00d4aa' : '#ff4444' }}>
                          {tr.side}
                        </td>
                        <td style={{ padding: '0.35rem 0.5rem' }}>${fmt(tr.price)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}

            {backtest.total_trades === 0 && (
              <p style={{ opacity: 0.4 }}>No crossover signals in this period.</p>
            )}
          </>
        )}
      </div>
    </div>
  )
}

function Stat({ label, value, color = '#e0e0e0' }: { label: string; value: string; color?: string }) {
  return (
    <div>
      <div style={{ fontSize: '0.75rem', opacity: 0.5, marginBottom: '0.2rem' }}>{label}</div>
      <div style={{ fontSize: '1.25rem', fontWeight: 'bold', color }}>{value}</div>
    </div>
  )
}
```

- [ ] **Step 3: Replace `web/src/components/Nav.tsx`**

```tsx
type Page = 'dashboard' | 'portfolio' | 'strategy'

interface NavProps {
  active: Page
  onChange: (page: Page) => void
}

export function Nav({ active, onChange }: NavProps) {
  return (
    <nav style={{ display: 'flex', gap: '1.5rem', marginBottom: '1.5rem' }}>
      {(['dashboard', 'portfolio', 'strategy'] as const).map(page => (
        <button
          key={page}
          onClick={() => onChange(page)}
          style={{
            background: 'none',
            border: 'none',
            borderBottom: active === page ? '2px solid #00d4aa' : '2px solid transparent',
            color: active === page ? '#00d4aa' : '#e0e0e0',
            fontFamily: 'monospace',
            fontSize: '0.95rem',
            cursor: 'pointer',
            padding: '0.25rem 0',
          }}
        >
          {page.charAt(0).toUpperCase() + page.slice(1)}
        </button>
      ))}
    </nav>
  )
}
```

- [ ] **Step 4: Replace `web/src/App.tsx`**

```tsx
import { useState } from 'react'
import { Dashboard } from './pages/Dashboard'
import { Portfolio } from './pages/Portfolio'
import { Strategy } from './pages/Strategy'
import { Nav } from './components/Nav'

type Page = 'dashboard' | 'portfolio' | 'strategy'

function App() {
  const [page, setPage] = useState<Page>('dashboard')

  return (
    <div
      style={{
        padding: '1.5rem',
        fontFamily: 'monospace',
        background: '#0f0f1a',
        minHeight: '100vh',
        color: '#e0e0e0',
      }}
    >
      <h1 style={{ marginBottom: '1rem' }}>Trader</h1>
      <Nav active={page} onChange={setPage} />
      {page === 'dashboard' && <Dashboard />}
      {page === 'portfolio' && <Portfolio />}
      {page === 'strategy' && <Strategy />}
    </div>
  )
}

export default App
```

- [ ] **Step 5: TypeScript check and build**

```
cd web && npx tsc --noEmit && npm run build
```
Expected: no errors; build succeeds.

- [ ] **Step 6: Commit**

```
git add web/src/hooks/useStrategySignals.ts web/src/pages/Strategy.tsx \
        web/src/components/Nav.tsx web/src/App.tsx
git commit -m "feat: add Strategy tab with live SMA signals and backtester UI"
```
