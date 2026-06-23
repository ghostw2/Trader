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
