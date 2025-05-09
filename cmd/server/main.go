package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"gitlab.com/digineat/go-broker-test/internal/db"
	"gitlab.com/digineat/go-broker-test/internal/model"
)

var statsPathRegex = regexp.MustCompile(`^/stats/([^/]+)$`)

func main() {
    // Command line flags
    dbPath := flag.String("db", "data.db", "path to SQLite database")
    listenAddr := flag.String("listen", "8080", "HTTP server listen address")
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
    
    // Initialize HTTP server
    mux := http.NewServeMux()

    // POST /trades endpoint
    mux.HandleFunc("POST /trades", func(w http.ResponseWriter, r *http.Request) {
        var trade model.Trade
        decoder := json.NewDecoder(r.Body)
        if err := decoder.Decode(&trade); err != nil {
            http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
            return
        }

        if err := validateTrade(trade); err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        }

        _, err := sqlDB.Exec(
            "INSERT INTO trades_q (account, symbol, volume, open, close, side) VALUES (?, ?, ?, ?, ?, ?)",
            trade.Account, trade.Symbol, trade.Volume, trade.Open, trade.Close, trade.Side,
        )
        if err != nil {
            log.Printf("Failed to insert trade: %v", err)
            http.Error(w, "Failed to process trade", http.StatusInternalServerError)
            return
        }

        w.WriteHeader(http.StatusOK)
    })

    // GET /stats/{acc} endpoint
    mux.HandleFunc("GET /stats/", func(w http.ResponseWriter, r *http.Request) {
        matches := statsPathRegex.FindStringSubmatch(r.URL.Path)
        if len(matches) != 2 {
            http.Error(w, "Invalid account ID", http.StatusBadRequest)
            return
        }
        
        account := matches[1]
        
        var stats model.Stats
        err := sqlDB.QueryRow(
            "SELECT account, trades, profit FROM account_stats WHERE account = ?",
            account,
        ).Scan(&stats.Account, &stats.Trades, &stats.Profit)
        
        if err == sql.ErrNoRows {
            stats = model.Stats{Account: account, Trades: 0, Profit: 0}
        } else if err != nil {
            log.Printf("Failed to query stats: %v", err)
            http.Error(w, "Failed to retrieve stats", http.StatusInternalServerError)
            return
        }
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(stats)
    })

    // GET /healthz endpoint
    mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
        if err := sqlDB.Ping(); err != nil {
            log.Printf("Health check failed: %v", err)
            http.Error(w, "Database connection failed", http.StatusServiceUnavailable)
            return
        }
        w.WriteHeader(http.StatusNoContent)
    })

    // Start server
    serverAddr := fmt.Sprintf(":%s", *listenAddr)
    log.Printf("Starting server on %s", serverAddr)
    if err := http.ListenAndServe(serverAddr, mux); err != nil {
        log.Fatalf("Server failed: %v", err)
    }
}

func validateTrade(trade model.Trade) error {
    if trade.Account == "" {
        return fmt.Errorf("account не должно быть пустым")
    }
    
    symbolRegex := regexp.MustCompile(`^[A-Z]{6}$`)
    if !symbolRegex.MatchString(trade.Symbol) {
        return fmt.Errorf("symbol должен соответствовать формату ^[A-Z]{6}$")
    }
    
    if trade.Volume <= 0 {
        return fmt.Errorf("volume должно быть > 0")
    }
    
    if trade.Open <= 0 {
        return fmt.Errorf("open должно быть > 0")
    }
    
    if trade.Close <= 0 {
        return fmt.Errorf("close должно быть > 0")
    }
    
    trade.Side = strings.ToLower(trade.Side)
    if trade.Side != "buy" && trade.Side != "sell" {
        return fmt.Errorf("side должно быть либо 'buy', либо 'sell'")
    }
    
    return nil
}
