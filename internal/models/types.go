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
	Direction   string  `json:"direction"`    // "above" or "below"
	CreatedAt   int64   `json:"created_at"`   // Unix ms
	TriggeredAt *int64  `json:"triggered_at"` // null until triggered
}
