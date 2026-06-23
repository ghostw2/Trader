# Phase 2 — Price Alerts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let users set price targets for BTC/USDT and get a live in-app notification when the price crosses them.

**Architecture:** SQLite (via `modernc.org/sqlite`, pure Go) stores alerts. A background worker goroutine subscribes to the hub and checks active alerts on every tick, marking them triggered. A REST API (POST/GET/DELETE `/api/alerts`) manages alerts. The React dashboard gains an alert form + live-updating list. The Vite dev server proxies `/api` to the Go backend so frontend code uses relative URLs.

**Tech Stack:** Go 1.24.4, `modernc.org/sqlite` (pure-Go, no CGO), Chi router (existing), React 19 + TypeScript + Vite proxy

## Global Constraints

- Module: `github.com/menribardhi/trader`
- SQLite driver: `modernc.org/sqlite` — blank-imported as `_ "modernc.org/sqlite"`, opened with `sql.Open("sqlite", path)`
- DB file path: `"./trader.db"` (relative to working directory, auto-created)
- Alert `direction` values: exactly `"above"` or `"below"` — enforced by SQL `CHECK` constraint and API validation
- All timestamps: Unix milliseconds (`int64`), matching Phase 1 `Tick.Timestamp` convention
- Frontend uses relative URL `/api/alerts` (no hardcoded port); Vite proxy forwards to `http://localhost:8080`
- `go test ./...` must pass after every task

---

### Task 1: Alert model + SQLite dependency

**Files:**
- Modify: `internal/models/types.go`

**Interfaces:**
- Produces: `models.Alert` struct consumed by all later tasks

- [ ] **Step 1: Add Alert struct to models**

Replace `internal/models/types.go` entirely:
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
	Direction   string  `json:"direction"`    // "above" or "below"
	CreatedAt   int64   `json:"created_at"`   // Unix ms
	TriggeredAt *int64  `json:"triggered_at"` // null until triggered
}
```

- [ ] **Step 2: Add SQLite dependency**

```bash
go get modernc.org/sqlite@latest
go mod tidy
```

Expected: `go.mod` lists `modernc.org/sqlite` as a direct dependency. No errors.

- [ ] **Step 3: Verify build**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add internal/models/types.go go.mod go.sum
git commit -m "feat: add Alert model and modernc.org/sqlite dependency"
```

---

### Task 2: SQLite database layer (TDD)

**Files:**
- Create: `internal/db/db.go`
- Create: `internal/db/alerts.go`
- Create: `internal/db/db_test.go`

**Interfaces:**
- Consumes: `models.Alert` from Task 1
- Produces (exact signatures — used verbatim in Tasks 3, 4, 5):
  - `db.Open(path string) (*sql.DB, error)`
  - `db.CreateAlert(sqldb *sql.DB, symbol, direction string, targetPrice float64) (models.Alert, error)`
  - `db.ListAlerts(sqldb *sql.DB) ([]models.Alert, error)` — always a slice, never nil
  - `db.DeleteAlert(sqldb *sql.DB, id int64) error`
  - `db.MarkTriggered(sqldb *sql.DB, id, at int64) error`

- [ ] **Step 1: Write failing tests**

Create `internal/db/db_test.go`:
```go
package db_test

import (
	"database/sql"
	"testing"

	"github.com/menribardhi/trader/internal/db"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	sqldb, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { sqldb.Close() })
	return sqldb
}

func TestCreateAndListAlerts(t *testing.T) {
	sqldb := openTestDB(t)
	alert, err := db.CreateAlert(sqldb, "BTCUSDT", "below", 60000.0)
	if err != nil {
		t.Fatal(err)
	}
	if alert.ID == 0 {
		t.Error("expected non-zero ID")
	}
	list, err := db.ListAlerts(sqldb)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(list))
	}
	if list[0].TriggeredAt != nil {
		t.Error("new alert must not be triggered")
	}
}

func TestListAlertsEmptyIsSlice(t *testing.T) {
	sqldb := openTestDB(t)
	list, err := db.ListAlerts(sqldb)
	if err != nil {
		t.Fatal(err)
	}
	if list == nil {
		t.Error("ListAlerts must return empty slice, not nil")
	}
}

func TestDeleteAlert(t *testing.T) {
	sqldb := openTestDB(t)
	alert, _ := db.CreateAlert(sqldb, "BTCUSDT", "above", 70000.0)
	if err := db.DeleteAlert(sqldb, alert.ID); err != nil {
		t.Fatal(err)
	}
	list, _ := db.ListAlerts(sqldb)
	if len(list) != 0 {
		t.Errorf("expected 0 alerts after delete, got %d", len(list))
	}
}

func TestMarkTriggered(t *testing.T) {
	sqldb := openTestDB(t)
	alert, _ := db.CreateAlert(sqldb, "BTCUSDT", "below", 60000.0)
	const ts = int64(1700000000000)
	if err := db.MarkTriggered(sqldb, alert.ID, ts); err != nil {
		t.Fatal(err)
	}
	list, _ := db.ListAlerts(sqldb)
	if list[0].TriggeredAt == nil {
		t.Error("expected TriggeredAt to be set")
	}
	if *list[0].TriggeredAt != ts {
		t.Errorf("wrong triggered_at: got %d, want %d", *list[0].TriggeredAt, ts)
	}
}
```

- [ ] **Step 2: Run tests — expect failure**

```bash
go test ./internal/db/...
```

Expected: FAIL — package not found.

- [ ] **Step 3: Implement db.go**

Create `internal/db/db.go`:
```go
package db

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

func Open(path string) (*sql.DB, error) {
	sqldb, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if err := migrate(sqldb); err != nil {
		sqldb.Close()
		return nil, err
	}
	return sqldb, nil
}

func migrate(sqldb *sql.DB) error {
	_, err := sqldb.Exec(`
		CREATE TABLE IF NOT EXISTS alerts (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			symbol       TEXT    NOT NULL,
			target_price REAL    NOT NULL,
			direction    TEXT    NOT NULL CHECK (direction IN ('above','below')),
			created_at   INTEGER NOT NULL,
			triggered_at INTEGER
		)
	`)
	return err
}
```

- [ ] **Step 4: Implement alerts.go**

Create `internal/db/alerts.go`:
```go
package db

import (
	"database/sql"
	"time"

	"github.com/menribardhi/trader/internal/models"
)

func CreateAlert(sqldb *sql.DB, symbol, direction string, targetPrice float64) (models.Alert, error) {
	now := time.Now().UnixMilli()
	res, err := sqldb.Exec(
		`INSERT INTO alerts (symbol, target_price, direction, created_at) VALUES (?,?,?,?)`,
		symbol, targetPrice, direction, now,
	)
	if err != nil {
		return models.Alert{}, err
	}
	id, _ := res.LastInsertId()
	return models.Alert{
		ID: id, Symbol: symbol, TargetPrice: targetPrice,
		Direction: direction, CreatedAt: now,
	}, nil
}

func ListAlerts(sqldb *sql.DB) ([]models.Alert, error) {
	rows, err := sqldb.Query(
		`SELECT id, symbol, target_price, direction, created_at, triggered_at FROM alerts ORDER BY id DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	alerts := []models.Alert{}
	for rows.Next() {
		var a models.Alert
		var triggeredAt sql.NullInt64
		if err := rows.Scan(&a.ID, &a.Symbol, &a.TargetPrice, &a.Direction, &a.CreatedAt, &triggeredAt); err != nil {
			return nil, err
		}
		if triggeredAt.Valid {
			v := triggeredAt.Int64
			a.TriggeredAt = &v
		}
		alerts = append(alerts, a)
	}
	return alerts, rows.Err()
}

func DeleteAlert(sqldb *sql.DB, id int64) error {
	_, err := sqldb.Exec(`DELETE FROM alerts WHERE id = ?`, id)
	return err
}

func MarkTriggered(sqldb *sql.DB, id, at int64) error {
	_, err := sqldb.Exec(`UPDATE alerts SET triggered_at = ? WHERE id = ?`, at, id)
	return err
}
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/db/... -v
```

Expected: 4 tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/db/
git commit -m "feat: add SQLite database layer with alerts CRUD"
```

---

### Task 3: REST API handlers + Vite proxy (TDD)

**Files:**
- Modify: `internal/api/server.go` — add `db *sql.DB` field, update `New`, add routes
- Create: `internal/api/alerts_handler.go`
- Create: `internal/api/alerts_handler_test.go`
- Modify: `internal/api/ws_handler_test.go` — fix `api.New(h)` → `api.New(h, nil)`
- Modify: `web/vite.config.ts` — add `/api` proxy

**Interfaces:**
- Consumes: `db.CreateAlert`, `db.ListAlerts`, `db.DeleteAlert` from Task 2
- Produces: `api.New(h *hub.Hub, sqldb *sql.DB) *Server` — BREAKING: all callers must pass `nil` when DB unneeded

- [ ] **Step 1: Write failing tests**

Create `internal/api/alerts_handler_test.go`:
```go
package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/menribardhi/trader/internal/api"
	"github.com/menribardhi/trader/internal/db"
	"github.com/menribardhi/trader/internal/hub"
	"github.com/menribardhi/trader/internal/models"
)

func newAlertTestServer(t *testing.T) (*httptest.Server, func()) {
	t.Helper()
	ticks := make(chan models.Tick, 1)
	h := hub.New(ticks)
	sqldb, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	srv := httptest.NewServer(api.New(h, sqldb))
	return srv, func() { srv.Close(); sqldb.Close() }
}

func TestCreateAlert(t *testing.T) {
	srv, cleanup := newAlertTestServer(t)
	defer cleanup()

	body, _ := json.Marshal(map[string]interface{}{
		"symbol": "BTCUSDT", "target_price": 60000.0, "direction": "below",
	})
	resp, err := http.Post(srv.URL+"/api/alerts", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected 201, got %d", resp.StatusCode)
	}
	var alert models.Alert
	json.NewDecoder(resp.Body).Decode(&alert)
	if alert.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if alert.Symbol != "BTCUSDT" {
		t.Errorf("got symbol %q, want BTCUSDT", alert.Symbol)
	}
}

func TestCreateAlertBadDirection(t *testing.T) {
	srv, cleanup := newAlertTestServer(t)
	defer cleanup()

	body, _ := json.Marshal(map[string]interface{}{
		"symbol": "BTCUSDT", "target_price": 60000.0, "direction": "sideways",
	})
	resp, _ := http.Post(srv.URL+"/api/alerts", "application/json", bytes.NewReader(body))
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid direction, got %d", resp.StatusCode)
	}
}

func TestListAlerts(t *testing.T) {
	srv, cleanup := newAlertTestServer(t)
	defer cleanup()

	body, _ := json.Marshal(map[string]interface{}{
		"symbol": "BTCUSDT", "target_price": 60000.0, "direction": "below",
	})
	http.Post(srv.URL+"/api/alerts", "application/json", bytes.NewReader(body))

	resp, err := http.Get(srv.URL + "/api/alerts")
	if err != nil {
		t.Fatal(err)
	}
	var alerts []models.Alert
	json.NewDecoder(resp.Body).Decode(&alerts)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
}

func TestDeleteAlert(t *testing.T) {
	srv, cleanup := newAlertTestServer(t)
	defer cleanup()

	body, _ := json.Marshal(map[string]interface{}{
		"symbol": "BTCUSDT", "target_price": 70000.0, "direction": "above",
	})
	resp, _ := http.Post(srv.URL+"/api/alerts", "application/json", bytes.NewReader(body))
	var alert models.Alert
	json.NewDecoder(resp.Body).Decode(&alert)

	req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/alerts/%d", srv.URL, alert.ID), nil)
	delResp, _ := http.DefaultClient.Do(req)
	if delResp.StatusCode != http.StatusNoContent {
		t.Errorf("expected 204, got %d", delResp.StatusCode)
	}

	listResp, _ := http.Get(srv.URL + "/api/alerts")
	var alerts []models.Alert
	json.NewDecoder(listResp.Body).Decode(&alerts)
	if len(alerts) != 0 {
		t.Errorf("expected 0 after delete, got %d", len(alerts))
	}
}
```

- [ ] **Step 2: Run tests — expect failure**

```bash
go test ./internal/api/... 2>&1 | head -10
```

Expected: compile errors — `api.New` wrong argument count.

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
)

type Server struct {
	hub    *hub.Hub
	db     *sql.DB
	router chi.Router
}

func New(h *hub.Hub, sqldb *sql.DB) *Server {
	s := &Server{hub: h, db: sqldb}
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Get("/ws", s.handleWS)
	r.Route("/api", func(r chi.Router) {
		r.Post("/alerts", s.handleCreateAlert)
		r.Get("/alerts", s.handleListAlerts)
		r.Delete("/alerts/{id}", s.handleDeleteAlert)
	})
	s.router = r
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}
```

- [ ] **Step 4: Update ws_handler_test.go**

In `internal/api/ws_handler_test.go`, find the line:
```go
srv := httptest.NewServer(api.New(h))
```
Change it to:
```go
srv := httptest.NewServer(api.New(h, nil))
```

- [ ] **Step 5: Create alerts_handler.go**

Create `internal/api/alerts_handler.go`:
```go
package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	dbpkg "github.com/menribardhi/trader/internal/db"
)

type createAlertRequest struct {
	Symbol      string  `json:"symbol"`
	TargetPrice float64 `json:"target_price"`
	Direction   string  `json:"direction"`
}

func (s *Server) handleCreateAlert(w http.ResponseWriter, r *http.Request) {
	var req createAlertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Direction != "above" && req.Direction != "below" {
		http.Error(w, `direction must be "above" or "below"`, http.StatusBadRequest)
		return
	}
	if req.Symbol == "" || req.TargetPrice <= 0 {
		http.Error(w, "symbol and positive target_price required", http.StatusBadRequest)
		return
	}
	alert, err := dbpkg.CreateAlert(s.db, req.Symbol, req.Direction, req.TargetPrice)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(alert)
}

func (s *Server) handleListAlerts(w http.ResponseWriter, r *http.Request) {
	alerts, err := dbpkg.ListAlerts(s.db)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(alerts)
}

func (s *Server) handleDeleteAlert(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := dbpkg.DeleteAlert(s.db, id); err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 6: Add Vite proxy**

Replace `web/vite.config.ts` entirely:
```typescript
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
})
```

- [ ] **Step 7: Run all API tests**

```bash
go test ./internal/api/... -v
```

Expected: 5 tests pass (1 ws handler + 4 alert handler).

- [ ] **Step 8: Commit**

```bash
git add internal/api/ web/vite.config.ts
git commit -m "feat: add alert REST API and Vite /api proxy"
```

---

### Task 4: Alert worker (TDD)

**Files:**
- Create: `internal/worker/alerts.go`
- Create: `internal/worker/alerts_test.go`

**Interfaces:**
- Consumes: `hub.Hub.Subscribe()`, `hub.Hub.Unsubscribe()`, `db.ListAlerts()`, `db.MarkTriggered()` from Task 2
- Produces:
  - `worker.NewAlertChecker(h *hub.Hub, sqldb *sql.DB) *AlertChecker`
  - `(*AlertChecker).Run(ctx context.Context)` — blocks until ctx cancelled

- [ ] **Step 1: Write failing tests**

Create `internal/worker/alerts_test.go`:
```go
package worker_test

import (
	"context"
	"testing"
	"time"

	"github.com/menribardhi/trader/internal/db"
	"github.com/menribardhi/trader/internal/hub"
	"github.com/menribardhi/trader/internal/models"
	"github.com/menribardhi/trader/internal/worker"
)

func setupWorkerTest(t *testing.T) (*hub.Hub, chan models.Tick, context.CancelFunc) {
	t.Helper()
	ticks := make(chan models.Tick, 4)
	h := hub.New(ticks)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	go h.Run(ctx)
	return h, ticks, cancel
}

func TestAlertCheckerTriggersBelow(t *testing.T) {
	h, ticks, cancel := setupWorkerTest(t)
	defer cancel()

	sqldb, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer sqldb.Close()
	if _, err = db.CreateAlert(sqldb, "BTCUSDT", "below", 60000.0); err != nil {
		t.Fatal(err)
	}

	ctx, checkerCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer checkerCancel()
	go worker.NewAlertChecker(h, sqldb).Run(ctx)
	time.Sleep(10 * time.Millisecond)

	ticks <- models.Tick{Symbol: "BTCUSDT", Price: "59000.00", Timestamp: 1700000000000}
	time.Sleep(200 * time.Millisecond)

	alerts, _ := db.ListAlerts(sqldb)
	if len(alerts) == 0 {
		t.Fatal("expected 1 alert")
	}
	if alerts[0].TriggeredAt == nil {
		t.Error("alert must trigger when price drops below target")
	}
}

func TestAlertCheckerNoTriggerAboveThreshold(t *testing.T) {
	h, ticks, cancel := setupWorkerTest(t)
	defer cancel()

	sqldb, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer sqldb.Close()
	if _, err = db.CreateAlert(sqldb, "BTCUSDT", "below", 60000.0); err != nil {
		t.Fatal(err)
	}

	ctx, checkerCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer checkerCancel()
	go worker.NewAlertChecker(h, sqldb).Run(ctx)
	time.Sleep(10 * time.Millisecond)

	ticks <- models.Tick{Symbol: "BTCUSDT", Price: "65000.00", Timestamp: 1700000000001}
	time.Sleep(200 * time.Millisecond)

	alerts, _ := db.ListAlerts(sqldb)
	if alerts[0].TriggeredAt != nil {
		t.Error("alert must NOT trigger when price is above the threshold")
	}
}

func TestAlertCheckerTriggersAbove(t *testing.T) {
	h, ticks, cancel := setupWorkerTest(t)
	defer cancel()

	sqldb, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer sqldb.Close()
	if _, err = db.CreateAlert(sqldb, "BTCUSDT", "above", 70000.0); err != nil {
		t.Fatal(err)
	}

	ctx, checkerCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer checkerCancel()
	go worker.NewAlertChecker(h, sqldb).Run(ctx)
	time.Sleep(10 * time.Millisecond)

	ticks <- models.Tick{Symbol: "BTCUSDT", Price: "71000.00", Timestamp: 1700000000002}
	time.Sleep(200 * time.Millisecond)

	alerts, _ := db.ListAlerts(sqldb)
	if alerts[0].TriggeredAt == nil {
		t.Error("alert must trigger when price rises above target")
	}
}
```

- [ ] **Step 2: Run tests — expect failure**

```bash
go test ./internal/worker/... 2>&1 | head -5
```

Expected: FAIL — package not found.

- [ ] **Step 3: Implement the worker**

Create `internal/worker/alerts.go`:
```go
package worker

import (
	"context"
	"database/sql"
	"strconv"
	"time"

	dbpkg "github.com/menribardhi/trader/internal/db"
	"github.com/menribardhi/trader/internal/hub"
	"github.com/menribardhi/trader/internal/models"
	"github.com/rs/zerolog/log"
)

type AlertChecker struct {
	hub *hub.Hub
	db  *sql.DB
}

func NewAlertChecker(h *hub.Hub, sqldb *sql.DB) *AlertChecker {
	return &AlertChecker{hub: h, db: sqldb}
}

func (ac *AlertChecker) Run(ctx context.Context) {
	sub := ac.hub.Subscribe()
	defer ac.hub.Unsubscribe(sub)
	for {
		select {
		case tick, ok := <-sub:
			if !ok {
				return
			}
			ac.check(tick)
		case <-ctx.Done():
			return
		}
	}
}

func (ac *AlertChecker) check(tick models.Tick) {
	price, err := strconv.ParseFloat(tick.Price, 64)
	if err != nil {
		return
	}
	alerts, err := dbpkg.ListAlerts(ac.db)
	if err != nil {
		return
	}
	now := time.Now().UnixMilli()
	for _, a := range alerts {
		if a.TriggeredAt != nil {
			continue
		}
		hit := (a.Direction == "above" && price >= a.TargetPrice) ||
			(a.Direction == "below" && price <= a.TargetPrice)
		if !hit {
			continue
		}
		if err := dbpkg.MarkTriggered(ac.db, a.ID, now); err != nil {
			log.Error().Err(err).Int64("id", a.ID).Msg("worker: mark triggered failed")
			continue
		}
		log.Info().
			Str("symbol", tick.Symbol).
			Float64("price", price).
			Float64("target", a.TargetPrice).
			Str("direction", a.Direction).
			Msg("alert triggered")
	}
}
```

- [ ] **Step 4: Run all tests**

```bash
go test ./... 2>&1
```

Expected: all packages pass.

- [ ] **Step 5: Commit**

```bash
git add internal/worker/
git commit -m "feat: add alert worker — checks price on every tick"
```

---

### Task 5: Wire main.go

**Files:**
- Modify: `cmd/trader/main.go`

**Interfaces:**
- Consumes: `db.Open`, `worker.NewAlertChecker`, updated `api.New(h *hub.Hub, sqldb *sql.DB)`

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

	ticks := make(chan models.Tick, 64)
	h := hub.New(ticks)
	client := binance.New("BTCUSDT", ticks)

	go client.Run(ctx)
	go h.Run(ctx)
	go worker.NewAlertChecker(h, sqldb).Run(ctx)

	httpSrv := &http.Server{Addr: ":8080", Handler: api.New(h, sqldb)}
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
git commit -m "feat: wire SQLite DB and alert worker into main"
```

---

### Task 6: Frontend alert UI

**Files:**
- Create: `web/src/hooks/useAlerts.ts`
- Modify: `web/src/pages/Dashboard.tsx`

**Interfaces:**
- Consumes: `/api/alerts` REST endpoints via Vite proxy (Task 3)
- Produces: `useAlerts()` → `{ alerts: Alert[], createAlert, deleteAlert }`

- [ ] **Step 1: Create useAlerts.ts**

Create `web/src/hooks/useAlerts.ts`:
```typescript
import { useState, useEffect, useCallback } from 'react'

export interface Alert {
  id: number
  symbol: string
  target_price: number
  direction: 'above' | 'below'
  created_at: number
  triggered_at: number | null
}

export function useAlerts() {
  const [alerts, setAlerts] = useState<Alert[]>([])

  const fetchAlerts = useCallback(async () => {
    const res = await fetch('/api/alerts')
    if (res.ok) setAlerts(await res.json())
  }, [])

  useEffect(() => {
    fetchAlerts()
    const id = setInterval(fetchAlerts, 5000)
    return () => clearInterval(id)
  }, [fetchAlerts])

  const createAlert = useCallback(async (
    symbol: string,
    targetPrice: number,
    direction: 'above' | 'below',
  ) => {
    const res = await fetch('/api/alerts', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ symbol, target_price: targetPrice, direction }),
    })
    if (res.ok) await fetchAlerts()
  }, [fetchAlerts])

  const deleteAlert = useCallback(async (id: number) => {
    await fetch(`/api/alerts/${id}`, { method: 'DELETE' })
    await fetchAlerts()
  }, [fetchAlerts])

  return { alerts, createAlert, deleteAlert }
}
```

- [ ] **Step 2: Replace Dashboard.tsx**

Replace `web/src/pages/Dashboard.tsx` entirely:
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

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    const price = parseFloat(targetPrice)
    if (!isNaN(price) && price > 0) {
      createAlert('BTCUSDT', price, direction)
      setTargetPrice('')
    }
  }

  return (
    <div style={{ padding: '1.5rem', fontFamily: 'monospace', background: '#0f0f1a', minHeight: '100vh', color: '#e0e0e0' }}>
      <h1 style={{ marginBottom: '0.5rem' }}>Trader</h1>
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

- [ ] **Step 3: TypeScript check and build**

```bash
cd web && npx tsc --noEmit && npm run build
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add web/src/hooks/useAlerts.ts web/src/pages/Dashboard.tsx
git commit -m "feat: add price alert UI — form, list, and 5s status polling"
```
