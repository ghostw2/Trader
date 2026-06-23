package db

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
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
	default:
		return models.Trade{}, fmt.Errorf("unknown order side: %q", side)
	}

	if _, err := tx.Exec(
		`UPDATE portfolio SET cash = ?, btc = ?, avg_buy_price = ? WHERE id = 1`,
		roundForDB(cash), roundForDB(btc), roundForDB(avgBuyPrice),
	); err != nil {
		return models.Trade{}, err
	}

	now := time.Now().UnixMilli()
	res, err := tx.Exec(
		`INSERT INTO trades (side, quantity, price, total, created_at) VALUES (?,?,?,?,?)`,
		side, quantity, price, roundForDB(total), now,
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

// roundForDB rounds a value to 8 decimal places for database storage only.
// This prevents floating-point precision issues in the stored data without
// affecting intermediate calculations.
func roundForDB(val float64) float64 {
	pow := math.Pow(10, 8)
	return math.Round(val*pow) / pow
}
