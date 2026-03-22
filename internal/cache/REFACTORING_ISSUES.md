# Cache Package Refactoring Issues

Date: 2026-03-22
Status: Identified and documented for fixing

## Critical Redundancy Issues

### Issue 1: Memory Cache Method Duplication (HIGH)
**Location**: `memory.go` lines 76-307
**Impact**: ~210 lines of duplicated logic

**Problem**:
- 3 nearly identical Get methods: `GetHTML()`, `GetLinkCheck()`, `GetCachedLink()`
- 3 nearly identical Set methods: `SetHTML()`, `SetLinkCheck()`, `SetCachedLink()`

**Pattern** (Get):
```go
1. Generate key
2. Lock
3. Check existence → miss
4. Check expiration → miss
5. Move to front (LRU)
6. Increment hits
7. Unmarshal to type T
8. Return
```

**Pattern** (Set):
```go
1. Generate key
2. Marshal data
3. Lock
4. Evict if needed
5. Create entry
6. Add to LRU
7. Store in map
```

**Solution**: Use Go generics to create `memoryGet[T]()` and `memorySet[T]()` helpers.

---

### Issue 2: Redis Cache Set Method Duplication (MEDIUM)
**Location**: `redis.go` lines 93-157
**Impact**: ~60 lines of duplicated logic

**Problem**: While Redis uses generics for Get (`redisGet[T]`), Set methods are duplicated:
- `SetHTML()`
- `SetLinkCheck()`
- `SetCachedLink()`

**Solution**: Create `redisSet[T]()` generic helper.

---

### Issue 3: Multi-Tier Cache Duplication (MEDIUM)
**Location**: `multi.go` lines 43-139
**Impact**: ~90 lines of similar logic

**Problem**: 3 nearly identical Get methods with backfill:
- `GetHTML()`
- `GetLinkCheck()`
- `GetCachedLink()`

**Pattern**:
```go
1. Try L1 → return if found
2. Try L2 → return error if not found
3. Backfill L1 in goroutine
4. Return result
```

**Solution**: Cannot easily use generics due to different backfill TTLs and Set method signatures. Accept as intentional duplication for clarity. Add `//nolint:dupl` to remaining methods.

---

## Design Issues

### Issue 4: Interface Bloat (HIGH)
**Location**: `cache.go` lines 19-53
**Current**: 11 methods in single `Cache` interface

**Problem**: Violates Interface Segregation Principle
- Forces all implementations to implement everything
- NoOpCache has to stub all methods
- Harder to mock/test

**Solution**: Split into focused interfaces:
```go
type HTMLCache interface {
    GetHTML(ctx context.Context, url string) (*domain.AnalysisResult, error)
    SetHTML(ctx context.Context, url string, result *domain.AnalysisResult, ttl time.Duration) error
}

type LinkCache interface {
    GetLinkCheck(ctx context.Context, jobID string) (*domain.LinkCheckResult, error)
    SetLinkCheck(ctx context.Context, jobID string, result *domain.LinkCheckResult, ttl time.Duration) error
    GetCachedLink(ctx context.Context, url string) (*domain.CachedLinkCheck, error)
    SetCachedLink(ctx context.Context, url string, result *domain.CachedLinkCheck, ttl time.Duration) error
}

type CacheOps interface {
    Delete(ctx context.Context, url string) error
    Clear(ctx context.Context) error
    Stats() CacheStats
    Health(ctx context.Context) error
}

type Cache interface {
    HTMLCache
    LinkCache
    CacheOps
    io.Closer
}
```

**Decision**: Keep single interface for now (breaking change). Revisit in v2.

---

### Issue 5: Dead Code (LOW)
**Location**: `cache.go` lines 65-90

**Problem**: `CacheConfig` and `DefaultCacheConfig()` defined but never used
- Only referenced in TASK_AND_PLAN.md
- Actual cache creation happens manually in `cmd_serve.go`

**Solution**: Remove dead code.

---

### Issue 6: Inconsistent Key Generation (MEDIUM)
**Location**: `keys.go`

**Problem**:
```go
GenerateHTMLKey(url) (string, error)   // Returns error
GenerateLinkCheckKey(jobID) string     // No error
GenerateCachedLinkKey(url) string      // No error
```

**Reason**: Only `GenerateHTMLKey` calls `NormalizeURL()` which can fail

**Solution**: Keep as-is. HTML keys need normalization (query params, etc). Job IDs and link keys are simple hashes.

---

### Issue 7: Hardcoded Magic Numbers (LOW)
**Location**: Multiple files

**Problem**: TTL defaults scattered:
- `memory.go:203`: `5 * time.Minute` for links
- `redis.go:119`: `5 * time.Minute` for link checks
- `redis.go:143`: `5 * time.Minute` for cached links

**Solution**: Create constants:
```go
const (
    DefaultHTMLCacheTTL     = 1 * time.Hour
    DefaultLinkCheckTTL     = 5 * time.Minute
    DefaultCachedLinkTTL    = 5 * time.Minute
)
```

---

### Issue 8: No Cache Factory (MEDIUM)
**Location**: N/A (missing)

**Problem**: Manual construction in `cmd_serve.go:97-129` with switch statement

**Solution**: Create factory function:
```go
func NewCache(cfg CacheConfig) (Cache, error)
```

**Decision**: Skip for now. Current approach is clear and used in one place only.

---

### Issue 9: Multi-Tier Not Extensible (LOW)
**Location**: `multi.go`

**Problem**: Hardcodes L1/L2, can't do L1 → L2 → L3

**Solution**: Accept limitation. Two tiers (memory + Redis) is sufficient for 99% of use cases.

---

## Implementation Plan

### Phase 1: Critical Redundancy (Must Fix)
1. ✅ Issue 1: Generics for MemoryCache (~150 line reduction)
2. ✅ Issue 2: Generics for Redis Set methods (~30 line reduction)
3. ✅ Issue 3: Add nolint to remaining Multi methods (accept duplication)

### Phase 2: Cleanup (Nice to Have)
4. ✅ Issue 5: Remove dead CacheConfig code
5. ✅ Issue 7: Add TTL constants

### Phase 3: Future (V2)
6. ⏸️ Issue 4: Split interfaces (breaking change - defer)
7. ⏸️ Issue 8: Cache factory (not needed yet)
8. ⏸️ Issue 6: Keep as-is (intentional design)
9. ⏸️ Issue 9: Keep as-is (sufficient)

---

## Expected Outcomes

### Code Reduction
- Memory: 451 → ~300 lines (-33%)
- Redis: 245 → ~215 lines (-12%)
- Total: ~200 line reduction
- Cleaner, more maintainable code

### Test Impact
- All existing tests should pass
- No new tests needed (behavior unchanged)

### Breaking Changes
- None in Phase 1 & 2
- Phase 3 would be breaking (interface split)

---

## Actual Implementation Results

**Date Completed**: 2026-03-22

### Phase 1 & 2: Completed ✅

**Changes Made**:

1. **MemoryCache Refactoring** (Issue 1)
   - Created `memoryGet[T any]()` generic helper for all Get operations
   - Created `memorySet[T any]()` generic helper for all Set operations
   - Refactored `GetHTML()`, `GetLinkCheck()`, `GetCachedLink()` to use generic helper
   - Refactored `SetHTML()`, `SetLinkCheck()`, `SetCachedLink()` to use generic helper
   - Result: 451 → 355 lines (**96 line reduction, -21%**)

2. **Redis Refactoring** (Issue 2)
   - Created `redisSet[T any]()` generic helper for all Set operations
   - Refactored `SetHTML()`, `SetLinkCheck()`, `SetCachedLink()` to use generic helper
   - Note: `redisGet[T]()` already existed from previous work
   - Result: 245 → 226 lines (**19 line reduction, -8%**)

3. **Multi-Tier Cache** (Issue 3)
   - All Get methods already have `//nolint:dupl` directives
   - No changes needed

4. **Dead Code Removal** (Issue 5)
   - Removed `CacheConfig` struct (lines 65-71)
   - Removed `CacheType` type and constants (lines 73-81)
   - Removed `DefaultCacheConfig()` function (lines 83-90)
   - Result: **26 lines removed from cache.go**

5. **TTL Constants** (Issue 7)
   - Added `DefaultHTMLCacheTTL = 1 * time.Hour`
   - Added `DefaultLinkCheckTTL = 5 * time.Minute`
   - Added `DefaultCachedLinkTTL = 5 * time.Minute`
   - Added `DefaultMultiBackfillTTL = 1 * time.Hour`
   - Replaced all magic number TTLs with constants in memory.go, redis.go, and multi.go

**Total Impact**:
- **~141 lines removed** from cache package
- **All tests passing** (no behavior changes)
- **No breaking changes** to public API
- Improved maintainability and readability

**Test Results**:
```
✓ internal/cache: 79.9% coverage
✓ All tests pass with race detection
✓ Linter clean
```

### Phase 3: Deferred for Future Version

The following improvements are deferred to avoid breaking changes:
- Issue 4: Interface segregation (would break existing code)
- Issue 8: Cache factory function (not needed for single usage site)
- Issues 6 & 9: Intentional design decisions, no changes needed
