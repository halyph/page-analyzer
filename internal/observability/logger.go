package observability

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

type contextKey string

const requestIDKey contextKey = "request_id"

// GenerateRequestID generates a random request ID
func GenerateRequestID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ContextWithRequestID adds a request ID to the context
func ContextWithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// RequestIDFromContext retrieves the request ID from the context
func RequestIDFromContext(ctx context.Context) string {
	if reqID, ok := ctx.Value(requestIDKey).(string); ok {
		return reqID
	}
	return ""
}

// LoggerWithContext creates a logger with context information (request ID, trace ID)
func LoggerWithContext(ctx context.Context, logger *slog.Logger) *slog.Logger {
	attrs := []any{}

	// Add request ID if present
	if reqID := RequestIDFromContext(ctx); reqID != "" {
		attrs = append(attrs, "request_id", reqID)
	}

	// Add trace ID if present
	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.IsValid() {
		attrs = append(attrs,
			"trace_id", spanCtx.TraceID().String(),
			"span_id", spanCtx.SpanID().String(),
		)
	}

	if len(attrs) > 0 {
		return logger.With(attrs...)
	}

	return logger
}
