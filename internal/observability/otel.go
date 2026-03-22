package observability

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// OTELConfig holds OpenTelemetry configuration
type OTELConfig struct {
	ServiceName    string
	ServiceVersion string
	Endpoint       string // OTLP HTTP endpoint (e.g., "localhost:4318")
	Enabled        bool
	Logger         *slog.Logger
}

// OTELShutdown is a function that shuts down OTEL resources
type OTELShutdown func(context.Context) error

// InitOTEL initializes OpenTelemetry with metrics and tracing
func InitOTEL(ctx context.Context, cfg OTELConfig) (OTELShutdown, error) {
	if !cfg.Enabled {
		// Return no-op shutdown when disabled
		return func(context.Context) error { return nil }, nil
	}

	// Create resource with service information
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.ServiceName),
			semconv.ServiceVersionKey.String(cfg.ServiceVersion),
		),
		resource.WithProcess(),
		resource.WithOS(),
		resource.WithContainer(),
		resource.WithHost(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTEL resource: %w", err)
	}

	// Initialize TracerProvider
	tracerShutdown, err := initTracing(ctx, res, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize tracing: %w", err)
	}

	// Initialize MeterProvider
	meterShutdown, err := initMetrics(ctx, res, cfg)
	if err != nil {
		// Clean up tracer if meter initialization fails
		_ = tracerShutdown(ctx)
		return nil, fmt.Errorf("failed to initialize metrics: %w", err)
	}

	if cfg.Logger != nil {
		cfg.Logger.Info("OpenTelemetry initialized",
			"service", cfg.ServiceName,
			"version", cfg.ServiceVersion,
			"endpoint", cfg.Endpoint,
		)
	}

	// Return combined shutdown function
	return func(ctx context.Context) error {
		var err error
		if meterErr := meterShutdown(ctx); meterErr != nil {
			err = fmt.Errorf("meter shutdown error: %w", meterErr)
		}
		if tracerErr := tracerShutdown(ctx); tracerErr != nil {
			if err != nil {
				err = fmt.Errorf("%v; tracer shutdown error: %w", err, tracerErr)
			} else {
				err = fmt.Errorf("tracer shutdown error: %w", tracerErr)
			}
		}
		return err
	}, nil
}

// initTracing initializes the OpenTelemetry TracerProvider
func initTracing(ctx context.Context, res *resource.Resource, cfg OTELConfig) (OTELShutdown, error) {
	// Create OTLP trace exporter
	traceExporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(cfg.Endpoint),
		otlptracehttp.WithInsecure(), // Use insecure for local development
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	// Create TracerProvider with batch span processor
	tracerProvider := trace.NewTracerProvider(
		trace.WithResource(res),
		trace.WithBatcher(traceExporter,
			trace.WithBatchTimeout(5*time.Second),
			trace.WithMaxExportBatchSize(512),
		),
	)

	// Set global TracerProvider
	otel.SetTracerProvider(tracerProvider)

	// Set global propagator for context propagation (W3C Trace Context)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tracerProvider.Shutdown, nil
}

// initMetrics initializes the OpenTelemetry MeterProvider
func initMetrics(ctx context.Context, res *resource.Resource, cfg OTELConfig) (OTELShutdown, error) {
	// Create OTLP metric exporter
	metricExporter, err := otlpmetrichttp.New(ctx,
		otlpmetrichttp.WithEndpoint(cfg.Endpoint),
		otlpmetrichttp.WithInsecure(), // Use insecure for local development
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create metric exporter: %w", err)
	}

	// Create MeterProvider with periodic reader
	meterProvider := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(metric.NewPeriodicReader(metricExporter,
			metric.WithInterval(10*time.Second), // Export every 10 seconds
		)),
	)

	// Set global MeterProvider
	otel.SetMeterProvider(meterProvider)

	return meterProvider.Shutdown, nil
}
