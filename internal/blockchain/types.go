// Package blockchain provides the core blockchain client and event types
// for sniffing whale activities on the Hyperliquid DEX.
package blockchain

import (
	"encoding/json"
	"time"
)

// EventType represents the type of blockchain event tracked by Tracehound.
// Like different scents, each event type tells a unique story about whale behavior.
type EventType string

const (
	// EventTypeTrade represents a trade execution event - the freshest scent of whale activity.
	EventTypeTrade EventType = "trade"

	// EventTypePosition represents a position change event - tracking the whale's trail.
	EventTypePosition EventType = "position"

	// EventTypeLiquidation represents a liquidation event - when whales get caught.
	EventTypeLiquidation EventType = "liquidation"
)

// RawEvent represents a raw blockchain event captured by Tracehound.
// Each event is a scent that helps us track whale movements on the blockchain.
type RawEvent struct {
	// TxHash is the unique transaction hash - the whale's pawprint
	TxHash string `json:"tx_hash"`

	// BlockHeight is the block number where this scent was found
	BlockHeight uint64 `json:"block_height"`

	// Timestamp is when this event occurred on the blockchain
	Timestamp time.Time `json:"timestamp"`

	// EventType categorizes the scent (trade, position, liquidation)
	EventType EventType `json:"event_type"`

	// Trader is the whale's address we're tracking
	Trader string `json:"trader"`

	// Symbol is the trading pair (e.g., "ETH", "BTC")
	Symbol string `json:"symbol"`

	// RawData contains the complete event data in its original form
	RawData json.RawMessage `json:"raw_data"`
}

// TradeData represents the decoded data for a trade event.
// TODO: Adjust according to actual Hyperliquid WebSocket API
type TradeData struct {
	Side   string `json:"side"`   // "BUY" or "SELL"
	Price  string `json:"px"`     // Execution price
	Size   string `json:"sz"`     // Trade size
	Fee    string `json:"fee"`    // Trading fee
	IsTake bool   `json:"isTake"` // Whether this was a taker order
}

// PositionData represents the decoded data for a position event.
// TODO: Adjust according to actual Hyperliquid WebSocket API
type PositionData struct {
	EntryPrice     string `json:"entryPx"`       // Average entry price
	Size           string `json:"szi"`           // Position size (negative for short)
	Leverage       int    `json:"leverage"`      // Position leverage
	UnrealizedPnL  string `json:"unrealizedPnl"` // Unrealized profit/loss
	LiquidationPx  string `json:"liquidationPx"` // Liquidation price
	MarginUsed     string `json:"marginUsed"`    // Margin used for position
	MaxTradeSz     string `json:"maxTradeSz"`    // Maximum trade size allowed
	ReturnOnEquity string `json:"returnOnEquity"`
}

// LiquidationData represents the decoded data for a liquidation event.
// TODO: Adjust according to actual Hyperliquid WebSocket API
type LiquidationData struct {
	LiquidatedUser string `json:"liquidatedUser"` // Address that got liquidated
	MarkPrice      string `json:"markPx"`         // Mark price at liquidation
	Size           string `json:"sz"`             // Liquidated position size
	Side           string `json:"side"`           // Position side that was liquidated
}

// WebSocketMessage represents a message received from the Hyperliquid WebSocket.
// TODO: Adjust according to actual Hyperliquid WebSocket API
type WebSocketMessage struct {
	Channel string          `json:"channel"`
	Data    json.RawMessage `json:"data"`
}

// SubscriptionRequest represents a WebSocket subscription request.
// TODO: Adjust according to actual Hyperliquid WebSocket API
type SubscriptionRequest struct {
	Method       string       `json:"method"`
	Subscription Subscription `json:"subscription"`
}

// Subscription defines the subscription parameters.
type Subscription struct {
	Type string `json:"type"`
	User string `json:"user,omitempty"`
	Coin string `json:"coin,omitempty"`
}
