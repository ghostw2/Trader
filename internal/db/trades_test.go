package db_test

import (
	"errors"
	"testing"

	"github.com/menribardhi/trader/internal/db"
)

func TestInitPortfolio(t *testing.T) {
	sqldb := openTestDB(t)
	if err := db.InitPortfolio(sqldb); err != nil {
		t.Fatal(err)
	}
	cash, btc, avg, err := db.GetPortfolio(sqldb)
	if err != nil {
		t.Fatal(err)
	}
	if cash != db.StartingCash {
		t.Errorf("cash: got %v, want %v", cash, db.StartingCash)
	}
	if btc != 0 {
		t.Errorf("btc: got %v, want 0", btc)
	}
	if avg != 0 {
		t.Errorf("avg: got %v, want 0", avg)
	}
}

func TestInitPortfolioIdempotent(t *testing.T) {
	sqldb := openTestDB(t)
	db.InitPortfolio(sqldb)
	if err := db.InitPortfolio(sqldb); err != nil {
		t.Errorf("second InitPortfolio must not error: %v", err)
	}
	cash, _, _, _ := db.GetPortfolio(sqldb)
	if cash != db.StartingCash {
		t.Errorf("cash changed after double init: got %v", cash)
	}
}

func TestExecuteOrderBuy(t *testing.T) {
	sqldb := openTestDB(t)
	db.InitPortfolio(sqldb)

	trade, err := db.ExecuteOrder(sqldb, "buy", 0.1, 60000.0)
	if err != nil {
		t.Fatal(err)
	}
	if trade.Side != "buy" || trade.Quantity != 0.1 || trade.Price != 60000.0 || trade.Total != 6000.0 {
		t.Errorf("unexpected trade: %+v", trade)
	}
	if trade.ID == 0 {
		t.Error("expected non-zero trade ID")
	}

	cash, btc, avg, _ := db.GetPortfolio(sqldb)
	wantCash := db.StartingCash - 6000.0
	if cash < wantCash-0.01 || cash > wantCash+0.01 {
		t.Errorf("cash: got %v, want %v (within 0.01)", cash, wantCash)
	}
	if btc != 0.1 {
		t.Errorf("btc: got %v, want 0.1", btc)
	}
	if avg != 60000.0 {
		t.Errorf("avg buy price: got %v, want 60000", avg)
	}
}

func TestExecuteOrderSell(t *testing.T) {
	sqldb := openTestDB(t)
	db.InitPortfolio(sqldb)
	db.ExecuteOrder(sqldb, "buy", 0.1, 60000.0)

	_, err := db.ExecuteOrder(sqldb, "sell", 0.05, 65000.0)
	if err != nil {
		t.Fatal(err)
	}
	cash, btc, _, _ := db.GetPortfolio(sqldb)
	wantCash := db.StartingCash - 6000.0 + 3250.0
	if cash < wantCash-0.01 || cash > wantCash+0.01 {
		t.Errorf("cash: got %v, want %v (within 0.01)", cash, wantCash)
	}
	if btc != 0.05 {
		t.Errorf("btc: got %v, want 0.05", btc)
	}
}

func TestExecuteOrderAvgBuyPrice(t *testing.T) {
	sqldb := openTestDB(t)
	db.InitPortfolio(sqldb)
	// Buy 0.1 BTC at 40k (4k cost) and 0.1 BTC at 60k (6k cost) = 10k total
	// Weighted average: (0.1*40000 + 0.1*60000) / 0.2 = 50000
	db.ExecuteOrder(sqldb, "buy", 0.1, 40000.0)
	db.ExecuteOrder(sqldb, "buy", 0.1, 60000.0)

	_, btc, avg, _ := db.GetPortfolio(sqldb)
	if btc != 0.2 {
		t.Errorf("btc: got %v, want 0.2", btc)
	}
	if avg != 50000.0 {
		t.Errorf("avg: got %v, want 50000 (weighted average)", avg)
	}
}

func TestExecuteOrderAvgResetOnFullSell(t *testing.T) {
	sqldb := openTestDB(t)
	db.InitPortfolio(sqldb)
	db.ExecuteOrder(sqldb, "buy", 0.1, 60000.0)
	db.ExecuteOrder(sqldb, "sell", 0.1, 65000.0)

	_, btc, avg, _ := db.GetPortfolio(sqldb)
	if btc != 0 {
		t.Errorf("btc: got %v, want 0", btc)
	}
	if avg != 0 {
		t.Errorf("avg should reset to 0 after full sell, got %v", avg)
	}
}

func TestExecuteOrderInsufficientFunds(t *testing.T) {
	sqldb := openTestDB(t)
	db.InitPortfolio(sqldb)
	_, err := db.ExecuteOrder(sqldb, "buy", 1.0, 20000.0) // costs $20k, have $10k
	if !errors.Is(err, db.ErrInsufficientFunds) {
		t.Errorf("expected ErrInsufficientFunds, got %v", err)
	}
	// portfolio must be unchanged
	cash, btc, _, _ := db.GetPortfolio(sqldb)
	if cash != db.StartingCash || btc != 0 {
		t.Errorf("portfolio changed after failed order: cash=%v btc=%v", cash, btc)
	}
}

func TestExecuteOrderInsufficientBTC(t *testing.T) {
	sqldb := openTestDB(t)
	db.InitPortfolio(sqldb)
	_, err := db.ExecuteOrder(sqldb, "sell", 0.1, 60000.0) // have 0 BTC
	if !errors.Is(err, db.ErrInsufficientBTC) {
		t.Errorf("expected ErrInsufficientBTC, got %v", err)
	}
}

func TestListTrades(t *testing.T) {
	sqldb := openTestDB(t)
	db.InitPortfolio(sqldb)
	db.ExecuteOrder(sqldb, "buy", 0.1, 60000.0)
	db.ExecuteOrder(sqldb, "sell", 0.05, 65000.0)

	trades, err := db.ListTrades(sqldb)
	if err != nil {
		t.Fatal(err)
	}
	if len(trades) != 2 {
		t.Fatalf("expected 2 trades, got %d", len(trades))
	}
}

func TestListTradesEmptyIsSlice(t *testing.T) {
	sqldb := openTestDB(t)
	db.InitPortfolio(sqldb)
	trades, err := db.ListTrades(sqldb)
	if err != nil {
		t.Fatal(err)
	}
	if trades == nil {
		t.Error("ListTrades must return empty slice, not nil")
	}
}
