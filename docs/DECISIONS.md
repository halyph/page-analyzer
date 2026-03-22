# Design Decisions & Trade-offs

## What Was Required vs What I Built

| Required (from spec) | What I Built | Why? |
|---------------------|--------------|------|
| Web form with URL input | ✅ Web UI | Required |
| Show HTML version | ✅ Version detection | Required |
| Show page title | ✅ Title extraction | Required |
| Show heading counts | ✅ H1-H6 counting | Required |
| Show internal/external links | ✅ Link classification | Required |
| Show inaccessible links | ✅ Link checking | Required |
| Show login form detection | ✅ Password input detection | Required |
| Error handling with HTTP codes | ✅ Custom error messages | Required |
| | | |
| ❌ Not required | ✅ CLI interface | Testing, automation, CI/CD |
| ❌ Not required | ✅ REST API | Programmatic access, integration |
| ❌ Not required | ✅ Multi-layer caching | Performance, scalability |
| ❌ Not required | ✅ OpenTelemetry + Jaeger | Production observability |
| ❌ Not required | ✅ Prometheus + Grafana | Metrics and dashboards |
| ❌ Not required | ✅ Demo mode (`make demo`) | Easy evaluation |
| ❌ Not required | ✅ Production Makefile | Build automation |

**Note**: Additional features were added to explore production-ready patterns and demonstrate scalability considerations.

---

## Decision Records

### Decision 1: Two-Phase Analysis (Sync + Async)

**Context**: Pages can have 1000+ links. Checking all before responding causes timeouts.

**Decision**: Split into two phases:
- Phase 1: Immediate (HTML parsing, link discovery)
- Phase 2: Async (link accessibility checking with job ID)

**Consequences**:
- ✅ Fast initial response (<1s typically)
- ✅ No timeout issues
- ✅ Better UX (results appear immediately)
- ❌ Requires polling for complete results
- ❌ Job management complexity

**Alternatives Considered**:
- ❌ All sync: Too slow, timeouts
- ❌ All async: User waits for everything
- ✅ Hybrid: Best of both worlds

---

### Decision 2: Streaming HTML Parser

**Context**: Pages can be 10MB+. DOM parsing loads entire document into memory.

**Decision**: Use streaming tokenizer (`golang.org/x/net/html`) with O(1) memory.

**Consequences**:
- ✅ Constant memory regardless of page size
- ✅ Can handle very large pages
- ✅ Single-pass processing
- ❌ Cannot traverse DOM tree
- ❌ Limited to linear token processing

**Alternatives Considered**:
- ❌ Full DOM (goquery): Too much memory
- ❌ Regex parsing: Unreliable, doesn't handle malformed HTML
- ✅ Streaming: Best for large pages

---

### Decision 3: Worker Pool (Fixed Size)

**Context**: Need to check many links in parallel, but unlimited goroutines = resource exhaustion.

**Decision**: Fixed-size worker pool (default: 20 workers, configurable).

**Consequences**:
- ✅ Predictable resource usage
- ✅ Controlled parallelism
- ✅ Fast enough for most cases
- ❌ Not adaptive to system load
- ❌ Head-of-line blocking possible

**Configuration**: `ANALYZER_CHECK_WORKERS=50` for more parallelism.

---

### Decision 4: Collector Pattern

**Context**: Need to extract many different things from HTML (title, headings, links, forms).

**Decision**: Each collector implements interface, processes tokens independently.

```
Collectors:
  - VersionCollector (DOCTYPE)
  - TitleCollector (<title>)
  - HeadingsCollector (H1-H6)
  - LinksCollector (<a> href)
  - FormsCollector (password inputs)
```

**Consequences**:
- ✅ Single-pass through HTML (efficient)
- ✅ Easy to add new collectors
- ✅ Clean separation of concerns
- ✅ Testable independently

---

### Decision 5: Multi-Layer Caching

**Context**: Popular pages (go.dev, github.com) analyzed frequently.

**Decision**: Support multiple cache modes:

| Mode | Description | Use Case |
|------|-------------|----------|
| `memory` | LRU in-memory | Single instance, development |
| `redis` | Redis backend | Multi-instance, production |
| `multi` | LRU + Redis (L1/L2) | Best performance |
| `disabled` | No caching | Testing, always fresh |

**Consequences**:
- ✅ 10x faster for cached pages (<10ms vs 500ms)
- ✅ Reduces load on target sites
- ✅ Flexible for different deployments
- ❌ Cache invalidation complexity
- ❌ Stale data up to TTL (default 1h)

---

### Decision 6: Three Interfaces (CLI, REST, Web)

**Context**: Only Web UI required, but added CLI and REST API.

**Decision**: Three independent presentation layers, all using same analyzer core.

```
User → CLI Handler → Analyzer Core
User → Web Handler → Analyzer Core
User → REST Handler → Analyzer Core
```

**Why?**
- **CLI**: Fast testing during development, automation, CI/CD
- **REST API**: Programmatic access, integration with other systems
- **Web UI**: Required, user-friendly interface

**Consequences**:
- ✅ Flexible usage patterns
- ✅ Better testability (API easier to test than UI)
- ✅ Clean architecture (separation of concerns)
- ❌ More code to maintain
- ❌ More testing required

**Note**: Web UI does NOT call REST API. They're separate interfaces.

---

### Decision 7: OpenTelemetry for Observability

**Context**: Production systems need debugging capabilities. Not required for this test.

**Decision**: Full observability stack:
- OpenTelemetry SDK for tracing and metrics
- Jaeger for distributed tracing visualization
- Prometheus for metrics storage
- Grafana for dashboards

**Rationale**:
- Enables debugging performance issues in production
- Provides operational visibility
- Demonstrates vendor-neutral observability patterns
- Simplifies evaluation via demo mode (`make demo`)

**Consequences**:
- ✅ Can see exactly where time is spent (traces)
- ✅ Can monitor performance trends (metrics)
- ✅ Easy to debug issues
- ❌ Additional complexity
- ❌ Additional infrastructure (demo mode)

---

### Decision 8: Production-Quality Makefile

**Context**: Need build automation. Could have just used `go build`.

**Decision**: Comprehensive Makefile with:

| Feature | Why |
|---------|-----|
| Cross-platform builds | Linux, macOS, Intel, ARM64 |
| `tools/tools.go` | Dev tools as Go dependencies |
| Self-documenting | `make help` auto-generates |
| Color output | Better readability |
| Demo targets | Easy full-stack testing |

**About `tools/tools.go`**:
```go
// Declares tools as Go dependencies
import _ "github.com/golangci/golangci-lint/cmd/golangci-lint"
```

Benefits:
- Tools versioned in `go.mod` (not global installs)
- `make install` puts tools in `.bin/` (local, not global)
- Everyone gets same tool versions
- CI/CD uses exact same versions
- No "install this, then that" instructions

**Consequences**:
- ✅ Reproducible builds
- ✅ Consistent tool versions across environments
- ✅ Automated build process
- ✅ Simplified evaluation (`make demo`)

---

## Architecture Patterns Used

| Pattern | Where | Why |
|---------|-------|-----|
| **Hexagonal (Ports & Adapters)** | `internal/domain` core, `internal/presentation` adapters | Testable, swappable implementations |
| **Collector** | `internal/analyzer/collectors/` | Single-pass HTML processing, extensible |
| **Worker Pool** | `internal/analyzer/linkchecker.go` | Bounded concurrency |
| **Repository** | `internal/cache/` | Abstract cache backends |
| **Dependency Injection** | `cmd/analyzer/main.go` | Wire dependencies at startup |

---

## Technology Choices

| Technology | Why Chosen |
|------------|------------|
| **Go 1.25** | Required by spec, excellent concurrency |
| **golang.org/x/net/html** | Streaming tokenizer, handles malformed HTML |
| **urfave/cli/v2** | Popular CLI framework |
| **go-redis/redis/v9** | Standard Redis client |
| **OpenTelemetry** | Vendor-neutral observability |
| **Docker Compose** | Easy local infrastructure |

---

## Key Assumptions

| Assumption | Rationale | Impact |
|------------|-----------|--------|
| HTTP/HTTPS only | FTP, file:// out of scope | Non-HTTP URLs rejected |
| 10MB page limit | Prevents abuse | Large SPAs may be truncated |
| 5s timeout per link | Most servers respond in 1-2s | Slow servers marked inaccessible |
| Password input = login form | Common pattern | False positives possible (registration forms) |
| Same domain = internal | Simple, unambiguous | Subdomains treated as external |
| 1h cache TTL | Balance freshness vs performance | Updates not reflected for 1h |
| 20 workers default | Balance speed vs resources | Configurable via `CHECK_WORKERS` |

---

## What I Didn't Build

See [ISSUES.md](ISSUES.md) for critical security issues (SSRF protection, rate limiting) and other improvements needed before production.

---

## Summary

The implementation meets all core requirements and includes additional features (CLI, REST API, caching, observability) to explore production-ready patterns.

Key aspects:
1. Scalability: Caching and worker pools for performance
2. Observability: Tracing and metrics for operational visibility
3. Flexibility: Multiple interfaces for different use cases

Trade-offs: Additional complexity in exchange for features and maintainability through clean architecture.
