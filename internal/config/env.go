package config

import (
	"os"
	"strconv"
	"time"
)

// LoadFromEnv loads configuration from environment variables,
// falling back to defaults for any missing values
func LoadFromEnv() (Config, error) {
	cfg := Defaults()

	// Server
	cfg.Server.Addr = getEnv("ANALYZER_ADDR", cfg.Server.Addr)
	if v := getEnvDuration("ANALYZER_READ_TIMEOUT"); v > 0 {
		cfg.Server.ReadTimeout = v
	}
	if v := getEnvDuration("ANALYZER_WRITE_TIMEOUT"); v > 0 {
		cfg.Server.WriteTimeout = v
	}

	// Fetching
	if v := getEnvDuration("ANALYZER_FETCH_TIMEOUT"); v > 0 {
		cfg.Fetching.Timeout = v
	}
	if v := getEnvInt64("ANALYZER_MAX_BODY_SIZE"); v > 0 {
		cfg.Fetching.MaxBodySize = v
	}
	cfg.Fetching.UserAgent = getEnv("ANALYZER_USER_AGENT", cfg.Fetching.UserAgent)

	// Link Checking
	cfg.LinkChecking.CheckMode = getEnv("ANALYZER_CHECK_MODE", cfg.LinkChecking.CheckMode)
	if v := getEnvDuration("ANALYZER_CHECK_TIMEOUT"); v > 0 {
		cfg.LinkChecking.CheckTimeout = v
	}
	if v := getEnvInt("ANALYZER_CHECK_WORKERS"); v > 0 {
		cfg.LinkChecking.Workers = v
	}
	if v := getEnvInt("ANALYZER_QUEUE_SIZE"); v > 0 {
		cfg.LinkChecking.QueueSize = v
	}
	if v := getEnvInt("ANALYZER_MAX_LINKS"); v > 0 {
		cfg.LinkChecking.MaxLinks = v
	}
	if v := getEnvInt("ANALYZER_SYNC_LIMIT"); v > 0 {
		cfg.LinkChecking.SyncLimit = v
	}

	// Caching
	cfg.Caching.Mode = getEnv("ANALYZER_CACHE_MODE", cfg.Caching.Mode)
	if v := getEnvDuration("ANALYZER_CACHE_TTL"); v > 0 {
		cfg.Caching.TTL = v
	}
	if v := getEnvDuration("ANALYZER_LINK_CACHE_TTL"); v > 0 {
		cfg.Caching.LinkCacheTTL = v
	}
	cfg.Caching.RedisAddr = getEnv("ANALYZER_REDIS_ADDR", cfg.Caching.RedisAddr)
	cfg.Caching.RedisPassword = getEnv("ANALYZER_REDIS_PASSWORD", cfg.Caching.RedisPassword)
	if v := getEnvInt("ANALYZER_MEMORY_CACHE_SIZE"); v > 0 {
		cfg.Caching.MemoryCacheSize = v
	}

	// Rate Limiting
	if v := getEnvBool("ANALYZER_RATE_LIMIT_ENABLED"); v != nil {
		cfg.RateLimiting.Enabled = *v
	}
	if v := getEnvInt("ANALYZER_RATE_LIMIT_RPS"); v > 0 {
		cfg.RateLimiting.RPS = v
	}
	if v := getEnvInt("ANALYZER_RATE_LIMIT_BURST"); v > 0 {
		cfg.RateLimiting.Burst = v
	}

	// Observability
	cfg.Observability.LogLevel = getEnv("ANALYZER_LOG_LEVEL", cfg.Observability.LogLevel)
	cfg.Observability.LogFormat = getEnv("ANALYZER_LOG_FORMAT", cfg.Observability.LogFormat)
	if v := getEnvBool("ANALYZER_OTEL_ENABLED"); v != nil {
		cfg.Observability.OTELEnabled = *v
	}
	cfg.Observability.OTELEndpoint = getEnv("ANALYZER_OTEL_ENDPOINT", cfg.Observability.OTELEndpoint)
	if v := getEnvBool("ANALYZER_METRICS_ENABLED"); v != nil {
		cfg.Observability.MetricsEnabled = *v
	}

	// Degradation
	if v := getEnvBool("ANALYZER_ALLOW_STALE"); v != nil {
		cfg.Degradation.AllowStale = *v
	}
	if v := getEnvDuration("ANALYZER_MAX_STALENESS"); v > 0 {
		cfg.Degradation.MaxStaleness = v
	}

	return cfg, nil
}

// Helper functions for environment variable parsing

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return 0
}

func getEnvInt64(key string) int64 {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.ParseInt(value, 10, 64); err == nil {
			return i
		}
	}
	return 0
}

func getEnvBool(key string) *bool {
	if value := os.Getenv(key); value != "" {
		if b, err := strconv.ParseBool(value); err == nil {
			return &b
		}
	}
	return nil
}

func getEnvDuration(key string) time.Duration {
	if value := os.Getenv(key); value != "" {
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
	}
	return 0
}
