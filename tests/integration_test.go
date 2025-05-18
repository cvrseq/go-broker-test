package tests

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"math"
	"net/http"
	"os"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"gitlab.com/digineat/go-broker-test/internal/db"
	"gitlab.com/digineat/go-broker-test/internal/model"
	"gitlab.com/digineat/go-broker-test/internal/service"
)

func TestIntegration(t *testing.T) {
	dbPath := "test_data.db"
	os.Remove(dbPath) // Remove any existing test database

	sqlDB, err := sql.Open("sqlite3", dbPath+"?_journal=WAL&_timeout=5000")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer func() {
		sqlDB.Close()
		os.Remove(dbPath)
	}()

	if err := db.InitDB(sqlDB); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	repository := db.NewRepository(sqlDB)

	// Start HTTP server
	handler := service.NewHandler(repository)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /trades", handler.HandleTradeSubmission)
	mux.HandleFunc("GET /stats/", handler.HandleGetStats)
	mux.HandleFunc("GET /healthz", handler.HandleHealthCheck)

	server := &http.Server{
		Addr:    ":8081",
		Handler: mux,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.Errorf("Server failed: %v", err)
		}
	}()
	defer server.Shutdown(context.Background())

	ctx, cancel := context.WithCancel(context.Background())
	processor := service.NewProcessor(repository, 10)
	go processor.Start(ctx, 100*time.Millisecond)
	defer cancel()

	time.Sleep(100 * time.Millisecond)

	t.Run("TestInvalidTrade", func(t *testing.T) {
		invalidTrade := model.Trade{
			Account: "123",
			Symbol:  "EUR", // Invalid symbol
			Volume:  1.0,
			Open:    1.1000,
			Close:   1.1050,
			Side:    "buy",
		}

		body, _ := json.Marshal(invalidTrade)
		resp, err := http.Post("http://localhost:8081/trades", "application/json", bytes.NewBuffer(body))
		if err != nil {
			t.Fatalf("Failed to send request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", resp.StatusCode)
		}
	})

	t.Run("TestValidTrade", func(t *testing.T) {
		validTrade := model.Trade{
			Account: "123",
			Symbol:  "EURUSD",
			Volume:  1.0,
			Open:    1.1000,
			Close:   1.1050,
			Side:    "buy",
		}

		body, _ := json.Marshal(validTrade)
		resp, err := http.Post("http://localhost:8081/trades", "application/json", bytes.NewBuffer(body))
		if err != nil {
			t.Fatalf("Failed to send request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		time.Sleep(500 * time.Millisecond)

		// Check stats
		resp, err = http.Get("http://localhost:8081/stats/123")
		if err != nil {
			t.Fatalf("Failed to send request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		var stats model.Stats
		if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if stats.Account != "123" {
			t.Errorf("Expected account 123, got %s", stats.Account)
		}

		if stats.Trades != 1 {
			t.Errorf("Expected 1 trade, got %d", stats.Trades)
		}

		expectedProfit := 500.0 // (1.1050 - 1.1000) * 1.0 * 100000
		const epsilon = 1e-9
		if math.Abs(stats.Profit-expectedProfit) > epsilon {
			t.Errorf("Expected profit %.2f, got %.2f", expectedProfit, stats.Profit)
		}
	})

	t.Run("TestHealthCheck", func(t *testing.T) {
		resp, err := http.Get("http://localhost:8081/healthz")
		if err != nil {
			t.Fatalf("Failed to send request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
	})
}

func TestTradeValidation(t *testing.T) {
	tests := []struct {
		name    string
		trade   model.Trade
		wantErr bool
	}{
		{
			name: "Valid trade",
			trade: model.Trade{
				Account: "123",
				Symbol:  "EURUSD",
				Volume:  1.0,
				Open:    1.1000,
				Close:   1.1050,
				Side:    "buy",
			},
			wantErr: false,
		},
		{
			name: "Empty account",
			trade: model.Trade{
				Account: "",
				Symbol:  "EURUSD",
				Volume:  1.0,
				Open:    1.1000,
				Close:   1.1050,
				Side:    "buy",
			},
			wantErr: true,
		},
		{
			name: "Invalid symbol",
			trade: model.Trade{
				Account: "123",
				Symbol:  "EUR",
				Volume:  1.0,
				Open:    1.1000,
				Close:   1.1050,
				Side:    "buy",
			},
			wantErr: true,
		},
		{
			name: "Zero volume",
			trade: model.Trade{
				Account: "123",
				Symbol:  "EURUSD",
				Volume:  0,
				Open:    1.1000,
				Close:   1.1050,
				Side:    "buy",
			},
			wantErr: true,
		},
		{
			name: "Invalid side",
			trade: model.Trade{
				Account: "123",
				Symbol:  "EURUSD",
				Volume:  1.0,
				Open:    1.1000,
				Close:   1.1050,
				Side:    "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.trade.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Trade.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestProfitCalculation(t *testing.T) {
	const epsilon = 1e-9

	tests := []struct {
		name     string
		trade    model.Trade
		expected float64
	}{
		{
			name: "Buy profit",
			trade: model.Trade{
				Volume: 1.0,
				Open:   1.1000,
				Close:  1.1050,
				Side:   "buy",
			},
			expected: 500.0,
		},
		{
			name: "Sell profit",
			trade: model.Trade{
				Volume: 1.0,
				Open:   1.1050,
				Close:  1.1000,
				Side:   "sell",
			},
			expected: 500.0,
		},
		{
			name: "Buy loss",
			trade: model.Trade{
				Volume: 1.0,
				Open:   1.1050,
				Close:  1.1000,
				Side:   "buy",
			},
			expected: -500.0,
		},
		{
			name: "Sell loss",
			trade: model.Trade{
				Volume: 1.0,
				Open:   1.1000,
				Close:  1.1050,
				Side:   "sell",
			},
			expected: -500.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profit := tt.trade.CalculateProfit()
			if math.Abs(profit-tt.expected) > epsilon {
				t.Errorf("CalculateProfit() = %v, want %v", profit, tt.expected)
			}
		})
	}
}
