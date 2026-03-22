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
		},
		Processing: ProcessingConfig{
			MaxTokens:  envInt("ANALYZER_MAX_TOKENS", 1_000_000), // 1M tokens
			Collectors: envStringSlice("ANALYZER_COLLECTORS", DefaultCollectors),
		},
		LinkChecking: LinkCheckingConfig{
			CheckMode:    envString("ANALYZER_CHECK_MODE", LinkCheckModeAsync),
			CheckTimeout: envDuration("ANALYZER_CHECK_TIMEOUT", 5*time.Second),
			Workers:      envInt("ANALYZER_CHECK_WORKERS", 20),
			QueueSize:    envInt("ANALYZER_QUEUE_SIZE", 100),
			MaxLinks:     envInt("ANALYZER_MAX_LINKS", 10000),
			JobWorkers:   envInt("ANALYZER_JOB_WORKERS", 10),
			JobMaxAge:    envDuration("ANALYZER_JOB_MAX_AGE", 10*time.Minute),
		},
		Caching: CachingConfig{
			Mode:            envString("ANALYZER_CACHE_MODE", CacheModeMemory),
			PageCacheTTL:    envDuration("ANALYZER_PAGE_CACHE_TTL", 1*time.Hour),
			LinkCacheTTL:    envDuration("ANALYZER_LINK_CACHE_TTL", 5*time.Minute),
			RedisAddr:       envString("ANALYZER_REDIS_ADDR", "redis://localhost:6379/0"),
			MemoryCacheSize: envInt("ANALYZER_MEMORY_CACHE_SIZE", 100),
		},
		Observability: ObservabilityConfig{
			LogLevel:       envString("ANALYZER_LOG_LEVEL", LogLevelInfo),
			LogFormat:      envString("ANALYZER_LOG_FORMAT", LogFormatJSON),
			TracingEnabled: envBool("ANALYZER_TRACING_ENABLED", false),
			OTELEndpoint:   envString("ANALYZER_OTEL_ENDPOINT", "localhost:4318"),
			MetricsEnabled: envBool("ANALYZER_METRICS_ENABLED", true),
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
	Observability ObservabilityConfig
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
	MaxTokens  int      // Maximum tokens to process
	Collectors []string // List of collectors to run (default: htmlversion, title, headings, loginform, links)
}

// FetchingConfig holds configuration for fetching target URLs
type FetchingConfig struct {
	Timeout     time.Duration
	MaxBodySize int64
}

// LinkCheckingConfig holds configuration for link checking
type LinkCheckingConfig struct {
	CheckMode    string        // sync, async, disabled
	CheckTimeout time.Duration // HTTP request timeout for individual link checks
	Workers      int           // Number of concurrent worker goroutines
	QueueSize    int           // Job queue buffer size
	MaxLinks     int           // Maximum links to check per request
	JobWorkers   int           // Concurrent checks within a single job
	JobMaxAge    time.Duration // How long to keep completed jobs in memory
}

// CachingConfig holds configuration for caching
type CachingConfig struct {
	Mode            string        // memory, redis, multi, disabled
	PageCacheTTL    time.Duration // TTL for HTML page analysis results
	LinkCacheTTL    time.Duration // TTL for individual link check results
	RedisAddr       string        // Redis URL (supports auth: redis://:password@host:port/db)
	MemoryCacheSize int
}

// ObservabilityConfig holds configuration for logging, metrics, and tracing
type ObservabilityConfig struct {
	LogLevel       string // debug, info, warn, error
	LogFormat      string // json, text
	TracingEnabled bool   // Enable OpenTelemetry tracing
	OTELEndpoint   string // OTLP endpoint for traces and metrics
	MetricsEnabled bool   // Enable OpenTelemetry metrics
}
