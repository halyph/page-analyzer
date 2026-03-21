package cache

import (
	"context"
	"testing"
	"time"

	"github.com/halyph/page-analyzer/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMemoryCache(t *testing.T) {
	cache := NewMemoryCache(10, 5*time.Minute)

	assert.NotNil(t, cache)
	assert.Equal(t, 10, cache.maxSize)
	assert.Equal(t, 5*time.Minute, cache.ttl)
}

func TestNewMemoryCache_Defaults(t *testing.T) {
	cache := NewMemoryCache(0, 0)

	assert.Equal(t, 100, cache.maxSize)
	assert.Equal(t, 1*time.Hour, cache.ttl)
}

func TestMemoryCache_SetAndGetHTML(t *testing.T) {
	cache := NewMemoryCache(10, 5*time.Minute)
	ctx := context.Background()

	url := "https://example.com"
	result := &domain.AnalysisResult{
		URL:         url,
		HTMLVersion: "HTML5",
		Title:       "Example Domain",
	}

	// Set
	err := cache.SetHTML(ctx, url, result, 0)
	require.NoError(t, err)

	// Get
	cached, err := cache.GetHTML(ctx, url)
	require.NoError(t, err)
	assert.Equal(t, result.URL, cached.URL)
	assert.Equal(t, result.Title, cached.Title)
	assert.True(t, cached.CacheHit)
}

func TestMemoryCache_GetHTML_Miss(t *testing.T) {
	cache := NewMemoryCache(10, 5*time.Minute)
	ctx := context.Background()

	_, err := cache.GetHTML(ctx, "https://nonexistent.com")
	assert.ErrorIs(t, err, ErrCacheMiss)

	stats := cache.Stats()
	assert.Equal(t, int64(0), stats.Hits)
	assert.Equal(t, int64(1), stats.Misses)
}

func TestMemoryCache_Expiration(t *testing.T) {
	cache := NewMemoryCache(10, 100*time.Millisecond)
	ctx := context.Background()

	url := "https://example.com"
	result := &domain.AnalysisResult{
		URL:   url,
		Title: "Test",
	}

	// Set with short TTL
	err := cache.SetHTML(ctx, url, result, 50*time.Millisecond)
	require.NoError(t, err)

	// Should be cached immediately
	_, err = cache.GetHTML(ctx, url)
	require.NoError(t, err)

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Should be expired
	_, err = cache.GetHTML(ctx, url)
	assert.ErrorIs(t, err, ErrCacheMiss)
}

func TestMemoryCache_LRUEviction(t *testing.T) {
	cache := NewMemoryCache(3, 5*time.Minute)
	ctx := context.Background()

	// Add 3 entries (max capacity)
	for i := 1; i <= 3; i++ {
		url := "https://example.com/page" + string(rune('0'+i))
		result := &domain.AnalysisResult{URL: url}
		err := cache.SetHTML(ctx, url, result, 0)
		require.NoError(t, err)
	}

	stats := cache.Stats()
	assert.Equal(t, int64(3), stats.Entries)

	// Add 4th entry - should evict oldest
	url4 := "https://example.com/page4"
	result4 := &domain.AnalysisResult{URL: url4}
	err := cache.SetHTML(ctx, url4, result4, 0)
	require.NoError(t, err)

	stats = cache.Stats()
	assert.Equal(t, int64(3), stats.Entries)
	assert.Equal(t, int64(1), stats.Evictions)

	// First entry should be evicted
	_, err = cache.GetHTML(ctx, "https://example.com/page1")
	assert.ErrorIs(t, err, ErrCacheMiss)

	// Fourth entry should exist
	_, err = cache.GetHTML(ctx, url4)
	require.NoError(t, err)
}

func TestMemoryCache_UpdateExisting(t *testing.T) {
	cache := NewMemoryCache(10, 5*time.Minute)
	ctx := context.Background()

	url := "https://example.com"
	result1 := &domain.AnalysisResult{
		URL:   url,
		Title: "First",
	}

	err := cache.SetHTML(ctx, url, result1, 0)
	require.NoError(t, err)

	// Update with new data
	result2 := &domain.AnalysisResult{
		URL:   url,
		Title: "Second",
	}

	err = cache.SetHTML(ctx, url, result2, 0)
	require.NoError(t, err)

	// Should have updated value
	cached, err := cache.GetHTML(ctx, url)
	require.NoError(t, err)
	assert.Equal(t, "Second", cached.Title)

	// Should still only have 1 entry
	stats := cache.Stats()
	assert.Equal(t, int64(1), stats.Entries)
}

func TestMemoryCache_LinkCheck(t *testing.T) {
	cache := NewMemoryCache(10, 5*time.Minute)
	ctx := context.Background()

	jobID := "test-job-123"
	result := &domain.LinkCheckResult{
		Checked:    10,
		Accessible: 9,
		Duration:   "1s",
	}

	// Set
	err := cache.SetLinkCheck(ctx, jobID, result, 0)
	require.NoError(t, err)

	// Get
	cached, err := cache.GetLinkCheck(ctx, jobID)
	require.NoError(t, err)
	assert.Equal(t, result.Checked, cached.Checked)
	assert.Equal(t, result.Accessible, cached.Accessible)
}

func TestMemoryCache_Delete(t *testing.T) {
	cache := NewMemoryCache(10, 5*time.Minute)
	ctx := context.Background()

	url := "https://example.com"
	result := &domain.AnalysisResult{URL: url}

	err := cache.SetHTML(ctx, url, result, 0)
	require.NoError(t, err)

	// Delete
	err = cache.Delete(ctx, url)
	require.NoError(t, err)

	// Should be gone
	_, err = cache.GetHTML(ctx, url)
	assert.ErrorIs(t, err, ErrCacheMiss)
}

func TestMemoryCache_Clear(t *testing.T) {
	cache := NewMemoryCache(10, 5*time.Minute)
	ctx := context.Background()

	// Add multiple entries
	for i := 1; i <= 5; i++ {
		url := "https://example.com/page" + string(rune('0'+i))
		result := &domain.AnalysisResult{URL: url}
		err := cache.SetHTML(ctx, url, result, 0)
		require.NoError(t, err)
	}

	stats := cache.Stats()
	assert.Equal(t, int64(5), stats.Entries)

	// Clear all
	err := cache.Clear(ctx)
	require.NoError(t, err)

	stats = cache.Stats()
	assert.Equal(t, int64(0), stats.Entries)
}

func TestMemoryCache_Stats(t *testing.T) {
	cache := NewMemoryCache(10, 5*time.Minute)
	ctx := context.Background()

	url := "https://example.com"
	result := &domain.AnalysisResult{URL: url, Title: "Test"}

	// Set
	err := cache.SetHTML(ctx, url, result, 0)
	require.NoError(t, err)

	// Hit
	_, err = cache.GetHTML(ctx, url)
	require.NoError(t, err)

	// Miss
	_, err = cache.GetHTML(ctx, "https://other.com")
	assert.Error(t, err)

	stats := cache.Stats()
	assert.Equal(t, int64(1), stats.Hits)
	assert.Equal(t, int64(1), stats.Misses)
	assert.Equal(t, int64(1), stats.Entries)
	assert.Equal(t, 0.5, stats.HitRate)
	assert.Greater(t, stats.AvgItemSize, int64(0))
}

func TestMemoryCache_CleanupExpired(t *testing.T) {
	cache := NewMemoryCache(10, 100*time.Millisecond)
	ctx := context.Background()

	// Add entries with short TTL
	for i := 1; i <= 3; i++ {
		url := "https://example.com/page" + string(rune('0'+i))
		result := &domain.AnalysisResult{URL: url}
		err := cache.SetHTML(ctx, url, result, 50*time.Millisecond)
		require.NoError(t, err)
	}

	stats := cache.Stats()
	assert.Equal(t, int64(3), stats.Entries)

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Cleanup
	removed := cache.CleanupExpired()
	assert.Equal(t, 3, removed)

	stats = cache.Stats()
	assert.Equal(t, int64(0), stats.Entries)
}

func TestMemoryCache_URLNormalization(t *testing.T) {
	cache := NewMemoryCache(10, 5*time.Minute)
	ctx := context.Background()

	url1 := "https://example.com/path?a=1&b=2"
	url2 := "https://example.com/path?b=2&a=1" // Different order

	result := &domain.AnalysisResult{URL: url1}

	// Set with url1
	err := cache.SetHTML(ctx, url1, result, 0)
	require.NoError(t, err)

	// Get with url2 (different query order) - should hit
	cached, err := cache.GetHTML(ctx, url2)
	require.NoError(t, err)
	assert.NotNil(t, cached)

	// Should count as a hit
	stats := cache.Stats()
	assert.Equal(t, int64(1), stats.Hits)
}
