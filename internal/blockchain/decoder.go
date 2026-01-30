// Package blockchain provides event decoding for Hyperliquid WebSocket messages.
package blockchain

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

// Decoder decodes raw WebSocket messages into structured RawEvent objects.
// It's the hound's nose - turning raw scents into actionable intelligence.
type Decoder struct {
	logger *logrus.Logger
}

// NewDecoder creates a new event decoder.
func NewDecoder(logger *logrus.Logger) *Decoder {
	if logger == nil {
		logger = logrus.New()
	}
	return &Decoder{logger: logger}
}

// DecodeMessage parses a WebSocket message and returns decoded events.
// TODO: Adjust according to actual Hyperliquid WebSocket API
// Current implementation is based on assumed message format.
func (d *Decoder) DecodeMessage(data []byte) ([]RawEvent, error) {
	// First, try to parse as a generic WebSocket message
	var msg WebSocketMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	// Route to appropriate decoder based on channel
	switch msg.Channel {
	case "trades":
		return d.decodeTradeEvents(msg.Data)
	case "positions":
		return d.decodePositionEvents(msg.Data)
	case "liquidations":
		return d.decodeLiquidationEvents(msg.Data)
	case "allMids":
		// allMids is for price updates, we'll skip these for now
		return nil, nil
	case "pong":
		// Ping/pong responses
		return nil, nil
	default:
		d.logger.WithField("channel", msg.Channel).Debug("Unknown channel, skipping")
		return nil, nil
	}
}

// TradeEventData represents the actual format of trade events from Hyperliquid.
// Based on: https://hyperliquid.gitbook.io/hyperliquid-docs/for-developers/api/websocket/subscriptions
type TradeEventData struct {
	Coin  string   `json:"coin"`
	Side  string   `json:"side"`
	Px    string   `json:"px"`
	Sz    string   `json:"sz"`
	Hash  string   `json:"hash"`
	Time  int64    `json:"time"`
	Tid   int64    `json:"tid"`            // 50-bit hash of (buyer_oid, seller_oid)
	Users []string `json:"users"`          // [buyer, seller]
}

// decodeTradeEvents decodes trade channel messages.
func (d *Decoder) decodeTradeEvents(data json.RawMessage) ([]RawEvent, error) {
	// Try to unmarshal as single event first
	var singleEvent TradeEventData
	if err := json.Unmarshal(data, &singleEvent); err == nil && singleEvent.Hash != "" {
		event := d.tradeToRawEvent(singleEvent, data)
		return []RawEvent{event}, nil
	}

	// Try as array of events
	var events []TradeEventData
	if err := json.Unmarshal(data, &events); err != nil {
		return nil, fmt.Errorf("failed to decode trade events: %w", err)
	}

	result := make([]RawEvent, 0, len(events))
	for _, e := range events {
		// Re-marshal individual event for raw data
		rawData, _ := json.Marshal(e)
		event := d.tradeToRawEvent(e, rawData)
		result = append(result, event)
	}

	return result, nil
}

// tradeToRawEvent converts a TradeEventData to a RawEvent.
// For trades, we track the taker (the one who initiated the trade).
// Side indicates taker's side: "B" = buyer is taker, "A" = seller is taker
func (d *Decoder) tradeToRawEvent(e TradeEventData, rawData []byte) RawEvent {
	timestamp := time.Unix(e.Time/1000, (e.Time%1000)*1000000)
	if e.Time < 1e12 {
		// If time is in seconds rather than milliseconds
		timestamp = time.Unix(e.Time, 0)
	}

	// Determine the taker based on side
	// "B" = buyer is taker (index 0), "A" = seller is taker (index 1)
	trader := ""
	if len(e.Users) >= 2 {
		if e.Side == "B" {
			trader = e.Users[0] // buyer is taker
		} else {
			trader = e.Users[1] // seller is taker
		}
	} else if len(e.Users) == 1 {
		trader = e.Users[0]
	}

	// Use tid as a pseudo block height since Hyperliquid doesn't provide block height
	return RawEvent{
		TxHash:      e.Hash,
		BlockHeight: uint64(e.Tid),
		Timestamp:   timestamp,
		EventType:   EventTypeTrade,
		Trader:      trader,
		Symbol:      e.Coin,
		RawData:     rawData,
	}
}

// PositionEventData represents position updates from Hyperliquid.
// TODO: Adjust according to actual Hyperliquid WebSocket API
type PositionEventData struct {
	Hash          string `json:"hash"`
	Height        uint64 `json:"height"`
	Time          int64  `json:"time"`
	User          string `json:"user"`
	Coin          string `json:"coin"`
	Szi           string `json:"szi"`
	EntryPx       string `json:"entryPx"`
	Leverage      int    `json:"leverage"`
	UnrealizedPnl string `json:"unrealizedPnl"`
}

// decodePositionEvents decodes position channel messages.
func (d *Decoder) decodePositionEvents(data json.RawMessage) ([]RawEvent, error) {
	var singleEvent PositionEventData
	if err := json.Unmarshal(data, &singleEvent); err == nil && singleEvent.Hash != "" {
		event := d.positionToRawEvent(singleEvent, data)
		return []RawEvent{event}, nil
	}

	var events []PositionEventData
	if err := json.Unmarshal(data, &events); err != nil {
		return nil, fmt.Errorf("failed to decode position events: %w", err)
	}

	result := make([]RawEvent, 0, len(events))
	for _, e := range events {
		rawData, _ := json.Marshal(e)
		event := d.positionToRawEvent(e, rawData)
		result = append(result, event)
	}

	return result, nil
}

// positionToRawEvent converts a PositionEventData to a RawEvent.
func (d *Decoder) positionToRawEvent(e PositionEventData, rawData []byte) RawEvent {
	timestamp := time.Unix(e.Time/1000, (e.Time%1000)*1000000)
	if e.Time < 1e12 {
		timestamp = time.Unix(e.Time, 0)
	}

	return RawEvent{
		TxHash:      e.Hash,
		BlockHeight: e.Height,
		Timestamp:   timestamp,
		EventType:   EventTypePosition,
		Trader:      e.User,
		Symbol:      e.Coin,
		RawData:     rawData,
	}
}

// LiquidationEventData represents liquidation events from Hyperliquid.
// TODO: Adjust according to actual Hyperliquid WebSocket API
type LiquidationEventData struct {
	Hash           string `json:"hash"`
	Height         uint64 `json:"height"`
	Time           int64  `json:"time"`
	LiquidatedUser string `json:"liquidatedUser"`
	Coin           string `json:"coin"`
	Side           string `json:"side"`
	Sz             string `json:"sz"`
	MarkPx         string `json:"markPx"`
}

// decodeLiquidationEvents decodes liquidation channel messages.
func (d *Decoder) decodeLiquidationEvents(data json.RawMessage) ([]RawEvent, error) {
	var singleEvent LiquidationEventData
	if err := json.Unmarshal(data, &singleEvent); err == nil && singleEvent.Hash != "" {
		event := d.liquidationToRawEvent(singleEvent, data)
		return []RawEvent{event}, nil
	}

	var events []LiquidationEventData
	if err := json.Unmarshal(data, &events); err != nil {
		return nil, fmt.Errorf("failed to decode liquidation events: %w", err)
	}

	result := make([]RawEvent, 0, len(events))
	for _, e := range events {
		rawData, _ := json.Marshal(e)
		event := d.liquidationToRawEvent(e, rawData)
		result = append(result, event)
	}

	return result, nil
}

// liquidationToRawEvent converts a LiquidationEventData to a RawEvent.
func (d *Decoder) liquidationToRawEvent(e LiquidationEventData, rawData []byte) RawEvent {
	timestamp := time.Unix(e.Time/1000, (e.Time%1000)*1000000)
	if e.Time < 1e12 {
		timestamp = time.Unix(e.Time, 0)
	}

	return RawEvent{
		TxHash:      e.Hash,
		BlockHeight: e.Height,
		Timestamp:   timestamp,
		EventType:   EventTypeLiquidation,
		Trader:      e.LiquidatedUser,
		Symbol:      e.Coin,
		RawData:     rawData,
	}
}
