package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/halyph/page-analyzer/internal/analyzer"
	"github.com/halyph/page-analyzer/internal/cache"
	"github.com/halyph/page-analyzer/internal/config"
	"github.com/halyph/page-analyzer/internal/presentation/rest"
	"github.com/halyph/page-analyzer/internal/presentation/web"
	"github.com/halyph/page-analyzer/internal/server"
	"github.com/spf13/cobra"
)

var (
	flagAddr string
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the HTTP server",
	Long: `Start the HTTP server with REST API endpoints.

The server provides:
  - POST /api/analyze - Analyze a webpage
  - GET /api/jobs/:id - Get link check job status
  - GET /api/health   - Health check

Examples:
  analyzer serve
  analyzer serve --addr :9090`,
	RunE: runServe,
}

func init() {
	serveCmd.Flags().StringVar(&flagAddr, "addr", ":8080", "Server address")

	rootCmd.AddCommand(serveCmd)
}

// parseLogLevel converts a string log level to slog.Level
func parseLogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func runServe(cmd *cobra.Command, args []string) error {
	// Load configuration from environment
	cfg, err := config.LoadFromEnv()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Override addr from flag if provided
	if cmd.Flags().Changed("addr") {
		cfg.Server.Addr = flagAddr
	}

	// Setup logger based on config
	logLevel := parseLogLevel(cfg.Observability.LogLevel)
	var handler slog.Handler
	if cfg.Observability.LogFormat == "text" {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	} else {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	}
	logger := slog.New(handler)

	logger.Info("starting page analyzer server",
		"version", version,
		"addr", cfg.Server.Addr,
		"cache_mode", cfg.Caching.Mode,
		"link_check_mode", cfg.LinkChecking.CheckMode,
	)

	// Create cache based on config
	var cacheImpl cache.Cache
	switch cfg.Caching.Mode {
	case "disabled":
		cacheImpl = cache.NewNoOpCache()
		logger.Info("cache disabled")
	case "memory":
		cacheImpl = cache.NewMemoryCache(cfg.Caching.MemoryCacheSize, cfg.Caching.TTL)
		logger.Info("using memory cache", "size", cfg.Caching.MemoryCacheSize)
	default:
		// Default to memory cache
		cacheImpl = cache.NewMemoryCache(cfg.Caching.MemoryCacheSize, cfg.Caching.TTL)
		logger.Warn("unknown cache mode, using memory", "mode", cfg.Caching.Mode)
	}

	// Create link checker based on config
	linkCheckCfg := analyzer.LinkCheckConfig{
		Timeout:    cfg.LinkChecking.CheckTimeout,
		Workers:    cfg.LinkChecking.Workers,
		QueueSize:  cfg.LinkChecking.QueueSize,
		JobMaxAge:  cfg.Caching.LinkCacheTTL,
		UserAgent:  cfg.Fetching.UserAgent,
		JobWorkers: 10, // Concurrent checks within a job
	}
	linkChecker := analyzer.NewLinkCheckWorkerPool(linkCheckCfg)

	// Only start link checker if not disabled
	if cfg.LinkChecking.CheckMode != "disabled" {
		linkChecker.Start()
		defer linkChecker.Stop()
		logger.Info("link checker started", "workers", cfg.LinkChecking.Workers)
	} else {
		logger.Info("link checking disabled")
	}

	// Create analyzer service
	serviceCfg := analyzer.ServiceConfig{
		Fetcher: analyzer.FetcherConfig{
			Timeout:     cfg.Fetching.Timeout,
			MaxBodySize: cfg.Fetching.MaxBodySize,
			UserAgent:   cfg.Fetching.UserAgent,
		},
		Walker: analyzer.WalkerConfig{
			MaxTokens: 1_000_000, // 1M tokens max
		},
		LinkCheckerPool: linkChecker,
		Cache:           cacheImpl,
		CacheTTL:        cfg.Caching.TTL,
	}

	analyzerService := analyzer.NewService(serviceCfg)
	defer analyzerService.Stop()

	// Create REST handler
	restHandler := rest.NewHandler(analyzerService, linkChecker, logger)

	// Create Web UI handler
	webHandler, err := web.NewHandler(analyzerService, logger)
	if err != nil {
		return fmt.Errorf("failed to create web handler: %w", err)
	}

	// Create router
	router := server.NewRouter(restHandler, webHandler, logger)

	// Create server
	serverCfg := server.Config{
		Addr:         cfg.Server.Addr,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  60 * time.Second,
	}

	srv := server.New(router, serverCfg, logger)

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := srv.Start(); err != nil {
			errChan <- err
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errChan:
		return fmt.Errorf("server failed to start: %w", err)
	case sig := <-sigChan:
		logger.Info("received signal, shutting down", "signal", sig)

		// Graceful shutdown with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			return fmt.Errorf("server shutdown failed: %w", err)
		}

		logger.Info("server stopped gracefully")
	}

	return nil
}
