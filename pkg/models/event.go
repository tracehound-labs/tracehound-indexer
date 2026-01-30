// Package models provides public data models for Tracehound Indexer.
// These models can be imported by other services that consume Tracehound data.
package models

import (
	"encoding/json"
	"time"
)

// EventType represents the type of blockchain event tracked by Tracehound.
type EventType string

const (
	// EventTypeTrade represents a trade execution event.
	EventTypeTrade EventType = "trade"

	// EventTypePosition represents a position change event.
	EventTypePosition EventType = "position"

	// EventTypeLiquidation represents a liquidation event.
	EventTypeLiquidation EventType = "liquidation"
)

// WhaleScent represents a tracked blockchain event.
// This is the primary model consumed by downstream services.
type WhaleScent struct {
	// ID is the unique database identifier
	ID int64 `json:"id,omitempty"`

	// TxHash is the blockchain transaction hash
	TxHash string `json:"tx_hash"`

	// BlockHeight is the block number of the event
	BlockHeight uint64 `json:"block_height"`

	// Timestamp is when the event occurred on-chain
	Timestamp time.Time `json:"timestamp"`

	// EventType categorizes the event (trade, position, liquidation)
	EventType EventType `json:"event_type"`

	// Trader is the wallet address involved
	Trader string `json:"trader"`

	// Symbol is the trading pair (e.g., "ETH", "BTC")
	Symbol string `json:"symbol"`

	// RawData contains the complete event data
	RawData json.RawMessage `json:"raw_data"`

	// CreatedAt is when Tracehound recorded this event
	CreatedAt time.Time `json:"created_at,omitempty"`
}

// TradeScent represents a decoded trade event.
type TradeScent struct {
	WhaleScent

	// Side is the trade direction ("BUY" or "SELL")
	Side string `json:"side"`

	// Price is the execution price
	Price string `json:"price"`

	// Size is the trade quantity
	Size string `json:"size"`

	// Fee is the trading fee paid
	Fee string `json:"fee,omitempty"`
}

// PositionScent represents a decoded position event.
type PositionScent struct {
	WhaleScent

	// EntryPrice is the average entry price
	EntryPrice string `json:"entry_price"`

	// Size is the position size (negative for short)
	Size string `json:"size"`

	// Leverage is the position leverage
	Leverage int `json:"leverage"`

	// UnrealizedPnL is the current unrealized profit/loss
	UnrealizedPnL string `json:"unrealized_pnl"`

	// LiquidationPrice is the liquidation price
	LiquidationPrice string `json:"liquidation_price,omitempty"`
}

// LiquidationScent represents a decoded liquidation event.
type LiquidationScent struct {
	WhaleScent

	// LiquidatedUser is the address that was liquidated
	LiquidatedUser string `json:"liquidated_user"`

	// MarkPrice is the mark price at liquidation
	MarkPrice string `json:"mark_price"`

	// Size is the liquidated position size
	Size string `json:"size"`

	// Side is the position side that was liquidated
	Side string `json:"side"`
}

// WhaleScentBatch represents a batch of events for bulk operations.
type WhaleScentBatch struct {
	Events    []WhaleScent `json:"events"`
	BatchID   string       `json:"batch_id,omitempty"`
	Timestamp time.Time    `json:"timestamp"`
}

// WhaleSummary provides aggregate statistics for a whale.
type WhaleSummary struct {
	Trader        string    `json:"trader"`
	TotalEvents   int64     `json:"total_events"`
	TradeCount    int64     `json:"trade_count"`
	PositionCount int64     `json:"position_count"`
	LiquidCount   int64     `json:"liquidation_count"`
	SymbolsTraded []string  `json:"symbols_traded"`
	FirstSeen     time.Time `json:"first_seen"`
	LastSeen      time.Time `json:"last_seen"`
}

// IsValid checks if the whale scent has required fields.
func (s *WhaleScent) IsValid() bool {
	return s.TxHash != "" &&
		s.Trader != "" &&
		s.Symbol != "" &&
		!s.Timestamp.IsZero()
}
