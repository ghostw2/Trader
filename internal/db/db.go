package db

import (
	"database/sql"
	"errors"

	_ "modernc.org/sqlite"
)

var ErrNotFound = errors.New("not found")

func Open(path string) (*sql.DB, error) {
	sqldb, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := sqldb.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		sqldb.Close()
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
