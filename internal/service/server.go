package service

import (
	"context"
	"log"
	"time"

	"gitlab.com/digineat/go-broker-test/internal/db"
	"gitlab.com/digineat/go-broker-test/internal/model"
)

// Processor processes trades from the queue
type Processor struct {
	repo      db.Repository
	batchSize int
}

// NewProcessor creates a new Processor
func NewProcessor(repo db.Repository, batchSize int) *Processor {
	return &Processor{
		repo:      repo,
		batchSize: batchSize,
	}
}

func (p *Processor) Start(ctx context.Context, pollInterval time.Duration) {
	for {
		select {
		case <-ctx.Done():
			log.Println("Worker context canceled, stopping...")
			return
		default:
			processed := p.processBatch(ctx)
			if !processed {
				// If no trades were processed, wait for the poll interval
				select {
				case <-ctx.Done():
					return
				case <-time.After(pollInterval):
				}
			}
		}
	}
}

func (p *Processor) processBatch(ctx context.Context) bool {
	trades, err := p.repo.GetUnprocessedTrades(p.batchSize)
	if err != nil {
		log.Printf("Failed to query trades: %v", err)
		return false
	}

	if len(trades) == 0 {
		return false
	}

	for _, trade := range trades {
		select {
		case <-ctx.Done():
			return true
		default:
			if err := p.processTrade(ctx, trade); err != nil {
				log.Printf("Failed to process trade %d: %v", trade.ID, err)
			} else {
				log.Printf("Processed trade %d for account %s", trade.ID, trade.Account)
			}
		}
	}

	return true
}

// processTrade processes a single trade
func (p *Processor) processTrade(ctx context.Context, trade model.Trade) error {
	profit := trade.CalculateProfit()

	// Begin transaction
	tx, err := p.repo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Update account stats
	if err := p.repo.UpdateAccountStats(ctx, tx, trade.Account, profit); err != nil {
		return err
	}

	// Mark trade as processed
	if err := p.repo.MarkTradeAsProcessed(ctx, tx, trade.ID); err != nil {
		return err
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return err
	}

	log.Printf("Processed trade %d for account %s: profit %.2f", trade.ID, trade.Account, profit)
	return nil
}
