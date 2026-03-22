package cache

import (
	"context"
	"time"

	"github.com/halyph/page-analyzer/internal/domain"
)

// MultiCache implements a multi-tier cache (L1=Memory, L2=Redis)
// L1 provides fast local access, L2 provides shared distributed cache
type MultiCache struct {
	l1          Cache         // Memory cache (fast, local)
	l2          Cache         // Redis cache (slower, shared)
	backfillTTL time.Duration // TTL for L1 backfill from L2
}

// NewMultiCache creates a new multi-tier cache with default backfill TTL
func NewMultiCache(l1, l2 Cache) *MultiCache {
	return &MultiCache{
		l1:          l1,
		l2:          l2,
		backfillTTL: DefaultMultiBackfillTTL,
	}
}

// NewMultiCacheWithTTL creates a new multi-tier cache with custom backfill TTL
func NewMultiCacheWithTTL(l1, l2 Cache, backfillTTL time.Duration) *MultiCache {
	return &MultiCache{
		l1:          l1,
		l2:          l2,
		backfillTTL: backfillTTL,
	}
}

// GetHTML retrieves cached HTML analysis result
// Strategy: Check L1 first, then L2, backfill L1 on L2 hit
//
//nolint:dupl // Similar to GetLinkCheck but different types
func (mc *MultiCache) GetHTML(ctx context.Context, url string) (*domain.AnalysisResult, error) {
	// Try L1 (memory) first
	if result, err := mc.l1.GetHTML(ctx, url); err == nil {
		return result, nil
	}

	// Try L2 (Redis)
	result, err := mc.l2.GetHTML(ctx, url)
	if err != nil {
		return nil, err
	}

	// Backfill L1 cache (fire-and-forget, don't wait)
	go func() {
		// Use background context to avoid cancellation
		bgCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_ = mc.l1.SetHTML(bgCtx, url, result, mc.backfillTTL)
	}()

	return result, nil
}

// SetHTML stores HTML analysis result in cache
// Strategy: Write to both L1 and L2 in parallel
func (mc *MultiCache) SetHTML(ctx context.Context, url string, result *domain.AnalysisResult, ttl time.Duration) error {
	// Write to L1 (best effort, ignore errors)
	_ = mc.l1.SetHTML(ctx, url, result, ttl)

	// Write to L2 (this is the source of truth)
	return mc.l2.SetHTML(ctx, url, result, ttl)
}

// GetCachedLink retrieves a cached individual link check result
// Strategy: Check L1 first, then L2, backfill L1 on L2 hit
//
//nolint:dupl // Similar to GetHTML but different types
func (mc *MultiCache) GetCachedLink(ctx context.Context, url string) (*domain.CachedLinkCheck, error) {
	// Try L1 (memory) first
	if result, err := mc.l1.GetCachedLink(ctx, url); err == nil {
		return result, nil
	}

	// Try L2 (Redis)
	result, err := mc.l2.GetCachedLink(ctx, url)
	if err != nil {
		return nil, err
	}

	// Backfill L1 cache (fire-and-forget)
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_ = mc.l1.SetCachedLink(bgCtx, url, result, mc.backfillTTL)
	}()

	return result, nil
}

// SetCachedLink stores an individual link check result in cache
// Strategy: Write to both L1 and L2 in parallel
func (mc *MultiCache) SetCachedLink(ctx context.Context, url string, result *domain.CachedLinkCheck, ttl time.Duration) error {
	// Write to L1 (best effort, ignore errors)
	_ = mc.l1.SetCachedLink(ctx, url, result, ttl)

	// Write to L2 (this is the source of truth)
	return mc.l2.SetCachedLink(ctx, url, result, ttl)
}

// Close closes both cache tiers
func (mc *MultiCache) Close() error {
	// Close L1 (ignore errors, best effort)
	_ = mc.l1.Close()
	// Close L2 (return its error)
	return mc.l2.Close()
}
