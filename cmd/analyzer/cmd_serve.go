package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/halyph/page-analyzer/internal/analyzer"
	"github.com/halyph/page-analyzer/internal/cache"
	"github.com/halyph/page-analyzer/internal/config"
	"github.com/halyph/page-analyzer/internal/observability"
	"github.com/halyph/page-analyzer/internal/presentation/rest"
	"github.com/halyph/page-analyzer/internal/presentation/web"
	"github.com/halyph/page-analyzer/internal/server"
	"github.com/urfave/cli/v2"
)

func newServeCommand() *cli.Command {
	return &cli.Command{
		Name:    "serve",
		Aliases: []string{"s"},
		Usage:   "Start the HTTP server",
		UsageText: `analyzer serve [options]

The server provides:
  - POST /api/analyze - Analyze a webpage
  - GET /api/jobs/:id - Get link check job status
  - GET /api/health   - Health check

Examples:
  analyzer serve
  analyzer serve --addr :9090`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "addr",
				Usage: "Server address",
				Value: ":8080",
			},
		},
		Action: runServe,
	}
}

func runServe(c *cli.Context) error {
	// Load configuration
	cfg := config.Load()

	if c.IsSet("addr") {
		cfg.Server.Addr = c.String("addr")
	}

	// Setup logger
	logger := setupLogger(cfg)
	logger.Info("starting page analyzer server",
		"version", Version,
		"addr", cfg.Server.Addr,
		"cache_mode", cfg.Caching.Mode,
		"link_check_mode", cfg.LinkChecking.CheckMode,
	)

	// Initialize OpenTelemetry
	ctx := context.Background()
	otelShutdown, err := observability.InitOTEL(ctx, observability.OTELConfig{
		ServiceName:    "page-analyzer",
		ServiceVersion: Version,
		Endpoint:       cfg.Observability.OTELEndpoint,
		Enabled:        cfg.Observability.TracingEnabled,
		Logger:         logger,
	})
	if err != nil {
		logger.Error("failed to initialize OpenTelemetry", "error", err)
		// Continue without OTEL rather than failing
	}
	defer func() {
		if otelShutdown != nil {
			if err := otelShutdown(context.Background()); err != nil {
				logger.Error("failed to shutdown OpenTelemetry", "error", err)
			}
		}
	}()

	// Initialize metrics
	var metrics *observability.Metrics
	if cfg.Observability.MetricsEnabled {
		metrics, err = observability.NewMetrics()
		if err != nil {
			logger.Error("failed to initialize metrics", "error", err)
			// Continue without metrics
		} else {
			logger.Info("metrics initialized")
		}
	}

	// Create dependencies
	cacheImpl := createCache(cfg, logger)
	defer cacheImpl.Close()

	linkChecker := createLinkChecker(cfg, cacheImpl, logger)
	analyzerService := createService(cfg, cacheImpl, linkChecker, logger)
	defer analyzerService.Stop()

	// Create health checker
	healthChecker := observability.NewHealthChecker(cacheImpl)

	// Create HTTP handlers
	restHandler := rest.NewHandler(analyzerService, linkChecker, healthChecker, logger, Version, GitHead)
	webHandler, err := web.NewHandler(analyzerService, logger, Version, GitHead)
	if err != nil {
		return fmt.Errorf("failed to create web handler: %w", err)
	}

	// Create and start server
	srv := createServer(cfg, restHandler, webHandler, metrics, logger)

	return runServerWithGracefulShutdown(srv, cfg, logger)
}

// parseLogLevel converts a string log level to slog.Level
func parseLogLevel(level string) slog.Level {
	switch level {
	case config.LogLevelDebug:
		return slog.LevelDebug
	case config.LogLevelInfo:
		return slog.LevelInfo
	case config.LogLevelWarn:
		return slog.LevelWarn
	case config.LogLevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func setupLogger(cfg config.Config) *slog.Logger {
	logLevel := parseLogLevel(cfg.Observability.LogLevel)
	var handler slog.Handler

	if cfg.Observability.LogFormat == config.LogFormatText {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	} else {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	}

	return slog.New(handler)
}

func createCache(cfg config.Config, logger *slog.Logger) cache.Cache {
	switch cfg.Caching.Mode {
	case config.CacheModeDisabled:
		logger.Info("cache disabled")
		return cache.NewNoOpCache()
	case config.CacheModeMemory:
		logger.Info("using memory cache", "size", cfg.Caching.MemoryCacheSize)
		return cache.NewMemoryCache(cfg.Caching.MemoryCacheSize)
	case config.CacheModeRedis:
		logger.Info("using redis cache", "addr", cfg.Caching.RedisAddr)
		redisCache, err := cache.NewRedisCache(cfg.Caching.RedisAddr)
		if err != nil {
			logger.Error("failed to create redis cache, falling back to memory", "error", err)
			return cache.NewMemoryCache(cfg.Caching.MemoryCacheSize)
		}
		return redisCache
	case config.CacheModeMulti:
		logger.Info("using multi-tier cache (L1=memory, L2=redis)",
			"l1_size", cfg.Caching.MemoryCacheSize,
			"l2_addr", cfg.Caching.RedisAddr)
		// Create L1 (memory)
		l1 := cache.NewMemoryCache(cfg.Caching.MemoryCacheSize)
		// Create L2 (redis)
		l2, err := cache.NewRedisCache(cfg.Caching.RedisAddr)
		if err != nil {
			logger.Error("failed to create redis L2 cache, using memory only", "error", err)
			return l1
		}
		return cache.NewMultiCache(l1, l2)
	default:
		logger.Warn("unknown cache mode, using memory", "mode", cfg.Caching.Mode)
		return cache.NewMemoryCache(cfg.Caching.MemoryCacheSize)
	}
}

func createLinkChecker(cfg config.Config, cacheImpl cache.Cache, logger *slog.Logger) *analyzer.LinkCheckWorkerPool {
	linkCheckCfg := analyzer.LinkCheckConfig{
		Timeout:      cfg.LinkChecking.CheckTimeout,
		Workers:      cfg.LinkChecking.Workers,
		QueueSize:    cfg.LinkChecking.QueueSize,
		JobMaxAge:    cfg.LinkChecking.JobMaxAge,
		JobWorkers:   cfg.LinkChecking.JobWorkers,
		Cache:        cacheImpl,
		LinkCacheTTL: cfg.Caching.LinkCacheTTL,
	}
	linkChecker := analyzer.NewLinkCheckWorkerPool(linkCheckCfg)

	if cfg.LinkChecking.CheckMode != config.LinkCheckModeDisabled {
		linkChecker.Start()
		logger.Info("link checker started", "workers", cfg.LinkChecking.Workers)
	} else {
		logger.Info("link checking disabled")
	}

	return linkChecker
}

func createService(cfg config.Config, cacheImpl cache.Cache, linkChecker *analyzer.LinkCheckWorkerPool, logger *slog.Logger) *analyzer.Service {
	serviceCfg := analyzer.ServiceConfig{
		Fetcher:         cfg.Fetching,
		Walker:          cfg.Processing,
		LinkCheckerPool: linkChecker,
		Cache:           cacheImpl,
		PageCacheTTL:    cfg.Caching.PageCacheTTL,
		Logger:          logger,
	}

	return analyzer.NewService(serviceCfg)
}

func createServer(cfg config.Config, restHandler *rest.Handler, webHandler *web.Handler, metrics *observability.Metrics, logger *slog.Logger) *server.Server {
	router := server.NewRouter(restHandler, webHandler, metrics, logger)

	serverCfg := server.Config{
		Addr:         cfg.Server.Addr,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	return server.New(router, serverCfg, logger)
}

func runServerWithGracefulShutdown(srv *server.Server, cfg config.Config, logger *slog.Logger) error {
	errChan := make(chan error, 1)
	go func() {
		if err := srv.Start(); err != nil {
			errChan <- err
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errChan:
		return fmt.Errorf("server failed to start: %w", err)
	case sig := <-sigChan:
		logger.Info("received signal, shutting down", "signal", sig)

		ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			return fmt.Errorf("server shutdown failed: %w", err)
		}

		logger.Info("server stopped gracefully")
	}

	return nil
}
