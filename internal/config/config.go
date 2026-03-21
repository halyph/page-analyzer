package config

import (
	"time"

	"github.com/halyph/page-analyzer/internal/envutil"
)

// Load loads configuration from environment variables with defaults
func Load() Config {
	return Config{
		Server: ServerConfig{
			Addr:         envutil.EnvString("ANALYZER_ADDR", ":8080"),
			ReadTimeout:  envutil.EnvDuration("ANALYZER_READ_TIMEOUT", 30*time.Second),
			WriteTimeout: envutil.EnvDuration("ANALYZER_WRITE_TIMEOUT", 30*time.Second),
		},
		Fetching: FetchingConfig{
			Timeout:     envutil.EnvDuration("ANALYZER_FETCH_TIMEOUT", 15*time.Second),
			MaxBodySize: envutil.EnvInt64("ANALYZER_MAX_BODY_SIZE", 10*1024*1024), // 10MB
			UserAgent:   envutil.EnvString("ANALYZER_USER_AGENT", "PageAnalyzer/1.0"),
		},
		LinkChecking: LinkCheckingConfig{
			CheckMode:    envutil.EnvString("ANALYZER_CHECK_MODE", "async"),
			CheckTimeout: envutil.EnvDuration("ANALYZER_CHECK_TIMEOUT", 5*time.Second),
			Workers:      envutil.EnvInt("ANALYZER_CHECK_WORKERS", 20),
			QueueSize:    envutil.EnvInt("ANALYZER_QUEUE_SIZE", 100),
			MaxLinks:     envutil.EnvInt("ANALYZER_MAX_LINKS", 10000),
			SyncLimit:    envutil.EnvInt("ANALYZER_SYNC_LIMIT", 10),
		},
		Caching: CachingConfig{
			Mode:            envutil.EnvString("ANALYZER_CACHE_MODE", "memory"),
			TTL:             envutil.EnvDuration("ANALYZER_CACHE_TTL", 1*time.Hour),
			LinkCacheTTL:    envutil.EnvDuration("ANALYZER_LINK_CACHE_TTL", 5*time.Minute),
			RedisAddr:       envutil.EnvString("ANALYZER_REDIS_ADDR", "localhost:6379"),
			RedisPassword:   envutil.EnvString("ANALYZER_REDIS_PASSWORD", ""),
			MemoryCacheSize: envutil.EnvInt("ANALYZER_MEMORY_CACHE_SIZE", 100),
		},
		RateLimiting: RateLimitingConfig{
			Enabled: envutil.EnvBool("ANALYZER_RATE_LIMIT_ENABLED", true),
			RPS:     envutil.EnvInt("ANALYZER_RATE_LIMIT_RPS", 10),
			Burst:   envutil.EnvInt("ANALYZER_RATE_LIMIT_BURST", 20),
		},
		Observability: ObservabilityConfig{
			LogLevel:       envutil.EnvString("ANALYZER_LOG_LEVEL", "info"),
			LogFormat:      envutil.EnvString("ANALYZER_LOG_FORMAT", "json"),
			OTELEnabled:    envutil.EnvBool("ANALYZER_OTEL_ENABLED", false),
			OTELEndpoint:   envutil.EnvString("ANALYZER_OTEL_ENDPOINT", "localhost:4318"),
			MetricsEnabled: envutil.EnvBool("ANALYZER_METRICS_ENABLED", true),
		},
		Degradation: DegradationConfig{
			AllowStale:   envutil.EnvBool("ANALYZER_ALLOW_STALE", true),
			MaxStaleness: envutil.EnvDuration("ANALYZER_MAX_STALENESS", 24*time.Hour),
		},
	}
}

// Config holds all application configuration
type Config struct {
	Server        ServerConfig
	Fetching      FetchingConfig
	LinkChecking  LinkCheckingConfig
	Caching       CachingConfig
	RateLimiting  RateLimitingConfig
	Observability ObservabilityConfig
	Degradation   DegradationConfig
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Addr         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
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
