# Web Page Analyzer — Production-Ready Implementation Plan

**Version:** 2.0
**Last Updated:** 2026-03-21
**Target:** High-load production service with extensibility

---

## Table of Contents
1. [Original Requirements](#original-requirements)
2. [Architecture Principles](#architecture-principles)
3. [Two-Phase Analysis Model](#two-phase-analysis-model)
4. [System Architecture](#system-architecture)
5. [Domain Model](#domain-model)
6. [Collector System](#collector-system)
7. [Link Checking Architecture](#link-checking-architecture)
8. [Caching Strategy](#caching-strategy)
9. [Configuration](#configuration)
10. [API Specification](#api-specification)
11. [Observability](#observability)
12. [Testing Strategy](#testing-strategy)
13. [Deployment](#deployment)
14. [Assumptions & Decisions](#assumptions--decisions)
15. [Future Improvements](#future-improvements)

---

## Original Requirements

### Objective

Build a web application that analyzes a webpage/URL:
- Present a form with a text input for the URL to analyze
- Include a submit button that sends a request to the server
- Display the analysis results after processing

### Required Analysis Results

| # | Analysis Item |
|---|--------------|
| 1 | HTML version of the document |
| 2 | Page title |
| 3 | Heading counts by level (H1–H6) |
| 4 | Internal vs. external link counts; count of inaccessible links |
| 5 | Whether the page contains a login form |

### Error Handling

If the given URL is unreachable, present an error message containing:
- The HTTP status code
- A useful, human-readable error description

### Technical Constraints

- Written in **Go 1.25**
- Must be under **git control**Option
- Any libraries / tools / AI assistance are permitted

### Deliverables

- Git repository
- Build/deploy documentation
- Assumptions and decisions documentation
- Suggestions for improvements

---

## Architecture Principles

### 1. High-Load & Memory Efficiency

**Goal:** Support 1000+ concurrent requests without memory bloat

**Strategies:**
- **Streaming HTML parsing** — O(1) memory, never buffer full document
- **Bounded resource limits** — cap body size (10MB), link count (10k)
- **Decoupled processing** — async link checking prevents request blocking
- **Connection pooling** — reuse HTTP connections efficiently

**Memory Profile:**
```
Synchronous (naive):     1000 requests × 5MB pages = 5GB RAM
Streaming (optimized):   1000 requests × ~50KB state = ~50MB RAM
```

### 2. Extensibility

**Goal:** Add new analysis types without modifying existing code

**Mechanisms:**
- **Collector Registry Pattern** — static registration at compile-time
- **Single-pass token stream** — new collectors automatically integrated
- **Versioned API responses** — backward compatibility via version field
- **Extension point** — `extra` field in result for future data

**Adding a new analysis type:**
```go
// 1. Create collector
type OpenGraphCollector struct { ... }

// 2. Register in init()
func init() {
    Registry.Register("opengraph", &OpenGraphFactory{})
}

// 3. Done! No changes to walker, service, or handlers
```

### 3. Modularity & Testability

**Goal:** Test components in isolation with predictable behavior

**Strategies:**
- **Interface-based design** — mock fetchers, checkers, caches
- **Table-driven tests** — parameterized fixtures for collectors
- **Property-based tests** — verify invariants (internal + external = total)
- **Hermetic integration tests** — httptest servers, no external deps

### 4. Production Readiness

**Requirements:**
- Graceful degradation under load
- Observability (metrics, traces, logs)
- Rate limiting (prevent abuse)
- Caching (reduce redundant work)
- Health checks (k8s probes)

---

## Two-Phase Analysis Model

**Critical Design Decision:** Decouple HTML parsing from link checking

### Phase 1: HTML Analysis (Fast Path)
**Target Latency:** <500ms
**Operations:**
1. Fetch URL with timeout (15s)
2. Stream HTML tokens through collectors (single pass)
3. Collect all links (bounded: max 10k)
4. Return immediate results with link counts
5. Submit link checking job to worker pool

**Memory:** O(1) for HTML parsing + O(n) for link collection (bounded)

### Phase 2: Link Checking (Slow Path)
**Target Latency:** 100ms per link × concurrency
**Operations:**
1. Worker pool picks up job from queue
2. HEAD requests to verify link accessibility
3. Concurrent checks (20 workers)
4. Results stored in Redis (shared across users)
5. Poll via JobID for completion

**Benefits:**
- User sees results immediately (title, headings, login form, link counts)
- Link checking happens asynchronously
- Can disable under high load without breaking service
- Link check results cached separately (reusable across users)

### Request Flow Diagram

```
User Request
    │
    ▼
┌─────────────────────────┐
│ HTTP Handler            │
│ - Validate URL          │
│ - Check L1 cache (LRU)  │
└───────┬─────────────────┘
        │ cache miss
        ▼
┌─────────────────────────┐
│ Check L2 cache (Redis)  │
│ html:{url_hash}         │
└───────┬─────────────────┘
        │ cache miss
        ▼
┌─────────────────────────┐
│ Fetcher                 │
│ - HTTP GET with timeout │
│ - MaxBodySize limit     │
└───────┬─────────────────┘
        │
        ▼
┌─────────────────────────┐
│ Walker + Collectors     │
│ - Single-pass streaming │
│ - All collectors run    │
└───────┬─────────────────┘
        │
        ▼
┌─────────────────────────┐
│ AnalysisResult          │
│ - HTML, title, headings │
│ - Link arrays collected │
│ - JobID for link check  │
└───────┬─────────────────┘
        │
        ├─> Cache in Redis (html:{hash})
        ├─> Submit LinkCheckJob to queue
        └─> Return to user (200 OK)

Background:
┌─────────────────────────┐
│ Worker Pool (20)        │
│ - Process link jobs     │
│ - Concurrent HEAD reqs  │
└───────┬─────────────────┘
        │
        ▼
┌─────────────────────────┐
│ Redis: links:{hash}     │
│ - Cache results (TTL 5m)│
└─────────────────────────┘
```

---

## System Architecture

### Core Principle

**One binary, three modes** — same executable, behavior driven by subcommand

```bash
# HTTP server: REST API + HTML UI
./analyzer serve
./analyzer serve --addr :9090 --redis redis://localhost:6379

# CLI one-shot analysis
./analyzer analyze https://example.com
./analyzer analyze https://example.com --json --check-links

# Health check (for k8s probes)
./analyzer healthcheck --addr :8080
```

### Component Layers

```
┌────────────────────────────────────────────────────────────────┐
│                         cmd/main.go                             │
│    Cobra CLI: "serve" | "analyze" | "healthcheck"              │
│    Composition root — wires all dependencies                    │
│    - Builds analyzer service                                    │
│    - Configures cache (LRU or Redis)                            │
│    - Starts worker pool                                         │
│    - Sets up observability                                      │
└──────────────────────────┬─────────────────────────────────────┘
                           │
┌──────────────────────────▼─────────────────────────────────────┐
│                    internal/server/                             │
│    Infrastructure layer — HTTP server lifecycle                 │
│    - chi router + middleware                                    │
│    - Rate limiting (per-IP)                                     │
│    - Request logging                                            │
│    - Panic recovery                                             │
│    - OTEL tracing                                               │
│    - Health check endpoint                                      │
└──────────────────────────┬─────────────────────────────────────┘
                           │
┌──────────────────────────▼─────────────────────────────────────┐
│                 internal/presentation/                          │
│                                                                 │
│  ┌───────────────┐  ┌──────────────┐  ┌────────────────────┐  │
│  │    cli/       │  │    rest/     │  │       web/         │  │
│  │  - Formatter  │  │  - Handlers  │  │  - Form handler    │  │
│  │  - Text table │  │  - JSON I/O  │  │  - Templates       │  │
│  │  - JSON dump  │  │  - /analyze  │  │  - SSE updates     │  │
│  │               │  │  - /jobs/:id │  │  - index.html      │  │
│  │               │  │  - /health   │  │  - result.html     │  │
│  └───────────────┘  └──────────────┘  └────────────────────┘  │
└──────────────────────────┬─────────────────────────────────────┘
                           │ domain.Analyzer interface
┌──────────────────────────▼─────────────────────────────────────┐
│                    internal/analyzer/                           │
│                                                                 │
│  service.go         — orchestrates: fetch → walk → submit job  │
│  fetcher.go         — HTTP client, redirect handling           │
│  walker.go          — HTML token stream → collectors           │
│  linkchecker.go     — worker pool + job queue                  │
│                                                                 │
│  collectors/                                                    │
│    registry.go      — static registration, metadata            │
│    htmlversion.go   — DOCTYPE token → version string           │
│    title.go         — <title> extraction                       │
│    headings.go      — H1-H6 counting                           │
│    links.go         — <a href> collection + classification     │
│    loginform.go     — <form> + <input type="password">         │
│    (future: opengraph, meta, images, scripts)                  │
└──────────────────────────┬─────────────────────────────────────┘
                           │
┌──────────────────────────▼─────────────────────────────────────┐
│                     internal/cache/                             │
│                                                                 │
│  interface.go       — Cache interface                           │
│  memory.go          — LRU cache (for CLI mode)                 │
│  redis.go           — Redis client (for service mode)          │
│  multi.go           — L1 (memory) + L2 (redis) cascade         │
└──────────────────────────┬─────────────────────────────────────┘
                           │
┌──────────────────────────▼─────────────────────────────────────┐
│                     internal/domain/                            │
│                                                                 │
│  Pure Go — zero external dependencies                           │
│                                                                 │
│  Types:                                                         │
│    - AnalysisRequest, AnalysisResult                            │
│    - HeadingCounts, LinkAnalysis, LinkCheckResult              │
│    - AnalysisError, LinkError                                  │
│                                                                 │
│  Interfaces:                                                    │
│    - Analyzer                                                   │
│    - Collector, CollectorFactory                                │
│    - Cache                                                      │
│    - LinkChecker                                                │
└─────────────────────────────────────────────────────────────────┘
```

---

### Directory Structure

```
page-analyzer/
├── cmd/
│   ├── main.go                       # cobra CLI entry point
│   ├── serve.go                      # serve subcommand
│   ├── analyze.go                    # analyze subcommand
│   └── healthcheck.go                # healthcheck subcommand
│
├── internal/
│   ├── domain/
│   │   ├── analysis.go               # AnalysisRequest, AnalysisResult
│   │   ├── links.go                  # LinkAnalysis, LinkCheckResult, LinkError
│   │   ├── errors.go                 # AnalysisError with status codes
│   │   └── interfaces.go             # Analyzer, Collector, Cache, LinkChecker
│   │
│   ├── analyzer/
│   │   ├── service.go                # orchestrates fetch → walk → submit job
│   │   ├── fetcher.go                # HTTP client, timeout, size limits
│   │   ├── walker.go                 # HTML tokenizer → collectors
│   │   ├── linkchecker.go            # worker pool + job queue
│   │   ├── workerpool.go             # generic worker pool implementation
│   │   └── collectors/
│   │       ├── registry.go           # static registration + metadata
│   │       ├── htmlversion.go        # DOCTYPE → version
│   │       ├── title.go              # <title> extractor
│   │       ├── headings.go           # H1-H6 counter
│   │       ├── links.go              # <a href> collector + classifier
│   │       └── loginform.go          # <form> + password detector
│   │
│   ├── cache/
│   │   ├── cache.go                  # Cache interface
│   │   ├── memory.go                 # LRU implementation (CLI mode)
│   │   ├── redis.go                  # Redis client (service mode)
│   │   ├── multi.go                  # L1 + L2 cascade
│   │   └── keys.go                   # cache key generation
│   │
│   ├── presentation/
│   │   ├── cli/
│   │   │   ├── handler.go            # CLI command handler
│   │   │   ├── formatter.go          # text table formatter
│   │   │   └── json.go               # JSON output formatter
│   │   ├── rest/
│   │   │   ├── handler.go            # HTTP handlers
│   │   │   ├── analyze.go            # POST /api/analyze
│   │   │   ├── jobs.go               # GET /api/jobs/:id
│   │   │   ├── health.go             # GET /api/health
│   │   │   └── dto.go                # request/response DTOs
│   │   └── web/
│   │       ├── handler.go            # web form handlers
│   │       ├── templates/
│   │       │   ├── base.html         # base template
│   │       │   ├── index.html        # home page + form
│   │       │   └── result.html       # results display
│   │       └── static/
│   │           ├── style.css         # minimal CSS
│   │           └── app.js            # polling logic
│   │
│   ├── server/
│   │   ├── server.go                 # HTTP server setup
│   │   ├── middleware.go             # logging, recovery, tracing
│   │   ├── ratelimit.go              # per-IP rate limiter
│   │   └── router.go                 # routes + mounts
│   │
│   ├── config/
│   │   ├── config.go                 # configuration struct
│   │   ├── env.go                    # environment variable loading
│   │   └── defaults.go               # default values
│   │
│   └── observability/
│       ├── logger.go                 # slog setup
│       ├── metrics.go                # Prometheus metrics
│       ├── tracing.go                # OTEL tracing setup
│       └── health.go                 # health check logic
│
├── deployments/
│   ├── tempo.yaml                    # Tempo configuration
│   ├── prometheus.yml                # Prometheus scrape config
│   └── grafana/
│       ├── datasources/
│       │   └── datasources.yml       # Prometheus + Tempo
│       └── dashboards/
│           ├── dashboard.yml         # Dashboard config
│           └── analyzer.json         # Main dashboard (created later)
│
├── scripts/
│   ├── build.sh                      # build binary
│   ├── test.sh                       # run all tests
│   └── dev.sh                        # start local dev environment
│
├── test/
│   ├── fixtures/                     # HTML test fixtures
│   │   ├── simple.html
│   │   ├── complex.html
│   │   └── malformed.html
│   └── integration/                  # integration tests
│       ├── analyzer_test.go
│       └── api_test.go
│
├── go.mod
├── go.sum
├── Makefile
├── Dockerfile
├── .dockerignore
├── .gitignore
├── README.md                         # build/deploy instructions
├── DECISIONS.md                      # architecture decisions
└── IMPROVEMENTS.md                   # future enhancements
```

---

---

## Domain Model

### Core Types

```go
// internal/domain/analysis.go

type AnalysisRequest struct {
    URL     string
    Options AnalysisOptions
}

type AnalysisOptions struct {
    CheckLinks     LinkCheckMode  // "sync", "async", "hybrid", "disabled"
    MaxLinks       int            // Upper bound (default: 10000)
    SyncCheckLimit int            // For hybrid mode (default: 10)
    Timeout        time.Duration  // Overall timeout (default: 30s)
}

type LinkCheckMode string

const (
    LinkCheckSync     LinkCheckMode = "sync"
    LinkCheckAsync    LinkCheckMode = "async"
    LinkCheckHybrid   LinkCheckMode = "hybrid"
    LinkCheckDisabled LinkCheckMode = "disabled"
)

type HeadingCounts struct {
    H1, H2, H3, H4, H5, H6 int
}

type AnalysisResult struct {
    Version      string        `json:"version"`           // API version: "v1"
    URL          string        `json:"url"`
    HTMLVersion  string        `json:"htmlVersion"`
    Title        string        `json:"title"`
    Headings     HeadingCounts `json:"headings"`
    Links        LinkAnalysis  `json:"links"`
    HasLoginForm bool          `json:"hasLoginForm"`
    AnalyzedAt   time.Time     `json:"analyzedAt"`
    CacheHit     bool          `json:"cacheHit"`

    // Extension point for future collectors
    Extra        map[string]json.RawMessage `json:"extra,omitempty"`
}

type AnalysisError struct {
    StatusCode  int
    Description string
    Cause       error  // Wrapped error
}

func (e *AnalysisError) Error() string {
    if e.Cause != nil {
        return fmt.Sprintf("HTTP %d: %s: %v", e.StatusCode, e.Description, e.Cause)
    }
    return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Description)
}

func (e *AnalysisError) Unwrap() error {
    return e.Cause
}
```

### Link Analysis Types

```go
// internal/domain/links.go

type LinkAnalysis struct {
    Internal    []string        `json:"internal"`           // Collected internal URLs
    External    []string        `json:"external"`           // Collected external URLs
    TotalFound  int             `json:"totalFound"`         // Total discovered (may exceed max)
    Truncated   bool            `json:"truncated"`          // Hit MaxLinks limit

    // Link checking (async)
    CheckJobID  string          `json:"checkJobID,omitempty"`
    CheckStatus LinkCheckStatus `json:"checkStatus"`
    CheckResult *LinkCheckResult `json:"checkResult,omitempty"`
}

type LinkCheckStatus string

const (
    LinkCheckPending    LinkCheckStatus = "pending"
    LinkCheckInProgress LinkCheckStatus = "in_progress"
    LinkCheckCompleted  LinkCheckStatus = "completed"
    LinkCheckFailed     LinkCheckStatus = "failed"
)

type LinkCheckResult struct {
    Checked      int         `json:"checked"`
    Accessible   int         `json:"accessible"`
    Inaccessible []LinkError `json:"inaccessible"`
    Duration     string      `json:"duration"`  // "2.5s"
    CompletedAt  time.Time   `json:"completedAt"`
}

type LinkError struct {
    URL        string `json:"url"`
    StatusCode int    `json:"statusCode,omitempty"`
    Reason     string `json:"reason"`  // "timeout", "404", "connection_refused", "invalid_url"
}

// LinkCheckJob represents async link checking work
type LinkCheckJob struct {
    ID          string
    URLs        []string
    BaseURL     string  // For context
    Result      *LinkCheckResult
    Status      LinkCheckStatus
    CreatedAt   time.Time
    StartedAt   *time.Time
    CompletedAt *time.Time
}
```

### Interfaces

```go
// internal/domain/interfaces.go

// Analyzer is the main service interface
type Analyzer interface {
    Analyze(ctx context.Context, req AnalysisRequest) (*AnalysisResult, error)
}

// Collector processes HTML tokens in a single pass
// Each collector is responsible for one analysis concern
type Collector interface {
    // Collect processes a single HTML token
    Collect(token html.Token)

    // Apply writes collected data into AnalysisResult
    // Called after all tokens have been processed
    Apply(result *AnalysisResult)
}

// CollectorFactory creates configured collectors
type CollectorFactory interface {
    Create(config CollectorConfig) (Collector, error)
    Metadata() CollectorMetadata
}

type CollectorMetadata struct {
    Name        string
    Description string
    Version     string
    Required    bool  // Core vs optional
}

type CollectorConfig struct {
    BaseURL  string
    MaxItems int
    Params   map[string]interface{}
}

// LinkChecker verifies link accessibility
type LinkChecker interface {
    // Submit enqueues a link check job and returns job ID
    Submit(ctx context.Context, urls []string, baseURL string) string

    // GetJob retrieves job status and results
    GetJob(jobID string) (*LinkCheckJob, bool)

    // CheckSync synchronously checks links (for CLI mode)
    CheckSync(ctx context.Context, urls []string) (*LinkCheckResult, error)
}

// Cache stores analysis results with TTL
type Cache interface {
    // GetHTML retrieves cached HTML analysis
    GetHTML(ctx context.Context, url string) (*AnalysisResult, error)

    // SetHTML stores HTML analysis (excludes link check results)
    SetHTML(ctx context.Context, url string, result *AnalysisResult, ttl time.Duration) error

    // GetLinkCheck retrieves cached link check results
    GetLinkCheck(ctx context.Context, url string) (*LinkCheckResult, error)

    // SetLinkCheck stores link check results (shared across users)
    SetLinkCheck(ctx context.Context, url string, result *LinkCheckResult, ttl time.Duration) error

    // Delete removes all cached data for a URL
    Delete(ctx context.Context, url string) error

    // Health checks cache connectivity
    Health(ctx context.Context) error
}
```

---

## Collector System

### Design Philosophy

**Single-pass streaming:** HTML tokenizer feeds all collectors simultaneously
**No central coordinator:** Each collector owns its domain
**Zero coupling:** Adding collectors requires zero edits to existing code

### Registry Pattern (Static Compile-Time)

```go
// internal/analyzer/collectors/registry.go

var DefaultRegistry = NewRegistry()

type Registry struct {
    factories map[string]CollectorFactory
    mu        sync.RWMutex
}

func (r *Registry) Register(name string, factory CollectorFactory) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.factories[name] = factory
}

func (r *Registry) Create(name string, config domain.CollectorConfig) (domain.Collector, error) {
    r.mu.RLock()
    factory, ok := r.factories[name]
    r.mu.RUnlock()

    if !ok {
        return nil, fmt.Errorf("unknown collector: %s", name)
    }

    return factory.Create(config)
}

func (r *Registry) List() []domain.CollectorMetadata {
    r.mu.RLock()
    defer r.mu.RUnlock()

    var metadata []domain.CollectorMetadata
    for _, factory := range r.factories {
        metadata = append(metadata, factory.Metadata())
    }
    return metadata
}

// Auto-registration via init()
func init() {
    DefaultRegistry.Register("htmlversion", &HTMLVersionFactory{})
    DefaultRegistry.Register("title", &TitleFactory{})
    DefaultRegistry.Register("headings", &HeadingsFactory{})
    DefaultRegistry.Register("links", &LinksFactory{})
    DefaultRegistry.Register("loginform", &LoginFormFactory{})
}
```

### Walker (Token Pump)

```go
// internal/analyzer/walker.go

type Walker struct {
    maxTokens int  // Safety: stop after N tokens (default: 1M)
}

func (w *Walker) Walk(r io.Reader, collectors []domain.Collector, result *domain.AnalysisResult) error {
    z := html.NewTokenizer(r)
    tokenCount := 0

    for {
        tt := z.Next()
        tokenCount++

        // Safety: prevent infinite loops on malformed HTML
        if tokenCount > w.maxTokens {
            return fmt.Errorf("exceeded max tokens: %d", w.maxTokens)
        }

        if tt == html.ErrorToken {
            err := z.Err()
            if err == io.EOF {
                break  // Normal completion
            }
            return fmt.Errorf("tokenization error: %w", err)
        }

        token := z.Token()

        // Feed token to all collectors
        for _, c := range collectors {
            c.Collect(token)
        }
    }

    // Finalize: collectors write results
    for _, c := range collectors {
        c.Apply(result)
    }

    return nil
}
```

### Example Collector: Headings

```go
// internal/analyzer/collectors/headings.go

type HeadingsCollector struct {
    counts   domain.HeadingCounts
    inHeader bool
}

func (c *HeadingsCollector) Collect(token html.Token) {
    switch token.Type {
    case html.StartTagToken:
        switch token.Data {
        case "h1":
            c.counts.H1++
        case "h2":
            c.counts.H2++
        case "h3":
            c.counts.H3++
        case "h4":
            c.counts.H4++
        case "h5":
            c.counts.H5++
        case "h6":
            c.counts.H6++
        }
    }
}

func (c *HeadingsCollector) Apply(result *domain.AnalysisResult) {
    result.Headings = c.counts
}

// Factory
type HeadingsFactory struct{}

func (f *HeadingsFactory) Create(config domain.CollectorConfig) (domain.Collector, error) {
    return &HeadingsCollector{}, nil
}

func (f *HeadingsFactory) Metadata() domain.CollectorMetadata {
    return domain.CollectorMetadata{
        Name:        "headings",
        Description: "Counts H1-H6 heading tags",
        Version:     "1.0.0",
        Required:    true,
    }
}
```

### Example Collector: Links (Bounded)

```go
// internal/analyzer/collectors/links.go

type LinksCollector struct {
    baseURL   *url.URL
    maxLinks  int
    internal  []string
    external  []string
    seen      map[string]bool  // Deduplication
    truncated bool
}

func NewLinksCollector(baseURL string, maxLinks int) (*LinksCollector, error) {
    u, err := url.Parse(baseURL)
    if err != nil {
        return nil, err
    }

    return &LinksCollector{
        baseURL:  u,
        maxLinks: maxLinks,
        seen:     make(map[string]bool),
    }, nil
}

func (c *LinksCollector) Collect(token html.Token) {
    if token.Type != html.StartTagToken || token.Data != "a" {
        return
    }

    href := extractAttr(token.Attr, "href")
    if href == "" {
        return
    }

    // Resolve relative URLs
    absolute, err := c.baseURL.Parse(href)
    if err != nil {
        return
    }

    normalized := absolute.String()

    // Deduplicate
    if c.seen[normalized] {
        return
    }

    // Check bounds
    total := len(c.internal) + len(c.external)
    if total >= c.maxLinks {
        c.truncated = true
        return
    }

    c.seen[normalized] = true

    // Classify: internal vs external
    if isSameOrigin(c.baseURL, absolute) {
        c.internal = append(c.internal, normalized)
    } else {
        c.external = append(c.external, normalized)
    }
}

func (c *LinksCollector) Apply(result *domain.AnalysisResult) {
    result.Links.Internal = c.internal
    result.Links.External = c.external
    result.Links.TotalFound = len(c.seen)
    result.Links.Truncated = c.truncated
}

func isSameOrigin(base, target *url.URL) bool {
    return base.Scheme == target.Scheme && base.Host == target.Host
}

func extractAttr(attrs []html.Attribute, key string) string {
    for _, attr := range attrs {
        if attr.Key == key {
            return attr.Val
        }
    }
    return ""
}
```

### Adding New Collectors (Extensibility Example)

```go
// Future: internal/analyzer/collectors/opengraph.go

type OpenGraphCollector struct {
    data map[string]string
}

func (c *OpenGraphCollector) Collect(token html.Token) {
    if token.Type == html.StartTagToken && token.Data == "meta" {
        property := extractAttr(token.Attr, "property")
        if strings.HasPrefix(property, "og:") {
            content := extractAttr(token.Attr, "content")
            c.data[property] = content
        }
    }
}

func (c *OpenGraphCollector) Apply(result *domain.AnalysisResult) {
    // Store in extension field
    data, _ := json.Marshal(c.data)
    result.Extra["opengraph"] = data
}

// Register in init()
func init() {
    DefaultRegistry.Register("opengraph", &OpenGraphFactory{})
}
```

**Extensibility achieved:** Zero changes to walker, service, or existing collectors.

---

## Link Checking Architecture

### Problem Statement

**Challenge:** Checking 10k links can take hours even with concurrency
**Solution:** Async worker pool with bounded resources

### Worker Pool Design

```
                    ┌─────────────────┐
User Request ──────>│  Submit Job     │
                    │  Return JobID   │
                    └────────┬────────┘
                             │
                             ▼
                    ┌─────────────────┐
                    │ Buffered Channel│
                    │   (Queue: 100)  │
                    └────────┬────────┘
                             │
              ┌──────────────┼──────────────┐
              ▼              ▼              ▼
        ┌─────────┐    ┌─────────┐    ┌─────────┐
        │Worker 1 │    │Worker 2 │ ...│Worker 20│
        └────┬────┘    └────┬────┘    └────┬────┘
             │              │              │
             └──────────────┼──────────────┘
                            ▼
                   ┌─────────────────┐
                   │  Results Store  │
                   │  (sync.Map)     │
                   └─────────────────┘
                            │
User Poll ──────────────────┘
```

### Implementation

```go
// internal/analyzer/linkchecker.go

type LinkCheckWorkerPool struct {
    workers     int
    jobs        chan *domain.LinkCheckJob
    results     sync.Map  // JobID → *LinkCheckJob
    client      *http.Client
    timeout     time.Duration
    maxAge      time.Duration  // Garbage collect old jobs
    stopCleanup chan struct{}
}

func NewLinkCheckWorkerPool(cfg Config) *LinkCheckWorkerPool {
    return &LinkCheckWorkerPool{
        workers:     cfg.CheckWorkers,
        jobs:        make(chan *domain.LinkCheckJob, cfg.QueueSize),
        timeout:     cfg.CheckTimeout,
        maxAge:      cfg.JobMaxAge,
        stopCleanup: make(chan struct{}),
        client: &http.Client{
            Timeout: cfg.CheckTimeout,
            Transport: &http.Transport{
                MaxIdleConns:        100,
                MaxIdleConnsPerHost: 10,
                IdleConnTimeout:     90 * time.Second,
            },
        },
    }
}

func (p *LinkCheckWorkerPool) Start(ctx context.Context) {
    // Start worker goroutines
    for i := 0; i < p.workers; i++ {
        go p.worker(ctx, i)
    }

    // Start cleanup goroutine (garbage collect old jobs)
    go p.cleanup(ctx)
}

func (p *LinkCheckWorkerPool) worker(ctx context.Context, id int) {
    for {
        select {
        case <-ctx.Done():
            return
        case job := <-p.jobs:
            p.processJob(ctx, job)
        }
    }
}

func (p *LinkCheckWorkerPool) processJob(ctx context.Context, job *domain.LinkCheckJob) {
    job.Status = domain.LinkCheckInProgress
    now := time.Now()
    job.StartedAt = &now

    var inaccessible []domain.LinkError
    accessible := 0

    // Bounded concurrency within job (prevent resource exhaustion)
    sem := make(chan struct{}, 10)  // Max 10 concurrent checks per job
    var mu sync.Mutex
    var wg sync.WaitGroup

    for _, urlStr := range job.URLs {
        wg.Add(1)
        sem <- struct{}{}  // Acquire

        go func(u string) {
            defer wg.Done()
            defer func() { <-sem }()  // Release

            if err := p.checkLink(ctx, u); err != nil {
                mu.Lock()
                inaccessible = append(inaccessible, domain.LinkError{
                    URL:        u,
                    StatusCode: extractStatusCode(err),
                    Reason:     extractReason(err),
                })
                mu.Unlock()
            } else {
                mu.Lock()
                accessible++
                mu.Unlock()
            }
        }(urlStr)
    }

    wg.Wait()

    // Finalize job
    completed := time.Now()
    job.CompletedAt = &completed
    job.Status = domain.LinkCheckCompleted
    job.Result = &domain.LinkCheckResult{
        Checked:      len(job.URLs),
        Accessible:   accessible,
        Inaccessible: inaccessible,
        Duration:     completed.Sub(*job.StartedAt).String(),
        CompletedAt:  completed,
    }

    p.results.Store(job.ID, job)
}

func (p *LinkCheckWorkerPool) checkLink(ctx context.Context, urlStr string) error {
    req, err := http.NewRequestWithContext(ctx, http.MethodHead, urlStr, nil)
    if err != nil {
        return fmt.Errorf("invalid_url: %w", err)
    }

    req.Header.Set("User-Agent", "PageAnalyzer/1.0")

    resp, err := p.client.Do(req)
    if err != nil {
        if os.IsTimeout(err) {
            return fmt.Errorf("timeout: %w", err)
        }
        return fmt.Errorf("connection_refused: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 200 && resp.StatusCode < 300 {
        return nil  // Accessible
    }

    return fmt.Errorf("http_%d", resp.StatusCode)
}

func (p *LinkCheckWorkerPool) Submit(ctx context.Context, urls []string, baseURL string) string {
    jobID := generateJobID()

    job := &domain.LinkCheckJob{
        ID:        jobID,
        URLs:      urls,
        BaseURL:   baseURL,
        Status:    domain.LinkCheckPending,
        CreatedAt: time.Now(),
    }

    p.results.Store(jobID, job)

    select {
    case p.jobs <- job:
        // Queued successfully
    default:
        // Queue full - mark as failed
        job.Status = domain.LinkCheckFailed
        job.Result = &domain.LinkCheckResult{
            Checked:      0,
            Inaccessible: []domain.LinkError{{Reason: "queue_full"}},
        }
    }

    return jobID
}

func (p *LinkCheckWorkerPool) GetJob(jobID string) (*domain.LinkCheckJob, bool) {
    val, ok := p.results.Load(jobID)
    if !ok {
        return nil, false
    }
    return val.(*domain.LinkCheckJob), true
}

func (p *LinkCheckWorkerPool) cleanup(ctx context.Context) {
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-p.stopCleanup:
            return
        case <-ticker.C:
            p.gcOldJobs()
        }
    }
}

func (p *LinkCheckWorkerPool) gcOldJobs() {
    cutoff := time.Now().Add(-p.maxAge)
    p.results.Range(func(key, value interface{}) bool {
        job := value.(*domain.LinkCheckJob)
        if job.CreatedAt.Before(cutoff) {
            p.results.Delete(key)
        }
        return true
    })
}

func generateJobID() string {
    b := make([]byte, 16)
    rand.Read(b)
    return base64.URLEncoding.EncodeToString(b)
}

func extractStatusCode(err error) int {
    if err == nil {
        return 0
    }
    var httpErr HTTPError
    if errors.As(err, &httpErr) {
        return httpErr.StatusCode
    }
    return 0
}

func extractReason(err error) string {
    if err == nil {
        return ""
    }
    if os.IsTimeout(err) {
        return "timeout"
    }
    if strings.Contains(err.Error(), "connection refused") {
        return "connection_refused"
    }
    if strings.HasPrefix(err.Error(), "http_") {
        return strings.TrimPrefix(err.Error(), "http_")
    }
    return "unknown"
}
```

### Hybrid Mode (Best UX)

```go
// internal/analyzer/service.go

func (s *Service) Analyze(ctx context.Context, req domain.AnalysisRequest) (*domain.AnalysisResult, error) {
    // Phase 1: Fetch & Parse HTML (fast)
    resp, err := s.fetcher.Fetch(ctx, req.URL)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    // Check cache first
    if cached, err := s.cache.GetHTML(ctx, req.URL); err == nil {
        cached.CacheHit = true
        return cached, nil
    }

    // Stream HTML through collectors
    result := &domain.AnalysisResult{
        Version:    "v1",
        URL:        req.URL,
        AnalyzedAt: time.Now(),
    }

    collectors := s.buildCollectors(req)

    if err := s.walker.Walk(resp.Body, collectors, result); err != nil {
        return nil, err
    }

    // Cache HTML analysis (no link checks yet)
    s.cache.SetHTML(ctx, req.URL, result, s.config.CacheTTL)

    // Phase 2: Link Checking (slow)
    allLinks := append(result.Links.Internal, result.Links.External...)

    switch req.Options.CheckLinks {
    case domain.LinkCheckDisabled:
        result.Links.CheckStatus = domain.LinkCheckCompleted
        // No checking

    case domain.LinkCheckSync:
        // Synchronous (CLI mode)
        checkResult, err := s.linkChecker.CheckSync(ctx, allLinks)
        if err != nil {
            return nil, err
        }
        result.Links.CheckResult = checkResult
        result.Links.CheckStatus = domain.LinkCheckCompleted

    case domain.LinkCheckAsync:
        // Async: submit job, return immediately
        jobID := s.linkChecker.Submit(ctx, allLinks, req.URL)
        result.Links.CheckJobID = jobID
        result.Links.CheckStatus = domain.LinkCheckPending

    case domain.LinkCheckHybrid:
        // Hybrid: check first N sync, rest async
        limit := req.Options.SyncCheckLimit
        if limit > len(allLinks) {
            limit = len(allLinks)
        }

        syncLinks := allLinks[:limit]
        asyncLinks := allLinks[limit:]

        // Check first N synchronously (fast feedback)
        checkResult, _ := s.linkChecker.CheckSync(ctx, syncLinks)
        result.Links.CheckResult = checkResult

        // Queue remaining for async
        if len(asyncLinks) > 0 {
            jobID := s.linkChecker.Submit(ctx, asyncLinks, req.URL)
            result.Links.CheckJobID = jobID
            result.Links.CheckStatus = domain.LinkCheckInProgress
        } else {
            result.Links.CheckStatus = domain.LinkCheckCompleted
        }
    }

    return result, nil
}
```

---

## Caching Strategy

### Multi-Layer Architecture

```
┌──────────────────────────────────────────┐
│              Request                      │
└────────────────┬─────────────────────────┘
                 │
                 ▼
┌──────────────────────────────────────────┐
│    L1: In-Memory LRU (100 entries)       │
│    - Hot cache for frequent URLs         │
│    - TTL: 1 hour                         │
│    - Eviction: LRU                       │
└────────────────┬─────────────────────────┘
                 │ miss
                 ▼
┌──────────────────────────────────────────┐
│    L2: Redis (shared across instances)   │
│                                           │
│    html:{hash}  → HTMLAnalysis (TTL: 1h) │
│    links:{hash} → LinkCheckResult (5m)   │
│                                           │
│    - Shared results across users         │
│    - Faster than re-fetching             │
└────────────────┬─────────────────────────┘
                 │ miss
                 ▼
┌──────────────────────────────────────────┐
│         Fetch & Analyze                   │
│         (slow path)                       │
└──────────────────────────────────────────┘
```

### Implementation

```go
// internal/cache/multi.go

type MultiCache struct {
    l1 Cache  // Memory LRU
    l2 Cache  // Redis
}

func (mc *MultiCache) GetHTML(ctx context.Context, url string) (*domain.AnalysisResult, error) {
    // Try L1 first
    if result, err := mc.l1.GetHTML(ctx, url); err == nil {
        return result, nil
    }

    // Try L2
    if result, err := mc.l2.GetHTML(ctx, url); err == nil {
        // Backfill L1
        mc.l1.SetHTML(ctx, url, result, 1*time.Hour)
        return result, nil
    }

    return nil, ErrCacheMiss
}

func (mc *MultiCache) SetHTML(ctx context.Context, url string, result *domain.AnalysisResult, ttl time.Duration) error {
    // Write to both layers
    _ = mc.l1.SetHTML(ctx, url, result, ttl)
    _ = mc.l2.SetHTML(ctx, url, result, ttl)
    return nil
}
```

```go
// internal/cache/redis.go

type RedisCache struct {
    client *redis.Client
}

func NewRedisCache(addr string) (*RedisCache, error) {
    client := redis.NewClient(&redis.Options{
        Addr:         addr,
        DialTimeout:  5 * time.Second,
        ReadTimeout:  3 * time.Second,
        WriteTimeout: 3 * time.Second,
        PoolSize:     50,
    })

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    if err := client.Ping(ctx).Err(); err != nil {
        return nil, fmt.Errorf("redis connection failed: %w", err)
    }

    return &RedisCache{client: client}, nil
}

func (rc *RedisCache) GetHTML(ctx context.Context, url string) (*domain.AnalysisResult, error) {
    key := cacheKey("html", url)
    data, err := rc.client.Get(ctx, key).Bytes()
    if err == redis.Nil {
        return nil, ErrCacheMiss
    }
    if err != nil {
        return nil, err
    }

    var result domain.AnalysisResult
    if err := json.Unmarshal(data, &result); err != nil {
        return nil, err
    }

    return &result, nil
}

func (rc *RedisCache) SetHTML(ctx context.Context, url string, result *domain.AnalysisResult, ttl time.Duration) error {
    key := cacheKey("html", url)
    data, err := json.Marshal(result)
    if err != nil {
        return err
    }

    return rc.client.Set(ctx, key, data, ttl).Err()
}

func (rc *RedisCache) GetLinkCheck(ctx context.Context, url string) (*domain.LinkCheckResult, error) {
    key := cacheKey("links", url)
    data, err := rc.client.Get(ctx, key).Bytes()
    if err == redis.Nil {
        return nil, ErrCacheMiss
    }
    if err != nil {
        return nil, err
    }

    var result domain.LinkCheckResult
    if err := json.Unmarshal(data, &result); err != nil {
        return nil, err
    }

    return &result, nil
}

func (rc *RedisCache) SetLinkCheck(ctx context.Context, url string, result *domain.LinkCheckResult, ttl time.Duration) error {
    key := cacheKey("links", url)
    data, err := json.Marshal(result)
    if err != nil {
        return err
    }

    return rc.client.Set(ctx, key, data, ttl).Err()
}

func (rc *RedisCache) Health(ctx context.Context) error {
    return rc.client.Ping(ctx).Err()
}
```

```go
// internal/cache/keys.go

func cacheKey(prefix, url string) string {
    // Normalize URL for consistent caching
    normalized := normalizeURL(url)
    hash := sha256.Sum256([]byte(normalized))
    return fmt.Sprintf("%s:%x", prefix, hash[:16])
}

func normalizeURL(rawURL string) string {
    u, err := url.Parse(rawURL)
    if err != nil {
        return rawURL
    }

    // Remove fragments
    u.Fragment = ""

    // Sort query params
    q := u.Query()
    u.RawQuery = q.Encode()

    // Lowercase scheme and host
    u.Scheme = strings.ToLower(u.Scheme)
    u.Host = strings.ToLower(u.Host)

    return u.String()
}
```

### Graceful Degradation

```go
// internal/analyzer/service.go

func (s *Service) Analyze(ctx context.Context, req domain.AnalysisRequest) (*domain.AnalysisResult, error) {
    // Try cache
    cached, err := s.cache.GetHTML(ctx, req.URL)
    if err == nil {
        cached.CacheHit = true

        // Check if link results are available
        if linkResult, err := s.cache.GetLinkCheck(ctx, req.URL); err == nil {
            cached.Links.CheckResult = linkResult
            cached.Links.CheckStatus = domain.LinkCheckCompleted
        }

        return cached, nil
    }

    // Graceful degradation: if under high load, return stale cache
    if s.isOverloaded() && req.Options.AllowStale {
        if stale, err := s.cache.GetHTMLStale(ctx, req.URL); err == nil {
            stale.CacheHit = true
            stale.Stale = true
            return stale, nil
        }
    }

    // ... normal fetch & analyze flow
}

func (s *Service) isOverloaded() bool {
    queueDepth := len(s.linkChecker.(*LinkCheckWorkerPool).jobs)
    capacity := cap(s.linkChecker.(*LinkCheckWorkerPool).jobs)

    return float64(queueDepth)/float64(capacity) > 0.8  // 80% full
}
```

---

## Configuration

### Environment Variables (12-Factor)

All configuration via environment variables, overridable via CLI flags:

| Variable | Default | Description |
|----------|---------|-------------|
| **Server** |||
| `ANALYZER_ADDR` | `:8080` | HTTP listen address |
| `ANALYZER_READ_TIMEOUT` | `30s` | HTTP read timeout |
| `ANALYZER_WRITE_TIMEOUT` | `30s` | HTTP write timeout |
| **Fetching** |||
| `ANALYZER_FETCH_TIMEOUT` | `15s` | Target URL fetch timeout |
| `ANALYZER_MAX_BODY_SIZE` | `10485760` | Max response size (10MB) |
| `ANALYZER_USER_AGENT` | `PageAnalyzer/1.0` | HTTP User-Agent |
| **Link Checking** |||
| `ANALYZER_CHECK_MODE` | `async` | `sync`, `async`, `hybrid`, `disabled` |
| `ANALYZER_CHECK_TIMEOUT` | `5s` | Per-link HEAD request timeout |
| `ANALYZER_CHECK_WORKERS` | `20` | Concurrent worker goroutines |
| `ANALYZER_QUEUE_SIZE` | `100` | Link check job queue size |
| `ANALYZER_MAX_LINKS` | `10000` | Max links per page (upper bound) |
| `ANALYZER_SYNC_LIMIT` | `10` | Hybrid mode: check first N sync |
| **Caching** |||
| `ANALYZER_CACHE_MODE` | `redis` | `memory`, `redis`, `multi`, `disabled` |
| `ANALYZER_CACHE_TTL` | `1h` | HTML analysis cache TTL |
| `ANALYZER_LINK_CACHE_TTL` | `5m` | Link check results TTL |
| `ANALYZER_REDIS_ADDR` | `localhost:6379` | Redis server address |
| `ANALYZER_REDIS_PASSWORD` | `` | Redis password (optional) |
| `ANALYZER_MEMORY_CACHE_SIZE` | `100` | LRU cache size (entries) |
| **Rate Limiting** |||
| `ANALYZER_RATE_LIMIT_ENABLED` | `true` | Enable per-IP rate limiting |
| `ANALYZER_RATE_LIMIT_RPS` | `10` | Requests per second per IP |
| `ANALYZER_RATE_LIMIT_BURST` | `20` | Burst allowance |
| **Observability** |||
| `ANALYZER_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `ANALYZER_LOG_FORMAT` | `json` | `json`, `text` |
| `ANALYZER_OTEL_ENABLED` | `false` | Enable OpenTelemetry tracing |
| `ANALYZER_OTEL_ENDPOINT` | `localhost:4318` | OTEL collector endpoint |
| `ANALYZER_METRICS_ENABLED` | `true` | Enable Prometheus metrics |
| **Degradation** |||
| `ANALYZER_ALLOW_STALE` | `true` | Return stale cache under load |
| `ANALYZER_MAX_STALENESS` | `24h` | Max age for stale cache |

### Configuration Loading

```go
// internal/config/config.go

type Config struct {
    Server    ServerConfig
    Fetcher   FetcherConfig
    LinkCheck LinkCheckConfig
    Cache     CacheConfig
    RateLimit RateLimitConfig
    Observability ObservabilityConfig
}

func Load() (*Config, error) {
    cfg := &Config{}

    // Load from environment
    if err := env.Parse(cfg); err != nil {
        return nil, err
    }

    // Validate
    if err := cfg.Validate(); err != nil {
        return nil, err
    }

    return cfg, nil
}

func (c *Config) Validate() error {
    if c.LinkCheck.MaxLinks > 100000 {
        return errors.New("maxLinks cannot exceed 100000")
    }
    if c.Cache.Mode == "redis" && c.Cache.RedisAddr == "" {
        return errors.New("redisAddr required when cacheMode=redis")
    }
    return nil
}
```

---

## API Specification

### REST Endpoints

#### POST /api/analyze

Analyze a webpage URL.

**Request:**
```json
{
  "url": "https://example.com",
  "options": {
    "checkLinks": "async",
    "maxLinks": 1000,
    "allowStale": false
  }
}
```

**Response (200 OK):**
```json
{
  "version": "v1",
  "url": "https://example.com",
  "htmlVersion": "HTML5",
  "title": "Example Domain",
  "headings": {
    "h1": 1,
    "h2": 3,
    "h3": 0,
    "h4": 0,
    "h5": 0,
    "h6": 0
  },
  "links": {
    "internal": ["https://example.com/about", "https://example.com/contact"],
    "external": ["https://other.com"],
    "totalFound": 3,
    "truncated": false,
    "checkJobID": "abc123xyz",
    "checkStatus": "pending",
    "checkResult": null
  },
  "hasLoginForm": false,
  "analyzedAt": "2026-03-21T10:30:00Z",
  "cacheHit": false
}
```

**Error Response (400 Bad Request):**
```json
{
  "error": {
    "code": "invalid_url",
    "message": "URL must be a valid HTTP/HTTPS URL"
  }
}
```

**Error Response (502 Bad Gateway):**
```json
{
  "error": {
    "code": "fetch_failed",
    "message": "HTTP 404: Not Found",
    "statusCode": 404
  }
}
```

#### GET /api/jobs/:jobId

Retrieve link check job status and results.

**Response (200 OK - Completed):**
```json
{
  "jobID": "abc123xyz",
  "status": "completed",
  "result": {
    "checked": 75,
    "accessible": 73,
    "inaccessible": [
      {
        "url": "https://broken.com",
        "statusCode": 404,
        "reason": "404"
      },
      {
        "url": "https://timeout.com",
        "reason": "timeout"
      }
    ],
    "duration": "2.5s",
    "completedAt": "2026-03-21T10:30:03Z"
  }
}
```

**Response (200 OK - Pending):**
```json
{
  "jobID": "abc123xyz",
  "status": "pending",
  "createdAt": "2026-03-21T10:30:00Z"
}
```

**Response (404 Not Found):**
```json
{
  "error": {
    "code": "job_not_found",
    "message": "Job abc123xyz not found or expired"
  }
}
```

#### GET /api/health

Health check endpoint for load balancers and k8s probes.

**Response (200 OK):**
```json
{
  "status": "healthy",
  "timestamp": "2026-03-21T10:30:00Z",
  "components": {
    "cache": "connected",
    "workers": {
      "active": 5,
      "capacity": 20,
      "queueDepth": 12,
      "utilization": 0.60
    }
  }
}
```

**Response (503 Service Unavailable):**
```json
{
  "status": "unhealthy",
  "timestamp": "2026-03-21T10:30:00Z",
  "components": {
    "cache": "disconnected",
    "workers": {
      "active": 20,
      "capacity": 20,
      "queueDepth": 100,
      "utilization": 1.0
    }
  }
}
```

#### DELETE /api/cache/:url

Invalidate cache for a specific URL (admin endpoint).

**Response (204 No Content):** Cache cleared successfully.

### Web Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/` | Home page with URL form |
| POST | `/analyze` | Submit URL for analysis (HTML form) |
| GET | `/results/:jobId` | View analysis results |
| GET | `/static/*` | Static assets (CSS, JS) |

### CLI Usage

```bash
# Analyze URL (table output)
./analyzer analyze https://example.com

# Analyze URL (JSON output)
./analyzer analyze https://example.com --json

# Check links synchronously (CLI mode)
./analyzer analyze https://example.com --check-links

# Custom limits
./analyzer analyze https://example.com --max-links 500

# Start HTTP server
./analyzer serve

# Start with custom config
./analyzer serve --addr :9090 --redis redis://localhost:6379

# Health check
./analyzer healthcheck --addr :8080
```

---

## Observability

### OpenTelemetry Integration

```go
// internal/observability/tracing.go

func InitTracing(cfg Config) (*sdktrace.TracerProvider, error) {
    if !cfg.OtelEnabled {
        return nil, nil
    }

    exporter, err := otlptracehttp.New(
        context.Background(),
        otlptracehttp.WithEndpoint(cfg.OtelEndpoint),
        otlptracehttp.WithInsecure(),
    )
    if err != nil {
        return nil, err
    }

    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exporter),
        sdktrace.WithResource(resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceName("page-analyzer"),
            semconv.ServiceVersion("1.0.0"),
        )),
    )

    otel.SetTracerProvider(tp)
    otel.SetTextMapPropagator(propagation.TraceContext{})

    return tp, nil
}

// Usage in handlers
func (h *Handler) Analyze(w http.ResponseWriter, r *http.Request) {
    ctx, span := otel.Tracer("analyzer").Start(r.Context(), "analyze_request")
    defer span.End()

    // ... business logic

    span.SetAttributes(
        attribute.String("url", req.URL),
        attribute.Int("links.count", len(result.Links.Internal)+len(result.Links.External)),
        attribute.Bool("cache.hit", result.CacheHit),
    )
}
```

### Prometheus Metrics

```go
// internal/observability/metrics.go

var (
    requestsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "analyzer_requests_total",
            Help: "Total analysis requests",
        },
        []string{"status", "cache_hit"},
    )

    requestDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "analyzer_request_duration_seconds",
            Help:    "Request duration in seconds",
            Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30},
        },
        []string{"endpoint"},
    )

    linksChecked = promauto.NewCounter(
        prometheus.CounterOpts{
            Name: "analyzer_links_checked_total",
            Help: "Total links checked",
        },
    )

    workerPoolUtilization = promauto.NewGauge(
        prometheus.GaugeOpts{
            Name: "analyzer_worker_pool_utilization",
            Help: "Worker pool utilization (0.0-1.0)",
        },
    )

    cacheHitRatio = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "analyzer_cache_hit_ratio",
            Help: "Cache hit ratio",
        },
        []string{"layer"},  // "l1", "l2"
    )
)

// Middleware
func MetricsMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        recorder := &responseRecorder{ResponseWriter: w, statusCode: 200}

        next.ServeHTTP(recorder, r)

        duration := time.Since(start).Seconds()
        requestDuration.WithLabelValues(r.URL.Path).Observe(duration)
        requestsTotal.WithLabelValues(
            http.StatusText(recorder.statusCode),
            strconv.FormatBool(recorder.cacheHit),
        ).Inc()
    })
}
```

### Structured Logging

```go
// internal/observability/logger.go

func NewLogger(cfg Config) *slog.Logger {
    var handler slog.Handler

    if cfg.LogFormat == "json" {
        handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
            Level: parseLogLevel(cfg.LogLevel),
        })
    } else {
        handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
            Level: parseLogLevel(cfg.LogLevel),
        })
    }

    return slog.New(handler)
}

// Usage
logger.Info("analysis completed",
    "url", req.URL,
    "duration_ms", duration.Milliseconds(),
    "links_found", len(result.Links.Internal)+len(result.Links.External),
    "cache_hit", result.CacheHit,
)
```

### Grafana Dashboard

**Docker Compose setup:**
```yaml
# deployments/docker-compose.yml

services:
  analyzer:
    build: .
    ports:
      - "8080:8080"
    environment:
      - ANALYZER_REDIS_ADDR=redis:6379
      - ANALYZER_OTEL_ENABLED=true
      - ANALYZER_OTEL_ENDPOINT=tempo:4318
    depends_on:
      - redis
      - tempo

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"

  tempo:
    image: grafana/tempo:latest
    ports:
      - "4318:4318"  # OTLP HTTP
    command: ["-config.file=/etc/tempo.yaml"]
    volumes:
      - ./tempo.yaml:/etc/tempo.yaml

  prometheus:
    image: prom/prometheus:latest
    ports:
      - "9090:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
    command:
      - "--config.file=/etc/prometheus/prometheus.yml"

  grafana:
    image: grafana/grafana:latest
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
    volumes:
      - ./grafana/dashboards:/etc/grafana/provisioning/dashboards
      - ./grafana/datasources:/etc/grafana/provisioning/datasources
```

**Key Metrics to Monitor:**
- Request rate (requests/sec)
- P50/P95/P99 latency
- Cache hit ratio (L1 vs L2)
- Worker pool utilization
- Error rate by type
- Links checked per second
- Queue depth over time

---

## Testing Strategy

### Unit Tests (Collectors)

```go
// internal/analyzer/collectors/headings_test.go

func TestHeadingsCollector(t *testing.T) {
    tests := []struct {
        name     string
        html     string
        want     domain.HeadingCounts
    }{
        {
            name: "no headings",
            html: "<html><body><p>text</p></body></html>",
            want: domain.HeadingCounts{},
        },
        {
            name: "mixed headings",
            html: "<html><h1>Title</h1><h2>Sub1</h2><h2>Sub2</h2><h3>Sub3</h3></html>",
            want: domain.HeadingCounts{H1: 1, H2: 2, H3: 1},
        },
        {
            name: "nested headings",
            html: "<div><h1>A</h1><div><h2>B</h2></div></div>",
            want: domain.HeadingCounts{H1: 1, H2: 1},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            collector := NewHeadingsCollector()
            result := &domain.AnalysisResult{}

            // Parse HTML and feed to collector
            z := html.NewTokenizer(strings.NewReader(tt.html))
            for {
                tt := z.Next()
                if tt == html.ErrorToken {
                    break
                }
                collector.Collect(z.Token())
            }

            collector.Apply(result)

            assert.Equal(t, tt.want, result.Headings)
        })
    }
}
```

### Integration Tests (Walker + Collectors)

```go
// internal/analyzer/walker_test.go

func TestWalker_Integration(t *testing.T) {
    html := `
<!DOCTYPE html>
<html>
<head><title>Test Page</title></head>
<body>
    <h1>Main</h1>
    <h2>Section 1</h2>
    <h2>Section 2</h2>
    <a href="https://example.com/internal">Internal</a>
    <a href="https://other.com">External</a>
    <form><input type="password"></form>
</body>
</html>`

    walker := NewWalker(1000000)
    result := &domain.AnalysisResult{}

    collectors := []domain.Collector{
        NewHTMLVersionCollector(),
        NewTitleCollector(),
        NewHeadingsCollector(),
        NewLinksCollector("https://example.com", 10000),
        NewLoginFormCollector(),
    }

    err := walker.Walk(strings.NewReader(html), collectors, result)
    require.NoError(t, err)

    assert.Equal(t, "HTML5", result.HTMLVersion)
    assert.Equal(t, "Test Page", result.Title)
    assert.Equal(t, 1, result.Headings.H1)
    assert.Equal(t, 2, result.Headings.H2)
    assert.Len(t, result.Links.Internal, 1)
    assert.Len(t, result.Links.External, 1)
    assert.True(t, result.HasLoginForm)
}
```

### HTTP Integration Tests

```go
// test/integration/api_test.go

func TestAnalyzeEndpoint(t *testing.T) {
    // Setup test server
    testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/html")
        fmt.Fprint(w, `<!DOCTYPE html><html><head><title>Test</title></head><body><h1>Hello</h1></body></html>`)
    }))
    defer testServer.Close()

    // Setup analyzer service
    cfg := DefaultConfig()
    svc := NewService(cfg)

    // Test analyze
    req := domain.AnalysisRequest{
        URL: testServer.URL,
        Options: domain.AnalysisOptions{
            CheckLinks: domain.LinkCheckDisabled,
        },
    }

    result, err := svc.Analyze(context.Background(), req)
    require.NoError(t, err)

    assert.Equal(t, "HTML5", result.HTMLVersion)
    assert.Equal(t, "Test", result.Title)
    assert.Equal(t, 1, result.Headings.H1)
}
```

### Property-Based Tests

```go
// internal/analyzer/collectors/links_test.go

func TestLinksCollector_Properties(t *testing.T) {
    // Property: Internal + External + Duplicates = TotalFound
    // Property: No URL appears twice in results
    // Property: All URLs are parseable

    rapid.Check(t, func(t *rapid.T) {
        baseURL := "https://example.com"
        html := generateRandomHTML(t, 100)

        collector := NewLinksCollector(baseURL, 10000)
        // ... parse and collect

        // Assert properties
        totalUnique := len(collector.internal) + len(collector.external)
        assert.LessOrEqual(t, totalUnique, collector.totalFound)

        // No duplicates
        seen := make(map[string]bool)
        for _, u := range append(collector.internal, collector.external...) {
            assert.False(t, seen[u], "duplicate URL: %s", u)
            seen[u] = true
        }

        // All parseable
        for _, u := range append(collector.internal, collector.external...) {
            _, err := url.Parse(u)
            assert.NoError(t, err, "invalid URL: %s", u)
        }
    })
}
```

### Load Tests

```bash
# scripts/load-test.sh

# 1000 requests, 50 concurrent
hey -n 1000 -c 50 -m POST \
  -H "Content-Type: application/json" \
  -d '{"url":"https://example.com"}' \
  http://localhost:8080/api/analyze
```

### Test Coverage Goals

| Package | Target | Rationale |
|---------|--------|-----------|
| `collectors/` | 90%+ | Core business logic |
| `analyzer/` | 85%+ | Orchestration layer |
| `cache/` | 80%+ | Infrastructure |
| `presentation/` | 70%+ | Integration tests cover most |

---

## Deployment

### Local Development (Primary Focus)

**Priority:** Get a working demo fast, improve iteratively

#### Option 1: Go Binary + Docker Compose (Recommended)

```bash
# Terminal 1: Start infrastructure
docker-compose up -d redis tempo prometheus grafana

# Terminal 2: Run Go binary directly (fast iteration)
go run cmd/main.go serve

# Access:
# - App:        http://localhost:8080
# - Grafana:    http://localhost:3000 (admin/admin)
# - Prometheus: http://localhost:9090
```

#### Option 2: Full Docker (Closer to Production)

```bash
# Build and start everything
docker-compose up --build

# Rebuild only app after code changes
docker-compose up --build analyzer
```

### Docker Compose Configuration

```yaml
# docker-compose.yml

version: '3.9'

services:
  analyzer:
    build: .
    ports:
      - "8080:8080"
    environment:
      - ANALYZER_ADDR=:8080
      - ANALYZER_REDIS_ADDR=redis:6379
      - ANALYZER_OTEL_ENABLED=true
      - ANALYZER_OTEL_ENDPOINT=tempo:4318
      - ANALYZER_LOG_LEVEL=debug
      - ANALYZER_LOG_FORMAT=json
    depends_on:
      redis:
        condition: service_healthy
      tempo:
        condition: service_started
    restart: unless-stopped

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 5
    volumes:
      - redis-data:/data

  tempo:
    image: grafana/tempo:latest
    command: ["-config.file=/etc/tempo.yaml"]
    ports:
      - "3200:3200"   # Tempo web UI
      - "4317:4317"   # OTLP gRPC
      - "4318:4318"   # OTLP HTTP
    volumes:
      - ./deployments/tempo.yaml:/etc/tempo.yaml
      - tempo-data:/tmp/tempo

  prometheus:
    image: prom/prometheus:latest
    command:
      - "--config.file=/etc/prometheus/prometheus.yml"
      - "--storage.tsdb.path=/prometheus"
      - "--web.console.libraries=/usr/share/prometheus/console_libraries"
      - "--web.console.templates=/usr/share/prometheus/consoles"
    ports:
      - "9090:9090"
    volumes:
      - ./deployments/prometheus.yml:/etc/prometheus/prometheus.yml
      - prometheus-data:/prometheus
    depends_on:
      - analyzer

  grafana:
    image: grafana/grafana:latest
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_USER=admin
      - GF_SECURITY_ADMIN_PASSWORD=admin
      - GF_USERS_ALLOW_SIGN_UP=false
    volumes:
      - ./deployments/grafana/dashboards:/etc/grafana/provisioning/dashboards
      - ./deployments/grafana/datasources:/etc/grafana/provisioning/datasources
      - grafana-data:/var/lib/grafana
    depends_on:
      - prometheus
      - tempo

volumes:
  redis-data:
  tempo-data:
  prometheus-data:
  grafana-data:
```

### Dockerfile (Multi-Stage Build)

```dockerfile
# Dockerfile

# Stage 1: Build
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.version=$(git describe --tags --always --dirty)" \
    -o /analyzer \
    ./cmd/main.go

# Stage 2: Runtime
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

COPY --from=builder /analyzer .

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD ./analyzer healthcheck || exit 1

EXPOSE 8080

ENTRYPOINT ["./analyzer"]
CMD ["serve"]
```

### Supporting Config Files

#### Tempo Configuration

```yaml
# deployments/tempo.yaml

server:
  http_listen_port: 3200

distributor:
  receivers:
    otlp:
      protocols:
        http:
        grpc:

storage:
  trace:
    backend: local
    local:
      path: /tmp/tempo/traces

compactor:
  compaction:
    block_retention: 1h
```

#### Prometheus Configuration

```yaml
# deployments/prometheus.yml

global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'analyzer'
    static_configs:
      - targets: ['analyzer:8080']
    metrics_path: '/metrics'
```

#### Grafana Datasources

```yaml
# deployments/grafana/datasources/datasources.yml

apiVersion: 1

datasources:
  - name: Prometheus
    type: prometheus
    access: proxy
    url: http://prometheus:9090
    isDefault: true
    editable: true

  - name: Tempo
    type: tempo
    access: proxy
    url: http://tempo:3200
    editable: true
    jsonData:
      httpMethod: GET
      tracesToLogs:
        datasourceUid: 'loki'
```

#### Grafana Dashboard (Basic)

```yaml
# deployments/grafana/dashboards/dashboard.yml

apiVersion: 1

providers:
  - name: 'Page Analyzer'
    orgId: 1
    folder: ''
    type: file
    disableDeletion: false
    updateIntervalSeconds: 10
    allowUiUpdates: true
    options:
      path: /etc/grafana/provisioning/dashboards
```

### Makefile (Developer Convenience)

```makefile
# Makefile

.PHONY: help build run test docker-build docker-up docker-down clean

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build the binary
	go build -o bin/analyzer ./cmd/main.go

run: ## Run the service locally
	go run ./cmd/main.go serve

test: ## Run tests
	go test ./... -v -race -coverprofile=coverage.out

test-coverage: test ## Run tests with coverage report
	go tool cover -html=coverage.out

lint: ## Run linters
	golangci-lint run

docker-build: ## Build Docker image
	docker build -t page-analyzer:latest .

docker-up: ## Start all services with docker-compose
	docker-compose up -d

docker-logs: ## Follow logs
	docker-compose logs -f analyzer

docker-down: ## Stop all services
	docker-compose down

docker-clean: ## Stop and remove volumes
	docker-compose down -v

dev: ## Start infrastructure only (run app with 'make run')
	docker-compose up -d redis tempo prometheus grafana

cli-analyze: ## Quick CLI test (usage: make cli-analyze URL=https://example.com)
	go run ./cmd/main.go analyze $(URL)

load-test: ## Run load test (requires hey: go install github.com/rakyll/hey@latest)
	hey -n 100 -c 10 -m POST \
		-H "Content-Type: application/json" \
		-d '{"url":"https://example.com"}' \
		http://localhost:8080/api/analyze

clean: ## Clean build artifacts
	rm -rf bin/ coverage.out

install-tools: ## Install development tools
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/rakyll/hey@latest
```

### Quick Start Scripts

```bash
# scripts/dev.sh
#!/bin/bash
set -e

echo "🚀 Starting Page Analyzer development environment..."

# Start infrastructure
echo "📦 Starting Redis, Tempo, Prometheus, Grafana..."
docker-compose up -d redis tempo prometheus grafana

# Wait for services
echo "⏳ Waiting for services to be ready..."
sleep 5

# Check health
echo "✅ Checking service health..."
docker-compose ps

echo ""
echo "🎉 Infrastructure ready!"
echo ""
echo "Next steps:"
echo "  1. Run the app:  go run ./cmd/main.go serve"
echo "  2. Visit:        http://localhost:8080"
echo "  3. Grafana:      http://localhost:3000 (admin/admin)"
echo "  4. Prometheus:   http://localhost:9090"
echo ""
```

```bash
# scripts/demo.sh
#!/bin/bash
set -e

echo "🎬 Running Page Analyzer Demo..."

BASE_URL="http://localhost:8080"

# Test 1: Analyze a simple page
echo ""
echo "Test 1: Analyze example.com"
curl -s -X POST "$BASE_URL/api/analyze" \
  -H "Content-Type: application/json" \
  -d '{"url":"https://example.com"}' | jq .

# Test 2: Health check
echo ""
echo "Test 2: Health Check"
curl -s "$BASE_URL/api/health" | jq .

# Test 3: CLI mode
echo ""
echo "Test 3: CLI Mode"
go run ./cmd/main.go analyze https://example.com --json | jq .

echo ""
echo "✅ Demo complete!"
```

### Development Workflow

```bash
# 1. Initial setup
make install-tools
make dev

# 2. Run app (fast iteration, no rebuild)
make run

# 3. Make changes, hot reload
# (App auto-reloads on save if using air: https://github.com/cosmtrek/air)

# 4. Run tests
make test

# 5. Test with real requests
make cli-analyze URL=https://example.com

# 6. Load test
make load-test

# 7. Check metrics
open http://localhost:3000  # Grafana
open http://localhost:9090  # Prometheus

# 8. View traces
# In Grafana: Explore → Tempo → Search traces
```

### Production Deployment (Future)

**Kubernetes**: All env vars are K8s-ready. When needed:
```bash
# deployments/k8s/ directory structure ready
# Apply with: kubectl apply -f deployments/k8s/
```

**Cloud Providers**: Environment variables make it easy to deploy to:
- AWS ECS/Fargate (use Task Definitions)
- GCP Cloud Run (single container, serverless)
- Azure Container Apps
- Fly.io (flyctl deploy)

For now: **Docker Compose is sufficient for demo and light production use.**

---

## Libraries

| Purpose | Library | Version |
|---------|---------|---------|
| HTML parsing | `golang.org/x/net/html` | latest |
| CLI framework | `github.com/spf13/cobra` | v1.8+ |
| HTTP router | `github.com/go-chi/chi/v5` | v5.0+ |
| Redis client | `github.com/redis/go-redis/v9` | v9.0+ |
| OpenTelemetry | `go.opentelemetry.io/otel` | v1.24+ |
| Prometheus | `github.com/prometheus/client_golang` | v1.19+ |
| Structured logging | `log/slog` | stdlib |
| Testing | `github.com/stretchr/testify` | v1.9+ |
| Property testing | `pgregory.net/rapid` | v1.1+ |

---

## Assumptions & Decisions

### Architecture Decisions

1. **Two-Phase Processing** *(Performance)*
   - **Decision:** Decouple HTML analysis from link checking
   - **Rationale:** HTML parsing is fast (<500ms), link checking is slow (seconds to minutes). User sees results immediately while links check in background.
   - **Trade-off:** Added complexity (job queue, polling) vs better UX and resource efficiency.

2. **Single-Pass Streaming** *(Memory Efficiency)*
   - **Decision:** Stream HTML tokens, never buffer full document
   - **Rationale:** Handles 100KB and 10MB pages with identical memory footprint (O(1))
   - **Trade-off:** Can't go back to reparse, but collectors get one shot at each token.

3. **Static Collector Registry** *(Extensibility vs Simplicity)*
   - **Decision:** Compile-time registration via `init()`, not runtime plugins
   - **Rationale:** Service will recompile anyway; no need for plugin complexity
   - **Trade-off:** Requires rebuild to add collectors, but zero runtime overhead.

4. **Bounded Resources** *(High Load)*
   - **Decision:** Hard limits on body size (10MB), links (10k), tokens (1M), queue (100 jobs)
   - **Rationale:** Predictable memory usage, prevents resource exhaustion
   - **Trade-off:** May truncate analysis of extreme pages, but protects service.

5. **Redis for Service, LRU for CLI** *(Cache Architecture)*
   - **Decision:** Two cache implementations; multi-layer cache for service
   - **Rationale:** CLI doesn't need Redis overhead; service benefits from shared cache
   - **Trade-off:** More code, but optimal for each use case.

6. **Separate HTML + Link Caching** *(Flexibility)*
   - **Decision:** Cache HTML analysis (TTL 1h) separate from link checks (TTL 5m)
   - **Rationale:** Link accessibility changes faster than page content; reuse link checks across users
   - **Trade-off:** More cache keys, but better cache hit ratio and freshness.

7. **Always Return 200 OK** *(API Design)*
   - **Decision:** Even with broken links, return 200 OK with results
   - **Rationale:** Our request succeeded; broken links are data, not errors
   - **Trade-off:** Some might expect 207 Multi-Status, but 200 is simpler.

### Business Logic Decisions

8. **Inaccessible Link Definition**
   - **Rule:** Non-2xx status, connection refused, timeout, or invalid URL
   - **Note:** 3xx redirects that resolve to 2xx are considered accessible
   - **Rationale:** Follows user behavior; browser redirects are transparent

9. **Internal vs External Link Classification**
   - **Rule:** Same scheme + host as analyzed URL = internal; all others = external
   - **Examples:**
     - `https://example.com` analyzing `https://example.com/about` → internal
     - `https://example.com` analyzing `http://example.com/about` → external (scheme differs)
     - `https://example.com` analyzing `https://sub.example.com` → external (host differs)
   - **Rationale:** Strict origin matching; subdomain != same site

10. **Login Form Detection**
    - **Rule:** `<input type="password">` inside a `<form>` element
    - **Note:** Doesn't verify form action or method
    - **Rationale:** Password input is strong signal; false positives acceptable

11. **HTML Version Detection**
    - **Rule:** Parse `<!DOCTYPE>` token only
    - **Mapping:**
      - `<!DOCTYPE html>` → "HTML5"
      - `<!DOCTYPE HTML PUBLIC "-//W3C//DTD HTML 4.01...">` → "HTML 4.01"
      - No DOCTYPE → "Unknown"
    - **Rationale:** Fast, no deep inspection needed

12. **Link Checking Strategy**
    - **Method:** HEAD requests (not GET) to minimize bandwidth
    - **Concurrency:** 20 workers globally, 10 concurrent per job
    - **Timeout:** 5s per link (configurable)
    - **Rationale:** HEAD is faster; timeouts prevent blocking on slow servers

### Operational Decisions

13. **Graceful Degradation Priority**
    - **Order:** Return stale cache → Queue request → Disable link checking → Reject request
    - **Rationale:** Stale data better than no data; keep service available

14. **Rate Limiting Scope**
    - **Decision:** Per-IP limiting (10 req/sec, burst 20)
    - **Alternatives considered:** Per-target-domain (too complex), global (too coarse)
    - **Rationale:** Prevents user abuse while allowing legitimate burst traffic

15. **URL Normalization for Caching**
    - **Rules:**
      - Remove fragment (#section)
      - Lowercase scheme and host
      - Sort query parameters
    - **Rationale:** `example.com?b=2&a=1` and `example.com?a=1&b=2` are same page

16. **Worker Pool Sizing**
    - **Workers:** 20 (configurable)
    - **Queue:** 100 jobs (configurable)
    - **Rationale:** Balance concurrency vs resource usage; prevents thundering herd

### Security & Reliability Decisions

17. **User-Agent Policy**
    - **Decision:** Use identifiable User-Agent: `PageAnalyzer/1.0`
    - **Rationale:** Polite crawling; allows servers to identify and block if needed

18. **Redirect Policy**
    - **Decision:** Follow redirects automatically (max 10)
    - **Rationale:** Matches browser behavior; stops infinite redirect loops

19. **TLS Verification**
    - **Decision:** Verify TLS certificates by default
    - **Rationale:** Security; users analyzing HTTPS sites expect valid certs

20. **Timeout Hierarchy**
    - **Overall request:** 30s (configurable)
    - **Target fetch:** 15s (configurable)
    - **Link check:** 5s (configurable)
    - **Rationale:** Nested timeouts ensure request doesn't hang indefinitely

---

## Future Improvements

### High Priority (Production Readiness)

1. **OpenAPI Specification**
   - Generate API docs with Swagger/OpenAPI
   - Auto-generate client SDKs
   - Tool: `oapi-codegen` or `swaggo`

2. **Circuit Breaker Pattern**
   - Protect against cascading failures
   - Auto-disable link checking if downstream service is down
   - Library: `github.com/sony/gobreaker`

3. **Request Tracing**
   - Distributed tracing with OTEL already implemented
   - Add detailed spans for each collector
   - Trace link checking jobs

4. **Admin API**
   - POST `/api/admin/cache/purge` - clear cache by URL pattern
   - GET `/api/admin/metrics` - internal metrics
   - POST `/api/admin/config` - dynamic config reload

### Medium Priority (Features)

5. **Robots.txt Respect**
   - Parse robots.txt before analyzing
   - Honor crawl-delay and user-agent rules
   - Configurable: respect or ignore

6. **JavaScript Rendering**
   - Use `chromedp` for SPA support
   - Async job (very slow: 5-10s)
   - Optional: `"renderJS": true` in request

7. **Depth Crawling**
   - Follow links N levels deep
   - Aggregate statistics across all pages
   - Async job with progress updates

8. **Webhook Callbacks**
   - POST results to user-provided URL when job completes
   - Alternative to polling
   - Include signature for verification

9. **Bulk Analysis API**
   - POST `/api/analyze/batch` - analyze multiple URLs
   - Return array of JobIDs
   - Rate limit per batch size

10. **Historical Tracking**
    - Store analysis history per URL
    - Detect changes over time (title changed, links broken)
    - Requires persistent storage (PostgreSQL)

### Low Priority (Nice to Have)

11. **Browser Extension**
    - Chrome/Firefox extension
    - Right-click "Analyze Page"
    - Uses existing REST API

12. **gRPC Interface**
    - Add `presentation/grpc/` package
    - Parallel to REST handlers
    - Zero changes to core logic

13. **Custom Collectors Plugin System**
    - Load collectors from `.so` files
    - Requires cgo, more complex
    - Only if user demand exists

14. **Machine Learning Analysis**
    - Classify page type (blog, e-commerce, documentation)
    - Extract sentiment from text
    - Detect spam/malicious content

15. **SEO Analysis**
    - Meta tags (description, keywords)
    - Open Graph data
    - Twitter Card data
    - Structured data (JSON-LD)

16. **Accessibility Analysis**
    - Check alt text on images
    - ARIA attributes
    - Color contrast
    - Semantic HTML

17. **Performance Analysis**
    - Resource sizes (CSS, JS, images)
    - Render-blocking resources
    - Lazy loading recommendations

18. **Security Analysis**
    - Mixed content detection (HTTPS page with HTTP resources)
    - CSP header analysis
    - XSS vulnerability patterns (basic heuristics)

19. **Multi-Language Support**
    - Internationalized UI
    - Detect page language
    - Translate analysis results

20. **Export Formats**
    - PDF report generation
    - CSV export for bulk analysis
    - Excel spreadsheet format

---

---

## Implementation Roadmap

### Phase 0: Project Setup (30 min)
**Goal:** Initialize Go project, dependencies, directory structure

- [ ] `go mod init github.com/yourusername/page-analyzer`
- [ ] Create directory structure (`internal/`, `cmd/`, `deployments/`, `scripts/`)
- [ ] Add dependencies to `go.mod`
- [ ] Create `.gitignore`
- [ ] Create `Makefile`
- [ ] Initial commit

**Deliverable:** Compiles successfully, tests pass (even if empty)

---

### Phase 1: Domain Layer (1 hour)
**Goal:** Pure Go types and interfaces, zero dependencies

**Files to create:**
- `internal/domain/analysis.go` - All request/result types
- `internal/domain/links.go` - Link checking types
- `internal/domain/errors.go` - Error types
- `internal/domain/interfaces.go` - All interfaces

**Validation:**
```bash
go test ./internal/domain/... -v
# Should compile, basic type tests pass
```

**Deliverable:** Complete domain model, compiles, basic tests

---

### Phase 2: Core Collectors (2-3 hours)
**Goal:** Implement collectors one by one with TDD

**Order (simplest → complex):**
1. `internal/analyzer/collectors/htmlversion.go` + `_test.go`
2. `internal/analyzer/collectors/title.go` + `_test.go`
3. `internal/analyzer/collectors/headings.go` + `_test.go`
4. `internal/analyzer/collectors/loginform.go` + `_test.go`
5. `internal/analyzer/collectors/links.go` + `_test.go` (most complex)
6. `internal/analyzer/collectors/registry.go` - Wire them up

**Validation:**
```bash
go test ./internal/analyzer/collectors/... -v -cover
# Target: 85%+ coverage
```

**Deliverable:** All collectors working, tested with HTML fixtures

---

### Phase 3: HTML Analyzer Core (1-2 hours)
**Goal:** Fetch + Walk + Collect pipeline

**Files to create:**
- `internal/analyzer/fetcher.go` - HTTP client
- `internal/analyzer/walker.go` - Token stream
- `internal/analyzer/service.go` - Orchestration (Phase 1 only, no link checking yet)

**Validation:**
```bash
# CLI test (no web server yet)
go run cmd/main.go analyze https://example.com
# Should print analysis results
```

**Deliverable:** End-to-end HTML analysis working via CLI

---

### Phase 4: CLI Interface (1 hour)
**Goal:** Working CLI tool

**Files to create:**
- `cmd/main.go` - Cobra root
- `cmd/analyze.go` - Analyze subcommand
- `internal/presentation/cli/handler.go` - Output formatting
- `internal/presentation/cli/formatter.go` - Table format
- `internal/presentation/cli/json.go` - JSON format

**Validation:**
```bash
./bin/analyzer analyze https://example.com
./bin/analyzer analyze https://example.com --json | jq .
```

**Deliverable:** Fully functional CLI tool

---

### Phase 5: Link Checking (2-3 hours)
**Goal:** Async worker pool + job queue

**Files to create:**
- `internal/analyzer/linkchecker.go` - Worker pool implementation
- `internal/analyzer/linkchecker_test.go` - Mock HTTP tests

**Validation:**
```bash
# Test with page having many links
./bin/analyzer analyze https://golang.org --check-links
```

**Deliverable:** Link checking working in CLI mode (sync)

---

### Phase 6: Caching (1-2 hours)
**Goal:** LRU for CLI, Redis interface ready

**Files to create:**
- `internal/cache/cache.go` - Interface
- `internal/cache/memory.go` - LRU implementation
- `internal/cache/redis.go` - Redis implementation (can be stub for now)
- `internal/cache/keys.go` - Key generation + normalization

**Validation:**
```bash
# Run same analysis twice, second should be instant
time ./bin/analyzer analyze https://example.com
time ./bin/analyzer analyze https://example.com  # <10ms
```

**Deliverable:** In-memory caching working

---

### Phase 7: HTTP Server + REST API (2-3 hours)
**Goal:** Web service with REST endpoints

**Files to create:**
- `cmd/serve.go` - Serve subcommand
- `internal/server/server.go` - HTTP server setup
- `internal/server/middleware.go` - Logging, recovery
- `internal/server/router.go` - Route definitions
- `internal/presentation/rest/handler.go` - HTTP handlers
- `internal/presentation/rest/analyze.go` - POST /api/analyze
- `internal/presentation/rest/jobs.go` - GET /api/jobs/:id
- `internal/presentation/rest/health.go` - GET /api/health
- `internal/presentation/rest/dto.go` - Request/response types

**Validation:**
```bash
./bin/analyzer serve &
curl http://localhost:8080/api/health
curl -X POST http://localhost:8080/api/analyze \
  -H "Content-Type: application/json" \
  -d '{"url":"https://example.com"}' | jq .
```

**Deliverable:** REST API fully functional

---

### Phase 8: Web UI (2-3 hours)
**Goal:** Simple HTML form + results page

**Files to create:**
- `internal/presentation/web/handler.go` - Form handlers
- `internal/presentation/web/templates/base.html` - Base layout
- `internal/presentation/web/templates/index.html` - Home + form
- `internal/presentation/web/templates/result.html` - Results display
- `internal/presentation/web/static/style.css` - Minimal styling
- `internal/presentation/web/static/app.js` - Polling for async jobs

**Validation:**
```bash
./bin/analyzer serve
open http://localhost:8080
# Submit form, see results
```

**Deliverable:** Complete web application

---

### Phase 9: Configuration (1 hour)
**Goal:** Environment-based config

**Files to create:**
- `internal/config/config.go` - Config struct
- `internal/config/env.go` - Load from environment
- `internal/config/defaults.go` - Default values

**Validation:**
```bash
ANALYZER_CACHE_MODE=disabled ./bin/analyzer serve
ANALYZER_MAX_LINKS=100 ./bin/analyzer analyze https://example.com
```

**Deliverable:** All env vars working

---

### Phase 10: Observability (2-3 hours)
**Goal:** Logging, metrics, tracing

**Files to create:**
- `internal/observability/logger.go` - Slog setup
- `internal/observability/metrics.go` - Prometheus metrics
- `internal/observability/tracing.go` - OTEL setup
- `internal/observability/health.go` - Health check logic

**Validation:**
```bash
./bin/analyzer serve
curl http://localhost:8080/metrics  # Prometheus metrics
```

**Deliverable:** Full observability

---

### Phase 11: Docker & Compose (1-2 hours)
**Goal:** Containerized demo

**Files to create:**
- `Dockerfile`
- `docker-compose.yml`
- `deployments/tempo.yaml`
- `deployments/prometheus.yml`
- `deployments/grafana/datasources/datasources.yml`
- `.dockerignore`

**Validation:**
```bash
docker-compose up --build
open http://localhost:8080
open http://localhost:3000  # Grafana
```

**Deliverable:** Full stack running in Docker

---

### Phase 12: Polish & Documentation (2-3 hours)
**Goal:** Production-ready documentation

**Files to create:**
- `README.md` - Quick start, usage, deployment
- `DECISIONS.md` - Architectural decisions (copy from this doc)
- `IMPROVEMENTS.md` - Future enhancements (copy from this doc)
- `scripts/dev.sh` - Development setup
- `scripts/demo.sh` - Quick demo script

**Validation:**
```bash
# Fresh clone test
git clone <repo>
cd page-analyzer
make dev
make run
make demo
```

**Deliverable:** Complete documentation

---

### Total Estimated Time: 20-30 hours

**Fastest Path to Demo (MVP):**
1. Phase 0-4: **5-7 hours** → Working CLI tool
2. Phase 7: **2-3 hours** → REST API
3. Phase 8: **2-3 hours** → Web UI
4. Phase 11: **1-2 hours** → Docker demo

**Result:** Working demo in **10-15 hours** (Phases 5-6, 9-10, 12 can be added later)

---

## Summary

This document specifies a **production-ready, high-load web page analyzer** with:

✅ **Memory-efficient streaming architecture** (O(1) memory per request)
✅ **Two-phase processing** (fast HTML analysis + async link checking)
✅ **Extensible collector system** (add analysis types without modifying core)
✅ **Multi-layer caching** (Redis + LRU for optimal performance)
✅ **Graceful degradation** (stays available under load)
✅ **Full observability** (OTEL tracing + Prometheus metrics + structured logs)
✅ **Comprehensive testing** (unit + integration + property-based + load tests)
✅ **Local development focus** (Docker Compose, Makefile, quick iteration)

**Implementation Strategy:**
- **MVP:** 10-15 hours (CLI + REST API + Web UI + Docker)
- **Complete:** 20-30 hours (all features + observability + polish)
- **Approach:** TDD, incremental, working demo at each phase

**Ready for implementation. 🚀**
