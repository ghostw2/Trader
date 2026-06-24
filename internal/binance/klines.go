package binance

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const binanceKlinesBase = "https://api.binance.com/api/v3/klines"

var klinesClient = &http.Client{Timeout: 30 * time.Second}

// FetchKlines fetches close prices from the Binance public klines REST API.
// No API key required.
func FetchKlines(symbol, interval string, limit int) ([]float64, error) {
	url := fmt.Sprintf("%s?symbol=%s&interval=%s&limit=%d", binanceKlinesBase, symbol, interval, limit)
	return FetchKlinesFromURL(url)
}

// FetchKlinesFromURL fetches close prices from an arbitrary klines URL.
// Exported for testing with a mock HTTP server.
func FetchKlinesFromURL(url string) ([]float64, error) {
	resp, err := klinesClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("klines request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("klines: unexpected status %d", resp.StatusCode)
	}
	var raw [][]json.RawMessage
	if err := json.NewDecoder(io.LimitReader(resp.Body, 10<<20)).Decode(&raw); err != nil {
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
