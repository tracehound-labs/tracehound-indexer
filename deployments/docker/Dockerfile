# Tracehound Indexer Dockerfile
# 🐕 Multi-stage build for minimal production image

# ============================================================================
# Build Stage
# ============================================================================
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go module files first for better caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
# CGO_ENABLED=0 for static binary
# -ldflags to reduce binary size
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.Version=$(git describe --tags --always --dirty 2>/dev/null || echo 'dev')" \
    -a -installsuffix cgo \
    -o indexer \
    ./cmd/indexer

# ============================================================================
# Production Stage
# ============================================================================
FROM alpine:3.19

# Add labels
LABEL org.opencontainers.image.title="Tracehound Indexer"
LABEL org.opencontainers.image.description="Real-time Hyperliquid blockchain indexer"
LABEL org.opencontainers.image.source="https://github.com/tracehound-labs/tracehound-indexer"
LABEL org.opencontainers.image.vendor="Tracehound Labs"
LABEL org.opencontainers.image.licenses="MIT"

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user for security
RUN addgroup -g 1000 tracehound && \
    adduser -u 1000 -G tracehound -s /bin/sh -D tracehound

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/indexer .

# Copy default config
COPY configs/config.yaml ./configs/

# Change ownership to non-root user
RUN chown -R tracehound:tracehound /app

# Switch to non-root user
USER tracehound

# Expose metrics port (if implemented)
# EXPOSE 9090

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD pgrep indexer || exit 1

# Run the indexer
ENTRYPOINT ["./indexer"]
CMD ["-config", "configs/config.yaml"]
