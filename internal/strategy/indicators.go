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
