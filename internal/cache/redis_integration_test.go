//go:build integration

package cache

import (
	"context"
	"testing"
	"time"

	"github.com/halyph/page-analyzer/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/redis"
)

// setupTestRedis creates a Redis container for integration tests
func setupTestRedis(t *testing.T) (string, func()) {
	t.Helper()

	ctx := context.Background()

	redisContainer, err := redis.Run(ctx,
		"redis:7-alpine",
		redis.WithSnapshotting(10, 1),
		redis.WithLogLevel(redis.LogLevelVerbose),
	)
	require.NoError(t, err, "failed to start Redis container")

	// Get connection string
	connectionString, err := redisContainer.ConnectionString(ctx)
	require.NoError(t, err, "failed to get connection string")

	// Cleanup function
	cleanup := func() {
		if err := redisContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %s", err)
		}
	}

	return connectionString, cleanup
}

func TestIntegration_RedisCache_FullLifecycle(t *testing.T) {
	redisURL, cleanup := setupTestRedis(t)
	defer cleanup()

	cache, err := NewRedisCache(redisURL, 1*time.Hour)
	require.NoError(t, err)
	defer cache.Close()

	ctx := context.Background()
	url := "https://example.com"

	// Verify empty cache
	_, err = cache.GetHTML(ctx, url)
	assert.ErrorIs(t, err, ErrCacheMiss)

	// Store HTML result
	result := &domain.AnalysisResult{
		URL:         url,
		HTMLVersion: "HTML5",
		Title:       "Integration Test",
		Headings: domain.HeadingCounts{
			H1: 1,
			H2: 3,
			H3: 5,
		},
		HasLoginForm: false,
	}

	err = cache.SetHTML(ctx, url, result, 1*time.Hour)
	require.NoError(t, err)

	// Retrieve and verify
	cached, err := cache.GetHTML(ctx, url)
	require.NoError(t, err)
	assert.Equal(t, result.URL, cached.URL)
	assert.Equal(t, result.Title, cached.Title)
	assert.Equal(t, result.Headings, cached.Headings)

	// Store link check result
	jobID := "test-job-123"
	linkResult := &domain.LinkCheckResult{
		Checked:    100,
		Accessible: 95,
		Inaccessible: []domain.LinkError{
			{URL: "https://broken.com", StatusCode: 404, Reason: "404"},
		},
		Duration: "5.2s",
	}

	err = cache.SetLinkCheck(ctx, jobID, linkResult, 5*time.Minute)
	require.NoError(t, err)

	// Retrieve link check result
	cachedLink, err := cache.GetLinkCheck(ctx, jobID)
	require.NoError(t, err)
	assert.Equal(t, linkResult.Checked, cachedLink.Checked)
	assert.Equal(t, linkResult.Accessible, cachedLink.Accessible)

	// Verify stats
	stats := cache.Stats()
	assert.Equal(t, int64(2), stats.Hits)
	assert.Equal(t, int64(1), stats.Misses)
	assert.Greater(t, stats.Entries, int64(0))
}

func TestIntegration_RedisCache_TTLExpiration(t *testing.T) {
	redisURL, cleanup := setupTestRedis(t)
	defer cleanup()

	cache, err := NewRedisCache(redisURL, 1*time.Hour)
	require.NoError(t, err)
	defer cache.Close()

	ctx := context.Background()
	url := "https://ttl-test.com"

	result := &domain.AnalysisResult{
		URL:   url,
		Title: "TTL Test",
	}

	// Store with 1 second TTL
	err = cache.SetHTML(ctx, url, result, 1*time.Second)
	require.NoError(t, err)

	// Verify immediately available
	_, err = cache.GetHTML(ctx, url)
	assert.NoError(t, err)

	// Wait for expiration
	time.Sleep(1500 * time.Millisecond)

	// Verify expired
	_, err = cache.GetHTML(ctx, url)
	assert.ErrorIs(t, err, ErrCacheMiss)
}

func TestIntegration_RedisCache_ConcurrentWrites(t *testing.T) {
	redisURL, cleanup := setupTestRedis(t)
	defer cleanup()

	cache, err := NewRedisCache(redisURL, 1*time.Hour)
	require.NoError(t, err)
	defer cache.Close()

	ctx := context.Background()

	// Concurrent writes to different keys
	const numGoroutines = 50
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(n int) {
			result := &domain.AnalysisResult{
				URL:   "https://concurrent-test.com/" + string(rune('0'+n)),
				Title: "Concurrent Test",
			}
			_ = cache.SetHTML(ctx, result.URL, result, 1*time.Hour)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify no data loss
	stats := cache.Stats()
	assert.GreaterOrEqual(t, stats.Entries, int64(numGoroutines))
}

func TestIntegration_RedisCache_LargeDataset(t *testing.T) {
	redisURL, cleanup := setupTestRedis(t)
	defer cleanup()

	cache, err := NewRedisCache(redisURL, 1*time.Hour)
	require.NoError(t, err)
	defer cache.Close()

	ctx := context.Background()

	// Store many entries
	const numEntries = 1000
	for i := 0; i < numEntries; i++ {
		result := &domain.AnalysisResult{
			URL:   "https://large-test.com/" + string(rune('0'+i)),
			Title: "Large Test",
		}
		err := cache.SetHTML(ctx, result.URL, result, 1*time.Hour)
		require.NoError(t, err)
	}

	// Verify entries stored
	stats := cache.Stats()
	assert.GreaterOrEqual(t, stats.Entries, int64(numEntries))

	// Clear all
	err = cache.Clear(ctx)
	require.NoError(t, err)

	// Verify cleared
	stats = cache.Stats()
	assert.Equal(t, int64(0), stats.Entries)
}

func TestIntegration_MultiCache_WithRealRedis(t *testing.T) {
	redisURL, cleanup := setupTestRedis(t)
	defer cleanup()

	// Create L1 (memory) + L2 (Redis)
	l1 := NewMemoryCache(100, 1*time.Hour)
	l2, err := NewRedisCache(redisURL, 1*time.Hour)
	require.NoError(t, err)
	defer l2.Close()

	cache := NewMultiCache(l1, l2)

	ctx := context.Background()
	url := "https://multi-test.com"

	result := &domain.AnalysisResult{
		URL:   url,
		Title: "Multi-tier Test",
	}

	// Store through multi-cache
	err = cache.SetHTML(ctx, url, result, 1*time.Hour)
	require.NoError(t, err)

	// Should be in both L1 and L2
	cachedL1, err := l1.GetHTML(ctx, url)
	assert.NoError(t, err)
	assert.Equal(t, result.Title, cachedL1.Title)

	cachedL2, err := l2.GetHTML(ctx, url)
	assert.NoError(t, err)
	assert.Equal(t, result.Title, cachedL2.Title)

	// Clear L1 only
	err = l1.Clear(ctx)
	require.NoError(t, err)

	// Should still retrieve from L2 and backfill L1
	cached, err := cache.GetHTML(ctx, url)
	assert.NoError(t, err)
	assert.Equal(t, result.Title, cached.Title)

	// Wait for async backfill
	time.Sleep(200 * time.Millisecond)

	// L1 should have data again
	cachedL1Again, err := l1.GetHTML(ctx, url)
	assert.NoError(t, err)
	assert.Equal(t, result.Title, cachedL1Again.Title)
}

func TestIntegration_RedisCache_ConnectionFailure(t *testing.T) {
	// Try to connect to non-existent Redis
	_, err := NewRedisCache("redis://localhost:9999/0", 1*time.Hour)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "redis connection failed")
}

func TestIntegration_RedisCache_Reconnection(t *testing.T) {
	redisURL, cleanup := setupTestRedis(t)
	defer cleanup()

	cache, err := NewRedisCache(redisURL, 1*time.Hour)
	require.NoError(t, err)
	defer cache.Close()

	ctx := context.Background()

	// Store data
	result := &domain.AnalysisResult{
		URL:   "https://reconnect-test.com",
		Title: "Reconnect Test",
	}
	err = cache.SetHTML(ctx, "https://reconnect-test.com", result, 1*time.Hour)
	require.NoError(t, err)

	// Verify can retrieve
	_, err = cache.GetHTML(ctx, "https://reconnect-test.com")
	assert.NoError(t, err)

	// Verify ping works
	err = cache.Ping(ctx)
	assert.NoError(t, err)
}
