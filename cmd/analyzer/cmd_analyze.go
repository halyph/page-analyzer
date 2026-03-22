package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/halyph/page-analyzer/internal/analyzer"
	"github.com/halyph/page-analyzer/internal/cache"
	"github.com/halyph/page-analyzer/internal/config"
	"github.com/halyph/page-analyzer/internal/domain"
	cliformatter "github.com/halyph/page-analyzer/internal/presentation/cli"
	"github.com/urfave/cli/v2"
)

func newAnalyzeCommand() *cli.Command {
	cfg := config.Load()
	return &cli.Command{
		Name:    "analyze",
		Aliases: []string{"a"},
		Usage:   "Analyze a webpage",
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
				Value: cfg.LinkChecking.MaxLinks,
			},
			&cli.DurationFlag{
				Name:  "timeout",
				Usage: "HTTP request timeout",
				Value: cfg.Fetching.Timeout,
			},
		},
		Action: runAnalyze,
	}
}

func runAnalyze(c *cli.Context) error {
	if c.NArg() != 1 {
		return fmt.Errorf("URL argument required")
	}
	url := strings.TrimSpace(c.Args().First())

	cfg := config.Load()

	// Create analyzer service
	service := createAnalyzerService(cfg, c)
	defer service.Stop()

	// Build analysis request
	req := buildAnalysisRequest(cfg, url, c)

	// Perform analysis
	result, err := service.Analyze(context.Background(), req)
	if err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}

	// Format and print output
	output, err := formatOutput(result, c.Bool("json"))
	if err != nil {
		return err
	}

	fmt.Print(output)
	return nil
}

func createAnalyzerService(cfg config.Config, c *cli.Context) *analyzer.Service {
	memCache := cache.NewMemoryCache(cfg.Caching.MemoryCacheSize)

	// Override timeout if specified via CLI flag
	fetchingCfg := cfg.Fetching
	if c.IsSet("timeout") {
		fetchingCfg.Timeout = c.Duration("timeout")
	}

	serviceCfg := analyzer.ServiceConfig{
		Fetcher:      fetchingCfg,
		Walker:       cfg.Processing,
		Cache:        memCache,
		PageCacheTTL: cfg.Caching.PageCacheTTL,
	}

	if c.Bool("check-links") {
		linkCheckCfg := analyzer.LinkCheckConfig{
			Timeout:      cfg.LinkChecking.CheckTimeout,
			Workers:      cfg.LinkChecking.Workers,
			QueueSize:    cfg.LinkChecking.QueueSize,
			JobMaxAge:    cfg.LinkChecking.JobMaxAge,
			JobWorkers:   cfg.LinkChecking.JobWorkers,
			Cache:        memCache,
			LinkCacheTTL: cfg.Caching.LinkCacheTTL,
		}
		serviceCfg.LinkChecker = &linkCheckCfg
	}

	return analyzer.NewService(serviceCfg)
}

func buildAnalysisRequest(cfg config.Config, url string, c *cli.Context) domain.AnalysisRequest {
	checkMode := domain.LinkCheckDisabled
	if c.Bool("check-links") {
		checkMode = domain.LinkCheckSync
	}

	return domain.AnalysisRequest{
		URL: url,
		Options: domain.AnalysisOptions{
			CheckLinks: checkMode,
			MaxLinks:   c.Int("max-links"),
			Timeout:    cfg.LinkChecking.CheckTimeout,
		},
	}
}

func formatOutput(result *domain.AnalysisResult, asJSON bool) (string, error) {
	if asJSON {
		output, err := cliformatter.FormatJSON(result)
		if err != nil {
			return "", fmt.Errorf("failed to format JSON: %w", err)
		}
		return output, nil
	}
	return cliformatter.FormatTable(result), nil
}
