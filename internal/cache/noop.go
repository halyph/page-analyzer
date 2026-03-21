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

// GetLinkCheck always returns cache miss
func (nc *NoOpCache) GetLinkCheck(ctx context.Context, jobID string) (*domain.LinkCheckResult, error) {
	return nil, ErrCacheMiss
}

// SetLinkCheck does nothing
func (nc *NoOpCache) SetLinkCheck(ctx context.Context, jobID string, result *domain.LinkCheckResult, ttl time.Duration) error {
	return nil
}

// Delete does nothing
func (nc *NoOpCache) Delete(ctx context.Context, url string) error {
	return nil
}

// Clear does nothing
func (nc *NoOpCache) Clear(ctx context.Context) error {
	return nil
}

// Stats returns zero stats
func (nc *NoOpCache) Stats() CacheStats {
	return CacheStats{}
}
