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

// Default TTL values for different cache entry types
const (
	DefaultHTMLCacheTTL     = 1 * time.Hour
	DefaultCachedLinkTTL    = 5 * time.Minute
	DefaultMultiBackfillTTL = 1 * time.Hour
)

// Cache defines the interface for caching analysis results
type Cache interface {
	// GetHTML retrieves cached HTML analysis result
	GetHTML(ctx context.Context, url string) (*domain.AnalysisResult, error)

	// SetHTML stores HTML analysis result in cache
	SetHTML(ctx context.Context, url string, result *domain.AnalysisResult, ttl time.Duration) error

	// GetCachedLink retrieves a cached individual link check result
	GetCachedLink(ctx context.Context, url string) (*domain.CachedLinkCheck, error)

	// SetCachedLink stores an individual link check result in cache
	SetCachedLink(ctx context.Context, url string, result *domain.CachedLinkCheck, ttl time.Duration) error

	// Close closes the cache and releases resources
	Close() error
}
