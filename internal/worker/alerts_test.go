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
