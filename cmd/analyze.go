package main

import (
	"context"
	"fmt"
	"time"

	"github.com/halyph/page-analyzer/internal/analyzer"
	"github.com/halyph/page-analyzer/internal/domain"
	"github.com/halyph/page-analyzer/internal/presentation/cli"
	"github.com/spf13/cobra"
)

var (
	flagJSON       bool
	flagCheckLinks bool
	flagMaxLinks   int
	flagTimeout    time.Duration
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze [URL]",
	Short: "Analyze a webpage",
	Long: `Analyze a webpage and extract its structure and metadata.

Examples:
  analyzer analyze https://example.com
  analyzer analyze https://example.com --json
  analyzer analyze https://example.com --check-links
  analyzer analyze https://example.com --max-links 500`,
	Args: cobra.ExactArgs(1),
	RunE: runAnalyze,
}

func init() {
	analyzeCmd.Flags().BoolVar(&flagJSON, "json", false, "Output results as JSON")
	analyzeCmd.Flags().BoolVar(&flagCheckLinks, "check-links", false, "Check link accessibility (synchronous)")
	analyzeCmd.Flags().IntVar(&flagMaxLinks, "max-links", 10000, "Maximum number of links to collect")
	analyzeCmd.Flags().DurationVar(&flagTimeout, "timeout", 15*time.Second, "HTTP request timeout")

	rootCmd.AddCommand(analyzeCmd)
}

func runAnalyze(cmd *cobra.Command, args []string) error {
	url := args[0]

	// Create analyzer service
	serviceCfg := analyzer.ServiceConfig{
		Fetcher: analyzer.FetcherConfig{
			Timeout:     flagTimeout,
			MaxBodySize: 10 * 1024 * 1024, // 10MB
			UserAgent:   fmt.Sprintf("page-analyzer/%s", version),
		},
		Walker: analyzer.WalkerConfig{
			MaxTokens: 1_000_000, // 1M tokens max
		},
	}

	// Enable link checking if requested
	if flagCheckLinks {
		linkCheckCfg := analyzer.DefaultLinkCheckConfig()
		linkCheckCfg.UserAgent = fmt.Sprintf("page-analyzer/%s", version)
		serviceCfg.LinkChecker = &linkCheckCfg
	}

	service := analyzer.NewService(serviceCfg)
	defer service.Stop()

	// Build analysis request
	checkMode := domain.LinkCheckDisabled
	if flagCheckLinks {
		checkMode = domain.LinkCheckSync
	}

	req := domain.AnalysisRequest{
		URL: url,
		Options: domain.AnalysisOptions{
			CheckLinks: checkMode,
			MaxLinks:   flagMaxLinks,
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
	if flagJSON {
		jsonOutput, err := cli.FormatJSON(result)
		if err != nil {
			return fmt.Errorf("failed to format JSON: %w", err)
		}
		output = jsonOutput
	} else {
		output = cli.FormatTable(result)
	}

	fmt.Print(output)
	return nil
}
