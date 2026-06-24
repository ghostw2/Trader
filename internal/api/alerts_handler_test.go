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
	srv := httptest.NewServer(api.New(h, sqldb, nil, nil))
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
	if err := json.NewDecoder(resp.Body).Decode(&alert); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
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
	if err := json.NewDecoder(resp.Body).Decode(&alerts); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
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
	if err := json.NewDecoder(resp.Body).Decode(&alert); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/alerts/%d", srv.URL, alert.ID), nil)
	delResp, _ := http.DefaultClient.Do(req)
	if delResp.StatusCode != http.StatusNoContent {
		t.Errorf("expected 204, got %d", delResp.StatusCode)
	}

	listResp, _ := http.Get(srv.URL + "/api/alerts")
	var alerts []models.Alert
	if err := json.NewDecoder(listResp.Body).Decode(&alerts); err != nil {
		t.Fatalf("decode list after delete: %v", err)
	}
	if len(alerts) != 0 {
		t.Errorf("expected 0 after delete, got %d", len(alerts))
	}
}
