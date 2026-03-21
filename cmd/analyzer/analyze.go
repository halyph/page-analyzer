package main

import (
	"context"
	"fmt"
	"time"

	"github.com/halyph/page-analyzer/internal/analyzer"
	"github.com/halyph/page-analyzer/internal/cache"
	"github.com/halyph/page-analyzer/internal/domain"
	cliformatter "github.com/halyph/page-analyzer/internal/presentation/cli"
	"github.com/urfave/cli/v2"
)

const (
	defaultCacheSize    = 100
	defaultMaxBodySize  = 10 * 1024 * 1024 // 10MB
	defaultMaxTokens    = 1_000_000        // 1M tokens
	defaultCacheTTL     = 1 * time.Hour
	defaultCheckTimeout = 30 * time.Second
	defaultFetchTimeout = 15 * time.Second
	defaultMaxLinks     = 10000
)

func newAnalyzeCommand() *cli.Command {
	return &cli.Command{
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
				Value: defaultMaxLinks,
			},
			&cli.DurationFlag{
				Name:  "timeout",
				Usage: "HTTP request timeout",
				Value: defaultFetchTimeout,
			},
		},
		Action: runAnalyze,
	}
}

func runAnalyze(c *cli.Context) error {
	if c.NArg() != 1 {
		return fmt.Errorf("URL argument required")
	}
	url := c.Args().First()

	// Create analyzer service
	service := createAnalyzerService(c)
	defer service.Stop()

	// Build analysis request
	req := buildAnalysisRequest(url, c)

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

func createAnalyzerService(c *cli.Context) *analyzer.Service {
	memCache := cache.NewMemoryCache(defaultCacheSize, defaultCacheTTL)

	serviceCfg := analyzer.ServiceConfig{
		Fetcher: analyzer.FetcherConfig{
			Timeout:     c.Duration("timeout"),
			MaxBodySize: defaultMaxBodySize,
			UserAgent:   userAgent(),
		},
		Walker: analyzer.WalkerConfig{
			MaxTokens: defaultMaxTokens,
		},
		Cache:    memCache,
		CacheTTL: defaultCacheTTL,
	}

	if c.Bool("check-links") {
		linkCheckCfg := analyzer.DefaultLinkCheckConfig()
		linkCheckCfg.UserAgent = userAgent()
		serviceCfg.LinkChecker = &linkCheckCfg
	}

	return analyzer.NewService(serviceCfg)
}

func buildAnalysisRequest(url string, c *cli.Context) domain.AnalysisRequest {
	checkMode := domain.LinkCheckDisabled
	if c.Bool("check-links") {
		checkMode = domain.LinkCheckSync
	}

	return domain.AnalysisRequest{
		URL: url,
		Options: domain.AnalysisOptions{
			CheckLinks: checkMode,
			MaxLinks:   c.Int("max-links"),
			Timeout:    defaultCheckTimeout,
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
