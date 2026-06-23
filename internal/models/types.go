package models

type Tick struct {
	Symbol    string `json:"symbol"`
	Price     string `json:"price"`
	Timestamp int64  `json:"timestamp"`
}

type Config struct {
	Symbol string
	Port   int
}

type Alert struct {
	ID          int64   `json:"id"`
	Symbol      string  `json:"symbol"`
	TargetPrice float64 `json:"target_price"`
	Direction   string  `json:"direction"`
	CreatedAt   int64   `json:"created_at"`
	TriggeredAt *int64  `json:"triggered_at"`
}

type Trade struct {
	ID        int64   `json:"id"`
	Side      string  `json:"side"`       // "buy" or "sell"
	Quantity  float64 `json:"quantity"`   // BTC amount
	Price     float64 `json:"price"`      // USD per BTC at execution
	Total     float64 `json:"total"`      // quantity * price
	CreatedAt int64   `json:"created_at"` // Unix ms
}

type PortfolioState struct {
	CashBalance  float64 `json:"cash_balance"`
	BTCBalance   float64 `json:"btc_balance"`
	AvgBuyPrice  float64 `json:"avg_buy_price"`
	CurrentPrice float64 `json:"current_price"`
	TotalValue   float64 `json:"total_value"`
	UnrealizedPL float64 `json:"unrealized_pl"`
}

type OrderRequest struct {
	Side     string  `json:"side"`     // "buy" or "sell"
	Quantity float64 `json:"quantity"` // BTC amount
}
