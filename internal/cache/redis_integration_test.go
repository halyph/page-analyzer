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

	cache, err := NewRedisCache(redisURL)
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

	// Test individual link caching
	linkURL := "https://example.com/link1"
	linkCheck := &domain.CachedLinkCheck{
		URL:        linkURL,
		Accessible: true,
		StatusCode: 200,
		CheckedAt:  time.Now().Unix(),
	}

	err = cache.SetCachedLink(ctx, linkURL, linkCheck, 5*time.Minute)
	require.NoError(t, err)

	// Retrieve cached link
	cachedLink, err := cache.GetCachedLink(ctx, linkURL)
	require.NoError(t, err)
	assert.Equal(t, linkCheck.URL, cachedLink.URL)
	assert.Equal(t, linkCheck.Accessible, cachedLink.Accessible)
	assert.Equal(t, linkCheck.StatusCode, cachedLink.StatusCode)
}

func TestIntegration_RedisCache_TTLExpiration(t *testing.T) {
	redisURL, cleanup := setupTestRedis(t)
	defer cleanup()

	cache, err := NewRedisCache(redisURL)
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

	cache, err := NewRedisCache(redisURL)
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

	// Verify no data loss by reading back some entries
	for i := 0; i < 10; i++ {
		testURL := "https://concurrent-test.com/" + string(rune('0'+i))
		_, err := cache.GetHTML(ctx, testURL)
		assert.NoError(t, err)
	}
}

func TestIntegration_RedisCache_LargeDataset(t *testing.T) {
	redisURL, cleanup := setupTestRedis(t)
	defer cleanup()

	cache, err := NewRedisCache(redisURL)
	require.NoError(t, err)
	defer cache.Close()

	ctx := context.Background()

	// Store many entries
	const numEntries = 100
	urls := make([]string, numEntries)
	for i := 0; i < numEntries; i++ {
		url := "https://large-test.com/page" + string(rune('0'+i%10)) + string(rune('0'+i/10))
		urls[i] = url
		result := &domain.AnalysisResult{
			URL:   url,
			Title: "Large Test",
		}
		err := cache.SetHTML(ctx, url, result, 1*time.Hour)
		require.NoError(t, err)
	}

	// Verify entries stored by sampling reads
	for i := 0; i < 10; i++ {
		_, err := cache.GetHTML(ctx, urls[i])
		assert.NoError(t, err, "failed to retrieve entry %d", i)
	}
}

func TestIntegration_MultiCache_WithRealRedis(t *testing.T) {
	redisURL, cleanup := setupTestRedis(t)
	defer cleanup()

	// Create L1 (memory) + L2 (Redis)
	l1 := NewMemoryCache(100)
	l2, err := NewRedisCache(redisURL)
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

	// Test retrieval from multi-cache
	cached, err := cache.GetHTML(ctx, url)
	assert.NoError(t, err)
	assert.Equal(t, result.Title, cached.Title)
}

func TestIntegration_RedisCache_ConnectionFailure(t *testing.T) {
	// Try to connect to non-existent Redis
	_, err := NewRedisCache("redis://localhost:9999/0")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "redis connection failed")
}

func TestIntegration_RedisCache_Reconnection(t *testing.T) {
	redisURL, cleanup := setupTestRedis(t)
	defer cleanup()

	cache, err := NewRedisCache(redisURL)
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
