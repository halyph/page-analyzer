# Deployments

This directory contains all deployment configurations and observability infrastructure for the Page Analyzer application.

## Directory Structure

```
deployments/
├── README.md                      # This file
├── docker-compose.yml             # → ../docker-compose.yml (main compose file)
│
├── otel-collector-config.yaml    # OpenTelemetry Collector configuration
├── prometheus.yml                 # Prometheus (metrics storage) configuration
│
└── grafana/                       # Grafana provisioning
    ├── datasources.yml            # Auto-provision Jaeger + Prometheus datasources
    ├── dashboards.yml             # Auto-provision dashboard loading
    └── dashboards/                # Pre-configured dashboards
        └── metrics.json           # Metrics dashboard (HTTP, cache, analysis)
```

## Pure OTLP Observability Architecture

## Architecture

```
┌─────────────┐
│  Application │
│ (Go binary) │
└──────┬──────┘
       │ OTLP (HTTP/gRPC)
       │ Traces + Metrics
       ▼
┌──────────────────┐
│  OTEL Collector  │ ← Backend-agnostic routing
└────┬────────┬────┘
     │        │
     │        └─────► Prometheus (scrapes metrics)
     │
     └──────────────► Jaeger (receives traces via OTLP gRPC)

                      Grafana ← Queries both backends
```

**Key Benefits:**
- ✅ App code is 100% backend-agnostic (pure OTLP)
- ✅ Single configuration point (OTEL Collector)
- ✅ Easy to swap backends (change collector config, not app code)
- ✅ Production-ready architecture

## Make Targets

| Command | Description |
|---------|-------------|
| `make demo` | Start infrastructure + run analyzer with OTEL |
| `make demo-infra` | Start only infrastructure (OTEL Collector, Jaeger, Prometheus, Grafana, Redis) |
| `make demo-run` | Run analyzer locally with OTEL enabled (sends to collector) |
| `make demo-status` | Show status of running services |
| `make demo-logs` | Follow logs from infrastructure |
| `make demo-down` | Stop infrastructure and remove volumes |

## Quick Start

### 1. Start Infrastructure

From the project root, run:

```bash
make demo-infra
```

This starts:
- **OpenTelemetry Collector** (ports 4318/4317) - OTLP receiver and router
- **Jaeger** (port 16686) - Tracing backend (receives from collector)
- **Prometheus** (port 9090) - Metrics storage (scrapes collector)
- **Grafana** (port 3000) - Unified observability UI
- **Redis** (port 6379) - Cache backend

### 2. Run the Analyzer

In a separate terminal:

```bash
make demo-run
```

This:
- Builds the binary for your local OS
- Starts the analyzer on port 8080 with:
  - OpenTelemetry tracing enabled
  - Redis cache
  - Metrics collection

The analyzer runs locally and connects to the Docker infrastructure.

### 3. Alternative: One Command

```bash
make demo
```

Starts infrastructure in the background, then runs the analyzer in the foreground.

### 4. View Demo Status

```bash
# Check infrastructure status
make demo-status

# Follow infrastructure logs
make demo-logs
```

### 5. Stop the Demo

```bash
# Stop infrastructure
make demo-down

# Stop analyzer: Ctrl+C in the terminal running `make demo-run`
```

### 5. Generate Some Traffic

```bash
# Analyze a page (creates traces)
curl -X POST http://localhost:8080/api/analyze \
  -H "Content-Type: application/json" \
  -d '{"url": "https://example.com", "options": {"check_links": "async"}}'

# Analyze multiple pages
for url in https://example.com https://golang.org https://github.com; do
  curl -X POST http://localhost:8080/api/analyze \
    -H "Content-Type: application/json" \
    -d "{\"url\": \"$url\"}"
  sleep 1
done
```

### 6. View Traces and Metrics in Grafana

Open **Grafana** at http://localhost:3000

**Pre-configured Dashboards:**

1. **Page Analyzer - Metrics** (`/d/page-analyzer-metrics`)
   - HTTP request rate and duration (p50, p95)
   - Active requests gauge
   - Cache hit rate gauge
   - Cache operations timeline (hits vs misses)
   - Link check rate
   - Analysis duration percentiles

**For traces, use Jaeger UI directly:** http://localhost:16686

**Direct Access:**
- **Jaeger UI**: http://localhost:16686 - Best for exploring traces
  - Service: page-analyzer
  - Operation: analyzer.Analyze, fetcher.Fetch, etc.
  - Tags: analyzer.cached, http.status_code, etc.

- **Prometheus**: http://localhost:9090 - Query metrics directly
  - Request rate: `rate(page_analyzer_http_server_request_count_total[5m])`
  - Cache hit rate: `rate(page_analyzer_analyzer_cache_hits_total[5m]) / (rate(page_analyzer_analyzer_cache_hits_total[5m]) + rate(page_analyzer_analyzer_cache_misses_total[5m]))`

## Understanding the Traces

### Trace Hierarchy

A typical page analysis creates this span tree:

```
analyzer.Analyze                    [root span]
├─ cache.GetHTML                   [cache lookup]
├─ analyzer.fetchAndAnalyze        [if cache miss]
│  ├─ fetcher.Fetch                [HTTP GET request]
│  ├─ walker.Walk                  [HTML parsing]
│  └─ cache.SetHTML                [store result]
└─ linkChecker.Submit              [optional, if check_links enabled]
```

### Key Span Attributes

**analyzer.Analyze**:
- `analyzer.url` - Target URL
- `analyzer.cached` - Whether result was cached
- `check_links` - Link checking mode

**fetcher.Fetch**:
- `http.url` - Target URL
- `http.method` - HTTP method (GET)
- `http.status_code` - Response status
- `http.response.body.size` - Response size in bytes
- `http.redirect.final_url` - Final URL after redirects (if different)

**cache.GetHTML/SetHTML**:
- `cache.operation` - get or set
- `cache.hit` - true/false
- `cache.layer` - memory, redis, etc.

**linkChecker.Submit**:
- `link_checker.url_count` - Number of URLs to check

## Viewing Traces in Jaeger

Open **Jaeger UI** at http://localhost:16686

1. Select **Service**: `page-analyzer`
2. Select **Operation** (optional): `analyzer.Analyze`, `fetcher.Fetch`, etc.
3. Add **Tags** (optional): `analyzer.cached`, `http.status_code`, etc.
4. Click **Find Traces**
5. Click on a trace to see the full waterfall view with spans and timing

The Jaeger UI provides the best experience for trace exploration with detailed span timing, dependencies, and flame graphs.

## Metrics (Internal Collection)

The application collects metrics internally. Current metrics:

- `http.server.request.count` - HTTP requests by endpoint and status
- `http.server.request.duration` - Request latency histogram
- `http.server.active_requests` - Active request gauge
- `analyzer.cache.hits` - Cache hits by type
- `analyzer.cache.misses` - Cache misses by type
- `analyzer.links.checked` - Links checked counter
- `analyzer.analysis.duration` - Analysis time histogram

## Configuration Reference

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `ANALYZER_OTEL_ENABLED` | `false` | Enable OpenTelemetry tracing |
| `ANALYZER_OTEL_ENDPOINT` | `localhost:4318` | OTLP HTTP endpoint (OTEL Collector) |
| `ANALYZER_METRICS_ENABLED` | `true` | Enable OTLP metrics export |
| `ANALYZER_LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |
| `ANALYZER_LOG_FORMAT` | `json` | Log format (json, text) |

### OTEL Collector Endpoints

- **HTTP**: `http://localhost:4318` (OTLP/HTTP) ← App sends here
- **gRPC**: `localhost:4317` (OTLP/gRPC)
- **Metrics**: `http://localhost:8889/metrics` (Prometheus exporter)

## Troubleshooting

### No traces appearing in Grafana

1. Check OTEL is enabled:
   ```bash
   curl http://localhost:8080/api/health | jq
   ```

2. Check Jaeger is receiving data:
   ```bash
   docker logs page-analyzer-jaeger
   ```

3. Verify endpoint is correct:
   ```bash
   echo $ANALYZER_OTEL_ENDPOINT
   # Should be: localhost:4318
   ```

### Application logs show OTEL errors

Check the logs:
```bash
docker logs page-analyzer-jaeger
```

Ensure Jaeger is running:
```bash
docker ps | grep jaeger
```

### Traces are incomplete

- Ensure context propagation is working
- Check for errors in application logs
- Verify all spans are properly closed (`defer span.End()`)

## Cleanup

Stop and remove all containers:

```bash
make demo-down
```

## Next Steps

### Add Prometheus /metrics Endpoint

To enable Prometheus scraping (pull model):

1. Add Prometheus exporter bridge in `observability/otel.go`
2. Add `/metrics` endpoint in router
3. Update `prometheus.yml` to scrape the analyzer

### Add Custom Dashboards

1. Create Grafana dashboards with Prometheus queries
2. Save as JSON in `observability/grafana/dashboards/`
3. Restart Grafana to load

### Production Deployment

For production, consider:
- OpenTelemetry Collector as a sidecar/agent
- Dedicated tracing backend (Grafana Cloud, Jaeger with Elasticsearch/Cassandra storage)
- Metrics aggregation (Prometheus + Thanos or Mimir)
- Security: TLS for OTLP endpoints
- Sampling: Reduce trace volume with head/tail sampling

## Resources

- [OpenTelemetry Go SDK](https://opentelemetry.io/docs/instrumentation/go/)
- [Jaeger Documentation](https://www.jaegertracing.io/docs/)
- [Grafana Documentation](https://grafana.com/docs/grafana/latest/)
- [OTLP Specification](https://opentelemetry.io/docs/specs/otlp/)
