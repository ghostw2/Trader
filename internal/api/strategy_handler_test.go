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
