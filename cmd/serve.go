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
	"github.com/halyph/page-analyzer/internal/presentation/rest"
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

func runServe(cmd *cobra.Command, args []string) error {
	// Setup logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	logger.Info("starting page analyzer server",
		"version", version,
		"addr", flagAddr,
	)

	// Create memory cache (100 entries, 1 hour TTL)
	memCache := cache.NewMemoryCache(100, 1*time.Hour)

	// Create link checker
	linkCheckCfg := analyzer.DefaultLinkCheckConfig()
	linkCheckCfg.UserAgent = fmt.Sprintf("page-analyzer/%s", version)
	linkChecker := analyzer.NewLinkCheckWorkerPool(linkCheckCfg)
	linkChecker.Start()
	defer linkChecker.Stop()

	// Create analyzer service
	serviceCfg := analyzer.ServiceConfig{
		Fetcher: analyzer.FetcherConfig{
			Timeout:     15 * time.Second,
			MaxBodySize: 10 * 1024 * 1024, // 10MB
			UserAgent:   fmt.Sprintf("page-analyzer/%s", version),
		},
		Walker: analyzer.WalkerConfig{
			MaxTokens: 1_000_000, // 1M tokens max
		},
		LinkCheckerPool: linkChecker, // Use the same link checker instance
		Cache:           memCache,
		CacheTTL:        1 * time.Hour,
	}

	analyzerService := analyzer.NewService(serviceCfg)
	defer analyzerService.Stop()

	// Create REST handler
	restHandler := rest.NewHandler(analyzerService, linkChecker, logger)

	// Create router
	router := server.NewRouter(restHandler, logger)

	// Create server
	serverCfg := server.Config{
		Addr:         flagAddr,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
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
