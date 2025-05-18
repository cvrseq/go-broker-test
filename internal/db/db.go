package db

import (
	"context"
	"database/sql"
	"log"

	"gitlab.com/digineat/go-broker-test/internal/model"
)

type Repository interface {
	InitDB() error
	EnqueueTrade(trade model.Trade) error
	GetUnprocessedTrades(limit int) ([]model.Trade, error)
	UpdateAccountStats(ctx context.Context, tx *sql.Tx, account string, profit float64) error
	MarkTradeAsProcessed(ctx context.Context, tx *sql.Tx, tradeID int64) error
	GetAccountStats(account string) (model.Stats, error)
	BeginTx(ctx context.Context) (*sql.Tx, error)
}

type SQLiteRepository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &SQLiteRepository{db: db}
}

func InitDB(db *sql.DB) error {
	_, err := db.Exec(`
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

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS account_stats (
		account TEXT PRIMARY KEY,
		trades INTEGER DEFAULT 0,
		profit REAL DEFAULT 0
	)`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_trades_q_processed ON trades_q(processed, created_at)`)
	if err != nil {
		return err
	}

	log.Println("Database initialized successfully")
	return nil
}

func (r *SQLiteRepository) InitDB() error {
	return InitDB(r.db)
}

func (r *SQLiteRepository) EnqueueTrade(trade model.Trade) error {
	stmt, err := r.db.Prepare(`
		INSERT INTO trades_q (account, symbol, volume, open, close, side) 
		VALUES (?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(trade.Account, trade.Symbol, trade.Volume, trade.Open, trade.Close, trade.Side)
	return err
}

func (r *SQLiteRepository) GetUnprocessedTrades(limit int) ([]model.Trade, error) {
	stmt, err := r.db.Prepare(`
		SELECT id, account, symbol, volume, open, close, side
		FROM trades_q 
		WHERE processed = 0 
		ORDER BY created_at ASC
		LIMIT ?
	`)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	rows, err := stmt.Query(limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trades []model.Trade
	for rows.Next() {
		var trade model.Trade
		if err := rows.Scan(&trade.ID, &trade.Account, &trade.Symbol, &trade.Volume, &trade.Open, &trade.Close, &trade.Side); err != nil {
			return nil, err
		}
		trades = append(trades, trade)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return trades, nil
}

func (r *SQLiteRepository) UpdateAccountStats(ctx context.Context, tx *sql.Tx, account string, profit float64) error {
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO account_stats (account, trades, profit) 
		VALUES (?, 1, ?) 
		ON CONFLICT(account) DO UPDATE SET 
		trades = trades + 1, 
		profit = profit + ?
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(ctx, account, profit, profit)
	return err
}

func (r *SQLiteRepository) MarkTradeAsProcessed(ctx context.Context, tx *sql.Tx, tradeID int64) error {
	stmt, err := tx.PrepareContext(ctx, "UPDATE trades_q SET processed = 1 WHERE id = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(ctx, tradeID)
	return err
}

func (r *SQLiteRepository) GetAccountStats(account string) (model.Stats, error) {
	stmt, err := r.db.Prepare("SELECT account, trades, profit FROM account_stats WHERE account = ?")
	if err != nil {
		return model.Stats{}, err
	}
	defer stmt.Close()

	var stats model.Stats
	err = stmt.QueryRow(account).Scan(&stats.Account, &stats.Trades, &stats.Profit)
	
	if err == sql.ErrNoRows {
		return model.Stats{Account: account, Trades: 0, Profit: 0}, nil
	}
	
	return stats, err
}

func (r *SQLiteRepository) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return r.db.BeginTx(ctx, nil)
}
