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

	// Clean up test database using Redis client directly
	ctx := context.Background()
	_ = cache.client.FlushDB(ctx).Err()

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
