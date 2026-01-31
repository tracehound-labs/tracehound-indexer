// Package blockchain provides WebSocket connectivity to the Hyperliquid blockchain.
// Like a loyal hound, this client maintains connection even when the trail goes cold.
package blockchain

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"nhooyr.io/websocket"
)

// Default configuration values for the WebSocket client
const (
	// DefaultEventBufferSize is the channel buffer for incoming events
	DefaultEventBufferSize = 1000

	// InitialReconnectDelay is the starting delay for exponential backoff
	InitialReconnectDelay = 1 * time.Second

	// MaxReconnectDelay is the maximum delay between reconnection attempts
	MaxReconnectDelay = 30 * time.Second

	// PingInterval is how often we send ping messages to keep connection alive
	PingInterval = 30 * time.Second

	// ReadTimeout is the timeout for reading messages
	ReadTimeout = 60 * time.Second
)

// HyperliquidClient tracks whale movements on the Hyperliquid blockchain.
// Like a loyal hound, it maintains connection even when the trail goes cold.
type HyperliquidClient struct {
	wsURL      string
	trackCoins []string
	conn       *websocket.Conn
	eventChan  chan RawEvent
	ctx        context.Context
	cancel     context.CancelFunc
	logger     *logrus.Logger

	mu           sync.RWMutex
	isConnected  bool
	reconnecting bool
}

// DefaultTrackCoins are the default coins to track if none specified.
var DefaultTrackCoins = []string{"BTC", "ETH", "SOL", "HYPE"}

// NewHyperliquidClient creates a new client ready to sniff blockchain events.
// The wsURL should be the Hyperliquid WebSocket endpoint (e.g., wss://api.hyperliquid.xyz/ws)
// trackCoins specifies which coins to subscribe to for trade events.
func NewHyperliquidClient(wsURL string, trackCoins []string, logger *logrus.Logger) *HyperliquidClient {
	ctx, cancel := context.WithCancel(context.Background())

	if logger == nil {
		logger = logrus.New()
		logger.SetFormatter(&logrus.JSONFormatter{})
	}

	// Use defaults if no coins specified
	if len(trackCoins) == 0 {
		trackCoins = DefaultTrackCoins
	}

	return &HyperliquidClient{
		wsURL:      wsURL,
		trackCoins: trackCoins,
		eventChan:  make(chan RawEvent, DefaultEventBufferSize),
		ctx:        ctx,
		cancel:     cancel,
		logger:     logger,
	}
}

// Connect establishes WebSocket connection and subscribes to blockchain events.
// Returns an error if initial connection fails.
func (c *HyperliquidClient) Connect() error {
	c.logger.Info("🐕 Attempting to connect to Hyperliquid...")

	conn, _, err := websocket.Dial(c.ctx, c.wsURL, &websocket.DialOptions{
		CompressionMode: websocket.CompressionContextTakeover,
	})
	if err != nil {
		return fmt.Errorf("failed to dial WebSocket: %w", err)
	}

	// Set read limit to 10MB to handle large messages
	conn.SetReadLimit(10 * 1024 * 1024)

	c.mu.Lock()
	c.conn = conn
	c.isConnected = true
	c.mu.Unlock()

	// Subscribe to relevant channels
	if err := c.subscribe(); err != nil {
		c.conn.Close(websocket.StatusInternalError, "subscription failed")
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	c.logger.Info("👃 Connected to Hyperliquid, sniffing blockchain events...")

	// Start the read loop in a goroutine
	go c.readLoop()

	// Start ping loop to keep connection alive
	go c.pingLoop()

	return nil
}

// subscribe sends subscription messages to the Hyperliquid WebSocket.
// Based on Hyperliquid WebSocket API: https://hyperliquid.gitbook.io/hyperliquid-docs/for-developers/api/websocket/subscriptions
func (c *HyperliquidClient) subscribe() error {
	// Subscribe to allMids for price updates (validates connection)
	allMidsSub := SubscriptionRequest{
		Method: "subscribe",
		Subscription: Subscription{
			Type: "allMids",
		},
	}

	data, err := json.Marshal(allMidsSub)
	if err != nil {
		return fmt.Errorf("failed to marshal allMids subscription: %w", err)
	}

	if err := c.conn.Write(c.ctx, websocket.MessageText, data); err != nil {
		return fmt.Errorf("failed to send allMids subscription: %w", err)
	}
	c.logger.Debug("📡 Subscribed to allMids")

	// Small delay to let the server process
	time.Sleep(100 * time.Millisecond)

	// Subscribe to trades for each configured coin
	// Note: Hyperliquid requires specifying coin for trades subscription
	for _, coin := range c.trackCoins {
		sub := SubscriptionRequest{
			Method: "subscribe",
			Subscription: Subscription{
				Type: "trades",
				Coin: coin,
			},
		}

		data, err := json.Marshal(sub)
		if err != nil {
			c.logger.WithError(err).WithField("coin", coin).Warn("Failed to marshal trades subscription")
			continue
		}

		if err := c.conn.Write(c.ctx, websocket.MessageText, data); err != nil {
			c.logger.WithError(err).WithField("coin", coin).Warn("Failed to send trades subscription")
			continue
		}

		c.logger.WithField("coin", coin).Debug("📡 Subscribed to trades")

		// Small delay between subscriptions to avoid overwhelming the server
		time.Sleep(50 * time.Millisecond)
	}

	c.logger.WithFields(logrus.Fields{
		"coins": c.trackCoins,
		"count": len(c.trackCoins),
	}).Info("👃 Subscribed to trade feeds")
	return nil
}

// StreamEvents returns a read-only channel that emits fresh scents (events) as detected.
// The channel remains open until Close() is called.
func (c *HyperliquidClient) StreamEvents() <-chan RawEvent {
	return c.eventChan
}

// readLoop continuously reads messages from the WebSocket connection.
// Like a vigilant hound, it never sleeps while on the hunt.
func (c *HyperliquidClient) readLoop() {
	decoder := NewDecoder(c.logger)

	for {
		select {
		case <-c.ctx.Done():
			c.logger.Info("🛑 Read loop stopping - context cancelled")
			return
		default:
		}

		c.mu.RLock()
		conn := c.conn
		c.mu.RUnlock()

		if conn == nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		// Set read deadline
		readCtx, cancel := context.WithTimeout(c.ctx, ReadTimeout)
		msgType, data, err := conn.Read(readCtx)
		cancel()

		if err != nil {
			c.mu.Lock()
			c.isConnected = false
			c.mu.Unlock()

			if c.ctx.Err() != nil {
				// Context was cancelled, exit gracefully
				return
			}

			c.logger.WithError(err).Warn("🔄 Lost connection, initiating reconnection...")
			go c.reconnect()
			return
		}

		if msgType != websocket.MessageText {
			continue
		}

		// Decode the message into events
		events, err := decoder.DecodeMessage(data)
		if err != nil {
			c.logger.WithError(err).Debug("Failed to decode message")
			continue
		}

		// Send events to channel
		for _, event := range events {
			select {
			case c.eventChan <- event:
				c.logger.WithFields(logrus.Fields{
					"trader": event.Trader,
					"symbol": event.Symbol,
					"type":   event.EventType,
				}).Debug("💨 Fresh scent detected!")
			case <-c.ctx.Done():
				return
			default:
				c.logger.Warn("⚠️ Event channel full, dropping event")
			}
		}
	}
}

// pingLoop sends periodic ping messages to keep the connection alive.
func (c *HyperliquidClient) pingLoop() {
	ticker := time.NewTicker(PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.mu.RLock()
			conn := c.conn
			isConnected := c.isConnected
			c.mu.RUnlock()

			if conn != nil && isConnected {
				pingCtx, cancel := context.WithTimeout(c.ctx, 10*time.Second)
				err := conn.Ping(pingCtx)
				cancel()

				if err != nil {
					c.logger.WithError(err).Debug("Ping failed")
				}
			}
		}
	}
}

// reconnect implements exponential backoff reconnection logic.
// Like a persistent hound, it never gives up on finding the trail.
func (c *HyperliquidClient) reconnect() {
	c.mu.Lock()
	if c.reconnecting {
		c.mu.Unlock()
		return
	}
	c.reconnecting = true
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.reconnecting = false
		c.mu.Unlock()
	}()

	delay := InitialReconnectDelay
	attempt := 0

	for {
		select {
		case <-c.ctx.Done():
			c.logger.Info("🛑 Reconnection cancelled - context done")
			return
		default:
		}

		attempt++
		c.logger.WithFields(logrus.Fields{
			"attempt": attempt,
			"delay":   delay,
		}).Info("🔄 Attempting reconnection...")

		// Close existing connection if any
		c.mu.Lock()
		if c.conn != nil {
			c.conn.Close(websocket.StatusGoingAway, "reconnecting")
			c.conn = nil
		}
		c.mu.Unlock()

		// Attempt to reconnect
		conn, _, err := websocket.Dial(c.ctx, c.wsURL, &websocket.DialOptions{
			CompressionMode: websocket.CompressionContextTakeover,
		})

		if err != nil {
			c.logger.WithError(err).WithField("delay", delay).Warn("❌ Reconnection failed, waiting...")

			select {
			case <-c.ctx.Done():
				return
			case <-time.After(delay):
			}

			// Exponential backoff with cap
			delay *= 2
			if delay > MaxReconnectDelay {
				delay = MaxReconnectDelay
			}
			continue
		}

		// Set read limit to 10MB
		conn.SetReadLimit(10 * 1024 * 1024)

		c.mu.Lock()
		c.conn = conn
		c.isConnected = true
		c.mu.Unlock()

		// Re-subscribe
		if err := c.subscribe(); err != nil {
			c.logger.WithError(err).Error("Failed to re-subscribe after reconnection")
			continue
		}

		c.logger.Info("✅ Reconnection successful! Back on the trail...")

		// Restart read loop
		go c.readLoop()
		return
	}
}

// IsConnected returns whether the client is currently connected.
func (c *HyperliquidClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.isConnected
}

// Close gracefully shuts down the WebSocket client.
// The hound returns home after a successful hunt.
func (c *HyperliquidClient) Close() error {
	c.logger.Info("🏠 Tracehound client shutting down...")

	// Cancel context to stop all goroutines
	c.cancel()

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		err := c.conn.Close(websocket.StatusNormalClosure, "client shutdown")
		c.conn = nil
		c.isConnected = false

		// Close the event channel
		close(c.eventChan)

		if err != nil {
			return fmt.Errorf("error closing WebSocket: %w", err)
		}
	}

	return nil
}
