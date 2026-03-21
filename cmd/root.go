package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "analyzer",
	Short: "Page Analyzer - Analyze webpage structure and content",
	Long: `Page Analyzer is a command-line tool for analyzing webpages.
It extracts HTML version, title, headings, links, and detects login forms.

Complete documentation is available at https://github.com/halyph/page-analyzer`,
	Version: fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date),
}

func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
