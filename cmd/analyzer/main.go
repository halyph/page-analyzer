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
	"github.com/halyph/page-analyzer/internal/domain"
	cliformatter "github.com/halyph/page-analyzer/internal/presentation/cli"
	"github.com/halyph/page-analyzer/internal/presentation/rest"
	"github.com/halyph/page-analyzer/internal/presentation/web"
	"github.com/halyph/page-analyzer/internal/server"
	"github.com/urfave/cli/v2"
)

var (
	Version = "dev"
	GitHead = "unknown"
)

func main() {
	app := &cli.App{
		Name:    "analyzer",
		Usage:   "Page Analyzer - Analyze webpage structure and content",
		Version: fmt.Sprintf("%s (commit: %s)", Version, GitHead),
		Commands: []*cli.Command{
			{
				Name:  "analyze",
				Usage: "Analyze a webpage",
				UsageText: `analyzer analyze [options] URL

Examples:
  analyzer analyze https://example.com
  analyzer analyze https://example.com --json
  analyzer analyze https://example.com --check-links`,
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:  "json",
						Usage: "Output results as JSON",
					},
					&cli.BoolFlag{
						Name:  "check-links",
						Usage: "Check link accessibility (synchronous)",
					},
					&cli.IntFlag{
						Name:  "max-links",
						Usage: "Maximum number of links to collect",
						Value: 10000,
					},
					&cli.DurationFlag{
						Name:  "timeout",
						Usage: "HTTP request timeout",
						Value: 15 * time.Second,
					},
				},
				Action: runAnalyze,
			},
			{
				Name:  "serve",
				Usage: "Start the HTTP server",
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
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runAnalyze(c *cli.Context) error {
	if c.NArg() != 1 {
		return fmt.Errorf("URL argument required")
	}
	url := c.Args().First()

	// Create memory cache for CLI (LRU cache with 100 entries, 1 hour TTL)
	memCache := cache.NewMemoryCache(100, 1*time.Hour)

	// Create analyzer service
	serviceCfg := analyzer.ServiceConfig{
		Fetcher: analyzer.FetcherConfig{
			Timeout:     c.Duration("timeout"),
			MaxBodySize: 10 * 1024 * 1024, // 10MB
			UserAgent:   fmt.Sprintf("page-analyzer/%s", Version),
		},
		Walker: analyzer.WalkerConfig{
			MaxTokens: 1_000_000, // 1M tokens max
		},
		Cache:    memCache,
		CacheTTL: 1 * time.Hour,
	}

	// Enable link checking if requested
	if c.Bool("check-links") {
		linkCheckCfg := analyzer.DefaultLinkCheckConfig()
		linkCheckCfg.UserAgent = fmt.Sprintf("page-analyzer/%s", Version)
		serviceCfg.LinkChecker = &linkCheckCfg
	}

	service := analyzer.NewService(serviceCfg)
	defer service.Stop()

	// Build analysis request
	checkMode := domain.LinkCheckDisabled
	if c.Bool("check-links") {
		checkMode = domain.LinkCheckSync
	}

	req := domain.AnalysisRequest{
		URL: url,
		Options: domain.AnalysisOptions{
			CheckLinks: checkMode,
			MaxLinks:   c.Int("max-links"),
			Timeout:    30 * time.Second, // Timeout for link checking
		},
	}

	// Perform analysis
	ctx := context.Background()
	result, err := service.Analyze(ctx, req)
	if err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}

	// Format output
	var output string
	if c.Bool("json") {
		jsonOutput, err := cliformatter.FormatJSON(result)
		if err != nil {
			return fmt.Errorf("failed to format JSON: %w", err)
		}
		output = jsonOutput
	} else {
		output = cliformatter.FormatTable(result)
	}

	fmt.Print(output)
	return nil
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

func runServe(c *cli.Context) error {
	// Load configuration from environment
	cfg, err := config.LoadFromEnv()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Override addr from flag if provided
	if c.IsSet("addr") {
		cfg.Server.Addr = c.String("addr")
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
		"version", Version,
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
