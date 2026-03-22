package observability

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "github.com/halyph/page-analyzer"

// Tracer returns the application tracer
func Tracer() trace.Tracer {
	return otel.Tracer(tracerName)
}

// StartSpan starts a new span with the given name
func StartSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	return Tracer().Start(ctx, name, trace.WithAttributes(attrs...))
}

// RecordError records an error on the current span
func RecordError(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
}

// SetSpanAttributes sets attributes on the current span
func SetSpanAttributes(span trace.Span, attrs ...attribute.KeyValue) {
	span.SetAttributes(attrs...)
}

// Common attribute keys for semantic conventions
var (
	AttrHTTPMethod     = attribute.Key("http.method")
	AttrHTTPStatusCode = attribute.Key("http.status_code")
	AttrHTTPURL        = attribute.Key("http.url")
	AttrHTTPRoute      = attribute.Key("http.route")

	AttrAnalyzerURL       = attribute.Key("analyzer.url")
	AttrAnalyzerCached    = attribute.Key("analyzer.cached")
	AttrAnalyzerLinkCount = attribute.Key("analyzer.link_count")

	AttrCacheOperation = attribute.Key("cache.operation")
	AttrCacheHit       = attribute.Key("cache.hit")
	AttrCacheLayer     = attribute.Key("cache.layer")

	AttrLinkCheckerJobID    = attribute.Key("link_checker.job_id")
	AttrLinkCheckerURLCount = attribute.Key("link_checker.url_count")
	AttrLinkCheckerStatus   = attribute.Key("link_checker.status")

	AttrCollectorName = attribute.Key("collector.name")
)

// SpanFromContext returns the current span from the context
func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}
