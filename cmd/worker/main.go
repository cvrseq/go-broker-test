package main

import (
	"database/sql"
	"flag"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"gitlab.com/digineat/go-broker-test/internal/db"
	"gitlab.com/digineat/go-broker-test/internal/model"
)

func main() {
    // Command line flags
    dbPath := flag.String("db", "data.db", "path to SQLite database")
    pollInterval := flag.Duration("poll", 100*time.Millisecond, "polling interval")
    flag.Parse()

    // Initialize database connection
    sqlDB, err := sql.Open("sqlite3", *dbPath)
    if err != nil {
        log.Fatalf("Failed to open database: %v", err)
    }
    defer sqlDB.Close()

    // Test database connection
    if err := sqlDB.Ping(); err != nil {
        log.Fatalf("Failed to ping database: %v", err)
    }

    // Initialize database schema
    if err := db.InitDB(sqlDB); err != nil {
        log.Fatalf("Failed to initialize database: %v", err)
    }

    log.Printf("Worker started with polling interval: %v", *pollInterval)

    // Main worker loop
    for {
        rows, err := sqlDB.Query(`
            SELECT id, account, symbol, volume, open, close, side 
            FROM trades_q 
            WHERE processed = 0 
            ORDER BY created_at ASC
            LIMIT 10
        `)
        if err != nil {
            log.Printf("Failed to query trades: %v", err)
            time.Sleep(*pollInterval)
            continue
        }

        var trades []model.Trade
        for rows.Next() {
            var trade model.Trade
            if err := rows.Scan(&trade.ID, &trade.Account, &trade.Symbol, &trade.Volume, &trade.Open, &trade.Close, &trade.Side); err != nil {
                log.Printf("Failed to scan trade: %v", err)
                continue
            }
            trades = append(trades, trade)
        }
        rows.Close()

        for _, trade := range trades {
            tx, err := sqlDB.Begin()
            if err != nil {
                log.Printf("Failed to begin transaction: %v", err)
                continue
            }

            lot := 100000.0
            profit := (trade.Close - trade.Open) * trade.Volume * lot
            if trade.Side == "sell" {
                profit = -profit
            }

            _, err = tx.Exec(`
                INSERT INTO account_stats (account, trades, profit) 
                VALUES (?, 1, ?) 
                ON CONFLICT(account) DO UPDATE SET 
                trades = trades + 1, 
                profit = profit + ?
            `, trade.Account, profit, profit)
            if err != nil {
                log.Printf("Failed to update account stats: %v", err)
                tx.Rollback()
                continue
            }

            _, err = tx.Exec("UPDATE trades_q SET processed = 1 WHERE id = ?", trade.ID)
            if err != nil {
                log.Printf("Failed to mark trade as processed: %v", err)
                tx.Rollback()
                continue
            }

            if err := tx.Commit(); err != nil {
                log.Printf("Failed to commit transaction: %v", err)
                tx.Rollback()
                continue
            }

            log.Printf("Processed trade %d for account %s: profit %.2f", trade.ID, trade.Account, profit)
        }

        if len(trades) == 0 {
            time.Sleep(*pollInterval)
        }
    }
}
