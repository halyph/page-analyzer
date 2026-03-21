package cache

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/halyph/page-analyzer/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// skipIfNoRedis skips the test if Redis is not available
func skipIfNoRedis(t *testing.T) string {
	t.Helper()

	redisURL := os.Getenv("TEST_REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379/15" // Use DB 15 for tests
	}

	// Try to connect
	cache, err := NewRedisCache(redisURL, 1*time.Hour)
	if err != nil {
		t.Skipf("Redis not available: %v", err)
		return ""
	}
	defer cache.Close()

	// Clean up test database
	ctx := context.Background()
	_ = cache.Clear(ctx)

	return redisURL
}

func TestNewRedisCache(t *testing.T) {
	redisURL := skipIfNoRedis(t)

	cache, err := NewRedisCache(redisURL, 1*time.Hour)
	require.NoError(t, err)
	require.NotNil(t, cache)
	defer cache.Close()

	// Test connection
	ctx := context.Background()
	err = cache.Ping(ctx)
	assert.NoError(t, err)
}

func TestNewRedisCache_InvalidURL(t *testing.T) {
	_, err := NewRedisCache("invalid://url", 1*time.Hour)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid redis URL")
}

func TestRedisCache_HTMLAnalysis(t *testing.T) {
	redisURL := skipIfNoRedis(t)

	cache, err := NewRedisCache(redisURL, 1*time.Hour)
	require.NoError(t, err)
	defer cache.Close()

	ctx := context.Background()
	url := "https://example.com"

	// Test cache miss
	_, err = cache.GetHTML(ctx, url)
	assert.ErrorIs(t, err, ErrCacheMiss)

	// Store result
	result := &domain.AnalysisResult{
		URL:         url,
		HTMLVersion: "HTML5",
		Title:       "Example Domain",
		Headings: domain.HeadingCounts{
			H1: 1,
			H2: 2,
		},
		HasLoginForm: false,
	}

	err = cache.SetHTML(ctx, url, result, 1*time.Hour)
	assert.NoError(t, err)

	// Retrieve result
	cached, err := cache.GetHTML(ctx, url)
	assert.NoError(t, err)
	assert.Equal(t, result.URL, cached.URL)
	assert.Equal(t, result.HTMLVersion, cached.HTMLVersion)
	assert.Equal(t, result.Title, cached.Title)
	assert.Equal(t, result.Headings, cached.Headings)
	assert.Equal(t, result.HasLoginForm, cached.HasLoginForm)

	// Check stats
	stats := cache.Stats()
	assert.Equal(t, int64(1), stats.Hits)
	assert.Equal(t, int64(1), stats.Misses)
	assert.Greater(t, stats.Entries, int64(0))
}

func TestRedisCache_LinkCheck(t *testing.T) {
	redisURL := skipIfNoRedis(t)

	cache, err := NewRedisCache(redisURL, 1*time.Hour)
	require.NoError(t, err)
	defer cache.Close()

	ctx := context.Background()
	jobID := "test-job-123"

	// Test cache miss
	_, err = cache.GetLinkCheck(ctx, jobID)
	assert.ErrorIs(t, err, ErrCacheMiss)

	// Store result
	result := &domain.LinkCheckResult{
		Checked:    10,
		Accessible: 8,
		Inaccessible: []domain.LinkError{
			{URL: "https://broken.com", StatusCode: 404, Reason: "404"},
			{URL: "https://timeout.com", Reason: "timeout"},
		},
		Duration: "2.5s",
	}

	err = cache.SetLinkCheck(ctx, jobID, result, 5*time.Minute)
	assert.NoError(t, err)

	// Retrieve result
	cached, err := cache.GetLinkCheck(ctx, jobID)
	assert.NoError(t, err)
	assert.Equal(t, result.Checked, cached.Checked)
	assert.Equal(t, result.Accessible, cached.Accessible)
	assert.Equal(t, len(result.Inaccessible), len(cached.Inaccessible))
	assert.Equal(t, result.Duration, cached.Duration)
}

func TestRedisCache_Delete(t *testing.T) {
	redisURL := skipIfNoRedis(t)

	cache, err := NewRedisCache(redisURL, 1*time.Hour)
	require.NoError(t, err)
	defer cache.Close()

	ctx := context.Background()
	url := "https://example.com"

	// Store result
	result := &domain.AnalysisResult{
		URL:   url,
		Title: "Test",
	}
	err = cache.SetHTML(ctx, url, result, 1*time.Hour)
	require.NoError(t, err)

	// Verify stored
	_, err = cache.GetHTML(ctx, url)
	assert.NoError(t, err)

	// Delete
	err = cache.Delete(ctx, url)
	assert.NoError(t, err)

	// Verify deleted
	_, err = cache.GetHTML(ctx, url)
	assert.ErrorIs(t, err, ErrCacheMiss)
}

func TestRedisCache_Clear(t *testing.T) {
	redisURL := skipIfNoRedis(t)

	cache, err := NewRedisCache(redisURL, 1*time.Hour)
	require.NoError(t, err)
	defer cache.Close()

	ctx := context.Background()

	// Store some data
	result := &domain.AnalysisResult{
		URL:   "https://example.com",
		Title: "Test",
	}
	err = cache.SetHTML(ctx, "https://example.com", result, 1*time.Hour)
	require.NoError(t, err)

	// Clear cache
	err = cache.Clear(ctx)
	assert.NoError(t, err)

	// Stats should be reset immediately after clear
	stats := cache.Stats()
	assert.Equal(t, int64(0), stats.Hits)
	assert.Equal(t, int64(0), stats.Misses)

	// Verify data cleared
	_, err = cache.GetHTML(ctx, "https://example.com")
	assert.ErrorIs(t, err, ErrCacheMiss)
}

func TestRedisCache_TTL(t *testing.T) {
	redisURL := skipIfNoRedis(t)

	cache, err := NewRedisCache(redisURL, 1*time.Hour)
	require.NoError(t, err)
	defer cache.Close()

	ctx := context.Background()
	url := "https://example.com"

	// Store with short TTL
	result := &domain.AnalysisResult{
		URL:   url,
		Title: "Test",
	}
	err = cache.SetHTML(ctx, url, result, 1*time.Second)
	require.NoError(t, err)

	// Verify stored
	_, err = cache.GetHTML(ctx, url)
	assert.NoError(t, err)

	// Wait for expiration
	time.Sleep(1500 * time.Millisecond)

	// Verify expired
	_, err = cache.GetHTML(ctx, url)
	assert.ErrorIs(t, err, ErrCacheMiss)
}

func TestRedisCache_Stats(t *testing.T) {
	redisURL := skipIfNoRedis(t)

	cache, err := NewRedisCache(redisURL, 1*time.Hour)
	require.NoError(t, err)
	defer cache.Close()

	ctx := context.Background()

	// Initial stats
	stats := cache.Stats()
	initialHits := stats.Hits
	initialMisses := stats.Misses

	// Cause a miss
	_, err = cache.GetHTML(ctx, "https://miss.com")
	assert.ErrorIs(t, err, ErrCacheMiss)

	// Store and retrieve (hit)
	result := &domain.AnalysisResult{
		URL:   "https://hit.com",
		Title: "Hit",
	}
	err = cache.SetHTML(ctx, "https://hit.com", result, 1*time.Hour)
	require.NoError(t, err)

	_, err = cache.GetHTML(ctx, "https://hit.com")
	require.NoError(t, err)

	// Check updated stats
	stats = cache.Stats()
	assert.Equal(t, initialHits+1, stats.Hits)
	assert.Equal(t, initialMisses+1, stats.Misses)

	// Verify hit rate calculation
	if stats.Hits+stats.Misses > 0 {
		expectedHitRate := float64(stats.Hits) / float64(stats.Hits+stats.Misses)
		assert.InDelta(t, expectedHitRate, stats.HitRate, 0.01)
	}
}

func TestRedisCache_ConcurrentAccess(t *testing.T) {
	redisURL := skipIfNoRedis(t)

	cache, err := NewRedisCache(redisURL, 1*time.Hour)
	require.NoError(t, err)
	defer cache.Close()

	ctx := context.Background()

	// Concurrent writes
	for i := 0; i < 10; i++ {
		go func() {
			result := &domain.AnalysisResult{
				URL:   "https://example.com",
				Title: "Test",
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
	time.Sleep(100 * time.Millisecond)

	// No panics = success
}

func TestRedisCache_Health(t *testing.T) {
	redisURL := skipIfNoRedis(t)

	cache, err := NewRedisCache(redisURL, 1*time.Hour)
	require.NoError(t, err)
	defer cache.Close()

	ctx := context.Background()

	// Health check should succeed
	err = cache.Health(ctx)
	assert.NoError(t, err)
}

func TestRedisCache_Close(t *testing.T) {
	redisURL := skipIfNoRedis(t)

	cache, err := NewRedisCache(redisURL, 1*time.Hour)
	require.NoError(t, err)

	// Close should succeed
	err = cache.Close()
	assert.NoError(t, err)

	// Operations after close should fail
	ctx := context.Background()
	result := &domain.AnalysisResult{URL: "https://example.com"}
	err = cache.SetHTML(ctx, "https://example.com", result, 1*time.Hour)
	assert.Error(t, err)
}
