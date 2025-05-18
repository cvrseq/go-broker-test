package main

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"gitlab.com/digineat/go-broker-test/internal/db"
	"gitlab.com/digineat/go-broker-test/internal/service"
)

func main() {
	// Command line flags
	dbPath := flag.String("db", "data.db", "path to SQLite database")
	pollInterval := flag.Duration("poll", 100*time.Millisecond, "polling interval")
	batchSize := flag.Int("batch", 10, "number of trades to process in a batch")
	flag.Parse()

	// Initialize database connection
	sqlDB, err := sql.Open("sqlite3", *dbPath+"?_journal=WAL&_timeout=5000")
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer sqlDB.Close()

	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	if err := sqlDB.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	if err := db.InitDB(sqlDB); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	repository := db.NewRepository(sqlDB)

	processor := service.NewProcessor(repository, *batchSize)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		log.Printf("Worker started with polling interval: %v", *pollInterval)
		processor.Start(ctx, *pollInterval)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down worker...")

	cancel()

	time.Sleep(500 * time.Millisecond)

	log.Println("Worker exited properly")
}
