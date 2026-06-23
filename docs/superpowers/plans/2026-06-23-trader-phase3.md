# Phase 3 — Paper Trading Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let users simulate buying and selling BTC with $10,000 of fake money, tracking their portfolio value and unrealized P&L in real time against the live price.

**Architecture:** A `PriceFeed` goroutine subscribes to the hub and keeps the latest BTC price in memory. An `ExecuteOrder` DB function atomically deducts cash and records a trade in a single SQLite transaction. Three new REST endpoints (`GET /api/portfolio`, `POST /api/orders`, `GET /api/trades`) serve the React frontend, which gets a new Portfolio page with a buy/sell form and trade history table. App.tsx gains simple tab navigation between Dashboard and Portfolio.

**Tech Stack:** Go 1.24 backend (existing), SQLite via modernc.org/sqlite (existing), React 19 + TypeScript + Vite (existing), Chi router (existing)

## Global Constraints

- Module: `github.com/menribardhi/trader`
- Starting cash: `10000.0` USD — exported constant `db.StartingCash = 10000.0`
- BTC quantity: float64, no rounding enforced at the model layer
- All timestamps: Unix milliseconds (`int64`) — consistent with Phase 1 & 2
- `api.New` signature changes from `(h *hub.Hub, sqldb *sql.DB)` to `(h *hub.Hub, sqldb *sql.DB, feed *portfolio.PriceFeed)` — BREAKING: all existing callers and test files must be updated
- Frontend uses relative URLs `/api/...` — Vite proxy already configured in `web/vite.config.ts`
- `go test ./...` must pass after every task
- Do NOT modify any Phase 2 logic (alerts worker, alert handlers, alert DB) — additive only

---

### Task 1: Models + DB schema migration

**Files:**
- Modify: `internal/models/types.go`
- Modify: `internal/db/db.go`

**Interfaces:**
- Produces:
  - `models.Trade{ID int64, Side string, Quantity float64, Price float64, Total float64, CreatedAt int64}` — JSON snake_case
  - `models.PortfolioState{CashBalance float64, BTCBalance float64, AvgBuyPrice float64, CurrentPrice float64, TotalValue float64, UnrealizedPL float64}` — JSON snake_case
  - `models.OrderRequest{Side string, Quantity float64}` — JSON snake_case
  - SQLite tables `portfolio` and `trades` created on startup via `db.Open` → `migrate()`

- [ ] **Step 1: Add structs to models**

Replace `internal/models/types.go` entirely with:
```go
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
```

- [ ] **Step 2: Add portfolio and trades tables to migration**

In `internal/db/db.go`, replace the `migrate` function body (keep the signature):
```go
func migrate(sqldb *sql.DB) error {
	_, err := sqldb.Exec(`
		CREATE TABLE IF NOT EXISTS alerts (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			symbol       TEXT    NOT NULL,
			target_price REAL    NOT NULL,
			direction    TEXT    NOT NULL CHECK (direction IN ('above','below')),
			created_at   INTEGER NOT NULL,
			triggered_at INTEGER
		);

		CREATE TABLE IF NOT EXISTS portfolio (
			id            INTEGER PRIMARY KEY CHECK (id = 1),
			cash          REAL    NOT NULL,
			btc           REAL    NOT NULL DEFAULT 0,
			avg_buy_price REAL    NOT NULL DEFAULT 0
		);

		CREATE TABLE IF NOT EXISTS trades (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			side       TEXT    NOT NULL CHECK (side IN ('buy','sell')),
			quantity   REAL    NOT NULL,
			price      REAL    NOT NULL,
			total      REAL    NOT NULL,
			created_at INTEGER NOT NULL
		)
	`)
	return err
}
```

- [ ] **Step 3: Build and test**

```bash
go build ./...
go test ./...
```

Expected: no errors, all existing tests pass.

- [ ] **Step 4: Commit**

```bash
git add internal/models/types.go internal/db/db.go
git commit -m "feat: add Trade/PortfolioState models and portfolio+trades DB tables"
```

---

### Task 2: SQLite trades layer (TDD)

**Files:**
- Create: `internal/db/trades.go`
- Create: `internal/db/trades_test.go`

**Interfaces:**
- Consumes: `models.Trade` from Task 1; `openTestDB(t)` helper already defined in `internal/db/db_test.go` (same `package db_test` — both files share it automatically)
- Produces (exact signatures — used verbatim in Tasks 4 and 5):
  - `db.StartingCash float64 = 10000.0` (exported constant)
  - `db.ErrInsufficientFunds = errors.New("insufficient funds")`
  - `db.ErrInsufficientBTC   = errors.New("insufficient BTC")`
  - `db.InitPortfolio(sqldb *sql.DB) error` — inserts the single portfolio row if absent (idempotent)
  - `db.GetPortfolio(sqldb *sql.DB) (cash, btc, avgBuyPrice float64, err error)`
  - `db.ExecuteOrder(sqldb *sql.DB, side string, quantity, price float64) (models.Trade, error)` — atomic transaction
  - `db.ListTrades(sqldb *sql.DB) ([]models.Trade, error)` — always a non-nil slice, newest first

- [ ] **Step 1: Write failing tests**

Create `internal/db/trades_test.go`:
```go
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
	if cash != db.StartingCash-6000.0 {
		t.Errorf("cash: got %v, want %v", cash, db.StartingCash-6000.0)
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
	if cash != wantCash {
		t.Errorf("cash: got %v, want %v", cash, wantCash)
	}
	if btc != 0.05 {
		t.Errorf("btc: got %v, want 0.05", btc)
	}
}

func TestExecuteOrderAvgBuyPrice(t *testing.T) {
	sqldb := openTestDB(t)
	db.InitPortfolio(sqldb)
	db.ExecuteOrder(sqldb, "buy", 0.1, 60000.0)
	db.ExecuteOrder(sqldb, "buy", 0.1, 70000.0)

	_, btc, avg, _ := db.GetPortfolio(sqldb)
	if btc != 0.2 {
		t.Errorf("btc: got %v, want 0.2", btc)
	}
	if avg != 65000.0 {
		t.Errorf("avg: got %v, want 65000 (weighted average)", avg)
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
```

- [ ] **Step 2: Run tests — expect failure**

```bash
go test ./internal/db/... 2>&1 | head -5
```

Expected: compile error — `db.StartingCash`, `db.InitPortfolio` etc. undefined.

- [ ] **Step 3: Implement trades.go**

Create `internal/db/trades.go`:
```go
package db

import (
	"database/sql"
	"errors"
	"time"

	"github.com/menribardhi/trader/internal/models"
)

const StartingCash = 10000.0

var ErrInsufficientFunds = errors.New("insufficient funds")
var ErrInsufficientBTC = errors.New("insufficient BTC")

func InitPortfolio(sqldb *sql.DB) error {
	_, err := sqldb.Exec(
		`INSERT OR IGNORE INTO portfolio (id, cash, btc, avg_buy_price) VALUES (1, ?, 0, 0)`,
		StartingCash,
	)
	return err
}

func GetPortfolio(sqldb *sql.DB) (cash, btc, avgBuyPrice float64, err error) {
	err = sqldb.QueryRow(`SELECT cash, btc, avg_buy_price FROM portfolio WHERE id = 1`).
		Scan(&cash, &btc, &avgBuyPrice)
	return
}

func ExecuteOrder(sqldb *sql.DB, side string, quantity, price float64) (models.Trade, error) {
	tx, err := sqldb.Begin()
	if err != nil {
		return models.Trade{}, err
	}
	defer tx.Rollback()

	var cash, btc, avgBuyPrice float64
	if err := tx.QueryRow(`SELECT cash, btc, avg_buy_price FROM portfolio WHERE id = 1`).
		Scan(&cash, &btc, &avgBuyPrice); err != nil {
		return models.Trade{}, err
	}

	total := quantity * price
	switch side {
	case "buy":
		if cash < total {
			return models.Trade{}, ErrInsufficientFunds
		}
		newBTC := btc + quantity
		avgBuyPrice = (btc*avgBuyPrice + quantity*price) / newBTC
		cash -= total
		btc = newBTC
	case "sell":
		if btc < quantity {
			return models.Trade{}, ErrInsufficientBTC
		}
		btc -= quantity
		cash += total
		if btc == 0 {
			avgBuyPrice = 0
		}
	}

	if _, err := tx.Exec(
		`UPDATE portfolio SET cash = ?, btc = ?, avg_buy_price = ? WHERE id = 1`,
		cash, btc, avgBuyPrice,
	); err != nil {
		return models.Trade{}, err
	}

	now := time.Now().UnixMilli()
	res, err := tx.Exec(
		`INSERT INTO trades (side, quantity, price, total, created_at) VALUES (?,?,?,?,?)`,
		side, quantity, price, total, now,
	)
	if err != nil {
		return models.Trade{}, err
	}
	id, _ := res.LastInsertId()

	if err := tx.Commit(); err != nil {
		return models.Trade{}, err
	}
	return models.Trade{
		ID: id, Side: side, Quantity: quantity,
		Price: price, Total: total, CreatedAt: now,
	}, nil
}

func ListTrades(sqldb *sql.DB) ([]models.Trade, error) {
	rows, err := sqldb.Query(
		`SELECT id, side, quantity, price, total, created_at FROM trades ORDER BY id DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	trades := []models.Trade{}
	for rows.Next() {
		var t models.Trade
		if err := rows.Scan(&t.ID, &t.Side, &t.Quantity, &t.Price, &t.Total, &t.CreatedAt); err != nil {
			return nil, err
		}
		trades = append(trades, t)
	}
	return trades, rows.Err()
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/db/... -v 2>&1 | tail -20
```

Expected: all tests pass (existing alert tests + 9 new trades tests).

- [ ] **Step 5: Commit**

```bash
git add internal/db/trades.go internal/db/trades_test.go
git commit -m "feat: add paper trading DB layer — portfolio init, order execution, trade history"
```

---

### Task 3: PriceFeed — track latest market price (TDD)

**Files:**
- Create: `internal/portfolio/pricer.go`
- Create: `internal/portfolio/pricer_test.go`

**Interfaces:**
- Consumes: `hub.Hub` (Subscribe, Unsubscribe methods); `models.Tick` (Price field is a string like "62000.00")
- Produces (exact signatures — used verbatim in Tasks 4 and 5):
  - `portfolio.NewPriceFeed(h *hub.Hub) *PriceFeed`
  - `(*PriceFeed).Run(ctx context.Context)` — blocks until ctx cancelled; subscribes to hub
  - `(*PriceFeed).Latest() (float64, bool)` — returns (price, hasPrice), safe for concurrent access

- [ ] **Step 1: Write failing tests**

Create `internal/portfolio/pricer_test.go`:
```go
package portfolio_test

import (
	"context"
	"testing"
	"time"

	"github.com/menribardhi/trader/internal/hub"
	"github.com/menribardhi/trader/internal/models"
	"github.com/menribardhi/trader/internal/portfolio"
)

func setupFeedTest(t *testing.T) (*portfolio.PriceFeed, chan models.Tick, context.CancelFunc) {
	t.Helper()
	ticks := make(chan models.Tick, 4)
	h := hub.New(ticks)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	go h.Run(ctx)
	feed := portfolio.NewPriceFeed(h)
	go feed.Run(ctx)
	time.Sleep(10 * time.Millisecond)
	return feed, ticks, cancel
}

func TestPriceFeedNoPrice(t *testing.T) {
	feed, _, cancel := setupFeedTest(t)
	defer cancel()

	_, hasPrice := feed.Latest()
	if hasPrice {
		t.Error("should have no price before any tick")
	}
}

func TestPriceFeedReceivesTick(t *testing.T) {
	feed, ticks, cancel := setupFeedTest(t)
	defer cancel()

	ticks <- models.Tick{Symbol: "BTCUSDT", Price: "62000.00", Timestamp: 1}
	time.Sleep(50 * time.Millisecond)

	price, hasPrice := feed.Latest()
	if !hasPrice {
		t.Error("should have price after tick")
	}
	if price != 62000.0 {
		t.Errorf("price: got %v, want 62000.0", price)
	}
}

func TestPriceFeedUpdatesToLatest(t *testing.T) {
	feed, ticks, cancel := setupFeedTest(t)
	defer cancel()

	ticks <- models.Tick{Symbol: "BTCUSDT", Price: "62000.00", Timestamp: 1}
	time.Sleep(50 * time.Millisecond)
	ticks <- models.Tick{Symbol: "BTCUSDT", Price: "63000.00", Timestamp: 2}
	time.Sleep(50 * time.Millisecond)

	price, _ := feed.Latest()
	if price != 63000.0 {
		t.Errorf("price: got %v, want 63000.0 (should be latest)", price)
	}
}
```

- [ ] **Step 2: Run tests — expect failure**

```bash
go test ./internal/portfolio/... 2>&1 | head -5
```

Expected: compile error — package `portfolio` not found.

- [ ] **Step 3: Implement pricer.go**

Create `internal/portfolio/pricer.go`:
```go
package portfolio

import (
	"context"
	"strconv"
	"sync"

	"github.com/menribardhi/trader/internal/hub"
	"github.com/menribardhi/trader/internal/models"
)

type PriceFeed struct {
	hub      *hub.Hub
	mu       sync.RWMutex
	price    float64
	hasPrice bool
}

func NewPriceFeed(h *hub.Hub) *PriceFeed {
	return &PriceFeed{hub: h}
}

func (p *PriceFeed) Run(ctx context.Context) {
	sub := p.hub.Subscribe()
	defer p.hub.Unsubscribe(sub)
	for {
		select {
		case tick, ok := <-sub:
			if !ok {
				return
			}
			p.update(tick)
		case <-ctx.Done():
			return
		}
	}
}

func (p *PriceFeed) update(tick models.Tick) {
	price, err := strconv.ParseFloat(tick.Price, 64)
	if err != nil {
		return
	}
	p.mu.Lock()
	p.price = price
	p.hasPrice = true
	p.mu.Unlock()
}

func (p *PriceFeed) Latest() (float64, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.price, p.hasPrice
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/portfolio/... -v
```

Expected: 3 tests pass.

- [ ] **Step 5: Run full suite**

```bash
go test ./...
```

Expected: all packages pass.

- [ ] **Step 6: Commit**

```bash
git add internal/portfolio/
git commit -m "feat: add PriceFeed — tracks latest hub price in memory for order execution"
```

---

### Task 4: Portfolio REST API (TDD)

**Files:**
- Create: `internal/api/portfolio_handler.go`
- Create: `internal/api/portfolio_handler_test.go`
- Modify: `internal/api/server.go` — add `feed *portfolio.PriceFeed` field, change `New` to 3 args, add 3 routes
- Modify: `internal/api/ws_handler_test.go` — fix broken call: `api.New(h, nil)` → `api.New(h, nil, nil)`
- Modify: `internal/api/alerts_handler_test.go` — fix broken call: `api.New(h, sqldb)` → `api.New(h, sqldb, nil)`

**Interfaces:**
- Consumes: `db.GetPortfolio`, `db.ExecuteOrder`, `db.ListTrades`, `db.ErrInsufficientFunds`, `db.ErrInsufficientBTC` from Task 2; `portfolio.PriceFeed.Latest()` from Task 3
- Produces:
  - `api.New(h *hub.Hub, sqldb *sql.DB, feed *portfolio.PriceFeed) *Server` — BREAKING signature change
  - `GET /api/portfolio` → 200 `models.PortfolioState` JSON
  - `POST /api/orders` → 201 `models.Trade` JSON, or 400/422/503
  - `GET /api/trades` → 200 `[]models.Trade` JSON (empty array, not null)

- [ ] **Step 1: Write failing tests**

Create `internal/api/portfolio_handler_test.go`:
```go
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
	http.Post(setup.srv.URL+"/api/orders", "application/json", bytes.NewReader(body))

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
```

- [ ] **Step 2: Run tests — expect compile failure**

```bash
go test ./internal/api/... 2>&1 | head -10
```

Expected: compile errors — `api.New` wrong number of arguments.

- [ ] **Step 3: Update server.go**

Replace `internal/api/server.go` entirely:
```go
package api

import (
	"database/sql"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/menribardhi/trader/internal/hub"
	"github.com/menribardhi/trader/internal/portfolio"
)

type Server struct {
	hub    *hub.Hub
	db     *sql.DB
	feed   *portfolio.PriceFeed
	router chi.Router
}

func New(h *hub.Hub, sqldb *sql.DB, feed *portfolio.PriceFeed) *Server {
	s := &Server{hub: h, db: sqldb, feed: feed}
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Get("/ws", s.handleWS)
	r.Route("/api", func(r chi.Router) {
		r.Post("/alerts", s.handleCreateAlert)
		r.Get("/alerts", s.handleListAlerts)
		r.Delete("/alerts/{id}", s.handleDeleteAlert)
		r.Get("/portfolio", s.handleGetPortfolio)
		r.Post("/orders", s.handleCreateOrder)
		r.Get("/trades", s.handleListTrades)
	})
	s.router = r
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}
```

- [ ] **Step 4: Fix ws_handler_test.go**

In `internal/api/ws_handler_test.go`, find the line:
```go
srv := httptest.NewServer(api.New(h, nil))
```
Change it to:
```go
srv := httptest.NewServer(api.New(h, nil, nil))
```

- [ ] **Step 5: Fix alerts_handler_test.go**

In `internal/api/alerts_handler_test.go`, in `newAlertTestServer`, find:
```go
srv := httptest.NewServer(api.New(h, sqldb))
```
Change it to:
```go
srv := httptest.NewServer(api.New(h, sqldb, nil))
```

- [ ] **Step 6: Create portfolio_handler.go**

Create `internal/api/portfolio_handler.go`:
```go
package api

import (
	"encoding/json"
	"errors"
	"net/http"

	dbpkg "github.com/menribardhi/trader/internal/db"
	"github.com/menribardhi/trader/internal/models"
)

func (s *Server) handleGetPortfolio(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "not configured", http.StatusServiceUnavailable)
		return
	}
	cash, btc, avgBuyPrice, err := dbpkg.GetPortfolio(s.db)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	var currentPrice, totalValue, unrealizedPL float64
	if s.feed != nil {
		if price, ok := s.feed.Latest(); ok {
			currentPrice = price
			totalValue = cash + btc*price
			unrealizedPL = btc * (price - avgBuyPrice)
		} else {
			totalValue = cash
		}
	} else {
		totalValue = cash
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.PortfolioState{
		CashBalance:  cash,
		BTCBalance:   btc,
		AvgBuyPrice:  avgBuyPrice,
		CurrentPrice: currentPrice,
		TotalValue:   totalValue,
		UnrealizedPL: unrealizedPL,
	})
}

func (s *Server) handleCreateOrder(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "not configured", http.StatusServiceUnavailable)
		return
	}
	var req models.OrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Side != "buy" && req.Side != "sell" {
		http.Error(w, `side must be "buy" or "sell"`, http.StatusBadRequest)
		return
	}
	if req.Quantity <= 0 {
		http.Error(w, "quantity must be positive", http.StatusBadRequest)
		return
	}
	if s.feed == nil {
		http.Error(w, "no price feed", http.StatusServiceUnavailable)
		return
	}
	price, ok := s.feed.Latest()
	if !ok {
		http.Error(w, "no price available yet", http.StatusServiceUnavailable)
		return
	}
	trade, err := dbpkg.ExecuteOrder(s.db, req.Side, req.Quantity, price)
	if err != nil {
		if errors.Is(err, dbpkg.ErrInsufficientFunds) {
			http.Error(w, "insufficient funds", http.StatusUnprocessableEntity)
			return
		}
		if errors.Is(err, dbpkg.ErrInsufficientBTC) {
			http.Error(w, "insufficient BTC", http.StatusUnprocessableEntity)
			return
		}
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(trade)
}

func (s *Server) handleListTrades(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "not configured", http.StatusServiceUnavailable)
		return
	}
	trades, err := dbpkg.ListTrades(s.db)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(trades)
}
```

- [ ] **Step 7: Run all API tests**

```bash
go test ./internal/api/... -v 2>&1 | tail -30
```

Expected: all tests pass — existing WS tests, existing alert tests, and 7 new portfolio tests.

- [ ] **Step 8: Commit**

```bash
git add internal/api/
git commit -m "feat: add portfolio REST API — GET /api/portfolio, POST /api/orders, GET /api/trades"
```

---

### Task 5: Wire main.go

**Files:**
- Modify: `cmd/trader/main.go`

**Interfaces:**
- Consumes: `db.InitPortfolio(sqldb)`, `portfolio.NewPriceFeed(h)`, updated `api.New(h, sqldb, feed)` from Tasks 2, 3, 4

- [ ] **Step 1: Replace main.go**

Replace `cmd/trader/main.go` entirely:
```go
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/menribardhi/trader/internal/api"
	"github.com/menribardhi/trader/internal/binance"
	dbpkg "github.com/menribardhi/trader/internal/db"
	"github.com/menribardhi/trader/internal/hub"
	"github.com/menribardhi/trader/internal/models"
	"github.com/menribardhi/trader/internal/portfolio"
	"github.com/menribardhi/trader/internal/worker"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	sqldb, err := dbpkg.Open("./trader.db")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to open database")
	}
	defer sqldb.Close()

	if err := dbpkg.InitPortfolio(sqldb); err != nil {
		log.Fatal().Err(err).Msg("failed to init portfolio")
	}

	ticks := make(chan models.Tick, 64)
	h := hub.New(ticks)
	client := binance.New("BTCUSDT", ticks)
	feed := portfolio.NewPriceFeed(h)

	go client.Run(ctx)
	go h.Run(ctx)
	go worker.NewAlertChecker(h, sqldb).Run(ctx)
	go feed.Run(ctx)

	httpSrv := &http.Server{Addr: ":8080", Handler: api.New(h, sqldb, feed)}
	go func() {
		<-ctx.Done()
		_ = httpSrv.Shutdown(context.Background())
	}()
	log.Info().Msg("trader listening on :8080")
	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal().Err(err).Msg("server error")
	}
}
```

- [ ] **Step 2: Build and test**

```bash
go build ./...
go test ./...
```

Expected: no errors, all tests pass.

- [ ] **Step 3: Commit**

```bash
git add cmd/trader/main.go
git commit -m "feat: wire PriceFeed and portfolio init into main"
```

---

### Task 6: Frontend Portfolio page + navigation

**Files:**
- Create: `web/src/hooks/usePortfolio.ts`
- Create: `web/src/pages/Portfolio.tsx`
- Create: `web/src/components/Nav.tsx`
- Modify: `web/src/App.tsx` — own the page shell, add tab state and Nav
- Modify: `web/src/pages/Dashboard.tsx` — remove outer `<div>` shell and `<h1>` (App.tsx now owns them)

**Interfaces:**
- Consumes: `GET /api/portfolio` → `PortfolioState`, `POST /api/orders` → `Trade`, `GET /api/trades` → `Trade[]`
- Produces: `usePortfolio()` → `{ portfolio: PortfolioState | null, trades: Trade[], placeOrder(side, qty): Promise<void> }`

- [ ] **Step 1: Create usePortfolio.ts**

Create `web/src/hooks/usePortfolio.ts`:
```typescript
import { useState, useEffect, useCallback } from 'react'

export interface PortfolioState {
  cash_balance: number
  btc_balance: number
  avg_buy_price: number
  current_price: number
  total_value: number
  unrealized_pl: number
}

export interface Trade {
  id: number
  side: 'buy' | 'sell'
  quantity: number
  price: number
  total: number
  created_at: number
}

export function usePortfolio() {
  const [portfolio, setPortfolio] = useState<PortfolioState | null>(null)
  const [trades, setTrades] = useState<Trade[]>([])

  const fetchPortfolio = useCallback(async () => {
    try {
      const res = await fetch('/api/portfolio')
      if (res.ok) setPortfolio(await res.json())
    } catch (err) {
      console.error('Failed to fetch portfolio:', err)
    }
  }, [])

  const fetchTrades = useCallback(async () => {
    try {
      const res = await fetch('/api/trades')
      if (res.ok) setTrades(await res.json())
    } catch (err) {
      console.error('Failed to fetch trades:', err)
    }
  }, [])

  useEffect(() => {
    fetchPortfolio()
    fetchTrades()
    const id = setInterval(() => {
      fetchPortfolio()
      fetchTrades()
    }, 3000)
    return () => clearInterval(id)
  }, [fetchPortfolio, fetchTrades])

  const placeOrder = useCallback(async (side: 'buy' | 'sell', quantity: number): Promise<void> => {
    const res = await fetch('/api/orders', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ side, quantity }),
    })
    if (!res.ok) {
      const text = await res.text()
      throw new Error(text.trim())
    }
    await fetchPortfolio()
    await fetchTrades()
  }, [fetchPortfolio, fetchTrades])

  return { portfolio, trades, placeOrder }
}
```

- [ ] **Step 2: Create Nav.tsx**

Create `web/src/components/Nav.tsx`:
```tsx
interface NavProps {
  active: 'dashboard' | 'portfolio'
  onChange: (page: 'dashboard' | 'portfolio') => void
}

export function Nav({ active, onChange }: NavProps) {
  return (
    <nav style={{ display: 'flex', gap: '1.5rem', marginBottom: '1.5rem' }}>
      {(['dashboard', 'portfolio'] as const).map(page => (
        <button
          key={page}
          onClick={() => onChange(page)}
          style={{
            background: 'none',
            border: 'none',
            borderBottom: active === page ? '2px solid #00d4aa' : '2px solid transparent',
            color: active === page ? '#00d4aa' : '#e0e0e0',
            fontFamily: 'monospace',
            fontSize: '0.95rem',
            cursor: 'pointer',
            padding: '0.25rem 0',
          }}
        >
          {page.charAt(0).toUpperCase() + page.slice(1)}
        </button>
      ))}
    </nav>
  )
}
```

- [ ] **Step 3: Create Portfolio.tsx**

Create `web/src/pages/Portfolio.tsx`:
```tsx
import { useState } from 'react'
import type { FormEvent } from 'react'
import { usePortfolio } from '../hooks/usePortfolio'

export function Portfolio() {
  const { portfolio, trades, placeOrder } = usePortfolio()
  const [side, setSide] = useState<'buy' | 'sell'>('buy')
  const [quantity, setQuantity] = useState('')
  const [error, setError] = useState<string | null>(null)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    const qty = parseFloat(quantity)
    if (isNaN(qty) || qty <= 0) return
    setError(null)
    try {
      await placeOrder(side, qty)
      setQuantity('')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Order failed')
    }
  }

  const pl = portfolio?.unrealized_pl ?? 0
  const plColor = pl >= 0 ? '#00d4aa' : '#ff4444'
  const fmt = (n: number) =>
    n.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })

  return (
    <div>
      {portfolio && (
        <div style={{ display: 'flex', gap: '2rem', flexWrap: 'wrap', marginBottom: '2rem' }}>
          <Stat label="Portfolio Value" value={`$${fmt(portfolio.total_value)}`} />
          <Stat label="Cash" value={`$${fmt(portfolio.cash_balance)}`} />
          <Stat label="BTC Held" value={`${portfolio.btc_balance.toFixed(6)}`} />
          <Stat
            label="Avg Buy Price"
            value={portfolio.avg_buy_price > 0 ? `$${fmt(portfolio.avg_buy_price)}` : '—'}
          />
          <Stat
            label="Unrealized P&L"
            value={`${pl >= 0 ? '+' : ''}$${fmt(pl)}`}
            color={plColor}
          />
          <Stat label="Current Price" value={`$${fmt(portfolio.current_price)}`} />
        </div>
      )}

      <form
        onSubmit={handleSubmit}
        style={{ display: 'flex', gap: '0.5rem', marginBottom: '2rem', flexWrap: 'wrap', alignItems: 'center' }}
      >
        <select
          value={side}
          onChange={e => setSide(e.target.value as 'buy' | 'sell')}
          style={{ background: '#1a1a2e', color: '#e0e0e0', border: '1px solid #2a2a4a', padding: '0.4rem 0.6rem' }}
        >
          <option value="buy">Buy BTC</option>
          <option value="sell">Sell BTC</option>
        </select>
        <input
          type="number"
          placeholder="Quantity (BTC)"
          value={quantity}
          onChange={e => setQuantity(e.target.value)}
          step="0.0001"
          min="0"
          style={{
            background: '#1a1a2e',
            color: '#e0e0e0',
            border: '1px solid #2a2a4a',
            padding: '0.4rem 0.6rem',
            width: '180px',
          }}
        />
        <button
          type="submit"
          style={{
            background: side === 'buy' ? '#00d4aa' : '#ff4444',
            color: '#0f0f1a',
            border: 'none',
            padding: '0.4rem 1rem',
            cursor: 'pointer',
            fontFamily: 'monospace',
            fontWeight: 'bold',
          }}
        >
          {side === 'buy' ? 'Buy' : 'Sell'}
        </button>
        {error && <span style={{ color: '#ff4444', fontSize: '0.875rem' }}>{error}</span>}
      </form>

      <h2 style={{ fontSize: '0.9rem', opacity: 0.5, marginBottom: '0.75rem' }}>Trade History</h2>
      {trades.length === 0 ? (
        <p style={{ opacity: 0.4 }}>No trades yet.</p>
      ) : (
        <table style={{ width: '100%', borderCollapse: 'collapse', maxWidth: '600px', fontSize: '0.9rem' }}>
          <thead>
            <tr style={{ opacity: 0.5, textAlign: 'left' }}>
              <th style={{ paddingBottom: '0.5rem' }}>Side</th>
              <th style={{ paddingBottom: '0.5rem' }}>Quantity</th>
              <th style={{ paddingBottom: '0.5rem' }}>Price</th>
              <th style={{ paddingBottom: '0.5rem' }}>Total</th>
            </tr>
          </thead>
          <tbody>
            {trades.map(t => (
              <tr key={t.id} style={{ borderTop: '1px solid #2a2a4a' }}>
                <td
                  style={{
                    padding: '0.4rem 0.5rem 0.4rem 0',
                    color: t.side === 'buy' ? '#00d4aa' : '#ff4444',
                  }}
                >
                  {t.side}
                </td>
                <td style={{ padding: '0.4rem 0.5rem' }}>{t.quantity.toFixed(6)}</td>
                <td style={{ padding: '0.4rem 0.5rem' }}>${fmt(t.price)}</td>
                <td style={{ padding: '0.4rem 0.5rem' }}>${fmt(t.total)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}

function Stat({ label, value, color = '#e0e0e0' }: { label: string; value: string; color?: string }) {
  return (
    <div>
      <div style={{ fontSize: '0.75rem', opacity: 0.5, marginBottom: '0.2rem' }}>{label}</div>
      <div style={{ fontSize: '1.25rem', fontWeight: 'bold', color }}>{value}</div>
    </div>
  )
}
```

- [ ] **Step 4: Update App.tsx**

Replace `web/src/App.tsx` entirely:
```tsx
import { useState } from 'react'
import { Dashboard } from './pages/Dashboard'
import { Portfolio } from './pages/Portfolio'
import { Nav } from './components/Nav'

type Page = 'dashboard' | 'portfolio'

function App() {
  const [page, setPage] = useState<Page>('dashboard')

  return (
    <div
      style={{
        padding: '1.5rem',
        fontFamily: 'monospace',
        background: '#0f0f1a',
        minHeight: '100vh',
        color: '#e0e0e0',
      }}
    >
      <h1 style={{ marginBottom: '1rem' }}>Trader</h1>
      <Nav active={page} onChange={setPage} />
      {page === 'dashboard' ? <Dashboard /> : <Portfolio />}
    </div>
  )
}

export default App
```

- [ ] **Step 5: Update Dashboard.tsx — remove outer shell**

The current `web/src/pages/Dashboard.tsx` wraps everything in:
```tsx
<div style={{ padding: '1.5rem', fontFamily: 'monospace', background: '#0f0f1a', minHeight: '100vh', color: '#e0e0e0' }}>
  <h1 style={{ marginBottom: '0.5rem' }}>Trader</h1>
  ...content...
</div>
```

Replace `web/src/pages/Dashboard.tsx` entirely — keep all existing logic and state, but remove the outer `<div>` shell and `<h1>`. The content root becomes `<div>` without the background/padding styles:
```tsx
import { useState } from 'react'
import type { FormEvent } from 'react'
import { useMarketStream } from '../hooks/useMarketStream'
import { useAlerts } from '../hooks/useAlerts'
import { Chart } from '../components/Chart'

export function Dashboard() {
  const wsUrl = import.meta.env.VITE_WS_URL ?? 'ws://localhost:8080/ws'
  const { tick, connected } = useMarketStream(wsUrl)
  const { alerts, createAlert, deleteAlert } = useAlerts()
  const [targetPrice, setTargetPrice] = useState('')
  const [direction, setDirection] = useState<'above' | 'below'>('below')

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    const price = parseFloat(targetPrice)
    if (!isNaN(price) && price > 0) {
      await createAlert('BTCUSDT', price, direction)
      setTargetPrice('')
    }
  }

  return (
    <div>
      <p style={{ color: connected ? '#00d4aa' : '#ff4444', marginBottom: '1rem' }}>
        {connected ? '● Connected' : '○ Disconnected'}
      </p>

      {tick && (
        <div style={{ marginBottom: '1.5rem' }}>
          <div style={{ fontSize: '0.9rem', opacity: 0.6 }}>{tick.symbol}</div>
          <div style={{ fontSize: '2.5rem', fontWeight: 'bold' }}>
            ${parseFloat(tick.price).toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
          </div>
        </div>
      )}

      <Chart tick={tick} />

      <div style={{ marginTop: '2rem' }}>
        <h2 style={{ fontSize: '1rem', marginBottom: '1rem' }}>Price Alerts</h2>

        <form onSubmit={handleSubmit} style={{ display: 'flex', gap: '0.5rem', marginBottom: '1.5rem', flexWrap: 'wrap' }}>
          <select
            value={direction}
            onChange={e => setDirection(e.target.value as 'above' | 'below')}
            style={{ background: '#1a1a2e', color: '#e0e0e0', border: '1px solid #2a2a4a', padding: '0.4rem 0.6rem' }}
          >
            <option value="below">Below</option>
            <option value="above">Above</option>
          </select>
          <input
            type="number"
            placeholder="Target price (USD)"
            value={targetPrice}
            onChange={e => setTargetPrice(e.target.value)}
            style={{ background: '#1a1a2e', color: '#e0e0e0', border: '1px solid #2a2a4a', padding: '0.4rem 0.6rem', width: '200px' }}
          />
          <button
            type="submit"
            style={{ background: '#00d4aa', color: '#0f0f1a', border: 'none', padding: '0.4rem 1rem', cursor: 'pointer', fontFamily: 'monospace', fontWeight: 'bold' }}
          >
            Set Alert
          </button>
        </form>

        {alerts.length === 0 ? (
          <p style={{ opacity: 0.4 }}>No alerts. Set one above.</p>
        ) : (
          <table style={{ width: '100%', borderCollapse: 'collapse', maxWidth: '520px' }}>
            <thead>
              <tr style={{ opacity: 0.5, textAlign: 'left', fontSize: '0.85rem' }}>
                <th style={{ paddingBottom: '0.5rem' }}>Direction</th>
                <th style={{ paddingBottom: '0.5rem' }}>Target</th>
                <th style={{ paddingBottom: '0.5rem' }}>Status</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {alerts.map(a => (
                <tr key={a.id} style={{ borderTop: '1px solid #2a2a4a' }}>
                  <td style={{ padding: '0.5rem 0.5rem 0.5rem 0' }}>{a.direction}</td>
                  <td style={{ padding: '0.5rem' }}>
                    ${a.target_price.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                  </td>
                  <td style={{ padding: '0.5rem', color: a.triggered_at ? '#00d4aa' : '#e0e0e0' }}>
                    {a.triggered_at ? '✓ Triggered' : '◉ Watching'}
                  </td>
                  <td style={{ padding: '0.5rem 0' }}>
                    <button
                      onClick={() => deleteAlert(a.id)}
                      style={{ background: 'none', border: 'none', color: '#ff4444', cursor: 'pointer', fontFamily: 'monospace', fontSize: '1rem' }}
                    >
                      ✕
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  )
}
```

- [ ] **Step 6: TypeScript check and build**

```bash
cd web && npx tsc --noEmit && npm run build
```

Expected: no errors.

- [ ] **Step 7: Commit**

```bash
git add web/src/
git commit -m "feat: add Portfolio page with buy/sell form, P&L display, and tab navigation"
```
