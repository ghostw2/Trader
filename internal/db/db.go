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
