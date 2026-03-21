package cache

import (
	"context"
	"errors"
	"time"

	"github.com/halyph/page-analyzer/internal/domain"
)

var (
	// ErrCacheMiss is returned when a key is not found in the cache
	ErrCacheMiss = errors.New("cache miss")

	// ErrCacheUnavailable is returned when the cache backend is unavailable
	ErrCacheUnavailable = errors.New("cache unavailable")
)

// Cache defines the interface for caching analysis results
type Cache interface {
	// GetHTML retrieves cached HTML analysis result
	GetHTML(ctx context.Context, url string) (*domain.AnalysisResult, error)

	// SetHTML stores HTML analysis result in cache
	SetHTML(ctx context.Context, url string, result *domain.AnalysisResult, ttl time.Duration) error

	// GetLinkCheck retrieves cached link check result
	GetLinkCheck(ctx context.Context, jobID string) (*domain.LinkCheckResult, error)

	// SetLinkCheck stores link check result in cache
	SetLinkCheck(ctx context.Context, jobID string, result *domain.LinkCheckResult, ttl time.Duration) error

	// Delete removes a cached entry
	Delete(ctx context.Context, url string) error

	// Clear removes all cached entries
	Clear(ctx context.Context) error

	// Stats returns cache statistics
	Stats() CacheStats
}

// CacheStats provides cache performance metrics
type CacheStats struct {
	Hits        int64
	Misses      int64
	Entries     int64
	Evictions   int64
	HitRate     float64
	AvgItemSize int64
}

// CacheConfig configures cache behavior
type CacheConfig struct {
	Type     CacheType     // "memory", "redis", "multi", "disabled"
	MaxSize  int           // Max entries for memory cache
	TTL      time.Duration // Default TTL
	RedisURL string        // Redis connection URL
}

// CacheType defines the type of cache implementation
type CacheType string

const (
	CacheTypeMemory   CacheType = "memory"
	CacheTypeRedis    CacheType = "redis"
	CacheTypeMulti    CacheType = "multi"
	CacheTypeDisabled CacheType = "disabled"
)

// DefaultCacheConfig returns sensible defaults
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		Type:    CacheTypeMemory,
		MaxSize: 100,
		TTL:     1 * time.Hour,
	}
}
