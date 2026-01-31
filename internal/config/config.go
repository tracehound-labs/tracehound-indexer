// Package config provides configuration management for Tracehound Indexer.
// It loads settings from YAML files and environment variables.
package config

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds all configuration for the Tracehound Indexer.
type Config struct {
	Hyperliquid HyperliquidConfig `yaml:"hyperliquid"`
	Postgres    PostgresConfig    `yaml:"postgres"`
	Kafka       KafkaConfig       `yaml:"kafka"`
	Logging     LoggingConfig     `yaml:"logging"`
	Batch       BatchConfig       `yaml:"batch"`
}

// HyperliquidConfig holds WebSocket connection settings.
type HyperliquidConfig struct {
	// WSURL is the WebSocket endpoint for Hyperliquid
	WSURL string `yaml:"ws_url"`

	// ReconnectInterval is the base interval for reconnection attempts
	ReconnectInterval time.Duration `yaml:"reconnect_interval"`

	// MaxReconnectAttempts is the maximum reconnection attempts (0 = infinite)
	MaxReconnectAttempts int `yaml:"max_reconnect_attempts"`

	// TrackCoins is the list of coins to track trades for
	TrackCoins []string `yaml:"track_coins"`
}

// PostgresConfig holds database connection settings.
type PostgresConfig struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	Database        string        `yaml:"database"`
	User            string        `yaml:"user"`
	Password        string        `yaml:"password"`
	SSLMode         string        `yaml:"sslmode"`
	MaxConnections  int           `yaml:"max_connections"`
	IdleConnections int           `yaml:"idle_connections"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`

	// URL can be used as an alternative to individual fields
	URL string `yaml:"url"`
}

// ConnectionString returns a PostgreSQL connection string.
func (c *PostgresConfig) ConnectionString() string {
	if c.URL != "" {
		return c.URL
	}

	sslmode := c.SSLMode
	if sslmode == "" {
		sslmode = "disable"
	}

	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Database, sslmode,
	)
}

// KafkaConfig holds Kafka producer settings.
type KafkaConfig struct {
	Brokers      []string      `yaml:"brokers"`
	Topic        string        `yaml:"topic"`
	BatchSize    int           `yaml:"batch_size"`
	BatchTimeout time.Duration `yaml:"batch_timeout"`
	Compression  string        `yaml:"compression"`
}

// LoggingConfig holds logging settings.
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"` // "json" or "text"
}

// BatchConfig holds event batching settings.
type BatchConfig struct {
	Size    int           `yaml:"size"`
	Timeout time.Duration `yaml:"timeout"`
}

// DefaultConfig returns a configuration with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Hyperliquid: HyperliquidConfig{
			WSURL:                "wss://api.hyperliquid.xyz/ws",
			ReconnectInterval:    5 * time.Second,
			MaxReconnectAttempts: 0, // Infinite
			TrackCoins:           []string{"BTC", "ETH", "SOL", "HYPE"},
		},
		Postgres: PostgresConfig{
			Host:            "localhost",
			Port:            5432,
			Database:        "tracehound",
			User:            "hound",
			Password:        "",
			SSLMode:         "disable",
			MaxConnections:  25,
			IdleConnections: 5,
			ConnMaxLifetime: 5 * time.Minute,
		},
		Kafka: KafkaConfig{
			Brokers:      []string{"localhost:9092"},
			Topic:        "whale_scents",
			BatchSize:    100,
			BatchTimeout: 10 * time.Millisecond,
			Compression:  "snappy",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
		Batch: BatchConfig{
			Size:    100,
			Timeout: 1 * time.Second,
		},
	}
}

// LoadConfig loads configuration from a YAML file with environment variable substitution.
func LoadConfig(path string) (*Config, error) {
	// Start with defaults
	cfg := DefaultConfig()

	// Read config file
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// No config file, use defaults with env overrides
			applyEnvOverrides(cfg)
			return cfg, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Expand environment variables in the YAML content
	expandedData := expandEnvVars(string(data))

	// Parse YAML
	if err := yaml.Unmarshal([]byte(expandedData), cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply environment variable overrides
	applyEnvOverrides(cfg)

	return cfg, nil
}

// expandEnvVars replaces ${VAR_NAME} patterns with environment variable values.
func expandEnvVars(content string) string {
	// Pattern matches ${VAR_NAME} or ${VAR_NAME:-default}
	re := regexp.MustCompile(`\$\{([^}:]+)(?::-([^}]*))?\}`)

	return re.ReplaceAllStringFunc(content, func(match string) string {
		submatches := re.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}

		varName := submatches[1]
		defaultVal := ""
		if len(submatches) >= 3 {
			defaultVal = submatches[2]
		}

		if val := os.Getenv(varName); val != "" {
			return val
		}
		return defaultVal
	})
}

// applyEnvOverrides applies environment variable overrides to the configuration.
// Environment variables take precedence over file configuration.
func applyEnvOverrides(cfg *Config) {
	// Hyperliquid settings
	if val := os.Getenv("HYPERLIQUID_WS_URL"); val != "" {
		cfg.Hyperliquid.WSURL = val
	}
	if val := os.Getenv("TRACK_COINS"); val != "" {
		// Parse comma-separated list of coins
		coins := strings.Split(val, ",")
		cleanCoins := make([]string, 0, len(coins))
		for _, coin := range coins {
			coin = strings.TrimSpace(coin)
			if coin != "" {
				cleanCoins = append(cleanCoins, strings.ToUpper(coin))
			}
		}
		if len(cleanCoins) > 0 {
			cfg.Hyperliquid.TrackCoins = cleanCoins
		}
	}

	// PostgreSQL settings
	if val := os.Getenv("POSTGRES_URL"); val != "" {
		cfg.Postgres.URL = val
	}
	if val := os.Getenv("POSTGRES_HOST"); val != "" {
		cfg.Postgres.Host = val
	}
	if val := os.Getenv("POSTGRES_PORT"); val != "" {
		if port, err := strconv.Atoi(val); err == nil {
			cfg.Postgres.Port = port
		}
	}
	if val := os.Getenv("POSTGRES_DATABASE"); val != "" {
		cfg.Postgres.Database = val
	}
	if val := os.Getenv("POSTGRES_USER"); val != "" {
		cfg.Postgres.User = val
	}
	if val := os.Getenv("POSTGRES_PASSWORD"); val != "" {
		cfg.Postgres.Password = val
	}

	// Kafka settings
	if val := os.Getenv("KAFKA_BROKERS"); val != "" {
		cfg.Kafka.Brokers = strings.Split(val, ",")
	}
	if val := os.Getenv("KAFKA_BROKER"); val != "" {
		cfg.Kafka.Brokers = []string{val}
	}
	if val := os.Getenv("KAFKA_TOPIC"); val != "" {
		cfg.Kafka.Topic = val
	}

	// Logging settings
	if val := os.Getenv("LOG_LEVEL"); val != "" {
		cfg.Logging.Level = val
	}
	if val := os.Getenv("LOG_FORMAT"); val != "" {
		cfg.Logging.Format = val
	}
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	if c.Hyperliquid.WSURL == "" {
		return fmt.Errorf("hyperliquid.ws_url is required")
	}

	if c.Postgres.URL == "" {
		if c.Postgres.Host == "" {
			return fmt.Errorf("postgres.host is required")
		}
		if c.Postgres.Database == "" {
			return fmt.Errorf("postgres.database is required")
		}
		if c.Postgres.User == "" {
			return fmt.Errorf("postgres.user is required")
		}
	}

	if len(c.Kafka.Brokers) == 0 {
		return fmt.Errorf("kafka.brokers is required")
	}

	return nil
}
