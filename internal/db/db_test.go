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
