package config

import "time"

// Defaults returns a Config with sensible default values
func Defaults() Config {
	return Config{
		Server: ServerConfig{
			Addr:         ":8080",
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
		},
		Fetching: FetchingConfig{
			Timeout:     15 * time.Second,
			MaxBodySize: 10 * 1024 * 1024, // 10MB
			UserAgent:   "PageAnalyzer/1.0",
		},
		LinkChecking: LinkCheckingConfig{
			CheckMode:    "async",
			CheckTimeout: 5 * time.Second,
			Workers:      20,
			QueueSize:    100,
			MaxLinks:     10000,
			SyncLimit:    10,
		},
		Caching: CachingConfig{
			Mode:           "memory",
			TTL:            1 * time.Hour,
			LinkCacheTTL:   5 * time.Minute,
			RedisAddr:      "localhost:6379",
			RedisPassword:  "",
			MemoryCacheSize: 100,
		},
		RateLimiting: RateLimitingConfig{
			Enabled: true,
			RPS:     10,
			Burst:   20,
		},
		Observability: ObservabilityConfig{
			LogLevel:       "info",
			LogFormat:      "json",
			OTELEnabled:    false,
			OTELEndpoint:   "localhost:4318",
			MetricsEnabled: true,
		},
		Degradation: DegradationConfig{
			AllowStale:   true,
			MaxStaleness: 24 * time.Hour,
		},
	}
}
