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
	if len(closes) == 0 {
		return models.BacktestSummary{
			Trades:     []models.BacktestTrade{},
			FinalValue: startingCash,
		}
	}

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
