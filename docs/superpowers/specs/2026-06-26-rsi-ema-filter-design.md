# RSI + EMA Confirmation Filter Design

## Overview

Add RSI and EMA confirmation filters to the existing SMA crossover strategy. The SMA(10/50) crossover still triggers a signal, but only if RSI and EMA agree. Crossovers that fail the filter emit HOLD instead of BUY/SELL.

---

## Signal Logic

### BUY — all three conditions must be true:
1. SMA(10) crosses above SMA(50) (existing crossover)
2. RSI(14) < 70 — not overbought
3. Price > EMA(20) — price is above the medium-term trend

### SELL — all three conditions must be true:
1. SMA(10) crosses below SMA(50) (existing crossover)
2. RSI(14) > 30 — not oversold
3. Price < EMA(20) — price is below the medium-term trend

### HOLD — any of:
- Fewer than 50 prices in window (pre-fill, unchanged)
- First full-window price (priming, unchanged)
- No crossover detected
- Crossover detected but RSI or EMA filter fails

---

## Behavior Details

**Blocked crossover is not remembered.** If a BUY crossover fires but RSI=75 (overbought) blocks it, the engine emits HOLD and moves on. The next tick compares current SMA values against the just-updated `fastPrev`/`slowPrev`. A future crossover can still fire a BUY.

**Signal struct is unchanged.** The `RSI`, `EMA`, and `Price` fields are already broadcast on every tick, so the frontend can show why a crossover was blocked without any model changes.

**Backtester applies the same filter.** Historical backtest results will reflect the smarter strategy — filtered-out crossovers are skipped and no simulated trade is executed.

---

## Files Changed

| File | Change |
|---|---|
| `internal/strategy/engine.go` | Add `&& rsi < 70 && price > ema` to BUY case; `&& rsi > 30 && price < ema` to SELL case in `ProcessPrice` switch |
| `internal/strategy/backtester.go` | Same two filter conditions added to backtester crossover switch |
| `internal/strategy/engine_test.go` | Replace spike price sequences (which produce RSI=100) with gradual-rise/fall sequences that satisfy filters; add `TestBuyBlockedByFilter` |
| `internal/strategy/backtester_test.go` | Update `TestBacktestBuyThenSell` price sequence to satisfy filters; add `TestBacktestFilterBlocksCrossover` |

No changes to: `internal/models/types.go`, `internal/api/`, `web/`, `cmd/`.

---

## Filter Constants

Defined inline (no new named constants needed — these match standard RSI interpretation):

| Filter | BUY threshold | SELL threshold |
|---|---|---|
| RSI(14) | < 70 (not overbought) | > 30 (not oversold) |
| EMA(20) | price > EMA | price < EMA |

---

## Test Price Sequences

Existing BUY/SELL tests used a spike pattern (49×10 then 200) which produces RSI=100, blocking the filter. Replacement sequences must satisfy:
- A valid SMA crossover (fast crosses above/below slow while window is already full)
- RSI < 70 for BUY, RSI > 30 for SELL (not spiked — avoid single large price jumps)
- Price above EMA(20) for BUY, below for SELL

**Pattern for BUY test:** Fill window (50 prices at a base level), let fast SMA fall below slow SMA (e.g., 20 prices slightly below base), then gradually rise so fast SMA crosses back above slow SMA. The gradual rise keeps RSI moderate and price above EMA at the crossover point.

**Pattern for SELL test:** Mirror of BUY — after window fill, fast SMA rises above slow SMA, then gradually falls through. The gradual fall keeps RSI moderate and price below EMA.

**Filter-blocked test** (`TestBuyBlockedByFilter`) — spike pattern (49×10, then 200) retained as-is: crossover fires, RSI=100 blocks it → assert signal is HOLD, not BUY.

---

## What Is Not Changing

- RSI and EMA are still computed and included in every Signal, even HOLDs.
- Thresholds (70, 30) are not user-configurable — hardcoded, same as existing periods.
- No frontend changes — the Strategy tab already displays RSI, EMA, and Price values.
- No API changes.
