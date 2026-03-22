package observability

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const meterName = "github.com/halyph/page-analyzer"

// Metrics holds all application metrics
type Metrics struct {
	// HTTP metrics
	httpRequestCount    metric.Int64Counter
	httpRequestDuration metric.Float64Histogram
	httpActiveRequests  metric.Int64UpDownCounter

	// Cache metrics
	cacheHits   metric.Int64Counter
	cacheMisses metric.Int64Counter

	// Link checking metrics
	linksChecked metric.Int64Counter
	queueSize    metric.Int64ObservableGauge

	// Analysis metrics
	analysisDuration metric.Float64Histogram
}

// NewMetrics creates and registers all application metrics
func NewMetrics() (*Metrics, error) {
	meter := otel.Meter(meterName)

	// HTTP metrics
	httpRequestCount, err := meter.Int64Counter(
		"http.server.request.count",
		metric.WithDescription("Number of HTTP requests received"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create http.server.request.count: %w", err)
	}

	httpRequestDuration, err := meter.Float64Histogram(
		"http.server.request.duration",
		metric.WithDescription("HTTP request duration"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create http.server.request.duration: %w", err)
	}

	httpActiveRequests, err := meter.Int64UpDownCounter(
		"http.server.active_requests",
		metric.WithDescription("Number of active HTTP requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create http.server.active_requests: %w", err)
	}

	// Cache metrics
	cacheHits, err := meter.Int64Counter(
		"analyzer.cache.hits",
		metric.WithDescription("Number of cache hits"),
		metric.WithUnit("{hit}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create analyzer.cache.hits: %w", err)
	}

	cacheMisses, err := meter.Int64Counter(
		"analyzer.cache.misses",
		metric.WithDescription("Number of cache misses"),
		metric.WithUnit("{miss}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create analyzer.cache.misses: %w", err)
	}

	// Link checking metrics
	linksChecked, err := meter.Int64Counter(
		"analyzer.links.checked",
		metric.WithDescription("Number of links checked"),
		metric.WithUnit("{link}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create analyzer.links.checked: %w", err)
	}

	queueSize, err := meter.Int64ObservableGauge(
		"analyzer.links.queue_size",
		metric.WithDescription("Current link check queue size"),
		metric.WithUnit("{job}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create analyzer.links.queue_size: %w", err)
	}

	// Analysis metrics
	analysisDuration, err := meter.Float64Histogram(
		"analyzer.analysis.duration",
		metric.WithDescription("Time taken to analyze a page"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create analyzer.analysis.duration: %w", err)
	}

	return &Metrics{
		httpRequestCount:    httpRequestCount,
		httpRequestDuration: httpRequestDuration,
		httpActiveRequests:  httpActiveRequests,
		cacheHits:           cacheHits,
		cacheMisses:         cacheMisses,
		linksChecked:        linksChecked,
		queueSize:           queueSize,
		analysisDuration:    analysisDuration,
	}, nil
}

// RecordHTTPRequest records an HTTP request
func (m *Metrics) RecordHTTPRequest(ctx context.Context, endpoint string, statusCode int, duration float64) {
	attrs := metric.WithAttributes(
		attribute.String("endpoint", endpoint),
		attribute.Int("status_code", statusCode),
	)

	m.httpRequestCount.Add(ctx, 1, attrs)
	m.httpRequestDuration.Record(ctx, duration, attrs)
}

// IncActiveRequests increments the active requests counter
func (m *Metrics) IncActiveRequests(ctx context.Context) {
	m.httpActiveRequests.Add(ctx, 1)
}

// DecActiveRequests decrements the active requests counter
func (m *Metrics) DecActiveRequests(ctx context.Context) {
	m.httpActiveRequests.Add(ctx, -1)
}

// RecordCacheHit records a cache hit
func (m *Metrics) RecordCacheHit(ctx context.Context, cacheType string) {
	m.cacheHits.Add(ctx, 1, metric.WithAttributes(
		attribute.String("cache_type", cacheType),
	))
}

// RecordCacheMiss records a cache miss
func (m *Metrics) RecordCacheMiss(ctx context.Context, cacheType string) {
	m.cacheMisses.Add(ctx, 1, metric.WithAttributes(
		attribute.String("cache_type", cacheType),
	))
}

// RecordLinksChecked records the number of links checked
func (m *Metrics) RecordLinksChecked(ctx context.Context, count int, accessible int, inaccessible int) {
	m.linksChecked.Add(ctx, int64(count), metric.WithAttributes(
		attribute.Int("accessible", accessible),
		attribute.Int("inaccessible", inaccessible),
	))
}

// RecordAnalysisDuration records the time taken to analyze a page
func (m *Metrics) RecordAnalysisDuration(ctx context.Context, duration float64, cached bool) {
	m.analysisDuration.Record(ctx, duration, metric.WithAttributes(
		attribute.Bool("cached", cached),
	))
}
