package service

import (
	"encoding/json"
	"log"
	"net/http"
	"regexp"
	"sync"
	"time"

	"gitlab.com/digineat/go-broker-test/internal/db"
	"gitlab.com/digineat/go-broker-test/internal/model"
)

type Handler struct {
	repo db.Repository
}

func NewHandler(repo db.Repository) *Handler {
	return &Handler{repo: repo}
}

var statsPathRegex = regexp.MustCompile(`^/stats/([^/]+)$`)

func (h *Handler) HandleTradeSubmission(w http.ResponseWriter, r *http.Request) {
	var trade model.Trade
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&trade); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	if err := trade.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.repo.EnqueueTrade(trade); err != nil {
		log.Printf("Failed to insert trade: %v", err)
		http.Error(w, "Failed to process trade", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// HandleGetStats handles GET /stats/{acc} requests
func (h *Handler) HandleGetStats(w http.ResponseWriter, r *http.Request) {
	matches := statsPathRegex.FindStringSubmatch(r.URL.Path)
	if len(matches) != 2 {
		http.Error(w, "Invalid account ID", http.StatusBadRequest)
		return
	}
	
	account := matches[1]
	
	stats, err := h.repo.GetAccountStats(account)
	if err != nil {
		log.Printf("Failed to query stats: %v", err)
		http.Error(w, "Failed to retrieve stats", http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// HandleHealthCheck handles GET /healthz requests
func (h *Handler) HandleHealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// RateLimiter implements a simple rate limiting middleware
func RateLimiter(next http.Handler) http.Handler {
	var (
		mu           sync.Mutex
		lastRequests = make(map[string]time.Time)
	)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr

		mu.Lock()
		lastReq, exists := lastRequests[ip]
		now := time.Now()
		
		// Allow 10 requests per second per IP
		if exists && now.Sub(lastReq) < 100*time.Millisecond {
			mu.Unlock()
			w.Header().Set("Retry-After", "1")
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		
		lastRequests[ip] = now
		mu.Unlock()

		next.ServeHTTP(w, r)
	})
}
