package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLoad_Defaults(t *testing.T) {
	// Clear any existing environment variables
	os.Clearenv()

	cfg := Load()

	// Server defaults
	assert.Equal(t, ":8080", cfg.Server.Addr)
	assert.Equal(t, 30*time.Second, cfg.Server.ReadTimeout)
	assert.Equal(t, 30*time.Second, cfg.Server.WriteTimeout)
	assert.Equal(t, 60*time.Second, cfg.Server.IdleTimeout)
	assert.Equal(t, 10*time.Second, cfg.Server.ShutdownTimeout)

	// Fetching defaults
	assert.Equal(t, 15*time.Second, cfg.Fetching.Timeout)
	assert.Equal(t, int64(10*1024*1024), cfg.Fetching.MaxBodySize)
	assert.Equal(t, "PageAnalyzer/1.0", cfg.Fetching.UserAgent)

	// Processing defaults
	assert.Equal(t, 1_000_000, cfg.Processing.MaxTokens)

	// Link checking defaults
	assert.Equal(t, "async", cfg.LinkChecking.CheckMode)
	assert.Equal(t, 5*time.Second, cfg.LinkChecking.CheckTimeout)
	assert.Equal(t, 20, cfg.LinkChecking.Workers)
	assert.Equal(t, 100, cfg.LinkChecking.QueueSize)
	assert.Equal(t, 10, cfg.LinkChecking.JobWorkers)

	// Caching defaults
	assert.Equal(t, "memory", cfg.Caching.Mode)
	assert.Equal(t, 1*time.Hour, cfg.Caching.TTL)
	assert.Equal(t, 5*time.Minute, cfg.Caching.LinkCacheTTL)

	// Rate limiting defaults
	assert.True(t, cfg.RateLimiting.Enabled)
	assert.Equal(t, 10, cfg.RateLimiting.RPS)

	// Observability defaults
	assert.Equal(t, "info", cfg.Observability.LogLevel)
	assert.Equal(t, "json", cfg.Observability.LogFormat)
	assert.False(t, cfg.Observability.OTELEnabled)

	// Degradation defaults
	assert.True(t, cfg.Degradation.AllowStale)
	assert.Equal(t, 24*time.Hour, cfg.Degradation.MaxStaleness)
}

func TestLoad_FromEnv(t *testing.T) {
	// Set environment variables
	os.Setenv("ANALYZER_ADDR", ":9090")
	os.Setenv("ANALYZER_FETCH_TIMEOUT", "30s")
	os.Setenv("ANALYZER_CHECK_WORKERS", "50")
	os.Setenv("ANALYZER_CACHE_MODE", "redis")
	os.Setenv("ANALYZER_LOG_LEVEL", "debug")
	defer func() {
		os.Unsetenv("ANALYZER_ADDR")
		os.Unsetenv("ANALYZER_FETCH_TIMEOUT")
		os.Unsetenv("ANALYZER_CHECK_WORKERS")
		os.Unsetenv("ANALYZER_CACHE_MODE")
		os.Unsetenv("ANALYZER_LOG_LEVEL")
	}()

	cfg := Load()

	assert.Equal(t, ":9090", cfg.Server.Addr)
	assert.Equal(t, 30*time.Second, cfg.Fetching.Timeout)
	assert.Equal(t, 50, cfg.LinkChecking.Workers)
	assert.Equal(t, "redis", cfg.Caching.Mode)
	assert.Equal(t, "debug", cfg.Observability.LogLevel)
}

func TestLoad_InvalidValues_Panic(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{"invalid_int", "ANALYZER_CHECK_WORKERS", "invalid"},
		{"invalid_int64", "ANALYZER_MAX_BODY_SIZE", "not-a-number"},
		{"invalid_bool", "ANALYZER_RATE_LIMIT_ENABLED", "maybe"},
		{"invalid_duration", "ANALYZER_CACHE_TTL", "forever"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv(tt.key, tt.value)
			defer os.Unsetenv(tt.key)

			assert.Panics(t, func() {
				Load()
			})
		})
	}
}

func TestLoad_BooleanParsing(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected bool
	}{
		{"true_lowercase", "true", true},
		{"true_uppercase", "TRUE", true},
		{"true_titlecase", "True", true},
		{"false_lowercase", "false", false},
		{"one", "1", true},
		{"zero", "0", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("ANALYZER_OTEL_ENABLED", tt.value)
			defer os.Unsetenv("ANALYZER_OTEL_ENABLED")

			cfg := Load()
			assert.Equal(t, tt.expected, cfg.Observability.OTELEnabled)
		})
	}
}

func TestLoad_DurationParsing(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected time.Duration
	}{
		{"seconds", "30s", 30 * time.Second},
		{"minutes", "5m", 5 * time.Minute},
		{"hours", "2h", 2 * time.Hour},
		{"mixed", "1h30m", 90 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("ANALYZER_CACHE_TTL", tt.value)
			defer os.Unsetenv("ANALYZER_CACHE_TTL")

			cfg := Load()
			assert.Equal(t, tt.expected, cfg.Caching.TTL)
		})
	}
}
