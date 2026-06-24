# Phase 4 — Strategy Automation Design

## Overview

Phase 4 adds automated trading strategy analysis to the paper trading app. A live strategy engine watches the BTC price stream and emits SMA crossover signals in real time. A backtester replays 30 days of historical 5-minute Binance klines through the same indicator logic and returns a simulated P&L summary. Signals are display-only — they do not auto-execute trades against the paper portfolio.

---

## Architecture

### New package: `internal/strategy/`

Three files with clear boundaries:

- **`indicators.go`** — pure functions, no state, no goroutines. Called by both the engine and the backtester.
- **`engine.go`** — goroutine that subscribes to the hub, maintains a rolling price window, computes indicators on every tick, and fans out `Signal` events to registered SSE clients.
- **`backtester.go`** — synchronous function that accepts a `[]float64` of historical close prices and returns `BacktestSummary`.

### New file: `internal/binance/klines.go`

Single exported function `FetchKlines(symbol, interval string, limit int) ([]float64, error)` — calls the Binance public REST klines endpoint, extracts close prices, returns them as `[]float64`. No API key required.

### New file: `internal/api/strategy_handler.go`

Two endpoints:
- `GET /api/strategy/signals` — SSE stream of live `Signal` JSON events
- `POST /api/strategy/backtest` — triggers `FetchKlines` + `Backtester.Run`, returns `BacktestSummary` JSON

### `api.New` signature change

Adds a 4th argument: `engine *strategy.Engine`. Nil-safe (same pattern as `feed *portfolio.PriceFeed`).

### Data flow

```
Live:
  hub tick → Engine.Run goroutine → rolling window [50 prices]
           → SMA(10), SMA(50), EMA(20), RSI(14)
           → crossover detection → Signal → SSE fan-out → browser

Backtest:
  POST /api/strategy/backtest
  → FetchKlines("BTCUSDT", "5m", 8640)   // Binance REST, no key
  → Backtester.Run(closes []float64)
  → BacktestSummary JSON → browser
```

---

## Models

Added to `internal/models/types.go`:

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
    Time  int64   `json:"time"`  // Unix ms (kline open time)
}

type BacktestSummary struct {
    Trades      []BacktestTrade `json:"trades"`
    TotalTrades int             `json:"total_trades"`
    FinalValue  float64         `json:"final_value"` // starting $10,000 +/- simulated P&L
    ReturnPct   float64         `json:"return_pct"`  // (FinalValue - 10000) / 10000 * 100
}
```

---

## Indicators

All implemented as pure functions in `internal/strategy/indicators.go`. Return `0.0` when there is insufficient data (caller checks before using).

| Function | Signature | Notes |
|---|---|---|
| SMA | `SMA(prices []float64, period int) float64` | average of last `period` values |
| EMA | `EMA(prices []float64, period int) float64` | seed = SMA(first period), then multiplier `2/(period+1)` applied forward |
| RSI | `RSI(prices []float64, period int) float64` | Wilder's method; average gain/loss over `period` windows; returns value in [0, 100] |

Engine constants (in `engine.go`):
```go
const (
    fastPeriod = 10
    slowPeriod = 50
    emaPeriod  = 20
    rsiPeriod  = 14
)
```

---

## Strategy: SMA Crossover

The only implemented strategy. Logic in `Engine.Run`:

1. On each tick, parse price string to float64, append to rolling window; trim to last 50 values.
2. If `len(window) < slowPeriod`: emit `Signal{Side: "HOLD"}` (insufficient data).
3. Compute `fastNow = SMA(window, fastPeriod)`, `slowNow = SMA(window, slowPeriod)`.
4. Compare with previous tick's `fastPrev`, `slowPrev`:
   - `fastPrev <= slowPrev && fastNow > slowNow` → `"BUY"` (crossover up)
   - `fastPrev >= slowPrev && fastNow < slowNow` → `"SELL"` (crossover down)
   - Otherwise → `"HOLD"`
5. Always include current EMA(20) and RSI(14) in the signal.
6. Broadcast `Signal` to all registered SSE clients via non-blocking fan-out.

---

## Backtester

`Backtester.Run(closes []float64) BacktestSummary` in `internal/strategy/backtester.go`:

- Runs the same SMA crossover logic over the full historical price slice.
- Simulates paper trading from $10,000: BUY allocates all cash at signal price (quantity = cash/price), SELL liquidates full BTC position.
- Ignores signals before `slowPeriod` (50) prices are accumulated.
- Returns all executed trades and final portfolio value.
- Pure computation — no network calls.

`FetchKlines` in `internal/binance/klines.go`:
- URL: `https://api.binance.com/api/v3/klines?symbol=BTCUSDT&interval=5m&limit=8640`
- Response: array of arrays; index 4 of each inner array is the close price string.
- Returns `[]float64` of close prices in chronological order.

---

## API

### `GET /api/strategy/signals` — SSE stream

Response headers:
```
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
```

Each event: `data: <Signal JSON>\n\n`

Client disconnect detected via `r.Context().Done()`. Per-client buffered channel (capacity 4); slow clients dropped with non-blocking send.

### `POST /api/strategy/backtest`

- No request body.
- `200 OK` + `BacktestSummary` JSON on success.
- `503 Service Unavailable` if Binance fetch fails or `eng` is nil.

### Updated `api.New` signature

```go
func New(h *hub.Hub, sqldb *sql.DB, feed *portfolio.PriceFeed, eng *strategy.Engine) http.Handler
```

---

## Frontend

### `web/src/hooks/useStrategySignals.ts`

Uses browser's native `EventSource`. Exports:
```ts
interface Signal {
  side: 'BUY' | 'SELL' | 'HOLD'
  sma_fast: number
  sma_slow: number
  ema: number
  rsi: number
  price: number
  timestamp: number
}

function useStrategySignals(): {
  latestSignal: Signal | null
  recentSignals: Signal[]   // last 20, newest first
  connected: boolean
}
```

Cleans up `EventSource` on unmount.

### `web/src/pages/Strategy.tsx`

Three sections:

1. **Live indicators panel** — connection badge; current price; SMAFast, SMASlow, EMA, RSI values; latest signal badge (BUY=green / SELL=red / HOLD=grey).
2. **Recent signals table** — columns: Time, Side, Price, SMA(10), SMA(50); last 20 signals; newest first.
3. **Backtest panel** — "Run Backtest" button; "Loading..." while pending; on success: summary card (Total Trades, Final Value, Return %) + scrollable trade history table (Time, Side, Price).

### `web/src/components/Nav.tsx`

Add third tab: `Dashboard | Portfolio | Strategy`

### `web/src/App.tsx`

Add `'strategy'` to page state union type; render `<Strategy />` for that case.

---

## Testing

### `internal/strategy/indicators_test.go`
- `TestSMA` — known slice, exact average
- `TestEMA` — verify multiplier against hand-computed values
- `TestRSI` — range [0,100], known gain/loss sequence
- `TestSMAInsufficientData` — returns 0.0 when len < period

### `internal/strategy/engine_test.go`
- `TestNoSignalBeforeWindow` — HOLD before 50 prices accumulated
- `TestBuySignalOnCrossoverUp` — price sequence producing fast > slow crossover → BUY
- `TestSellSignalOnCrossoverDown` — inverse → SELL

### `internal/strategy/backtester_test.go`
- `TestBacktestNoTrades` — flat prices → zero trades
- `TestBacktestBuyThenSell` — one BUY + one SELL → correct P&L

### `internal/binance/klines_test.go`
- `TestFetchKlines` — mock HTTP server with Binance JSON shape → correct close prices

### `internal/api/strategy_handler_test.go`
- `TestBacktestEndpoint` — mock engine with canned BacktestSummary → 200 + JSON shape
- `TestSignalsSSE` — SSE connection → inject signal → assert `data:` line received

---

## Files Changed

| Action | File |
|---|---|
| Create | `internal/strategy/indicators.go` |
| Create | `internal/strategy/indicators_test.go` |
| Create | `internal/strategy/engine.go` |
| Create | `internal/strategy/engine_test.go` |
| Create | `internal/strategy/backtester.go` |
| Create | `internal/strategy/backtester_test.go` |
| Create | `internal/binance/klines.go` |
| Create | `internal/binance/klines_test.go` |
| Create | `internal/api/strategy_handler.go` |
| Create | `internal/api/strategy_handler_test.go` |
| Modify | `internal/models/types.go` |
| Modify | `internal/api/server.go` |
| Modify | `internal/api/ws_handler_test.go` |
| Modify | `internal/api/alerts_handler_test.go` |
| Modify | `internal/api/portfolio_handler_test.go` |
| Modify | `cmd/trader/main.go` |
| Create | `web/src/hooks/useStrategySignals.ts` |
| Create | `web/src/pages/Strategy.tsx` |
| Modify | `web/src/components/Nav.tsx` |
| Modify | `web/src/App.tsx` |
