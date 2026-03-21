package main

import (
	"fmt"
	"os"

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
			newAnalyzeCommand(),
			newServeCommand(),
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
