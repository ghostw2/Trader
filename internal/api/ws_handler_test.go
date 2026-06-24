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

	srv := httptest.NewServer(api.New(h, nil, nil, nil))
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
