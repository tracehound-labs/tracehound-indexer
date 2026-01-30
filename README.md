# Tracehound Indexer

```
     /\_/\
    ( o.o )  ~~~ 💨📊
     > ^ <
    /|   |\
   (_|   |_)
```

🐕 **Sniff Out Smart Money** | Real-time Hyperliquid blockchain indexer

[![License: MIT](https://img.shields.io/badge/License-MIT-orange.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://golang.org)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?logo=docker)](https://www.docker.com)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-15-336791?logo=postgresql)](https://www.postgresql.org)
[![Kafka](https://img.shields.io/badge/Kafka-3.5-231F20?logo=apache-kafka)](https://kafka.apache.org)

---

## 📖 Overview

Tracehound Indexer is an AI-powered platform's Layer 1 data pipeline component that tracks whale behaviors on the Hyperliquid DEX. Like a loyal hound following a scent trail, it captures raw blockchain events in real-time and stores them for downstream analysis.

**Key Capabilities:**
- 🔌 Real-time WebSocket connection to Hyperliquid blockchain
- 📊 Captures Trade, Position, and Liquidation events
- 🗄️ Persistent storage in PostgreSQL
- 📡 Event streaming via Kafka
- 🔄 Automatic reconnection with exponential backoff
- 🛡️ Graceful shutdown handling

---

## 🐕 Why Tracehound?

| Feature | Description |
|---------|-------------|
| **Real-time** | Sniffs blockchain events as they happen |
| **Reliable** | Never loses the trail - auto-reconnects on failures |
| **Scalable** | Batch processing and Kafka streaming for high throughput |
| **Observable** | Structured JSON logging for easy monitoring |
| **Production-ready** | Docker support, graceful shutdown, connection pooling |

---

## ✨ Features

- **WebSocket Client**: Maintains persistent connection to Hyperliquid with automatic reconnection
- **Event Decoding**: Parses trade, position, and liquidation events from the blockchain
- **PostgreSQL Storage**: Batch inserts with transaction support and optimized indexes
- **Kafka Producer**: Publishes events for real-time consumption by downstream services
- **Flexible Configuration**: YAML config with environment variable overrides
- **Graceful Shutdown**: Handles SIGINT/SIGTERM for clean termination
- **Exponential Backoff**: Reconnection delays: 1s → 2s → 4s → ... → 30s max

---

## 🚀 Quick Start

### Using Docker Compose (Recommended)

The fastest way to get started with all dependencies:

```bash
# Clone the repository
git clone https://github.com/tracehound-labs/tracehound-indexer.git
cd tracehound-indexer

# Start all services (PostgreSQL, Kafka, Zookeeper, Indexer)
make docker-up

# View logs
make logs

# Stop services
make docker-down
```

### Manual Installation

```bash
# Clone the repository
git clone https://github.com/tracehound-labs/tracehound-indexer.git
cd tracehound-indexer

# Install dependencies
go mod download

# Build the binary
make build

# Configure environment
cp .env.example .env
# Edit .env with your settings

# Run the indexer
./bin/indexer -config configs/config.yaml
```

---

## ⚙️ Configuration

### Configuration File

The indexer reads configuration from `configs/config.yaml`:

```yaml
# Hyperliquid WebSocket connection
hyperliquid:
  ws_url: "wss://api.hyperliquid.xyz/ws"
  reconnect_interval: 5s
  max_reconnect_attempts: 0  # 0 = infinite

# PostgreSQL database
postgres:
  host: localhost
  port: 5432
  database: tracehound
  user: hound
  password: ${POSTGRES_PASSWORD}
  max_connections: 25

# Kafka producer
kafka:
  brokers:
    - localhost:9092
  topic: whale_scents
  batch_size: 100
  compression: snappy

# Logging
logging:
  level: info
  format: json
```

### Environment Variables

All configuration can be overridden via environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `HYPERLIQUID_WS_URL` | WebSocket endpoint | `wss://api.hyperliquid.xyz/ws` |
| `POSTGRES_URL` | Full PostgreSQL URL | - |
| `POSTGRES_HOST` | Database host | `localhost` |
| `POSTGRES_PORT` | Database port | `5432` |
| `POSTGRES_USER` | Database user | `hound` |
| `POSTGRES_PASSWORD` | Database password | - |
| `POSTGRES_DATABASE` | Database name | `tracehound` |
| `KAFKA_BROKERS` | Kafka brokers (comma-separated) | `localhost:9092` |
| `KAFKA_TOPIC` | Kafka topic name | `whale_scents` |
| `LOG_LEVEL` | Log level (debug/info/warn/error) | `info` |
| `LOG_FORMAT` | Log format (json/text) | `json` |

---

## 🛠️ Development

### Prerequisites

- Go 1.21+
- Docker & Docker Compose
- PostgreSQL 15+ (or use Docker)
- Kafka 3.5+ (or use Docker)

### Commands

```bash
# Build
make build           # Build binary
make build-linux     # Build for Linux

# Run
make run             # Run locally
make run-dev         # Run with debug logging

# Test
make test            # Run tests
make test-coverage   # Run tests with coverage
make test-race       # Run tests with race detector

# Code Quality
make fmt             # Format code
make vet             # Run go vet
make lint            # Run linter (requires golangci-lint)
make tidy            # Tidy go modules

# Docker
make docker-build    # Build Docker image
make docker-up       # Start all services
make docker-down     # Stop all services
make logs            # View logs

# Database
make db-migrate      # Run migrations
make db-shell        # Open psql shell

# Cleanup
make clean           # Clean build artifacts
make clean-all       # Clean everything including volumes
```

---

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     Hyperliquid Blockchain                       │
│                    (wss://api.hyperliquid.xyz)                   │
└─────────────────────────────────┬───────────────────────────────┘
                                  │
                                  │ WebSocket
                                  ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Tracehound Indexer                          │
│  ┌───────────────┐  ┌──────────────┐  ┌───────────────────────┐ │
│  │   WebSocket   │  │    Event     │  │    Batch Processor    │ │
│  │    Client     │─▶│   Decoder    │─▶│   (100 events/batch)  │ │
│  └───────────────┘  └──────────────┘  └───────────┬───────────┘ │
│         │                                          │             │
│         │ Reconnect                     ┌──────────┴──────────┐ │
│         │ (exponential backoff)         │                     │ │
│         ▼                               ▼                     ▼ │
│  ┌───────────────┐              ┌───────────────┐  ┌──────────┐ │
│  │ Retry Logic   │              │  PostgreSQL   │  │  Kafka   │ │
│  │ 1s→2s→4s→30s  │              │    Store      │  │ Producer │ │
│  └───────────────┘              └───────────────┘  └──────────┘ │
└─────────────────────────────────────────────────────────────────┘
                                  │                      │
                                  ▼                      ▼
                          ┌───────────────┐    ┌─────────────────┐
                          │  PostgreSQL   │    │      Kafka      │
                          │   Database    │    │  (whale_scents) │
                          └───────────────┘    └─────────────────┘
```

### Data Flow

1. **Connect**: WebSocket client establishes connection to Hyperliquid
2. **Subscribe**: Client subscribes to trade/position/liquidation events
3. **Receive**: Raw events stream in via WebSocket
4. **Decode**: Events are parsed and structured
5. **Batch**: Events accumulate (max 100 or 1s timeout)
6. **Store**: Batch is written to PostgreSQL (concurrent with Kafka)
7. **Publish**: Batch is published to Kafka topic
8. **Repeat**: Process continues until shutdown signal

---

## 📊 Database Schema

The `whale_scents` table stores all tracked events:

```sql
CREATE TABLE whale_scents (
    id BIGSERIAL PRIMARY KEY,
    tx_hash TEXT UNIQUE NOT NULL,
    block_height BIGINT NOT NULL,
    event_type TEXT NOT NULL,  -- 'trade', 'position', 'liquidation'
    timestamp TIMESTAMPTZ NOT NULL,
    trader TEXT NOT NULL,
    symbol TEXT NOT NULL,
    raw_data JSONB NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
```

### Indexes

| Index | Purpose |
|-------|---------|
| `idx_scent_tracker` | Query by trader + time |
| `idx_scent_trail` | Query by block height |
| `idx_scent_type` | Filter by event type |
| `idx_fresh_scents` | Get latest events |
| `idx_scent_data` | Search within raw JSON |
| `idx_scent_symbol` | Query by symbol + time |

---

## 📡 Kafka Events

Events are published to the `whale_scents` topic with:

- **Key**: Transaction hash (for partitioning)
- **Value**: JSON-encoded event
- **Headers**: `event_type`, `trader`, `symbol`

### Message Format

```json
{
  "tx_hash": "0xabc123...",
  "block_height": 1234567,
  "timestamp": "2024-01-15T10:30:00Z",
  "event_type": "trade",
  "trader": "0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb",
  "symbol": "ETH",
  "raw_data": {
    "side": "BUY",
    "px": "2250.50",
    "sz": "1.5"
  }
}
```

---

## 🔧 API Reference

### Public Models (`pkg/models`)

```go
// WhaleScent represents a tracked blockchain event
type WhaleScent struct {
    ID          int64           `json:"id,omitempty"`
    TxHash      string          `json:"tx_hash"`
    BlockHeight uint64          `json:"block_height"`
    Timestamp   time.Time       `json:"timestamp"`
    EventType   EventType       `json:"event_type"`
    Trader      string          `json:"trader"`
    Symbol      string          `json:"symbol"`
    RawData     json.RawMessage `json:"raw_data"`
    CreatedAt   time.Time       `json:"created_at,omitempty"`
}

// EventType constants
const (
    EventTypeTrade       EventType = "trade"
    EventTypePosition    EventType = "position"
    EventTypeLiquidation EventType = "liquidation"
)
```

---

## 🤝 Contributing

We welcome contributions! Here's how you can help:

1. **Fork** the repository
2. **Create** a feature branch (`git checkout -b feature/amazing-feature`)
3. **Commit** your changes (`git commit -m 'Add amazing feature'`)
4. **Push** to the branch (`git push origin feature/amazing-feature`)
5. **Open** a Pull Request

### Guidelines

- Follow Go best practices and idioms
- Add tests for new functionality
- Update documentation as needed
- Use meaningful commit messages

---

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

## 🌐 Community

- **GitHub Issues**: [Report bugs](https://github.com/tracehound-labs/tracehound-indexer/issues)
- **Discussions**: [Ask questions](https://github.com/tracehound-labs/tracehound-indexer/discussions)

---

## 🙏 Acknowledgments

- [Hyperliquid](https://hyperliquid.xyz) for the amazing DEX
- The Go community for excellent tools and libraries

---

<p align="center">
  <strong>🐕 Happy whale hunting!</strong><br>
  <em>Built with ❤️ by Tracehound Labs</em>
</p>
