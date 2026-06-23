# Trader Phase 1 — Live Price Dashboard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Go backend that streams real-time BTC/USDT prices from Binance over WebSocket and displays them in a React browser dashboard with a live chart.

**Architecture:** A single Go binary runs three concurrent goroutines: a Binance WebSocket client that reads price ticks, a hub that fans ticks out to all connected browser clients, and a Chi HTTP server that upgrades browser connections to WebSocket. The React frontend uses a custom hook to connect to the Go server and feeds ticks into a TradingView lightweight-charts line chart.

**Tech Stack:** Go 1.22+, `github.com/go-chi/chi/v5`, `github.com/gorilla/websocket`, React 18 + TypeScript, Vite, `lightweight-charts` v4

## Global Constraints

- Module path: `github.com/menribardhi/trader`
- Go backend listens on `:8080`
- Vite dev server runs on `:5173`
- Only BTC/USDT pair in Phase 1 (hardcoded)
- No authentication required
- No database in Phase 1
- All Go packages live under `internal/`

---

## File Map

| File | Responsibility |
|---|---|
| `go.mod` | Module declaration and dependencies |
| `internal/models/types.go` | Shared types: `Tick`, `Config` |
| `internal/binance/client.go` | WS client — reads Binance ticks, sends to channel, reconnects |
| `internal/binance/client_test.go` | Tests with mock WS server |
| `internal/hub/hub.go` | Broadcast hub — fans ticks to all subscriber channels |
| `internal/hub/hub_test.go` | Tests for fan-out and subscribe/unsubscribe |
| `internal/api/server.go` | Chi router setup |
| `internal/api/ws_handler.go` | Upgrades HTTP to WS, reads from hub, writes to browser |
| `internal/api/ws_handler_test.go` | Tests WS upgrade and message delivery |
| `cmd/trader/main.go` | Wires all pieces, starts goroutines |
| `Makefile` | `make dev`, `make test`, `make build` targets |
| `web/` | Vite scaffold (npm creates this) |
| `web/src/hooks/useMarketStream.ts` | WS hook — connects, retries, exposes tick state |
| `web/src/components/Chart.tsx` | lightweight-charts line chart |
| `web/src/pages/Dashboard.tsx` | Price display + chart |
| `web/src/App.tsx` | Root component (replace Vite default) |

---

### Task 1: Project Scaffold

**Files:**
- Create: `go.mod`
- Create: `Makefile`
- Create: directories `cmd/trader/`, `internal/models/`, `internal/binance/`, `internal/hub/`, `internal/api/`

**Interfaces:**
- Produces: module `github.com/menribardhi/trader`, `make dev` / `make test` / `make build` targets

- [ ] **Step 1: Initialize git and Go module**

Run from `/Users/menribardhi/Documents/Projects/go-projects/trader`:
```bash
git init
go mod init github.com/menribardhi/trader
```
Expected: `go.mod` created with `module github.com/menribardhi/trader` and `go 1.22`

- [ ] **Step 2: Create directory structure**

```bash
mkdir -p cmd/trader internal/models internal/binance internal/hub internal/api
```

- [ ] **Step 3: Install Go dependencies**

```bash
go get github.com/go-chi/chi/v5
go get github.com/gorilla/websocket
go get github.com/rs/zerolog
```
Expected: `go.sum` created, `go.mod` updated with require blocks.

- [ ] **Step 4: Write Makefile**

Create `Makefile`:
```makefile
.PHONY: dev test build

build:
	go build ./...

test:
	go test ./...

dev:
	@trap 'kill %1 %2' INT; \
	go run ./cmd/trader & \
	cd web && npm run dev & \
	wait
```

- [ ] **Step 5: Commit scaffold**

```bash
git add go.mod go.sum Makefile
git add cmd/ internal/
git commit -m "chore: project scaffold"
```

---

### Task 2: Shared Models

**Files:**
- Create: `internal/models/types.go`

**Interfaces:**
- Produces: `models.Tick{Symbol string, Price string, Timestamp int64}`, `models.Config{Symbol string, Port int}`
- Consumed by: binance client, hub, ws_handler

- [ ] **Step 1: Write types**

Create `internal/models/types.go`:
```go
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
```

- [ ] **Step 2: Verify it compiles**

```bash
go build ./internal/models/...
```
Expected: no output (success).

- [ ] **Step 3: Commit**

```bash
git add internal/models/types.go
git commit -m "feat: add shared Tick and Config types"
```

---

### Task 3: Binance WebSocket Client

**Files:**
- Create: `internal/binance/client.go`
- Create: `internal/binance/client_test.go`

**Interfaces:**
- Consumes: `models.Tick` (from `internal/models/types.go`)
- Produces:
  - `binance.New(symbol string, out chan<- models.Tick) *Client`
  - `binance.NewWithURL(url string, out chan<- models.Tick) *Client` (for tests)
  - `(*Client).Run(ctx context.Context)` — blocking; reconnects with exponential backoff

- [ ] **Step 1: Write the failing test**

Create `internal/binance/client_test.go`:
```go
package binance_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/menribardhi/trader/internal/binance"
	"github.com/menribardhi/trader/internal/models"
)

func TestClientReceivesTick(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		up := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
		conn, err := up.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()

		payload, _ := json.Marshal(map[string]interface{}{
			"s": "BTCUSDT",
			"c": "50000.00",
			"E": int64(1700000000000),
		})
		conn.WriteMessage(websocket.TextMessage, payload)
		time.Sleep(200 * time.Millisecond)
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	out := make(chan models.Tick, 1)
	client := binance.NewWithURL(wsURL, out)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	go client.Run(ctx)

	select {
	case tick := <-out:
		if tick.Symbol != "BTCUSDT" {
			t.Errorf("symbol: got %q want %q", tick.Symbol, "BTCUSDT")
		}
		if tick.Price != "50000.00" {
			t.Errorf("price: got %q want %q", tick.Price, "50000.00")
		}
		if tick.Timestamp != 1700000000000 {
			t.Errorf("timestamp: got %d want %d", tick.Timestamp, 1700000000000)
		}
	case <-ctx.Done():
		t.Fatal("timeout: no tick received")
	}
}

func TestClientReconnectsOnDisconnect(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		up := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
		conn, _ := up.Upgrade(w, r, nil)
		conn.Close()
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	out := make(chan models.Tick, 1)
	client := binance.NewWithURL(wsURL, out)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	client.Run(ctx)

	if calls < 2 {
		t.Errorf("expected at least 2 connection attempts, got %d", calls)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/binance/... -v
```
Expected: compilation error — package `binance` not found.

- [ ] **Step 3: Write the client implementation**

Create `internal/binance/client.go`:
```go
package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/menribardhi/trader/internal/models"
	"github.com/rs/zerolog/log"
)

const binanceStreamURL = "wss://stream.binance.com:9443/ws/%s@miniTicker"

type miniTickerMsg struct {
	Symbol string `json:"s"`
	Close  string `json:"c"`
	Time   int64  `json:"E"`
}

// Client connects to a WebSocket price stream and writes Ticks to out.
type Client struct {
	url string
	out chan<- models.Tick
}

// New builds a Client for the given symbol using the Binance stream URL.
func New(symbol string, out chan<- models.Tick) *Client {
	return &Client{
		url: fmt.Sprintf(binanceStreamURL, strings.ToLower(symbol)),
		out: out,
	}
}

// NewWithURL builds a Client with an explicit WebSocket URL (for testing).
func NewWithURL(url string, out chan<- models.Tick) *Client {
	return &Client{url: url, out: out}
}

// Run connects and reads ticks until ctx is cancelled.
// On error it reconnects with exponential backoff (1s doubling to 30s max).
func (c *Client) Run(ctx context.Context) {
	backoff := time.Second
	for {
		if err := c.connect(ctx); err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Warn().Err(err).Msgf("binance: reconnecting in %s", backoff)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return
			}
			if backoff < 30*time.Second {
				backoff *= 2
			}
			continue
		}
		backoff = time.Second
	}
}

func (c *Client) connect(ctx context.Context) error {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, c.url, nil)
	if err != nil {
		return err
	}
	defer conn.Close()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		var m miniTickerMsg
		if err := json.Unmarshal(msg, &m); err != nil {
			continue
		}
		select {
		case c.out <- models.Tick{Symbol: m.Symbol, Price: m.Close, Timestamp: m.Time}:
		case <-ctx.Done():
			return nil
		}
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/binance/... -v
```
Expected:
```
--- PASS: TestClientReceivesTick (0.20s)
--- PASS: TestClientReconnectsOnDisconnect (0.50s)
PASS
```

- [ ] **Step 5: Commit**

```bash
git add internal/binance/
git commit -m "feat: add Binance WebSocket client with auto-reconnect"
```

---

### Task 4: Broadcast Hub

**Files:**
- Create: `internal/hub/hub.go`
- Create: `internal/hub/hub_test.go`

**Interfaces:**
- Consumes: `models.Tick`, `chan models.Tick`
- Produces:
  - `hub.New(ticks chan models.Tick) *Hub`
  - `(*Hub).Run(ctx context.Context)` — reads from ticks, fans out to all subscribers
  - `(*Hub).Subscribe() chan models.Tick` — returns buffered channel (size 16)
  - `(*Hub).Unsubscribe(ch chan models.Tick)` — removes subscriber and closes channel

- [ ] **Step 1: Write the failing tests**

Create `internal/hub/hub_test.go`:
```go
package hub_test

import (
	"context"
	"testing"
	"time"

	"github.com/menribardhi/trader/internal/hub"
	"github.com/menribardhi/trader/internal/models"
)

func TestHubFansOutToMultipleSubscribers(t *testing.T) {
	ticks := make(chan models.Tick, 1)
	h := hub.New(ticks)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go h.Run(ctx)

	sub1 := h.Subscribe()
	sub2 := h.Subscribe()
	defer h.Unsubscribe(sub1)
	defer h.Unsubscribe(sub2)

	want := models.Tick{Symbol: "BTCUSDT", Price: "50000", Timestamp: 123}
	ticks <- want

	for i, sub := range []chan models.Tick{sub1, sub2} {
		select {
		case got := <-sub:
			if got != want {
				t.Errorf("subscriber %d: got %+v want %+v", i, got, want)
			}
		case <-ctx.Done():
			t.Fatalf("subscriber %d: timeout waiting for tick", i)
		}
	}
}

func TestHubDropsSlowConsumer(t *testing.T) {
	ticks := make(chan models.Tick, 16)
	h := hub.New(ticks)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go h.Run(ctx)

	_ = h.Subscribe() // subscribe but never read — should not block hub

	fast := h.Subscribe()
	defer h.Unsubscribe(fast)

	for i := 0; i < 20; i++ {
		ticks <- models.Tick{Symbol: "BTCUSDT", Price: "1", Timestamp: int64(i)}
	}

	select {
	case <-fast:
	case <-ctx.Done():
		t.Fatal("fast subscriber blocked by slow subscriber")
	}
}

func TestHubUnsubscribeClosesChannel(t *testing.T) {
	ticks := make(chan models.Tick, 1)
	h := hub.New(ticks)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go h.Run(ctx)

	sub := h.Subscribe()
	h.Unsubscribe(sub)

	select {
	case _, ok := <-sub:
		if ok {
			t.Error("expected closed channel, got open channel with value")
		}
	case <-ctx.Done():
		t.Fatal("timeout: channel was not closed")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/hub/... -v
```
Expected: compilation error — package `hub` not found.

- [ ] **Step 3: Write the hub implementation**

Create `internal/hub/hub.go`:
```go
package hub

import (
	"context"
	"sync"

	"github.com/menribardhi/trader/internal/models"
)

// Hub fans price ticks from one source channel to many subscriber channels.
type Hub struct {
	mu      sync.RWMutex
	clients map[chan models.Tick]struct{}
	ticks   chan models.Tick
}

// New creates a Hub that reads from ticks.
func New(ticks chan models.Tick) *Hub {
	return &Hub{
		clients: make(map[chan models.Tick]struct{}),
		ticks:   ticks,
	}
}

// Subscribe registers a new subscriber and returns its buffered channel.
func (h *Hub) Subscribe() chan models.Tick {
	ch := make(chan models.Tick, 16)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

// Unsubscribe removes the subscriber and closes its channel.
func (h *Hub) Unsubscribe(ch chan models.Tick) {
	h.mu.Lock()
	delete(h.clients, ch)
	h.mu.Unlock()
	close(ch)
}

// Run reads ticks and broadcasts each to all subscribers.
// Slow subscribers are dropped (non-blocking send).
func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case tick, ok := <-h.ticks:
			if !ok {
				return
			}
			h.mu.RLock()
			for ch := range h.clients {
				select {
				case ch <- tick:
				default:
				}
			}
			h.mu.RUnlock()
		case <-ctx.Done():
			return
		}
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/hub/... -v
```
Expected:
```
--- PASS: TestHubFansOutToMultipleSubscribers (0.00s)
--- PASS: TestHubDropsSlowConsumer (0.00s)
--- PASS: TestHubUnsubscribeClosesChannel (0.00s)
PASS
```

- [ ] **Step 5: Commit**

```bash
git add internal/hub/
git commit -m "feat: add broadcast hub with fan-out to subscribers"
```

---

### Task 5: HTTP Server + WebSocket Handler

**Files:**
- Create: `internal/api/server.go`
- Create: `internal/api/ws_handler.go`
- Create: `internal/api/ws_handler_test.go`

**Interfaces:**
- Consumes: `*hub.Hub` — uses `(*Hub).Subscribe()` and `(*Hub).Unsubscribe(ch)`
- Produces:
  - `api.New(h *hub.Hub) *Server`
  - `*Server` implements `http.Handler`
  - Route `GET /ws` upgrades to WebSocket; writes JSON-encoded `models.Tick` to browser

- [ ] **Step 1: Write the failing test**

Create `internal/api/ws_handler_test.go`:
```go
package api_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/menribardhi/trader/internal/api"
	"github.com/menribardhi/trader/internal/hub"
	"github.com/menribardhi/trader/internal/models"
)

func TestWSHandlerDeliversTick(t *testing.T) {
	ticks := make(chan models.Tick, 1)
	h := hub.New(ticks)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go h.Run(ctx)

	srv := httptest.NewServer(api.New(h))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	want := models.Tick{Symbol: "BTCUSDT", Price: "60000.00", Timestamp: 1700000001000}
	ticks <- want

	conn.SetReadDeadline(time.Now().Add(time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	var got models.Tick
	if err := json.Unmarshal(msg, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got != want {
		t.Errorf("got %+v want %+v", got, want)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/api/... -v
```
Expected: compilation error — package `api` not found.

- [ ] **Step 3: Write server.go**

Create `internal/api/server.go`:
```go
package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/menribardhi/trader/internal/hub"
)

// Server is the HTTP server. It implements http.Handler.
type Server struct {
	hub    *hub.Hub
	router chi.Router
}

// New creates a Server wired to the given hub.
func New(h *hub.Hub) *Server {
	s := &Server{hub: h}
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Get("/ws", s.handleWS)
	s.router = r
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}
```

- [ ] **Step 4: Write ws_handler.go**

Create `internal/api/ws_handler.go`:
```go
package api

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	sub := s.hub.Subscribe()
	defer s.hub.Unsubscribe(sub)

	closed := make(chan struct{})
	go func() {
		defer close(closed)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	for {
		select {
		case tick, ok := <-sub:
			if !ok {
				return
			}
			data, err := json.Marshal(tick)
			if err != nil {
				continue
			}
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		case <-closed:
			return
		}
	}
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./internal/api/... -v
```
Expected:
```
--- PASS: TestWSHandlerDeliversTick (0.00s)
PASS
```

- [ ] **Step 6: Run all Go tests**

```bash
go test ./...
```
Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/api/
git commit -m "feat: add Chi HTTP server with WebSocket handler"
```

---

### Task 6: Wire main.go

**Files:**
- Create: `cmd/trader/main.go`

**Interfaces:**
- Consumes: `binance.New`, `hub.New`, `api.New`
- Produces: runnable binary listening on `:8080`

- [ ] **Step 1: Write main.go**

Create `cmd/trader/main.go`:
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
	"github.com/menribardhi/trader/internal/hub"
	"github.com/menribardhi/trader/internal/models"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	ticks := make(chan models.Tick, 64)

	h := hub.New(ticks)
	client := binance.New("BTCUSDT", ticks)

	go client.Run(ctx)
	go h.Run(ctx)

	srv := api.New(h)

	log.Info().Msg("trader listening on :8080")
	if err := http.ListenAndServe(":8080", srv); err != nil && err != http.ErrServerClosed {
		log.Fatal().Err(err).Msg("server error")
	}
}
```

- [ ] **Step 2: Build**

```bash
go build ./...
```
Expected: no output (success).

- [ ] **Step 3: Run all tests**

```bash
go test ./...
```
Expected: all PASS.

- [ ] **Step 4: Commit**

```bash
git add cmd/trader/main.go
git commit -m "feat: wire main.go — Binance client + hub + HTTP server"
```

---

### Task 7: React Frontend Scaffold

**Files:**
- Create: `web/` (via npm)
- Modify: `web/src/App.tsx`
- Create: `web/src/pages/Dashboard.tsx` (placeholder)

**Interfaces:**
- Produces: `npm run dev` starts Vite on `:5173`

- [ ] **Step 1: Scaffold Vite project**

Run from `/Users/menribardhi/Documents/Projects/go-projects/trader`:
```bash
npm create vite@latest web -- --template react-ts
cd web && npm install
npm install lightweight-charts
```

- [ ] **Step 2: Replace App.tsx**

Replace `web/src/App.tsx` with:
```tsx
import { Dashboard } from './pages/Dashboard'

function App() {
  return <Dashboard />
}

export default App
```

- [ ] **Step 3: Create placeholder Dashboard**

Create `web/src/pages/Dashboard.tsx`:
```tsx
export function Dashboard() {
  return <div>Loading...</div>
}
```

- [ ] **Step 4: Verify build**

```bash
cd web && npm run build
```
Expected: `web/dist/` created, no TypeScript errors.

- [ ] **Step 5: Commit**

```bash
cd ..
git add web/
git commit -m "chore: scaffold Vite React TypeScript frontend"
```

---

### Task 8: useMarketStream Hook

**Files:**
- Create: `web/src/hooks/useMarketStream.ts`

**Interfaces:**
- Produces:
  - `export interface Tick { symbol: string; price: string; timestamp: number }`
  - `export function useMarketStream(url: string): { tick: Tick | null; connected: boolean }`
  - Reconnects every 2 seconds on disconnect

- [ ] **Step 1: Write the hook**

Create `web/src/hooks/useMarketStream.ts`:
```ts
import { useEffect, useRef, useState } from 'react'

export interface Tick {
  symbol: string
  price: string
  timestamp: number
}

export function useMarketStream(url: string): { tick: Tick | null; connected: boolean } {
  const [tick, setTick] = useState<Tick | null>(null)
  const [connected, setConnected] = useState(false)
  const retryRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    let ws: WebSocket
    let cancelled = false

    function connect() {
      ws = new WebSocket(url)

      ws.onopen = () => { if (!cancelled) setConnected(true) }

      ws.onclose = () => {
        if (!cancelled) {
          setConnected(false)
          retryRef.current = setTimeout(connect, 2000)
        }
      }

      ws.onerror = () => ws.close()

      ws.onmessage = (e: MessageEvent<string>) => {
        if (!cancelled) setTick(JSON.parse(e.data) as Tick)
      }
    }

    connect()

    return () => {
      cancelled = true
      if (retryRef.current) clearTimeout(retryRef.current)
      ws?.close()
    }
  }, [url])

  return { tick, connected }
}
```

- [ ] **Step 2: Verify TypeScript**

```bash
cd web && npx tsc --noEmit
```
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
cd ..
git add web/src/hooks/useMarketStream.ts
git commit -m "feat: add useMarketStream hook with auto-reconnect"
```

---

### Task 9: Chart Component + Dashboard + End-to-End Verify

**Files:**
- Create: `web/src/components/Chart.tsx`
- Modify: `web/src/pages/Dashboard.tsx`

**Interfaces:**
- Consumes:
  - `Tick` from `../hooks/useMarketStream`
  - `useMarketStream(url: string)` from `../hooks/useMarketStream`
  - `createChart`, `IChartApi`, `ISeriesApi`, `UTCTimestamp` from `lightweight-charts`
- Produces: live chart at `http://localhost:5173`

- [ ] **Step 1: Write Chart.tsx**

Create `web/src/components/Chart.tsx`:
```tsx
import { useEffect, useRef } from 'react'
import { createChart, IChartApi, ISeriesApi, UTCTimestamp } from 'lightweight-charts'
import { Tick } from '../hooks/useMarketStream'

interface ChartProps {
  tick: Tick | null
}

export function Chart({ tick }: ChartProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const chartRef = useRef<IChartApi | null>(null)
  const seriesRef = useRef<ISeriesApi<'Line'> | null>(null)

  useEffect(() => {
    if (!containerRef.current) return
    const chart = createChart(containerRef.current, {
      width: containerRef.current.clientWidth,
      height: 300,
      layout: { background: { color: '#1a1a2e' }, textColor: '#e0e0e0' },
      grid: { vertLines: { color: '#2a2a4a' }, horzLines: { color: '#2a2a4a' } },
    })
    const series = chart.addLineSeries({ color: '#00d4aa', lineWidth: 2 })
    chartRef.current = chart
    seriesRef.current = series

    const handleResize = () => {
      if (containerRef.current) chart.applyOptions({ width: containerRef.current.clientWidth })
    }
    window.addEventListener('resize', handleResize)

    return () => {
      window.removeEventListener('resize', handleResize)
      chart.remove()
    }
  }, [])

  useEffect(() => {
    if (!tick || !seriesRef.current) return
    seriesRef.current.update({
      time: Math.floor(tick.timestamp / 1000) as UTCTimestamp,
      value: parseFloat(tick.price),
    })
  }, [tick])

  return <div ref={containerRef} style={{ width: '100%' }} />
}
```

- [ ] **Step 2: Write Dashboard.tsx**

Replace `web/src/pages/Dashboard.tsx` with:
```tsx
import { useMarketStream } from '../hooks/useMarketStream'
import { Chart } from '../components/Chart'

export function Dashboard() {
  const { tick, connected } = useMarketStream('ws://localhost:8080/ws')

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
    </div>
  )
}
```

- [ ] **Step 3: Verify TypeScript**

```bash
cd web && npx tsc --noEmit
```
Expected: no errors.

- [ ] **Step 4: End-to-end verification**

Terminal 1 — start Go backend:
```bash
go run ./cmd/trader
```
Expected log: `trader listening on :8080` followed by Binance connection log.

Terminal 2 — start React frontend:
```bash
cd web && npm run dev
```
Expected: `Local: http://localhost:5173/`

Open browser at `http://localhost:5173`:
- Status shows `● Connected` in green
- BTC/USDT price appears and updates in real time (e.g. `$65,432.10`)
- Chart line grows as ticks arrive

Stop the Go server with Ctrl-C: browser shows `○ Disconnected`
Restart Go server: browser reconnects within 2 seconds.

- [ ] **Step 5: Run all tests**

```bash
go test ./...
```
Expected: all PASS.

- [ ] **Step 6: Final commit**

```bash
cd ..
git add web/src/components/Chart.tsx web/src/pages/Dashboard.tsx
git commit -m "feat: add Chart component and Dashboard — Phase 1 complete"
```

---

## Verification Checklist

- [ ] `go build ./...` — no errors
- [ ] `go test ./...` — all PASS
- [ ] `make dev` — Go server on `:8080`, Vite on `:5173` both start
- [ ] Browser at `http://localhost:5173` shows live BTC/USDT price updating in real time
- [ ] Stopping Go server shows `○ Disconnected`; restarting reconnects within 2s
- [ ] Killing network to Binance — Go server reconnects automatically within 30s
