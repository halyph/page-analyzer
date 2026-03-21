# Page Analyzer

A high-performance web page analyzer built in Go that extracts HTML metadata, analyzes structure, and verifies link accessibility.

## Features

- ✅ **HTML Analysis**: Version detection, title, headings (H1-H6), login form detection
- ✅ **Link Analysis**: Internal/external classification, accessibility checking
- ✅ **Memory Efficient**: Streaming HTML parsing with O(1) memory usage
- ✅ **High Performance**: Async link checking with worker pool
- ✅ **Multiple Interfaces**: CLI, REST API, and Web UI
- ✅ **Caching**: Multi-layer caching (LRU + Redis)
- ✅ **Observability**: OpenTelemetry tracing, Prometheus metrics, structured logging

## Quick Start

### Prerequisites

- Go 1.25+
- Docker & Docker Compose (for full stack)
- Make (optional, but recommended)

### Option 1: CLI Only (Fastest)

```bash
# Build and run
make build
./bin/analyzer analyze https://example.com

# Or directly with go run
go run cmd/main.go analyze https://example.com --json
```

### Option 2: Full Stack with Docker Compose

```bash
# Start all services (app + redis + grafana + tempo + prometheus)
make docker-up

# Access the application
open http://localhost:8080

# View metrics in Grafana
open http://localhost:3000  # admin/admin

# Stop services
make docker-down
```

### Option 3: Development Mode

```bash
# Start infrastructure only
make dev

# Run app directly (hot reload)
make run

# In another terminal, test it
curl -X POST http://localhost:8080/api/analyze \
  -H "Content-Type: application/json" \
  -d '{"url":"https://example.com"}' | jq .
```

## Usage

### CLI Commands

```bash
# Analyze a URL (table output)
analyzer analyze https://example.com

# JSON output
analyzer analyze https://example.com --json

# With link checking
analyzer analyze https://example.com --check-links

# Custom limits
analyzer analyze https://example.com --max-links 500

# Start HTTP server
analyzer serve

# Custom address
analyzer serve --addr :9090

# Health check
analyzer healthcheck --addr :8080
```

### REST API

#### Analyze a URL

```bash
POST /api/analyze
Content-Type: application/json

{
  "url": "https://example.com",
  "options": {
    "checkLinks": "async",
    "maxLinks": 1000
  }
}
```

#### Get Link Check Results

```bash
GET /api/jobs/{jobId}
```

#### Health Check

```bash
GET /api/health
```

### Web UI

Open http://localhost:8080 in your browser and use the form to analyze any webpage.

## Configuration

All settings can be configured via environment variables:

```bash
# Server
ANALYZER_ADDR=:8080
ANALYZER_READ_TIMEOUT=30s
ANALYZER_WRITE_TIMEOUT=30s

# Fetching
ANALYZER_FETCH_TIMEOUT=15s
ANALYZER_MAX_BODY_SIZE=10485760  # 10MB

# Link Checking
ANALYZER_CHECK_MODE=async  # sync|async|hybrid|disabled
ANALYZER_CHECK_WORKERS=20
ANALYZER_MAX_LINKS=10000

# Caching
ANALYZER_CACHE_MODE=redis  # memory|redis|multi|disabled
ANALYZER_REDIS_ADDR=localhost:6379
ANALYZER_CACHE_TTL=1h

# Observability
ANALYZER_LOG_LEVEL=info
ANALYZER_OTEL_ENABLED=true
ANALYZER_OTEL_ENDPOINT=localhost:4318
```

See [TASK_AND_PLAN.md](TASK_AND_PLAN.md) for complete configuration reference.

## Development

### Available Make Commands

```bash
make help           # Show all available commands
make build          # Build binary
make run            # Run service
make test           # Run tests
make test-coverage  # Generate coverage report
make lint           # Run linters
make fmt            # Format code
make docker-build   # Build Docker image
make docker-up      # Start all services
make dev            # Start infrastructure only
make clean          # Clean artifacts
make install-tools  # Install dev tools
```

### Running Tests

```bash
# All tests
make test

# Unit tests only
make test-unit

# Integration tests
make test-integration

# With coverage
make test-coverage
open coverage.html
```

### Project Structure

```
page-analyzer/
├── cmd/                    # CLI entry points
├── internal/
│   ├── domain/            # Core types and interfaces
│   ├── analyzer/          # Analysis logic
│   │   └── collectors/    # HTML collectors (title, headings, links, etc.)
│   ├── cache/             # Caching implementations
│   ├── presentation/      # UI layers (CLI, REST, Web)
│   ├── server/            # HTTP server
│   ├── config/            # Configuration
│   └── observability/     # Logging, metrics, tracing
├── deployments/           # Docker, Grafana, Prometheus configs
├── scripts/               # Helper scripts
└── test/                  # Test fixtures and integration tests
```

## Architecture

See [TASK_AND_PLAN.md](TASK_AND_PLAN.md) for detailed architecture documentation including:
- Two-phase analysis model
- Collector system design
- Link checking worker pool
- Caching strategy
- Observability setup

## Implementation Status

- [x] Phase 0: Project Setup ✅
- [ ] Phase 1: Domain Layer
- [ ] Phase 2: Core Collectors
- [ ] Phase 3: HTML Analyzer Core
- [ ] Phase 4: CLI Interface
- [ ] Phase 5: Link Checking
- [ ] Phase 6: Caching
- [ ] Phase 7: HTTP Server + REST API
- [ ] Phase 8: Web UI
- [ ] Phase 9: Configuration
- [ ] Phase 10: Observability
- [ ] Phase 11: Docker & Compose
- [ ] Phase 12: Documentation

See [TASK_AND_PLAN.md](TASK_AND_PLAN.md) for detailed roadmap.

## License

MIT

## Contributing

This is a test project. See [TASK_AND_PLAN.md](TASK_AND_PLAN.md) for implementation details.
