# Tracehound Indexer Makefile
# 🐕 Sniff Out Smart Money
#
# Usage:
#   make build     - Build the indexer binary
#   make run       - Run the indexer locally
#   make test      - Run all tests
#   make docker-up - Start all services with Docker Compose
#   make help      - Show this help message

.PHONY: all build run test clean docker-up docker-down docker-build logs lint fmt vet help

# Variables
BINARY_NAME=indexer
BINARY_DIR=bin
CMD_DIR=cmd/indexer
DOCKER_COMPOSE=docker-compose -f deployments/docker-compose.yml
GO=go
GOFMT=gofmt
GOVET=$(GO) vet

# Build flags
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-w -s -X main.Version=$(VERSION)"

# Default target
all: build

# ============================================================================
# Build
# ============================================================================

## build: Build the indexer binary
build:
	@echo "🔨 Building Tracehound Indexer..."
	@mkdir -p $(BINARY_DIR)
	$(GO) build $(LDFLAGS) -o $(BINARY_DIR)/$(BINARY_NAME) ./$(CMD_DIR)
	@echo "✅ Build complete: $(BINARY_DIR)/$(BINARY_NAME)"

## build-linux: Build for Linux (useful for Docker)
build-linux:
	@echo "🔨 Building for Linux..."
	@mkdir -p $(BINARY_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BINARY_DIR)/$(BINARY_NAME)-linux ./$(CMD_DIR)
	@echo "✅ Linux build complete"

## install: Install the binary to GOPATH/bin
install:
	@echo "📦 Installing Tracehound Indexer..."
	$(GO) install $(LDFLAGS) ./$(CMD_DIR)
	@echo "✅ Installed to $(shell go env GOPATH)/bin/$(BINARY_NAME)"

# ============================================================================
# Run
# ============================================================================

## run: Run the indexer locally
run:
	@echo "🐕 Starting Tracehound Indexer..."
	$(GO) run ./$(CMD_DIR)

## run-dev: Run with development config
run-dev:
	@echo "🐕 Starting Tracehound Indexer (dev mode)..."
	LOG_LEVEL=debug $(GO) run ./$(CMD_DIR) -config configs/config.yaml

# ============================================================================
# Test
# ============================================================================

## test: Run all tests
test:
	@echo "🧪 Running tests..."
	$(GO) test -v ./...

## test-coverage: Run tests with coverage report
test-coverage:
	@echo "🧪 Running tests with coverage..."
	$(GO) test -v -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "✅ Coverage report: coverage.html"

## test-race: Run tests with race detector
test-race:
	@echo "🧪 Running tests with race detector..."
	$(GO) test -v -race ./...

# ============================================================================
# Code Quality
# ============================================================================

## lint: Run linter (requires golangci-lint)
lint:
	@echo "🔍 Running linter..."
	@which golangci-lint > /dev/null || (echo "Please install golangci-lint" && exit 1)
	golangci-lint run ./...

## fmt: Format code
fmt:
	@echo "🎨 Formatting code..."
	$(GOFMT) -w -s .
	@echo "✅ Code formatted"

## vet: Run go vet
vet:
	@echo "🔍 Running go vet..."
	$(GOVET) ./...
	@echo "✅ Vet complete"

## tidy: Tidy go modules
tidy:
	@echo "📦 Tidying modules..."
	$(GO) mod tidy
	@echo "✅ Modules tidied"

# ============================================================================
# Docker
# ============================================================================

## docker-build: Build Docker image
docker-build:
	@echo "🐳 Building Docker image..."
	docker build -f deployments/docker/Dockerfile -t tracehound-indexer:latest .
	@echo "✅ Docker image built"

## docker-up: Start all services with Docker Compose
docker-up:
	@echo "🐳 Starting Tracehound stack..."
	$(DOCKER_COMPOSE) up -d
	@echo "✅ Stack started"
	@echo "📊 PostgreSQL: localhost:5432"
	@echo "📊 Kafka: localhost:9092"

## docker-down: Stop all services
docker-down:
	@echo "🛑 Stopping Tracehound stack..."
	$(DOCKER_COMPOSE) down
	@echo "✅ Stack stopped"

## docker-down-v: Stop all services and remove volumes
docker-down-v:
	@echo "🛑 Stopping Tracehound stack and removing volumes..."
	$(DOCKER_COMPOSE) down -v
	@echo "✅ Stack stopped and volumes removed"

## docker-logs: View logs from all services
logs:
	$(DOCKER_COMPOSE) logs -f

## docker-logs-indexer: View indexer logs
logs-indexer:
	$(DOCKER_COMPOSE) logs -f indexer

## docker-ps: Show running containers
docker-ps:
	$(DOCKER_COMPOSE) ps

## docker-restart: Restart all services
docker-restart: docker-down docker-up

# ============================================================================
# Database
# ============================================================================

## db-migrate: Run database migrations
db-migrate:
	@echo "🗄️ Running database migrations..."
	@PGPASSWORD=$${POSTGRES_PASSWORD:-trackhound123} psql -h localhost -U hound -d tracehound -f internal/storage/migrations/001_initial_schema.sql
	@echo "✅ Migrations complete"

## db-shell: Open PostgreSQL shell
db-shell:
	@PGPASSWORD=$${POSTGRES_PASSWORD:-trackhound123} psql -h localhost -U hound -d tracehound

# ============================================================================
# Cleanup
# ============================================================================

## clean: Clean build artifacts
clean:
	@echo "🧹 Cleaning build artifacts..."
	rm -rf $(BINARY_DIR)
	rm -f coverage.out coverage.html
	@echo "✅ Cleaned"

## clean-all: Clean everything including Docker volumes
clean-all: clean docker-down-v
	@echo "✅ Full cleanup complete"

# ============================================================================
# Help
# ============================================================================

## help: Show this help message
help:
	@echo ""
	@echo "     /\\_/\\"
	@echo "    ( o.o )  ~~~ 💨📊"
	@echo "     > ^ <"
	@echo "    /|   |\\"
	@echo "   (_|   |_)"
	@echo ""
	@echo "  Tracehound Indexer - Makefile Commands"
	@echo "  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo ""
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/  /'
	@echo ""
