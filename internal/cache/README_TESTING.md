# Cache Testing Guide

This package contains two types of Redis tests:

## 1. Unit Tests (Default - Skip if Redis Unavailable)

**Run:** `go test ./internal/cache/...`

These tests use the skip-if-unavailable pattern:
- If Redis is running on localhost:6379, tests run against DB 15
- If Redis is not available, tests are automatically skipped
- Fast for local development

**Starting local Redis for tests:**
```bash
# Option 1: Native Redis
redis-server

# Option 2: Docker
docker run -d -p 6379:6379 redis:7-alpine

# Run tests
go test ./internal/cache/...
```

## 2. Integration Tests (Testcontainers - Always Run)

**Run:** `go test -tags=integration ./internal/cache/...`

These tests use testcontainers:
- Automatically start Redis container
- Always run (no manual setup needed)
- Slower but more reliable
- **Requires Docker**

**Benefits:**
- ✅ No manual Redis setup
- ✅ Isolated test environment
- ✅ Guaranteed to run in CI
- ✅ Tests real Redis behavior

**Running both:**
```bash
# Unit tests only (fast)
go test ./internal/cache/...

# Integration tests only (slow, requires Docker)
go test -tags=integration ./internal/cache/...

# All tests (unit + integration)
go test -tags=integration ./internal/cache/...
```

## Redis Databases

Redis has 16 databases (0-15):
- DB 0: Default (production/development)
- DB 1-14: Available for staging/other uses
- DB 15: Used for unit tests (isolated)

**Why DB 15 for tests?**
- Isolates test data from production data
- Safe to `FLUSHDB` without affecting other data
- Common testing pattern

## CI/CD Recommendations

```yaml
# GitHub Actions example
- name: Start Redis
  run: docker run -d -p 6379:6379 redis:7-alpine

- name: Run unit tests
  run: go test ./internal/cache/...

- name: Run integration tests
  run: go test -tags=integration ./internal/cache/...
```

## Test Coverage

- **redis_test.go** - Unit tests (skip if no Redis)
  - Connection tests
  - CRUD operations
  - TTL expiration
  - Statistics
  - Concurrent access

- **redis_integration_test.go** - Integration tests (testcontainers)
  - Full lifecycle tests
  - Large dataset handling
  - Connection failure scenarios
  - Multi-tier cache with real Redis
  - Reconnection handling

- **multi_test.go** - Multi-tier cache tests
  - L1/L2 hit scenarios
  - Backfill behavior
  - Graceful degradation
  - Combined statistics
