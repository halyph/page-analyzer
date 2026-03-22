package cache

import (
	"container/list"
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/halyph/page-analyzer/internal/domain"
)

const (
	// cleanupInterval is how often expired entries are cleaned up
	cleanupInterval = 5 * time.Minute

	// maxTTL is the maximum allowed TTL for cache entries
	// Prevents extremely long-lived entries that defeat cleanup
	maxTTL = 24 * time.Hour
)

// MemoryCache implements an LRU cache in memory
type MemoryCache struct {
	maxSize int
	ttl     time.Duration

	mu    sync.RWMutex
	items map[string]*cacheEntry
	lru   *list.List // Least recently used tracking

	// Statistics
	hits      int64
	misses    int64
	evictions int64

	// Cleanup
	stopCleanup chan struct{}
	cleanupDone chan struct{}
	closeOnce   sync.Once
}

// cacheEntry represents a cached item
type cacheEntry struct {
	key       string
	value     []byte
	expiresAt time.Time
	element   *list.Element // Position in LRU list
	size      int64
}

// NewMemoryCache creates a new in-memory LRU cache
func NewMemoryCache(maxSize int, ttl time.Duration) *MemoryCache {
	if maxSize <= 0 {
		maxSize = 100
	}
	if ttl <= 0 {
		ttl = 1 * time.Hour
	}

	mc := &MemoryCache{
		maxSize:     maxSize,
		ttl:         ttl,
		items:       make(map[string]*cacheEntry),
		lru:         list.New(),
		stopCleanup: make(chan struct{}),
		cleanupDone: make(chan struct{}),
	}

	// Start background cleanup goroutine
	go mc.cleanupLoop()

	return mc
}

// memoryGet is a generic helper for Get operations
func memoryGet[T any](mc *MemoryCache, key string) (*T, error) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	entry, exists := mc.items[key]
	if !exists {
		mc.misses++
		return nil, ErrCacheMiss
	}

	// Check expiration
	if time.Now().After(entry.expiresAt) {
		mc.removeEntry(entry)
		mc.misses++
		return nil, ErrCacheMiss
	}

	// Move to front (most recently used)
	mc.lru.MoveToFront(entry.element)
	mc.hits++

	// Deserialize
	var result T
	if err := json.Unmarshal(entry.value, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// memorySet is a generic helper for Set operations
func memorySet[T any](mc *MemoryCache, key string, data T, ttl time.Duration) error {
	// Enforce maximum TTL
	if ttl > maxTTL {
		ttl = maxTTL
	}

	// Serialize
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}

	mc.mu.Lock()
	defer mc.mu.Unlock()

	// Check if already exists
	if entry, exists := mc.items[key]; exists {
		// Update existing entry
		entry.value = bytes
		entry.expiresAt = time.Now().Add(ttl)
		entry.size = int64(len(bytes))
		mc.lru.MoveToFront(entry.element)
		return nil
	}

	// Evict if at capacity
	if mc.lru.Len() >= mc.maxSize {
		mc.evictOldest()
	}

	// Add new entry
	entry := &cacheEntry{
		key:       key,
		value:     bytes,
		expiresAt: time.Now().Add(ttl),
		size:      int64(len(bytes)),
	}

	entry.element = mc.lru.PushFront(entry)
	mc.items[key] = entry

	return nil
}

// GetHTML retrieves cached HTML analysis result
func (mc *MemoryCache) GetHTML(ctx context.Context, url string) (*domain.AnalysisResult, error) {
	key, err := GenerateHTMLKey(url)
	if err != nil {
		return nil, err
	}

	result, err := memoryGet[domain.AnalysisResult](mc, key)
	if err != nil {
		return nil, err
	}

	result.CacheHit = true
	return result, nil
}

// SetHTML stores HTML analysis result in cache
func (mc *MemoryCache) SetHTML(ctx context.Context, url string, result *domain.AnalysisResult, ttl time.Duration) error {
	key, err := GenerateHTMLKey(url)
	if err != nil {
		return err
	}

	if ttl == 0 {
		ttl = mc.ttl
	}

	return memorySet(mc, key, result, ttl)
}

// GetLinkCheck retrieves cached link check result
func (mc *MemoryCache) GetLinkCheck(ctx context.Context, jobID string) (*domain.LinkCheckResult, error) {
	key := GenerateLinkCheckKey(jobID)
	return memoryGet[domain.LinkCheckResult](mc, key)
}

// SetLinkCheck stores link check result in cache
func (mc *MemoryCache) SetLinkCheck(ctx context.Context, jobID string, result *domain.LinkCheckResult, ttl time.Duration) error {
	key := GenerateLinkCheckKey(jobID)

	if ttl == 0 {
		ttl = DefaultLinkCheckTTL
	}

	return memorySet(mc, key, result, ttl)
}

// GetCachedLink retrieves a cached individual link check result
func (mc *MemoryCache) GetCachedLink(ctx context.Context, url string) (*domain.CachedLinkCheck, error) {
	key := GenerateCachedLinkKey(url)
	return memoryGet[domain.CachedLinkCheck](mc, key)
}

// SetCachedLink stores an individual link check result in cache
func (mc *MemoryCache) SetCachedLink(ctx context.Context, url string, result *domain.CachedLinkCheck, ttl time.Duration) error {
	key := GenerateCachedLinkKey(url)

	if ttl == 0 {
		ttl = DefaultCachedLinkTTL
	}

	return memorySet(mc, key, result, ttl)
}

// Delete removes a cached entry
func (mc *MemoryCache) Delete(ctx context.Context, url string) error {
	key, err := GenerateHTMLKey(url)
	if err != nil {
		return err
	}

	mc.mu.Lock()
	defer mc.mu.Unlock()

	if entry, exists := mc.items[key]; exists {
		mc.removeEntry(entry)
	}

	return nil
}

// Clear removes all cached entries
func (mc *MemoryCache) Clear(ctx context.Context) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.items = make(map[string]*cacheEntry)
	mc.lru.Init()

	return nil
}

// Stats returns cache statistics
func (mc *MemoryCache) Stats() CacheStats {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	total := mc.hits + mc.misses
	hitRate := 0.0
	if total > 0 {
		hitRate = float64(mc.hits) / float64(total)
	}

	// Calculate average item size
	var totalSize int64
	for _, entry := range mc.items {
		totalSize += entry.size
	}
	avgSize := int64(0)
	if len(mc.items) > 0 {
		avgSize = totalSize / int64(len(mc.items))
	}

	return CacheStats{
		Hits:        mc.hits,
		Misses:      mc.misses,
		Entries:     int64(len(mc.items)),
		Evictions:   mc.evictions,
		HitRate:     hitRate,
		AvgItemSize: avgSize,
	}
}

// Health checks cache availability (always healthy for memory cache)
func (mc *MemoryCache) Health(ctx context.Context) error {
	return nil
}

// Close stops the cleanup goroutine and releases resources
// It's safe to call Close multiple times
func (mc *MemoryCache) Close() error {
	mc.closeOnce.Do(func() {
		close(mc.stopCleanup)
		<-mc.cleanupDone // Wait for cleanup goroutine to finish
	})
	return nil
}

// cleanupLoop periodically removes expired entries
func (mc *MemoryCache) cleanupLoop() {
	defer close(mc.cleanupDone)

	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-mc.stopCleanup:
			return
		case <-ticker.C:
			mc.cleanupExpired()
		}
	}
}

// cleanupExpired removes all expired entries
func (mc *MemoryCache) cleanupExpired() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	now := time.Now()
	for _, entry := range mc.items {
		if now.After(entry.expiresAt) {
			mc.removeEntry(entry)
		}
	}
}

// evictOldest removes the least recently used entry
func (mc *MemoryCache) evictOldest() {
	element := mc.lru.Back()
	if element == nil {
		return
	}

	entry := element.Value.(*cacheEntry)
	mc.removeEntry(entry)
	mc.evictions++
}

// removeEntry removes an entry from the cache
func (mc *MemoryCache) removeEntry(entry *cacheEntry) {
	mc.lru.Remove(entry.element)
	delete(mc.items, entry.key)
}

// CleanupExpired removes expired entries (should be called periodically)
func (mc *MemoryCache) CleanupExpired() int {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	now := time.Now()
	removed := 0

	// Iterate through all entries
	for key, entry := range mc.items {
		if now.After(entry.expiresAt) {
			mc.lru.Remove(entry.element)
			delete(mc.items, key)
			removed++
		}
	}

	return removed
}
