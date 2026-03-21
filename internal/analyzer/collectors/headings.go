package collectors

import (
	"github.com/oivasiv/page-analyzer/internal/domain"
	"golang.org/x/net/html"
)

// HeadingsCollector counts heading tags (H1-H6)
type HeadingsCollector struct {
	counts domain.HeadingCounts
}

// Collect processes HTML tokens to count headings
func (c *HeadingsCollector) Collect(token html.Token) {
	if token.Type != html.StartTagToken {
		return
	}

	switch token.Data {
	case "h1":
		c.counts.H1++
	case "h2":
		c.counts.H2++
	case "h3":
		c.counts.H3++
	case "h4":
		c.counts.H4++
	case "h5":
		c.counts.H5++
	case "h6":
		c.counts.H6++
	}
}

// Apply writes the heading counts to the result
func (c *HeadingsCollector) Apply(result *domain.AnalysisResult) {
	result.Headings = c.counts
}

// HeadingsFactory creates HeadingsCollector instances
type HeadingsFactory struct{}

// Create instantiates a new HeadingsCollector
func (f *HeadingsFactory) Create(config domain.CollectorConfig) (domain.Collector, error) {
	return &HeadingsCollector{}, nil
}

// Metadata returns information about this collector
func (f *HeadingsFactory) Metadata() domain.CollectorMetadata {
	return domain.CollectorMetadata{
		Name:        "headings",
		Description: "Counts heading tags (H1-H6)",
		Version:     "1.0.0",
		Required:    true,
	}
}
