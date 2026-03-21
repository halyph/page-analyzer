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
	l1 := NewMemoryCache(100, 1*time.Hour)
	l2 := NewMemoryCache(1000, 1*time.Hour)

	cache := NewMultiCache(l1, l2)
	assert.NotNil(t, cache)
}

func TestMultiCache_HTMLAnalysis_L1Hit(t *testing.T) {
	l1 := NewMemoryCache(100, 1*time.Hour)
	l2 := NewMemoryCache(1000, 1*time.Hour)
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

	// L1 should have hit
	l1Stats := l1.Stats()
	assert.Equal(t, int64(1), l1Stats.Hits)

	// L2 should not have been accessed
	l2Stats := l2.Stats()
	assert.Equal(t, int64(0), l2Stats.Hits)
}

func TestMultiCache_HTMLAnalysis_L2HitBackfillL1(t *testing.T) {
	l1 := NewMemoryCache(100, 1*time.Hour)
	l2 := NewMemoryCache(1000, 1*time.Hour)
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

	// L2 should have hit
	l2Stats := l2.Stats()
	assert.Equal(t, int64(1), l2Stats.Hits)

	// Wait for async backfill
	time.Sleep(100 * time.Millisecond)

	// L1 should now have the data
	cachedL1, err := l1.GetHTML(ctx, url)
	assert.NoError(t, err)
	assert.Equal(t, result.Title, cachedL1.Title)
}

func TestMultiCache_HTMLAnalysis_CacheMiss(t *testing.T) {
	l1 := NewMemoryCache(100, 1*time.Hour)
	l2 := NewMemoryCache(1000, 1*time.Hour)
	cache := NewMultiCache(l1, l2)

	ctx := context.Background()
	url := "https://nonexistent.com"

	// Should miss both caches
	_, err := cache.GetHTML(ctx, url)
	assert.ErrorIs(t, err, ErrCacheMiss)

	// Both should have misses
	l1Stats := l1.Stats()
	l2Stats := l2.Stats()
	assert.Equal(t, int64(1), l1Stats.Misses)
	assert.Equal(t, int64(1), l2Stats.Misses)
}

func TestMultiCache_SetHTML_WritesBothTiers(t *testing.T) {
	l1 := NewMemoryCache(100, 1*time.Hour)
	l2 := NewMemoryCache(1000, 1*time.Hour)
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

func TestMultiCache_LinkCheck_L1Hit(t *testing.T) {
	l1 := NewMemoryCache(100, 1*time.Hour)
	l2 := NewMemoryCache(1000, 1*time.Hour)
	cache := NewMultiCache(l1, l2)

	ctx := context.Background()
	jobID := "test-job-123"

	result := &domain.LinkCheckResult{
		Checked:    10,
		Accessible: 8,
		Duration:   "2.5s",
	}

	// Store in L1 only
	err := l1.SetLinkCheck(ctx, jobID, result, 5*time.Minute)
	require.NoError(t, err)

	// Should retrieve from L1
	cached, err := cache.GetLinkCheck(ctx, jobID)
	assert.NoError(t, err)
	assert.Equal(t, result.Checked, cached.Checked)
	assert.Equal(t, result.Accessible, cached.Accessible)
}

func TestMultiCache_LinkCheck_L2HitBackfillL1(t *testing.T) {
	l1 := NewMemoryCache(100, 1*time.Hour)
	l2 := NewMemoryCache(1000, 1*time.Hour)
	cache := NewMultiCache(l1, l2)

	ctx := context.Background()
	jobID := "test-job-456"

	result := &domain.LinkCheckResult{
		Checked:    20,
		Accessible: 18,
		Duration:   "5s",
	}

	// Store in L2 only
	err := l2.SetLinkCheck(ctx, jobID, result, 5*time.Minute)
	require.NoError(t, err)

	// Should retrieve from L2 and backfill L1
	cached, err := cache.GetLinkCheck(ctx, jobID)
	assert.NoError(t, err)
	assert.Equal(t, result.Checked, cached.Checked)

	// Wait for async backfill
	time.Sleep(100 * time.Millisecond)

	// L1 should now have the data
	cachedL1, err := l1.GetLinkCheck(ctx, jobID)
	assert.NoError(t, err)
	assert.Equal(t, result.Checked, cachedL1.Checked)
}

func TestMultiCache_Delete_BothTiers(t *testing.T) {
	l1 := NewMemoryCache(100, 1*time.Hour)
	l2 := NewMemoryCache(1000, 1*time.Hour)
	cache := NewMultiCache(l1, l2)

	ctx := context.Background()
	url := "https://example.com"

	result := &domain.AnalysisResult{
		URL:   url,
		Title: "Delete Test",
	}

	// Store in both tiers
	err := cache.SetHTML(ctx, url, result, 1*time.Hour)
	require.NoError(t, err)

	// Delete
	err = cache.Delete(ctx, url)
	assert.NoError(t, err)

	// Both tiers should be empty
	_, err = l1.GetHTML(ctx, url)
	assert.ErrorIs(t, err, ErrCacheMiss)

	_, err = l2.GetHTML(ctx, url)
	assert.ErrorIs(t, err, ErrCacheMiss)
}

func TestMultiCache_Clear_BothTiers(t *testing.T) {
	l1 := NewMemoryCache(100, 1*time.Hour)
	l2 := NewMemoryCache(1000, 1*time.Hour)
	cache := NewMultiCache(l1, l2)

	ctx := context.Background()

	// Store some data
	result := &domain.AnalysisResult{
		URL:   "https://example.com",
		Title: "Clear Test",
	}
	err := cache.SetHTML(ctx, "https://example.com", result, 1*time.Hour)
	require.NoError(t, err)

	// Clear
	err = cache.Clear(ctx)
	assert.NoError(t, err)

	// Both tiers should be empty
	_, err = l1.GetHTML(ctx, "https://example.com")
	assert.ErrorIs(t, err, ErrCacheMiss)

	_, err = l2.GetHTML(ctx, "https://example.com")
	assert.ErrorIs(t, err, ErrCacheMiss)
}

func TestMultiCache_Stats_Combined(t *testing.T) {
	l1 := NewMemoryCache(100, 1*time.Hour)
	l2 := NewMemoryCache(1000, 1*time.Hour)
	cache := NewMultiCache(l1, l2)

	ctx := context.Background()

	// Cause some hits and misses
	result := &domain.AnalysisResult{
		URL:   "https://hit.com",
		Title: "Hit",
	}
	_ = l1.SetHTML(ctx, "https://hit.com", result, 1*time.Hour)
	_, _ = cache.GetHTML(ctx, "https://hit.com")   // L1 hit
	_, _ = cache.GetHTML(ctx, "https://miss1.com") // L1 miss, L2 miss
	_, _ = cache.GetHTML(ctx, "https://miss2.com") // L1 miss, L2 miss

	// Get combined stats
	stats := cache.Stats()
	assert.Equal(t, int64(1), stats.Hits)   // 1 L1 hit
	assert.Equal(t, int64(4), stats.Misses) // 2 L1 misses + 2 L2 misses
}

func TestMultiCache_StatsDetailed(t *testing.T) {
	l1 := NewMemoryCache(100, 1*time.Hour)
	l2 := NewMemoryCache(1000, 1*time.Hour)
	cache := NewMultiCache(l1, l2)

	ctx := context.Background()

	// Store in L1
	result1 := &domain.AnalysisResult{
		URL:   "https://l1.com",
		Title: "L1",
	}
	_ = l1.SetHTML(ctx, "https://l1.com", result1, 1*time.Hour)

	// Store in L2
	result2 := &domain.AnalysisResult{
		URL:   "https://l2.com",
		Title: "L2",
	}
	_ = l2.SetHTML(ctx, "https://l2.com", result2, 1*time.Hour)

	// Get detailed stats
	l1Stats, l2Stats := cache.StatsDetailed()

	// L1 should have 1 entry
	assert.Equal(t, int64(1), l1Stats.Entries)

	// L2 should have 1 entry
	assert.Equal(t, int64(1), l2Stats.Entries)
}

func TestMultiCache_L2Failure_GracefulDegradation(t *testing.T) {
	l1 := NewMemoryCache(100, 1*time.Hour)
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

func TestMultiCache_Stats_ZeroDivision(t *testing.T) {
	l1 := NewMemoryCache(100, 1*time.Hour)
	l2 := NewMemoryCache(1000, 1*time.Hour)
	cache := NewMultiCache(l1, l2)

	// Get stats with no activity (should not panic on division by zero)
	stats := cache.Stats()
	assert.Equal(t, int64(0), stats.Hits)
	assert.Equal(t, int64(0), stats.Misses)
	assert.Equal(t, 0.0, stats.HitRate)
}

func TestMultiCache_ConcurrentAccess(t *testing.T) {
	l1 := NewMemoryCache(100, 1*time.Hour)
	l2 := NewMemoryCache(1000, 1*time.Hour)
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

func TestMultiCache_Health(t *testing.T) {
	l1 := NewMemoryCache(100, 1*time.Hour)
	l2 := NewMemoryCache(1000, 1*time.Hour)
	cache := NewMultiCache(l1, l2)
	defer cache.Close()

	ctx := context.Background()

	// Health check should succeed
	err := cache.Health(ctx)
	assert.NoError(t, err)
}

func TestMultiCache_Close(t *testing.T) {
	l1 := NewMemoryCache(100, 1*time.Hour)
	l2 := NewMemoryCache(1000, 1*time.Hour)
	cache := NewMultiCache(l1, l2)

	// Close should succeed
	err := cache.Close()
	assert.NoError(t, err)

	// Calling close again should be safe (both memory caches use sync.Once)
	err = cache.Close()
	assert.NoError(t, err)
}

func TestMultiCache_CustomBackfillTTL(t *testing.T) {
	l1 := NewMemoryCache(100, 1*time.Hour)
	l2 := NewMemoryCache(1000, 1*time.Hour)

	// Create multi-cache with custom backfill TTLs
	customHTMLTTL := 30 * time.Minute
	customLinkTTL := 2 * time.Minute
	cache := NewMultiCacheWithTTL(l1, l2, customHTMLTTL, customLinkTTL)
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
