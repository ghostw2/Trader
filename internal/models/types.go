package models

// Tick is one price update from the exchange.
type Tick struct {
	Symbol    string `json:"symbol"`
	Price     string `json:"price"`
	Timestamp int64  `json:"timestamp"`
}

// Config holds startup configuration.
type Config struct {
	Symbol string
	Port   int
}
