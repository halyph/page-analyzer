package cache

import (
	"context"
	"time"

	"github.com/halyph/page-analyzer/internal/domain"
)

// NoOpCache is a cache implementation that does nothing (always misses)
type NoOpCache struct{}

// NewNoOpCache creates a new no-op cache
func NewNoOpCache() *NoOpCache {
	return &NoOpCache{}
}

// GetHTML always returns cache miss
func (nc *NoOpCache) GetHTML(ctx context.Context, url string) (*domain.AnalysisResult, error) {
	return nil, ErrCacheMiss
}

// SetHTML does nothing
func (nc *NoOpCache) SetHTML(ctx context.Context, url string, result *domain.AnalysisResult, ttl time.Duration) error {
	return nil
}

// GetCachedLink always returns cache miss
func (nc *NoOpCache) GetCachedLink(ctx context.Context, url string) (*domain.CachedLinkCheck, error) {
	return nil, ErrCacheMiss
}

// SetCachedLink does nothing
func (nc *NoOpCache) SetCachedLink(ctx context.Context, url string, result *domain.CachedLinkCheck, ttl time.Duration) error {
	return nil
}

// Close does nothing
func (nc *NoOpCache) Close() error {
	return nil
}
