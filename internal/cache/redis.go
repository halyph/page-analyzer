package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/halyph/page-analyzer/internal/domain"
	"github.com/redis/go-redis/v9"
)

// RedisCache implements cache using Redis
type RedisCache struct {
	client *redis.Client
	ttl    time.Duration
}

// NewRedisCache creates a new Redis cache client
func NewRedisCache(redisURL string, ttl time.Duration) (*RedisCache, error) {
	if ttl <= 0 {
		ttl = 1 * time.Hour
	}

	// Parse Redis URL
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid redis URL: %w", err)
	}

	// Set reasonable defaults
	if opts.DialTimeout == 0 {
		opts.DialTimeout = 5 * time.Second
	}
	if opts.ReadTimeout == 0 {
		opts.ReadTimeout = 3 * time.Second
	}
	if opts.WriteTimeout == 0 {
		opts.WriteTimeout = 3 * time.Second
	}
	if opts.PoolSize == 0 {
		opts.PoolSize = 50
	}

	client := redis.NewClient(opts)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis connection failed: %w", err)
	}

	return &RedisCache{
		client: client,
		ttl:    ttl,
	}, nil
}

// redisGet is a generic helper for retrieving and unmarshaling cached data
func redisGet[T any](rc *RedisCache, ctx context.Context, key string) (*T, error) {
	data, err := rc.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, ErrCacheMiss
	}
	if err != nil {
		return nil, fmt.Errorf("redis get failed: %w", err)
	}

	var result T
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("unmarshal failed: %w", err)
	}

	return &result, nil
}

// redisSet is a generic helper for marshaling and storing cached data
func redisSet[T any](rc *RedisCache, ctx context.Context, key string, data T, ttl time.Duration) error {
	bytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal failed: %w", err)
	}

	if err := rc.client.Set(ctx, key, bytes, ttl).Err(); err != nil {
		return fmt.Errorf("redis set failed: %w", err)
	}

	return nil
}

// GetHTML retrieves cached HTML analysis result
func (rc *RedisCache) GetHTML(ctx context.Context, url string) (*domain.AnalysisResult, error) {
	return redisGet[domain.AnalysisResult](rc, ctx, htmlKey(url))
}

// SetHTML stores HTML analysis result in cache
func (rc *RedisCache) SetHTML(ctx context.Context, url string, result *domain.AnalysisResult, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = rc.ttl
	}
	return redisSet(rc, ctx, htmlKey(url), result, ttl)
}

// GetCachedLink retrieves a cached individual link check result
func (rc *RedisCache) GetCachedLink(ctx context.Context, url string) (*domain.CachedLinkCheck, error) {
	return redisGet[domain.CachedLinkCheck](rc, ctx, cachedLinkKey(url))
}

// SetCachedLink stores an individual link check result in cache
func (rc *RedisCache) SetCachedLink(ctx context.Context, url string, result *domain.CachedLinkCheck, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = DefaultCachedLinkTTL
	}
	return redisSet(rc, ctx, cachedLinkKey(url), result, ttl)
}

// Close closes the Redis connection
func (rc *RedisCache) Close() error {
	return rc.client.Close()
}

// Ping checks Redis connectivity
func (rc *RedisCache) Ping(ctx context.Context) error {
	return rc.client.Ping(ctx).Err()
}

// Helper functions for key generation
func htmlKey(url string) string {
	key, err := GenerateHTMLKey(url)
	if err != nil {
		// Fallback to simple key if normalization fails
		return fmt.Sprintf("html:%s", url)
	}
	return key
}

func cachedLinkKey(url string) string {
	return GenerateCachedLinkKey(url)
}
