// Package storage provides data persistence for Tracehound events.
// This is where we store the whale scents we've tracked.
package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
	"github.com/sirupsen/logrus"
	"github.com/tracehound-labs/tracehound-indexer/internal/blockchain"
)

// PostgresStore manages whale scent storage in PostgreSQL.
// This is the hound's scent library - organized and searchable.
type PostgresStore struct {
	db     *sql.DB
	logger *logrus.Logger
}

// PostgresConfig holds configuration for PostgreSQL connection.
type PostgresConfig struct {
	Host            string
	Port            int
	Database        string
	User            string
	Password        string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// NewPostgresStore creates a new PostgreSQL store with connection pooling.
// Returns the store ready to track whale scents.
func NewPostgresStore(cfg PostgresConfig, logger *logrus.Logger) (*PostgresStore, error) {
	if logger == nil {
		logger = logrus.New()
	}

	// Build connection string
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database, cfg.SSLMode,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	maxOpen := cfg.MaxOpenConns
	if maxOpen == 0 {
		maxOpen = 25
	}
	maxIdle := cfg.MaxIdleConns
	if maxIdle == 0 {
		maxIdle = 5
	}
	connLifetime := cfg.ConnMaxLifetime
	if connLifetime == 0 {
		connLifetime = 5 * time.Minute
	}

	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxLifetime(connLifetime)

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Info("🗄️ PostgreSQL connection established")

	return &PostgresStore{
		db:     db,
		logger: logger,
	}, nil
}

// NewPostgresStoreFromURL creates a PostgreSQL store from a connection URL.
func NewPostgresStoreFromURL(connURL string, logger *logrus.Logger) (*PostgresStore, error) {
	if logger == nil {
		logger = logrus.New()
	}

	db, err := sql.Open("postgres", connURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool with defaults
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Info("🗄️ PostgreSQL connection established")

	return &PostgresStore{
		db:     db,
		logger: logger,
	}, nil
}

// BatchInsert stores multiple whale scents in a single transaction.
// This is more efficient than individual inserts - like tracking a whole pack at once.
func (s *PostgresStore) BatchInsert(ctx context.Context, events []blockchain.RawEvent) error {
	if len(events) == 0 {
		return nil
	}

	// Start transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // nolint:errcheck

	// Prepare the batch insert statement
	// Using ON CONFLICT to handle duplicate tx_hash gracefully
	stmt, err := tx.PrepareContext(ctx, pq.CopyIn(
		"whale_scents",
		"tx_hash", "block_height", "event_type", "timestamp", "trader", "symbol", "raw_data",
	))
	if err != nil {
		// Fall back to individual inserts with ON CONFLICT
		return s.batchInsertFallback(ctx, tx, events)
	}
	defer stmt.Close()

	for _, event := range events {
		rawData, err := json.Marshal(event.RawData)
		if err != nil {
			rawData = event.RawData
		}

		_, err = stmt.ExecContext(ctx,
			event.TxHash,
			event.BlockHeight,
			string(event.EventType),
			event.Timestamp,
			event.Trader,
			event.Symbol,
			rawData,
		)
		if err != nil {
			s.logger.WithError(err).Warn("Failed to add event to batch, using fallback")
			return s.batchInsertFallback(ctx, tx, events)
		}
	}

	// Execute the batch
	_, err = stmt.ExecContext(ctx)
	if err != nil {
		return s.batchInsertFallback(ctx, tx, events)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.logger.WithField("count", len(events)).Debug("📝 Scents recorded to database")

	return nil
}

// batchInsertFallback uses individual INSERT statements with ON CONFLICT.
// This handles cases where COPY doesn't work or conflicts need to be ignored.
func (s *PostgresStore) batchInsertFallback(ctx context.Context, tx *sql.Tx, events []blockchain.RawEvent) error {
	// Build multi-value INSERT with ON CONFLICT DO NOTHING
	const baseQuery = `
		INSERT INTO whale_scents (tx_hash, block_height, event_type, timestamp, trader, symbol, raw_data)
		VALUES `

	const conflictClause = ` ON CONFLICT (tx_hash) DO NOTHING`

	// Build values placeholders
	valueStrings := make([]string, 0, len(events))
	valueArgs := make([]interface{}, 0, len(events)*7)

	for i, event := range events {
		rawData, err := json.Marshal(event.RawData)
		if err != nil {
			rawData = event.RawData
		}

		base := i * 7
		valueStrings = append(valueStrings,
			fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d)",
				base+1, base+2, base+3, base+4, base+5, base+6, base+7))

		valueArgs = append(valueArgs,
			event.TxHash,
			event.BlockHeight,
			string(event.EventType),
			event.Timestamp,
			event.Trader,
			event.Symbol,
			rawData,
		)
	}

	query := baseQuery + strings.Join(valueStrings, ", ") + conflictClause

	_, err := tx.ExecContext(ctx, query, valueArgs...)
	if err != nil {
		return fmt.Errorf("failed to execute batch insert: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit fallback transaction: %w", err)
	}

	s.logger.WithField("count", len(events)).Debug("📝 Scents recorded to database (fallback)")

	return nil
}

// GetLatestBlockHeight returns the highest block height we've tracked.
// Useful for knowing where we left off.
func (s *PostgresStore) GetLatestBlockHeight(ctx context.Context) (uint64, error) {
	var height sql.NullInt64
	err := s.db.QueryRowContext(ctx,
		"SELECT MAX(block_height) FROM whale_scents").Scan(&height)
	if err != nil {
		return 0, fmt.Errorf("failed to get latest block height: %w", err)
	}

	if !height.Valid {
		return 0, nil
	}

	return uint64(height.Int64), nil
}

// GetEventsByTrader retrieves all scents for a specific whale.
func (s *PostgresStore) GetEventsByTrader(ctx context.Context, trader string, limit int) ([]blockchain.RawEvent, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT tx_hash, block_height, event_type, timestamp, trader, symbol, raw_data
		FROM whale_scents
		WHERE trader = $1
		ORDER BY timestamp DESC
		LIMIT $2
	`, trader, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}
	defer rows.Close()

	return s.scanEvents(rows)
}

// GetRecentEvents retrieves the most recent scents.
func (s *PostgresStore) GetRecentEvents(ctx context.Context, limit int) ([]blockchain.RawEvent, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT tx_hash, block_height, event_type, timestamp, trader, symbol, raw_data
		FROM whale_scents
		ORDER BY timestamp DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent events: %w", err)
	}
	defer rows.Close()

	return s.scanEvents(rows)
}

// scanEvents converts database rows into RawEvent objects.
func (s *PostgresStore) scanEvents(rows *sql.Rows) ([]blockchain.RawEvent, error) {
	var events []blockchain.RawEvent

	for rows.Next() {
		var event blockchain.RawEvent
		var eventType string
		var rawData []byte

		err := rows.Scan(
			&event.TxHash,
			&event.BlockHeight,
			&eventType,
			&event.Timestamp,
			&event.Trader,
			&event.Symbol,
			&rawData,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		event.EventType = blockchain.EventType(eventType)
		event.RawData = rawData
		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return events, nil
}

// Ping checks if the database connection is alive.
func (s *PostgresStore) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// Close gracefully closes the database connection.
// The hound rests after a long hunt.
func (s *PostgresStore) Close() error {
	s.logger.Info("🗄️ Closing PostgreSQL connection...")
	return s.db.Close()
}
