// Tracehound Indexer - Main Entry Point
//
// 🐕 Sniff Out Smart Money
//
// This is the primary entry point for the Tracehound Indexer, a real-time
// blockchain event tracker for the Hyperliquid DEX. Like a loyal hound,
// it tirelessly follows the scent of whale activity.
package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"github.com/tracehound-labs/tracehound-indexer/internal/blockchain"
	"github.com/tracehound-labs/tracehound-indexer/internal/config"
	"github.com/tracehound-labs/tracehound-indexer/internal/kafka"
	"github.com/tracehound-labs/tracehound-indexer/internal/storage"
)

const (
	// DefaultConfigPath is the default location for the config file
	DefaultConfigPath = "configs/config.yaml"

	// Version is the current version of Tracehound Indexer
	Version = "0.1.0"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", DefaultConfigPath, "Path to configuration file")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	if *showVersion {
		printBanner()
		os.Exit(0)
	}

	// Initialize logger
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: time.RFC3339,
	})

	printBanner()
	logger.Info("🐕 Tracehound starting up...")

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		logger.WithError(err).Fatal("❌ Failed to load configuration")
	}

	// Set log level
	level, err := logrus.ParseLevel(cfg.Logging.Level)
	if err != nil {
		level = logrus.InfoLevel
	}
	logger.SetLevel(level)

	// Set log format
	if cfg.Logging.Format == "text" {
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: time.RFC3339,
		})
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		logger.WithError(err).Fatal("❌ Invalid configuration")
	}

	logger.WithFields(logrus.Fields{
		"ws_url":      cfg.Hyperliquid.WSURL,
		"pg_host":     cfg.Postgres.Host,
		"kafka_topic": cfg.Kafka.Topic,
	}).Info("📋 Configuration loaded")

	// Create root context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize PostgreSQL store
	var store *storage.PostgresStore
	if cfg.Postgres.URL != "" {
		store, err = storage.NewPostgresStoreFromURL(cfg.Postgres.URL, logger)
	} else {
		store, err = storage.NewPostgresStore(storage.PostgresConfig{
			Host:            cfg.Postgres.Host,
			Port:            cfg.Postgres.Port,
			Database:        cfg.Postgres.Database,
			User:            cfg.Postgres.User,
			Password:        cfg.Postgres.Password,
			SSLMode:         cfg.Postgres.SSLMode,
			MaxOpenConns:    cfg.Postgres.MaxConnections,
			MaxIdleConns:    cfg.Postgres.IdleConnections,
			ConnMaxLifetime: cfg.Postgres.ConnMaxLifetime,
		}, logger)
	}
	if err != nil {
		logger.WithError(err).Fatal("❌ Failed to connect to PostgreSQL")
	}
	defer store.Close()

	// Initialize Kafka producer
	producer, err := kafka.NewProducer(kafka.ProducerConfig{
		Brokers:      cfg.Kafka.Brokers,
		Topic:        cfg.Kafka.Topic,
		BatchSize:    cfg.Kafka.BatchSize,
		BatchTimeout: cfg.Kafka.BatchTimeout,
		Compression:  cfg.Kafka.Compression,
	}, logger)
	if err != nil {
		logger.WithError(err).Fatal("❌ Failed to initialize Kafka producer")
	}
	defer producer.Close()

	// Initialize Hyperliquid WebSocket client
	client := blockchain.NewHyperliquidClient(cfg.Hyperliquid.WSURL, logger)

	// Connect to Hyperliquid
	if err := client.Connect(); err != nil {
		logger.WithError(err).Fatal("❌ Failed to connect to Hyperliquid")
	}
	defer client.Close()

	// Create event processor
	processor := &EventProcessor{
		store:    store,
		producer: producer,
		logger:   logger,
		batchCfg: cfg.Batch,
	}

	// Start processing events
	go processor.Run(ctx, client.StreamEvents())

	logger.Info("✅ Tracehound is now tracking whale activity!")

	// Wait for shutdown signal
	waitForShutdown(ctx, cancel, logger, client, store, producer)
}

// EventProcessor handles batching and processing of blockchain events.
type EventProcessor struct {
	store    *storage.PostgresStore
	producer *kafka.Producer
	logger   *logrus.Logger
	batchCfg config.BatchConfig
}

// Run processes events from the event channel with batching.
func (p *EventProcessor) Run(ctx context.Context, events <-chan blockchain.RawEvent) {
	batch := make([]blockchain.RawEvent, 0, p.batchCfg.Size)
	ticker := time.NewTicker(p.batchCfg.Timeout)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Context cancelled, flush remaining batch
			if len(batch) > 0 {
				p.processBatch(ctx, batch)
			}
			p.logger.Info("🛑 Event processor stopped")
			return

		case event, ok := <-events:
			if !ok {
				// Channel closed, flush remaining batch
				if len(batch) > 0 {
					p.processBatch(ctx, batch)
				}
				p.logger.Info("📭 Event channel closed")
				return
			}

			batch = append(batch, event)

			// Log fresh scent detection
			p.logger.WithFields(logrus.Fields{
				"trader": event.Trader,
				"symbol": event.Symbol,
				"type":   event.EventType,
			}).Debug("💨 Fresh scent detected!")

			// Flush if batch is full
			if len(batch) >= p.batchCfg.Size {
				p.processBatch(ctx, batch)
				batch = make([]blockchain.RawEvent, 0, p.batchCfg.Size)
				ticker.Reset(p.batchCfg.Timeout)
			}

		case <-ticker.C:
			// Timeout, flush current batch
			if len(batch) > 0 {
				p.processBatch(ctx, batch)
				batch = make([]blockchain.RawEvent, 0, p.batchCfg.Size)
			}
		}
	}
}

// processBatch writes events to PostgreSQL and Kafka concurrently.
func (p *EventProcessor) processBatch(ctx context.Context, batch []blockchain.RawEvent) {
	if len(batch) == 0 {
		return
	}

	startTime := time.Now()

	// Process PostgreSQL and Kafka concurrently
	g, gctx := errgroup.WithContext(ctx)

	// Write to PostgreSQL
	g.Go(func() error {
		if err := p.store.BatchInsert(gctx, batch); err != nil {
			p.logger.WithError(err).Error("❌ Failed to store scents in database")
			return err
		}
		return nil
	})

	// Publish to Kafka
	g.Go(func() error {
		if err := p.producer.PublishBatch(gctx, batch); err != nil {
			p.logger.WithError(err).Error("❌ Failed to broadcast scents to Kafka")
			return err
		}
		return nil
	})

	// Wait for both to complete
	if err := g.Wait(); err != nil {
		p.logger.WithError(err).Warn("⚠️ Batch processing completed with errors")
	} else {
		p.logger.WithFields(logrus.Fields{
			"count":    len(batch),
			"duration": time.Since(startTime),
		}).Info("🦴 Batch processed successfully!")
	}
}

// waitForShutdown waits for OS signals and performs graceful shutdown.
func waitForShutdown(
	ctx context.Context,
	cancel context.CancelFunc,
	logger *logrus.Logger,
	client *blockchain.HyperliquidClient,
	store *storage.PostgresStore,
	producer *kafka.Producer,
) {
	// Create signal channel
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for signal
	sig := <-sigChan
	logger.WithField("signal", sig).Info("🏠 Received shutdown signal, initiating graceful shutdown...")

	// Cancel context to stop all goroutines
	cancel()

	// Give processing some time to flush
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Close in order
	done := make(chan struct{})
	go func() {
		// Close WebSocket client first (stops receiving new events)
		if err := client.Close(); err != nil {
			logger.WithError(err).Warn("Error closing WebSocket client")
		}

		// Close Kafka producer
		if err := producer.Close(); err != nil {
			logger.WithError(err).Warn("Error closing Kafka producer")
		}

		// Close PostgreSQL last
		if err := store.Close(); err != nil {
			logger.WithError(err).Warn("Error closing PostgreSQL connection")
		}

		close(done)
	}()

	select {
	case <-done:
		logger.Info("✅ Graceful shutdown completed. The hound rests.")
	case <-shutdownCtx.Done():
		logger.Warn("⚠️ Shutdown timeout exceeded, forcing exit")
	}
}

// printBanner prints the Tracehound ASCII art banner.
func printBanner() {
	banner := `
     /\_/\
    ( o.o )  ~~~ 💨📊
     > ^ <
    /|   |\
   (_|   |_)

  ████████╗██████╗  █████╗  ██████╗███████╗██╗  ██╗ ██████╗ ██╗   ██╗███╗   ██╗██████╗
  ╚══██╔══╝██╔══██╗██╔══██╗██╔════╝██╔════╝██║  ██║██╔═══██╗██║   ██║████╗  ██║██╔══██╗
     ██║   ██████╔╝███████║██║     █████╗  ███████║██║   ██║██║   ██║██╔██╗ ██║██║  ██║
     ██║   ██╔══██╗██╔══██║██║     ██╔══╝  ██╔══██║██║   ██║██║   ██║██║╚██╗██║██║  ██║
     ██║   ██║  ██║██║  ██║╚██████╗███████╗██║  ██║╚██████╔╝╚██████╔╝██║ ╚████║██████╔╝
     ╚═╝   ╚═╝  ╚═╝╚═╝  ╚═╝ ╚═════╝╚══════╝╚═╝  ╚═╝ ╚═════╝  ╚═════╝ ╚═╝  ╚═══╝╚═════╝
                                                                                  v` + Version + `
  🐕 Sniff Out Smart Money | Hyperliquid Blockchain Indexer
  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
`
	println(banner)
}
