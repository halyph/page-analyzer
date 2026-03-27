# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Page Analyzer is a high-performance web page analyzer written in Go 1.25 that extracts HTML metadata, analyzes structure, and verifies link accessibility. The project demonstrates production-ready patterns including multi-layer caching, observability with OpenTelemetry, and multiple interfaces (CLI, REST API, Web UI).

## Common Development Commands

```bash
# Build (creates binaries in build/<os>/<arch>/)
make build              # Local OS/arch
make build.linux        # Linux AMD64/ARM64
make build.darwin       # macOS Intel/Apple Silicon

# Testing
make test               # Unit tests + linting
make test-all           # Including integration tests (requires Docker)
make cover              # Coverage report (opens in browser)

# Run specific tests
go test ./internal/analyzer/...           # Package tests
go test -run TestName ./internal/...      # Single test

# Linting
make run-lint           # Run linter
make fix-lint           # Auto-fix issues

# Tools installation
make install            # Install dev tools to .bin/ (golangci-lint, etc.)
make ensure-tools       # Install only if needed

# Demo mode (full observability stack)
make demo               # Start infrastructure + app
make demo-infra         # Infrastructure only (Redis, Jaeger, Prometheus, Grafana)
make demo-run           # Run analyzer with OTEL enabled
make demo-down          # Stop and cleanup

# Clean
make clean              # Remove build artifacts
```

## Architecture Overview

### Hexagonal Architecture (Ports & Adapters)

The project follows hexagonal architecture with clear separation between domain logic and external concerns:

- **Domain Layer** (`internal/domain/`): Core business logic, interfaces, and models
  - `Analyzer` interface: Main service contract
  - `Collector` interface: Token processor contract
  - `Cache`, `Fetcher`, `Walker`, `LinkChecker`: Infrastructure contracts

- **Implementation Layer** (`internal/analyzer/`): Concrete implementations
  - `Service`: Main analyzer orchestrator
  - `Fetcher`: HTTP client with streaming support
  - `Walker`: Streaming HTML tokenizer (O(1) memory)
  - Link checker with worker pool pattern

- **Presentation Layer** (`internal/presentation/`): User interfaces
  - `cli/`: Command-line interface (urfave/cli/v2)
  - `rest/`: REST API handlers
  - `web/`: Web UI with templates

### Collector Pattern

Analysis uses a **single-pass streaming parser** that processes HTML tokens once:

1. `Walker` tokenizes HTML using `golang.org/x/net/html`
2. Each `Collector` processes tokens independently:
   - `VersionCollector`: Detects HTML version from DOCTYPE
   - `TitleCollector`: Extracts `<title>` content
   - `HeadingsCollector`: Counts H1-H6 tags
   - `LinksCollector`: Extracts `<a href>` links
   - `LoginFormCollector`: Detects password inputs
3. After tokenization, each collector applies results to `AnalysisResult`

**Key Benefit**: Single pass through HTML, O(1) memory regardless of page size, easy to add new collectors.

See `internal/analyzer/collectors/registry.go` for collector registration.

### Link Checking Architecture

Link checking supports multiple modes:

- **async**: Submit job, poll for results (default, non-blocking)
- **sync**: Wait for all checks before returning (CLI mode)
- **hybrid**: Check first N links sync, rest async
- **disabled**: Skip link checking

Implementation uses **fixed-size worker pool** (default 20 workers) to bound resource usage. Pool configuration: `ANALYZER_CHECK_WORKERS`, `ANALYZER_QUEUE_SIZE`.

**CRITICAL**: Async jobs use background context, not request context. Request context gets cancelled after HTTP response, causing link checks to fail. See `internal/analyzer/linkchecker_pool.go`.

### Caching Strategy

Multi-layer caching with configurable modes:

- **memory**: LRU cache (single instance, development)
- **redis**: Redis backend (multi-instance, production)
- **multi**: L1 (LRU) + L2 (Redis) for best performance
- **disabled**: No caching (testing)

Separate cache keys for HTML analysis and link check results. HTML cache includes TTL, excludes link checks to avoid stale accessibility data.

Cache keys: `page:sha256(url)` for HTML, `linkcheck:sha256(url)` for link results.

## Configuration

All configuration via environment variables (see `internal/config/config.go`):

```bash
# Server
ANALYZER_ADDR=:8080                          # Server address
ANALYZER_READ_TIMEOUT=30s
ANALYZER_WRITE_TIMEOUT=30s

# Link Checking
ANALYZER_CHECK_MODE=async                    # sync|async|hybrid|disabled
ANALYZER_CHECK_WORKERS=20                    # Worker pool size
ANALYZER_CHECK_TIMEOUT=5s                    # Per-link timeout
ANALYZER_MAX_LINKS=10000                     # Max links to check

# Caching
ANALYZER_CACHE_MODE=memory                   # memory|redis|multi|disabled
ANALYZER_PAGE_CACHE_TTL=1h                   # HTML analysis TTL
ANALYZER_LINK_CACHE_TTL=5m                   # Link check TTL
ANALYZER_REDIS_ADDR=redis://localhost:6379/0
ANALYZER_MEMORY_CACHE_SIZE=100               # LRU cache size

# Fetching
ANALYZER_FETCH_TIMEOUT=15s
ANALYZER_MAX_BODY_SIZE=10485760              # 10MB limit

# Observability
ANALYZER_TRACING_ENABLED=false               # OpenTelemetry tracing
ANALYZER_OTEL_ENDPOINT=localhost:4318        # OTLP HTTP endpoint
ANALYZER_METRICS_ENABLED=true                # Prometheus metrics
ANALYZER_LOG_LEVEL=info                      # debug|info|warn|error
```

## Key Design Decisions

### Streaming Parser (Not DOM)

Uses `golang.org/x/net/html` tokenizer instead of full DOM parser (like goquery):
- **Pro**: O(1) memory, handles huge pages, single-pass processing
- **Con**: Cannot traverse DOM tree, cannot execute JavaScript (SPAs show incomplete results)

### Async Link Checking

Pages can have 1000+ links. Checking all synchronously causes timeouts. Solution: two-phase analysis:
1. **Phase 1** (immediate): HTML parsing, link discovery
2. **Phase 2** (async): Link accessibility checking with job polling

### Worker Pool Pattern

Fixed-size worker pool prevents resource exhaustion from unlimited goroutines. Trade-off: not adaptive to load, possible head-of-line blocking.

### Three Interfaces

CLI, REST API, and Web UI all use the same analyzer core but provide different user experiences:
- **CLI**: Fast testing, automation, CI/CD
- **REST API**: Programmatic access, integration
- **Web UI**: User-friendly interface (does NOT call REST API internally)

## Known Issues & Limitations

### Critical (Must Fix for Production)

1. **SSRF Protection**: No validation of private IPs (10.x, 192.168.x, 127.0.0.1). Can be used to scan internal networks.
2. **Rate Limiting**: No protection against abuse. Easy to DoS.
3. **Authentication**: API is completely open.
4. **Cost**: High bandwidth/compute usage at scale. Need quotas and monitoring.

### Observability Issues

**FIXED**: Async job trace disconnection - uses background context now. Jobs appear as separate traces, correlated by `link_checker.job_id` attribute.

**Gaps**:
- Metrics defined but not called (`RecordCacheHit/Miss`, `RecordLinksChecked`, etc.)
- Missing spans for individual collectors and Redis operations
- No error tracking metrics
- Dashboard incomplete (missing cache breakdown, error panels, percentiles)

### Architecture Limitations

- **Max page size**: 10MB (configurable, prevents memory exhaustion)
- **No JavaScript execution**: SPAs (React, Vue) show incomplete results
- **Subdomains treated as external**: `blog.example.com` → `example.com` is external
- **No authentication support**: Cannot analyze pages behind login
- **Link checker false positives**: Some sites block automated tools (Medium, StackOverflow)

## Testing Strategy

- **Unit tests**: High coverage (86-93%), see `*_test.go` files
- **Integration tests**: Redis cache tests use testcontainers, require Docker. Run with `make test-all`.
- **Tag-based exclusion**: Integration tests use `//go:build integration` tag, skipped by default

Run integration tests: `go test -tags=integration ./internal/cache/`

## Project Structure

```
cmd/analyzer/           # CLI entry point, command definitions
internal/
  ├── domain/           # Core interfaces, models, business logic (Analyzer, Collector, etc.)
  ├── analyzer/         # Service implementation, fetcher, walker, link checker
  │   └── collectors/   # HTML collectors (version, title, headings, links, login form)
  ├── cache/            # Cache implementations (memory, redis, multi, noop)
  ├── config/           # Configuration loading and constants
  ├── server/           # HTTP server, router, middleware
  ├── presentation/     # User interfaces (cli, rest, web)
  └── observability/    # OpenTelemetry tracing, metrics, logging
deployments/            # Grafana dashboards, Prometheus config
docs/                   # Design decisions, known issues, diagrams
tools/                  # Dev tool dependencies (tools.go pattern)
```

## Development Notes

- **Tools versioning**: Dev tools (golangci-lint, etc.) are Go dependencies in `tools/tools.go`, installed to `.bin/` via `make install`. Everyone gets same versions.
- **Makefile**: Self-documenting (`make help`), supports cross-platform builds, includes demo targets.
- **Git hooks**: Pre-commit hooks may be configured; check `.git/hooks/`.
- **Context cancellation**: Be careful with async work - use background context, not request context.
- **Collector extension**: To add new collectors, implement `Collector` interface and register in `collectors/registry.go`.

## Dependencies

Key external dependencies:
- `golang.org/x/net/html`: Streaming HTML tokenizer
- `urfave/cli/v2`: CLI framework
- `go-chi/chi/v5`: HTTP router
- `redis/go-redis/v9`: Redis client
- `go.opentelemetry.io/otel`: OpenTelemetry SDK
- `testcontainers-go`: Integration testing with Docker
