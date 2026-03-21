package cache

import (
	"container/list"
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/halyph/page-analyzer/internal/domain"
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

	return &MemoryCache{
		maxSize: maxSize,
		ttl:     ttl,
		items:   make(map[string]*cacheEntry),
		lru:     list.New(),
	}
}

// GetHTML retrieves cached HTML analysis result
func (mc *MemoryCache) GetHTML(ctx context.Context, url string) (*domain.AnalysisResult, error) {
	key, err := GenerateHTMLKey(url)
	if err != nil {
		return nil, err
	}

	mc.mu.Lock()
	defer mc.mu.Unlock()

	entry, exists := mc.items[key]
	if !exists {
		mc.misses++
		return nil, ErrCacheMiss
	}

	// Check expiration
	if time.Now().After(entry.expiresAt) {
		// Expired - remove it
		mc.removeEntry(entry)
		mc.misses++
		return nil, ErrCacheMiss
	}

	// Move to front (most recently used)
	mc.lru.MoveToFront(entry.element)
	mc.hits++

	// Deserialize
	var result domain.AnalysisResult
	if err := json.Unmarshal(entry.value, &result); err != nil {
		return nil, err
	}

	result.CacheHit = true
	return &result, nil
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

	// Serialize
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}

	mc.mu.Lock()
	defer mc.mu.Unlock()

	// Check if already exists
	if entry, exists := mc.items[key]; exists {
		// Update existing entry
		entry.value = data
		entry.expiresAt = time.Now().Add(ttl)
		entry.size = int64(len(data))
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
		value:     data,
		expiresAt: time.Now().Add(ttl),
		size:      int64(len(data)),
	}

	entry.element = mc.lru.PushFront(entry)
	mc.items[key] = entry

	return nil
}

// GetLinkCheck retrieves cached link check result
func (mc *MemoryCache) GetLinkCheck(ctx context.Context, jobID string) (*domain.LinkCheckResult, error) {
	key := GenerateLinkCheckKey(jobID)

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

	mc.lru.MoveToFront(entry.element)
	mc.hits++

	var result domain.LinkCheckResult
	if err := json.Unmarshal(entry.value, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// SetLinkCheck stores link check result in cache
func (mc *MemoryCache) SetLinkCheck(ctx context.Context, jobID string, result *domain.LinkCheckResult, ttl time.Duration) error {
	key := GenerateLinkCheckKey(jobID)

	if ttl == 0 {
		ttl = 5 * time.Minute // Shorter TTL for link checks
	}

	data, err := json.Marshal(result)
	if err != nil {
		return err
	}

	mc.mu.Lock()
	defer mc.mu.Unlock()

	// Evict if at capacity
	if mc.lru.Len() >= mc.maxSize {
		mc.evictOldest()
	}

	entry := &cacheEntry{
		key:       key,
		value:     data,
		expiresAt: time.Now().Add(ttl),
		size:      int64(len(data)),
	}

	entry.element = mc.lru.PushFront(entry)
	mc.items[key] = entry

	return nil
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
