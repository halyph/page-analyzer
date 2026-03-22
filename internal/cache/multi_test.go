package cache

import (
	"context"
	"testing"
	"time"

	"github.com/halyph/page-analyzer/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMultiCache(t *testing.T) {
	l1 := NewMemoryCache(100)
	l2 := NewMemoryCache(1000)

	cache := NewMultiCache(l1, l2)
	assert.NotNil(t, cache)
}

func TestMultiCache_HTMLAnalysis_L1Hit(t *testing.T) {
	l1 := NewMemoryCache(100)
	l2 := NewMemoryCache(1000)
	cache := NewMultiCache(l1, l2)

	ctx := context.Background()
	url := "https://example.com"

	result := &domain.AnalysisResult{
		URL:   url,
		Title: "L1 Cache Test",
	}

	// Store in L1 only
	err := l1.SetHTML(ctx, url, result, 1*time.Hour)
	require.NoError(t, err)

	// Should retrieve from L1
	cached, err := cache.GetHTML(ctx, url)
	assert.NoError(t, err)
	assert.Equal(t, result.Title, cached.Title)
}

func TestMultiCache_HTMLAnalysis_L2HitBackfillL1(t *testing.T) {
	l1 := NewMemoryCache(100)
	l2 := NewMemoryCache(1000)
	cache := NewMultiCache(l1, l2)

	ctx := context.Background()
	url := "https://example.com"

	result := &domain.AnalysisResult{
		URL:   url,
		Title: "L2 Cache Test",
	}

	// Store in L2 only
	err := l2.SetHTML(ctx, url, result, 1*time.Hour)
	require.NoError(t, err)

	// Should retrieve from L2 and backfill L1
	cached, err := cache.GetHTML(ctx, url)
	assert.NoError(t, err)
	assert.Equal(t, result.Title, cached.Title)

	// Wait for async backfill
	time.Sleep(100 * time.Millisecond)

	// L1 should now have the data
	cachedL1, err := l1.GetHTML(ctx, url)
	assert.NoError(t, err)
	assert.Equal(t, result.Title, cachedL1.Title)
}

func TestMultiCache_HTMLAnalysis_CacheMiss(t *testing.T) {
	l1 := NewMemoryCache(100)
	l2 := NewMemoryCache(1000)
	cache := NewMultiCache(l1, l2)

	ctx := context.Background()
	url := "https://nonexistent.com"

	// Should miss both caches
	_, err := cache.GetHTML(ctx, url)
	assert.ErrorIs(t, err, ErrCacheMiss)
}

func TestMultiCache_SetHTML_WritesBothTiers(t *testing.T) {
	l1 := NewMemoryCache(100)
	l2 := NewMemoryCache(1000)
	cache := NewMultiCache(l1, l2)

	ctx := context.Background()
	url := "https://example.com"

	result := &domain.AnalysisResult{
		URL:   url,
		Title: "Both Tiers Test",
	}

	// Write through multi-cache
	err := cache.SetHTML(ctx, url, result, 1*time.Hour)
	assert.NoError(t, err)

	// Both tiers should have the data
	cachedL1, err := l1.GetHTML(ctx, url)
	assert.NoError(t, err)
	assert.Equal(t, result.Title, cachedL1.Title)

	cachedL2, err := l2.GetHTML(ctx, url)
	assert.NoError(t, err)
	assert.Equal(t, result.Title, cachedL2.Title)
}

func TestMultiCache_L2Failure_GracefulDegradation(t *testing.T) {
	l1 := NewMemoryCache(100)
	l2 := NewNoOpCache() // No-op cache simulates L2 failure
	cache := NewMultiCache(l1, l2)

	ctx := context.Background()
	url := "https://example.com"

	result := &domain.AnalysisResult{
		URL:   url,
		Title: "Degradation Test",
	}

	// Write should succeed (L2 failure ignored)
	err := cache.SetHTML(ctx, url, result, 1*time.Hour)
	assert.NoError(t, err)

	// L1 should have the data
	cached, err := l1.GetHTML(ctx, url)
	assert.NoError(t, err)
	assert.Equal(t, result.Title, cached.Title)
}

func TestMultiCache_ConcurrentAccess(t *testing.T) {
	l1 := NewMemoryCache(100)
	l2 := NewMemoryCache(1000)
	cache := NewMultiCache(l1, l2)

	ctx := context.Background()

	// Concurrent writes
	for i := 0; i < 10; i++ {
		go func() {
			result := &domain.AnalysisResult{
				URL:   "https://example.com",
				Title: "Concurrent",
			}
			_ = cache.SetHTML(ctx, "https://example.com", result, 1*time.Hour)
		}()
	}

	// Concurrent reads
	for i := 0; i < 10; i++ {
		go func() {
			_, _ = cache.GetHTML(ctx, "https://example.com")
		}()
	}

	// Give goroutines time to complete
	time.Sleep(200 * time.Millisecond)

	// No panics = success
}

func TestMultiCache_Close(t *testing.T) {
	l1 := NewMemoryCache(100)
	l2 := NewMemoryCache(1000)
	cache := NewMultiCache(l1, l2)

	// Close should succeed
	err := cache.Close()
	assert.NoError(t, err)

	// Calling close again should be safe (both memory caches use sync.Once)
	err = cache.Close()
	assert.NoError(t, err)
}

func TestMultiCache_CustomBackfillTTL(t *testing.T) {
	l1 := NewMemoryCache(100)
	l2 := NewMemoryCache(1000)

	// Create multi-cache with custom backfill TTL
	customBackfillTTL := 30 * time.Minute
	cache := NewMultiCacheWithTTL(l1, l2, customBackfillTTL)
	defer cache.Close()

	ctx := context.Background()

	// Store in L2 only
	url := "https://example.com"
	result := &domain.AnalysisResult{
		URL:   url,
		Title: "Test",
	}
	err := l2.SetHTML(ctx, url, result, 1*time.Hour)
	require.NoError(t, err)

	// Get from multi-cache (L1 miss, L2 hit, backfills L1)
	cached, err := cache.GetHTML(ctx, url)
	require.NoError(t, err)
	assert.Equal(t, "Test", cached.Title)

	// Give backfill time to complete
	time.Sleep(50 * time.Millisecond)

	// L1 should now have it (backfilled with custom TTL)
	l1Cached, err := l1.GetHTML(ctx, url)
	require.NoError(t, err)
	assert.Equal(t, "Test", l1Cached.Title)
}
