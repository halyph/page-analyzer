package config

import (
	"time"
)

// Load loads configuration from environment variables with defaults
func Load() Config {
	return Config{
		Server: ServerConfig{
			Addr:            envString("ANALYZER_ADDR", ":8080"),
			ReadTimeout:     envDuration("ANALYZER_READ_TIMEOUT", 30*time.Second),
			WriteTimeout:    envDuration("ANALYZER_WRITE_TIMEOUT", 30*time.Second),
			IdleTimeout:     envDuration("ANALYZER_IDLE_TIMEOUT", 60*time.Second),
			ShutdownTimeout: envDuration("ANALYZER_SHUTDOWN_TIMEOUT", 10*time.Second),
		},
		Fetching: FetchingConfig{
			Timeout:     envDuration("ANALYZER_FETCH_TIMEOUT", 15*time.Second),
			MaxBodySize: envInt64("ANALYZER_MAX_BODY_SIZE", 10*1024*1024), // 10MB
			UserAgent:   envString("ANALYZER_USER_AGENT", "PageAnalyzer/1.0"),
		},
		Processing: ProcessingConfig{
			MaxTokens: envInt("ANALYZER_MAX_TOKENS", 1_000_000), // 1M tokens
		},
		LinkChecking: LinkCheckingConfig{
			CheckMode:    envString("ANALYZER_CHECK_MODE", "async"),
			CheckTimeout: envDuration("ANALYZER_CHECK_TIMEOUT", 5*time.Second),
			Workers:      envInt("ANALYZER_CHECK_WORKERS", 20),
			QueueSize:    envInt("ANALYZER_QUEUE_SIZE", 100),
			MaxLinks:     envInt("ANALYZER_MAX_LINKS", 10000),
			SyncLimit:    envInt("ANALYZER_SYNC_LIMIT", 10),
			JobWorkers:   envInt("ANALYZER_JOB_WORKERS", 10),
		},
		Caching: CachingConfig{
			Mode:            envString("ANALYZER_CACHE_MODE", "memory"),
			TTL:             envDuration("ANALYZER_CACHE_TTL", 1*time.Hour),
			LinkCacheTTL:    envDuration("ANALYZER_LINK_CACHE_TTL", 5*time.Minute),
			RedisAddr:       envString("ANALYZER_REDIS_ADDR", "redis://localhost:6379/0"),
			RedisPassword:   envString("ANALYZER_REDIS_PASSWORD", ""),
			MemoryCacheSize: envInt("ANALYZER_MEMORY_CACHE_SIZE", 100),
		},
		RateLimiting: RateLimitingConfig{
			Enabled: envBool("ANALYZER_RATE_LIMIT_ENABLED", true),
			RPS:     envInt("ANALYZER_RATE_LIMIT_RPS", 10),
			Burst:   envInt("ANALYZER_RATE_LIMIT_BURST", 20),
		},
		Observability: ObservabilityConfig{
			LogLevel:       envString("ANALYZER_LOG_LEVEL", "info"),
			LogFormat:      envString("ANALYZER_LOG_FORMAT", "json"),
			OTELEnabled:    envBool("ANALYZER_OTEL_ENABLED", false),
			OTELEndpoint:   envString("ANALYZER_OTEL_ENDPOINT", "localhost:4318"),
			MetricsEnabled: envBool("ANALYZER_METRICS_ENABLED", true),
		},
		Degradation: DegradationConfig{
			AllowStale:   envBool("ANALYZER_ALLOW_STALE", true),
			MaxStaleness: envDuration("ANALYZER_MAX_STALENESS", 24*time.Hour),
		},
	}
}

// Config holds all application configuration
type Config struct {
	Server        ServerConfig
	Fetching      FetchingConfig
	Processing    ProcessingConfig
	LinkChecking  LinkCheckingConfig
	Caching       CachingConfig
	RateLimiting  RateLimitingConfig
	Observability ObservabilityConfig
	Degradation   DegradationConfig
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Addr            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

// ProcessingConfig holds configuration for HTML processing
type ProcessingConfig struct {
	MaxTokens int // Maximum tokens to process
}

// FetchingConfig holds configuration for fetching target URLs
type FetchingConfig struct {
	Timeout     time.Duration
	MaxBodySize int64
	UserAgent   string
}

// LinkCheckingConfig holds configuration for link checking
type LinkCheckingConfig struct {
	CheckMode    string // sync, async, hybrid, disabled
	CheckTimeout time.Duration
	Workers      int
	QueueSize    int
	MaxLinks     int
	SyncLimit    int // For hybrid mode: check first N synchronously
	JobWorkers   int // Concurrent checks within a single job
}

// CachingConfig holds configuration for caching
type CachingConfig struct {
	Mode            string // memory, redis, multi, disabled
	TTL             time.Duration
	LinkCacheTTL    time.Duration
	RedisAddr       string
	RedisPassword   string
	MemoryCacheSize int
}

// RateLimitingConfig holds configuration for rate limiting
type RateLimitingConfig struct {
	Enabled bool
	RPS     int
	Burst   int
}

// ObservabilityConfig holds configuration for logging, metrics, and tracing
type ObservabilityConfig struct {
	LogLevel       string // debug, info, warn, error
	LogFormat      string // json, text
	OTELEnabled    bool
	OTELEndpoint   string
	MetricsEnabled bool
}

// DegradationConfig holds configuration for graceful degradation
type DegradationConfig struct {
	AllowStale   bool
	MaxStaleness time.Duration
}
