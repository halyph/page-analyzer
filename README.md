# Page Analyzer

High-performance web page analyzer that extracts HTML metadata, analyzes structure, and verifies link accessibility.

Useful for SEO audits, site monitoring, content analysis, and quality assurance. Analyzes any webpage to extract structured data and verify link health.

## Features

- **HTML Analysis**: Version detection, title, headings (H1-H6), login forms
- **Link Analysis**: Internal/external classification, async accessibility checking
- **Multiple Interfaces**: CLI, REST API, Web UI
- **Caching**: Multi-layer (LRU + Redis)
- **Observability**: OpenTelemetry tracing, Prometheus metrics, Grafana dashboards

## Quick Start

### CLI (Fastest)

```bash
make build
# Binary is at: build/<os>/<arch>/analyzer
# Examples:
#   macOS ARM:   ./build/darwin/arm64/analyzer
#   macOS Intel: ./build/darwin/amd64/analyzer
#   Linux x64:   ./build/linux/amd64/analyzer

./build/darwin/arm64/analyzer analyze https://go.dev
```

### Web UI

```bash
make build
# Adjust binary path for your OS/arch (see above)
./build/darwin/arm64/analyzer serve
# Open http://localhost:8080
```

### Full Demo (with Observability)

```bash
make demo
```

**Access:**
- Analyzer: http://localhost:8080
- Jaeger: http://localhost:16686
- Grafana: http://localhost:3000
- Prometheus: http://localhost:9090

**Stop:**
```bash
make demo-down
```

## Usage

### CLI

```bash
analyzer analyze <url>                        # Basic analysis
analyzer analyze <url> --json                 # JSON output
analyzer analyze <url> --check-links          # Check links
analyzer serve                                # Start server
analyzer serve --addr :9090                   # Custom port
```

### REST API

```bash
# Analyze
curl -X POST http://localhost:8080/api/analyze \
  -H "Content-Type: application/json" \
  -d '{"url": "https://go.dev", "options": {"checkLinks": "async"}}'

# Get results
curl http://localhost:8080/api/jobs/{jobId}

# Health
curl http://localhost:8080/api/health
```

### Web UI

Open http://localhost:8080 and enter a URL.

## Demo Mode

Showcases full observability stack with tracing, metrics, and dashboards.

**Start:**
```bash
make demo        # Infra + app
# or
make demo-infra  # Infra only
make demo-run    # App only (in another terminal)
```

**Generate traffic:**
```bash
for url in go.dev github.com golang.org; do
  curl -X POST http://localhost:8080/api/analyze \
    -H "Content-Type: application/json" \
    -d "{\"url\":\"https://$url\",\"options\":{\"checkLinks\":\"async\"}}"
done
```

View traces in Jaeger and metrics in Grafana.

**Commands:**

```bash
make demo-status  # Service status
make demo-logs    # Follow logs
make demo-down    # Stop all
```

## Build

**Prerequisites:** Go 1.25+, Docker (for demo), Make

```bash
make build         # Local OS
make build.linux   # Linux AMD64/ARM64
make build.darwin  # macOS Intel/Apple Silicon
make docker        # Docker image
```

## Configuration

Configure via environment variables. Key settings:

| Variable | Default | Description |
|----------|---------|-------------|
| `ANALYZER_ADDR` | `:8080` | Server address |
| `ANALYZER_CHECK_MODE` | `async` | Link checking: `sync\|async\|hybrid\|disabled` |
| `ANALYZER_CACHE_MODE` | `memory` | Cache: `memory\|redis\|multi\|disabled` |
| `ANALYZER_REDIS_ADDR` | `localhost:6379` | Redis connection (if cache=redis) |
| `ANALYZER_LOG_LEVEL` | `info` | Log level: `debug\|info\|warn\|error` |
| `ANALYZER_TRACING_ENABLED` | `false` | Enable OpenTelemetry tracing |

See `internal/config/config.go` for complete configuration options including timeouts, worker pools, and cache TTLs.

Additional docs: [Design Decisions](docs/DECISIONS.md) | [Known Issues](docs/ISSUES.md) | [Architecture](docs/DIAGRAMS.md)

## Development

```bash
make help      # Show all available commands
make test      # Unit tests + linting
make test-all  # Including integration tests
make cover     # Coverage report
```

## Project Structure

```
cmd/analyzer/      # CLI entry point
internal/          # Core logic (domain, analyzer, cache, server, observability)
deployments/       # Grafana, Prometheus configs
docs/              # Design decisions, issues, diagrams
```

## License

MIT - see [LICENSE](LICENSE) file for details
