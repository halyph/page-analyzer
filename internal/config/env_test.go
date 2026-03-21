package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLoadFromEnv_Defaults(t *testing.T) {
	// Clear all env vars
	clearEnv(t)

	cfg, err := LoadFromEnv()
	assert.NoError(t, err)

	// Verify defaults
	assert.Equal(t, ":8080", cfg.Server.Addr)
	assert.Equal(t, 30*time.Second, cfg.Server.ReadTimeout)
	assert.Equal(t, "async", cfg.LinkChecking.CheckMode)
	assert.Equal(t, 20, cfg.LinkChecking.Workers)
	assert.Equal(t, "memory", cfg.Caching.Mode)
	assert.Equal(t, "info", cfg.Observability.LogLevel)
	assert.True(t, cfg.RateLimiting.Enabled)
}

func TestLoadFromEnv_ServerConfig(t *testing.T) {
	clearEnv(t)
	os.Setenv("ANALYZER_ADDR", ":9090")
	os.Setenv("ANALYZER_READ_TIMEOUT", "60s")
	os.Setenv("ANALYZER_WRITE_TIMEOUT", "60s")
	defer clearEnv(t)

	cfg, err := LoadFromEnv()
	assert.NoError(t, err)

	assert.Equal(t, ":9090", cfg.Server.Addr)
	assert.Equal(t, 60*time.Second, cfg.Server.ReadTimeout)
	assert.Equal(t, 60*time.Second, cfg.Server.WriteTimeout)
}

func TestLoadFromEnv_FetchingConfig(t *testing.T) {
	clearEnv(t)
	os.Setenv("ANALYZER_FETCH_TIMEOUT", "30s")
	os.Setenv("ANALYZER_MAX_BODY_SIZE", "20971520") // 20MB
	os.Setenv("ANALYZER_USER_AGENT", "CustomBot/2.0")
	defer clearEnv(t)

	cfg, err := LoadFromEnv()
	assert.NoError(t, err)

	assert.Equal(t, 30*time.Second, cfg.Fetching.Timeout)
	assert.Equal(t, int64(20971520), cfg.Fetching.MaxBodySize)
	assert.Equal(t, "CustomBot/2.0", cfg.Fetching.UserAgent)
}

func TestLoadFromEnv_LinkCheckingConfig(t *testing.T) {
	clearEnv(t)
	os.Setenv("ANALYZER_CHECK_MODE", "sync")
	os.Setenv("ANALYZER_CHECK_TIMEOUT", "10s")
	os.Setenv("ANALYZER_CHECK_WORKERS", "50")
	os.Setenv("ANALYZER_QUEUE_SIZE", "200")
	os.Setenv("ANALYZER_MAX_LINKS", "5000")
	os.Setenv("ANALYZER_SYNC_LIMIT", "20")
	defer clearEnv(t)

	cfg, err := LoadFromEnv()
	assert.NoError(t, err)

	assert.Equal(t, "sync", cfg.LinkChecking.CheckMode)
	assert.Equal(t, 10*time.Second, cfg.LinkChecking.CheckTimeout)
	assert.Equal(t, 50, cfg.LinkChecking.Workers)
	assert.Equal(t, 200, cfg.LinkChecking.QueueSize)
	assert.Equal(t, 5000, cfg.LinkChecking.MaxLinks)
	assert.Equal(t, 20, cfg.LinkChecking.SyncLimit)
}

func TestLoadFromEnv_CachingConfig(t *testing.T) {
	clearEnv(t)
	os.Setenv("ANALYZER_CACHE_MODE", "redis")
	os.Setenv("ANALYZER_CACHE_TTL", "2h")
	os.Setenv("ANALYZER_LINK_CACHE_TTL", "10m")
	os.Setenv("ANALYZER_REDIS_ADDR", "redis:6379")
	os.Setenv("ANALYZER_REDIS_PASSWORD", "secret")
	os.Setenv("ANALYZER_MEMORY_CACHE_SIZE", "500")
	defer clearEnv(t)

	cfg, err := LoadFromEnv()
	assert.NoError(t, err)

	assert.Equal(t, "redis", cfg.Caching.Mode)
	assert.Equal(t, 2*time.Hour, cfg.Caching.TTL)
	assert.Equal(t, 10*time.Minute, cfg.Caching.LinkCacheTTL)
	assert.Equal(t, "redis:6379", cfg.Caching.RedisAddr)
	assert.Equal(t, "secret", cfg.Caching.RedisPassword)
	assert.Equal(t, 500, cfg.Caching.MemoryCacheSize)
}

func TestLoadFromEnv_RateLimitingConfig(t *testing.T) {
	clearEnv(t)
	os.Setenv("ANALYZER_RATE_LIMIT_ENABLED", "false")
	os.Setenv("ANALYZER_RATE_LIMIT_RPS", "100")
	os.Setenv("ANALYZER_RATE_LIMIT_BURST", "200")
	defer clearEnv(t)

	cfg, err := LoadFromEnv()
	assert.NoError(t, err)

	assert.False(t, cfg.RateLimiting.Enabled)
	assert.Equal(t, 100, cfg.RateLimiting.RPS)
	assert.Equal(t, 200, cfg.RateLimiting.Burst)
}

func TestLoadFromEnv_ObservabilityConfig(t *testing.T) {
	clearEnv(t)
	os.Setenv("ANALYZER_LOG_LEVEL", "debug")
	os.Setenv("ANALYZER_LOG_FORMAT", "text")
	os.Setenv("ANALYZER_OTEL_ENABLED", "true")
	os.Setenv("ANALYZER_OTEL_ENDPOINT", "tempo:4318")
	os.Setenv("ANALYZER_METRICS_ENABLED", "false")
	defer clearEnv(t)

	cfg, err := LoadFromEnv()
	assert.NoError(t, err)

	assert.Equal(t, "debug", cfg.Observability.LogLevel)
	assert.Equal(t, "text", cfg.Observability.LogFormat)
	assert.True(t, cfg.Observability.OTELEnabled)
	assert.Equal(t, "tempo:4318", cfg.Observability.OTELEndpoint)
	assert.False(t, cfg.Observability.MetricsEnabled)
}

func TestLoadFromEnv_DegradationConfig(t *testing.T) {
	clearEnv(t)
	os.Setenv("ANALYZER_ALLOW_STALE", "false")
	os.Setenv("ANALYZER_MAX_STALENESS", "48h")
	defer clearEnv(t)

	cfg, err := LoadFromEnv()
	assert.NoError(t, err)

	assert.False(t, cfg.Degradation.AllowStale)
	assert.Equal(t, 48*time.Hour, cfg.Degradation.MaxStaleness)
}

func TestLoadFromEnv_PartialOverride(t *testing.T) {
	clearEnv(t)
	// Only set some variables, others should use defaults
	os.Setenv("ANALYZER_ADDR", ":9000")
	os.Setenv("ANALYZER_CACHE_MODE", "disabled")
	defer clearEnv(t)

	cfg, err := LoadFromEnv()
	assert.NoError(t, err)

	// Overridden values
	assert.Equal(t, ":9000", cfg.Server.Addr)
	assert.Equal(t, "disabled", cfg.Caching.Mode)

	// Default values
	assert.Equal(t, 30*time.Second, cfg.Server.ReadTimeout)
	assert.Equal(t, 15*time.Second, cfg.Fetching.Timeout)
	assert.Equal(t, "info", cfg.Observability.LogLevel)
}

func TestLoadFromEnv_InvalidValues(t *testing.T) {
	clearEnv(t)
	defer clearEnv(t)

	// Invalid integer should panic (fail-fast)
	t.Run("invalid_int", func(t *testing.T) {
		os.Setenv("ANALYZER_CHECK_WORKERS", "invalid")
		defer os.Unsetenv("ANALYZER_CHECK_WORKERS")
		assert.Panics(t, func() {
			_, _ = LoadFromEnv()
		})
	})

	// Invalid int64 should panic
	t.Run("invalid_int64", func(t *testing.T) {
		os.Setenv("ANALYZER_MAX_BODY_SIZE", "not-a-number")
		defer os.Unsetenv("ANALYZER_MAX_BODY_SIZE")
		assert.Panics(t, func() {
			_, _ = LoadFromEnv()
		})
	})

	// Invalid boolean should panic
	t.Run("invalid_bool", func(t *testing.T) {
		os.Setenv("ANALYZER_RATE_LIMIT_ENABLED", "maybe")
		defer os.Unsetenv("ANALYZER_RATE_LIMIT_ENABLED")
		assert.Panics(t, func() {
			_, _ = LoadFromEnv()
		})
	})

	// Invalid duration should panic
	t.Run("invalid_duration", func(t *testing.T) {
		os.Setenv("ANALYZER_CACHE_TTL", "forever")
		defer os.Unsetenv("ANALYZER_CACHE_TTL")
		assert.Panics(t, func() {
			_, _ = LoadFromEnv()
		})
	})
}

// Helper to clear all ANALYZER_* environment variables
func clearEnv(t *testing.T) {
	t.Helper()
	envVars := []string{
		"ANALYZER_ADDR",
		"ANALYZER_READ_TIMEOUT",
		"ANALYZER_WRITE_TIMEOUT",
		"ANALYZER_FETCH_TIMEOUT",
		"ANALYZER_MAX_BODY_SIZE",
		"ANALYZER_USER_AGENT",
		"ANALYZER_CHECK_MODE",
		"ANALYZER_CHECK_TIMEOUT",
		"ANALYZER_CHECK_WORKERS",
		"ANALYZER_QUEUE_SIZE",
		"ANALYZER_MAX_LINKS",
		"ANALYZER_SYNC_LIMIT",
		"ANALYZER_CACHE_MODE",
		"ANALYZER_CACHE_TTL",
		"ANALYZER_LINK_CACHE_TTL",
		"ANALYZER_REDIS_ADDR",
		"ANALYZER_REDIS_PASSWORD",
		"ANALYZER_MEMORY_CACHE_SIZE",
		"ANALYZER_RATE_LIMIT_ENABLED",
		"ANALYZER_RATE_LIMIT_RPS",
		"ANALYZER_RATE_LIMIT_BURST",
		"ANALYZER_LOG_LEVEL",
		"ANALYZER_LOG_FORMAT",
		"ANALYZER_OTEL_ENABLED",
		"ANALYZER_OTEL_ENDPOINT",
		"ANALYZER_METRICS_ENABLED",
		"ANALYZER_ALLOW_STALE",
		"ANALYZER_MAX_STALENESS",
	}

	for _, key := range envVars {
		os.Unsetenv(key)
	}
}
