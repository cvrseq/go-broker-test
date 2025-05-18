package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"gitlab.com/digineat/go-broker-test/internal/service"
	"gitlab.com/digineat/go-broker-test/internal/db"
)

func main() {
	// Command line flags
	dbPath := flag.String("db", "data.db", "path to SQLite database")
	listenAddr := flag.String("listen", "8080", "HTTP server listen address")
	flag.Parse()

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

	handler := service.NewHandler(repository)
	
	mux := http.NewServeMux()

	mux.HandleFunc("POST /trades", handler.HandleTradeSubmission)
	mux.HandleFunc("GET /stats/", handler.HandleGetStats)
	mux.HandleFunc("GET /healthz", handler.HandleHealthCheck)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", *listenAddr),
		Handler:      service.RateLimiter(mux),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Printf("Starting server on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited properly")
}
