package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/menribardhi/trader/internal/api"
	"github.com/menribardhi/trader/internal/db"
	"github.com/menribardhi/trader/internal/hub"
	"github.com/menribardhi/trader/internal/models"
	"github.com/menribardhi/trader/internal/portfolio"
)

type portfolioTestSetup struct {
	srv   *httptest.Server
	ticks chan models.Tick
}

func newPortfolioTestServer(t *testing.T) portfolioTestSetup {
	t.Helper()
	ticks := make(chan models.Tick, 4)
	h := hub.New(ticks)
	sqldb, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.InitPortfolio(sqldb); err != nil {
		t.Fatalf("init portfolio: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	feed := portfolio.NewPriceFeed(h)
	go h.Run(ctx)
	go feed.Run(ctx)
	srv := httptest.NewServer(api.New(h, sqldb, feed))
	t.Cleanup(func() {
		srv.Close()
		sqldb.Close()
		cancel()
	})
	return portfolioTestSetup{srv: srv, ticks: ticks}
}

func sendPriceTick(ticks chan models.Tick, price string) {
	ticks <- models.Tick{Symbol: "BTCUSDT", Price: price, Timestamp: 1}
	time.Sleep(50 * time.Millisecond)
}

func TestGetPortfolio(t *testing.T) {
	setup := newPortfolioTestServer(t)
	sendPriceTick(setup.ticks, "62000.00")

	resp, err := http.Get(setup.srv.URL + "/api/portfolio")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	var state models.PortfolioState
	if err := json.NewDecoder(resp.Body).Decode(&state); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if state.CashBalance != db.StartingCash {
		t.Errorf("cash: got %v, want %v", state.CashBalance, db.StartingCash)
	}
	if state.CurrentPrice != 62000.0 {
		t.Errorf("current_price: got %v, want 62000", state.CurrentPrice)
	}
	if state.TotalValue != db.StartingCash {
		t.Errorf("total_value: got %v, want %v (no BTC held)", state.TotalValue, db.StartingCash)
	}
}

func TestCreateOrderBuy(t *testing.T) {
	setup := newPortfolioTestServer(t)
	sendPriceTick(setup.ticks, "60000.00")

	body, _ := json.Marshal(map[string]any{"side": "buy", "quantity": 0.1})
	resp, err := http.Post(setup.srv.URL+"/api/orders", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected 201, got %d", resp.StatusCode)
	}
	var trade models.Trade
	if err := json.NewDecoder(resp.Body).Decode(&trade); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if trade.Side != "buy" || trade.Quantity != 0.1 || trade.Price != 60000.0 {
		t.Errorf("unexpected trade: %+v", trade)
	}
}

func TestCreateOrderInsufficientFunds(t *testing.T) {
	setup := newPortfolioTestServer(t)
	sendPriceTick(setup.ticks, "60000.00")

	body, _ := json.Marshal(map[string]any{"side": "buy", "quantity": 1.0}) // costs $60k, have $10k
	resp, _ := http.Post(setup.srv.URL+"/api/orders", "application/json", bytes.NewReader(body))
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", resp.StatusCode)
	}
}

func TestCreateOrderInsufficientBTC(t *testing.T) {
	setup := newPortfolioTestServer(t)
	sendPriceTick(setup.ticks, "60000.00")

	body, _ := json.Marshal(map[string]any{"side": "sell", "quantity": 0.1}) // have 0 BTC
	resp, _ := http.Post(setup.srv.URL+"/api/orders", "application/json", bytes.NewReader(body))
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", resp.StatusCode)
	}
}

func TestCreateOrderBadSide(t *testing.T) {
	setup := newPortfolioTestServer(t)
	body, _ := json.Marshal(map[string]any{"side": "hold", "quantity": 0.1})
	resp, _ := http.Post(setup.srv.URL+"/api/orders", "application/json", bytes.NewReader(body))
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCreateOrderNoPriceYet(t *testing.T) {
	setup := newPortfolioTestServer(t)
	// no tick sent — feed has no price

	body, _ := json.Marshal(map[string]any{"side": "buy", "quantity": 0.1})
	resp, _ := http.Post(setup.srv.URL+"/api/orders", "application/json", bytes.NewReader(body))
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503 when no price available, got %d", resp.StatusCode)
	}
}

func TestListTradesEndpoint(t *testing.T) {
	setup := newPortfolioTestServer(t)
	sendPriceTick(setup.ticks, "60000.00")

	body, _ := json.Marshal(map[string]any{"side": "buy", "quantity": 0.05})
	orderResp, err := http.Post(setup.srv.URL+"/api/orders", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if orderResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected order 201, got %d", orderResp.StatusCode)
	}

	resp, err := http.Get(setup.srv.URL + "/api/trades")
	if err != nil {
		t.Fatal(err)
	}
	var trades []models.Trade
	if err := json.NewDecoder(resp.Body).Decode(&trades); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}
	if trades[0].Side != "buy" {
		t.Errorf("trade side: got %q, want buy", trades[0].Side)
	}
}
