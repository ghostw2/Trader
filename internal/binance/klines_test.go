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
