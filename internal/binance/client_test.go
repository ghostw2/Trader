package binance_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
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
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
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

	if calls.Load() < 2 {
		t.Errorf("expected at least 2 connection attempts, got %d", calls.Load())
	}
}
