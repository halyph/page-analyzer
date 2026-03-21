package config

import "github.com/halyph/page-analyzer/internal/envutil"

// LoadFromEnv loads configuration from environment variables,
// falling back to defaults for any missing values.
// Panics if environment variable values are invalid (fail-fast approach).
func LoadFromEnv() (Config, error) {
	cfg := Defaults()

	// Server
	cfg.Server.Addr = envutil.EnvString("ANALYZER_ADDR", cfg.Server.Addr)
	cfg.Server.ReadTimeout = envutil.EnvDuration("ANALYZER_READ_TIMEOUT", cfg.Server.ReadTimeout)
	cfg.Server.WriteTimeout = envutil.EnvDuration("ANALYZER_WRITE_TIMEOUT", cfg.Server.WriteTimeout)

	// Fetching
	cfg.Fetching.Timeout = envutil.EnvDuration("ANALYZER_FETCH_TIMEOUT", cfg.Fetching.Timeout)
	cfg.Fetching.MaxBodySize = envutil.EnvInt64("ANALYZER_MAX_BODY_SIZE", cfg.Fetching.MaxBodySize)
	cfg.Fetching.UserAgent = envutil.EnvString("ANALYZER_USER_AGENT", cfg.Fetching.UserAgent)

	// Link Checking
	cfg.LinkChecking.CheckMode = envutil.EnvString("ANALYZER_CHECK_MODE", cfg.LinkChecking.CheckMode)
	cfg.LinkChecking.CheckTimeout = envutil.EnvDuration("ANALYZER_CHECK_TIMEOUT", cfg.LinkChecking.CheckTimeout)
	cfg.LinkChecking.Workers = envutil.EnvInt("ANALYZER_CHECK_WORKERS", cfg.LinkChecking.Workers)
	cfg.LinkChecking.QueueSize = envutil.EnvInt("ANALYZER_QUEUE_SIZE", cfg.LinkChecking.QueueSize)
	cfg.LinkChecking.MaxLinks = envutil.EnvInt("ANALYZER_MAX_LINKS", cfg.LinkChecking.MaxLinks)
	cfg.LinkChecking.SyncLimit = envutil.EnvInt("ANALYZER_SYNC_LIMIT", cfg.LinkChecking.SyncLimit)

	// Caching
	cfg.Caching.Mode = envutil.EnvString("ANALYZER_CACHE_MODE", cfg.Caching.Mode)
	cfg.Caching.TTL = envutil.EnvDuration("ANALYZER_CACHE_TTL", cfg.Caching.TTL)
	cfg.Caching.LinkCacheTTL = envutil.EnvDuration("ANALYZER_LINK_CACHE_TTL", cfg.Caching.LinkCacheTTL)
	cfg.Caching.RedisAddr = envutil.EnvString("ANALYZER_REDIS_ADDR", cfg.Caching.RedisAddr)
	cfg.Caching.RedisPassword = envutil.EnvString("ANALYZER_REDIS_PASSWORD", cfg.Caching.RedisPassword)
	cfg.Caching.MemoryCacheSize = envutil.EnvInt("ANALYZER_MEMORY_CACHE_SIZE", cfg.Caching.MemoryCacheSize)

	// Rate Limiting
	cfg.RateLimiting.Enabled = envutil.EnvBool("ANALYZER_RATE_LIMIT_ENABLED", cfg.RateLimiting.Enabled)
	cfg.RateLimiting.RPS = envutil.EnvInt("ANALYZER_RATE_LIMIT_RPS", cfg.RateLimiting.RPS)
	cfg.RateLimiting.Burst = envutil.EnvInt("ANALYZER_RATE_LIMIT_BURST", cfg.RateLimiting.Burst)

	// Observability
	cfg.Observability.LogLevel = envutil.EnvString("ANALYZER_LOG_LEVEL", cfg.Observability.LogLevel)
	cfg.Observability.LogFormat = envutil.EnvString("ANALYZER_LOG_FORMAT", cfg.Observability.LogFormat)
	cfg.Observability.OTELEnabled = envutil.EnvBool("ANALYZER_OTEL_ENABLED", cfg.Observability.OTELEnabled)
	cfg.Observability.OTELEndpoint = envutil.EnvString("ANALYZER_OTEL_ENDPOINT", cfg.Observability.OTELEndpoint)
	cfg.Observability.MetricsEnabled = envutil.EnvBool("ANALYZER_METRICS_ENABLED", cfg.Observability.MetricsEnabled)

	// Degradation
	cfg.Degradation.AllowStale = envutil.EnvBool("ANALYZER_ALLOW_STALE", cfg.Degradation.AllowStale)
	cfg.Degradation.MaxStaleness = envutil.EnvDuration("ANALYZER_MAX_STALENESS", cfg.Degradation.MaxStaleness)

	return cfg, nil
}
