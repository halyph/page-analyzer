package config

import "time"

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
