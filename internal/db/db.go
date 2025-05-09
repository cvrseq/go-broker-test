package db

import (
	"database/sql"
	"log"
)

func InitDB(dtbs *sql.DB) error {
    _, err := dtbs.Exec(`
    CREATE TABLE IF NOT EXISTS trades_q (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        account TEXT NOT NULL,
        symbol TEXT NOT NULL,
        volume REAL NOT NULL,
        open REAL NOT NULL,
        close REAL NOT NULL,
        side TEXT NOT NULL,
        processed INTEGER DEFAULT 0,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    )`)
    if err != nil {
        return err
    }

    _, err = dtbs.Exec(`
    CREATE TABLE IF NOT EXISTS account_stats (
        account TEXT PRIMARY KEY,
        trades INTEGER DEFAULT 0,
        profit REAL DEFAULT 0
    )`)
    if err != nil {
        return err
    }

    log.Println("Database initialized successfully")
    return nil
}
