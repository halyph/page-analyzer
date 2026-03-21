package collectors

import (
	"strings"

	"github.com/halyph/page-analyzer/internal/domain"
	"golang.org/x/net/html"
)

// TitleCollector extracts the page title from <title> tags
type TitleCollector struct {
	title       string
	inTitle     bool
	found       bool
	titleBuffer strings.Builder
}

// Collect processes HTML tokens to extract the title
func (c *TitleCollector) Collect(token html.Token) {
	// Stop processing if we've already found a title
	if c.found {
		return
	}

	switch token.Type {
	case html.StartTagToken:
		if token.Data == "title" {
			c.inTitle = true
			c.titleBuffer.Reset()
		}

	case html.TextToken:
		if c.inTitle {
			c.titleBuffer.WriteString(token.Data)
		}

	case html.EndTagToken:
		if token.Data == "title" && c.inTitle {
			c.title = strings.TrimSpace(c.titleBuffer.String())
			c.inTitle = false
			c.found = true
		}
	}
}

// Apply writes the extracted title to the result
func (c *TitleCollector) Apply(result *domain.AnalysisResult) {
	result.Title = c.title
}

// TitleFactory creates TitleCollector instances
type TitleFactory struct{}

// Create instantiates a new TitleCollector
func (f *TitleFactory) Create(config domain.CollectorConfig) (domain.Collector, error) {
	return &TitleCollector{}, nil
}

// Metadata returns information about this collector
func (f *TitleFactory) Metadata() domain.CollectorMetadata {
	return domain.CollectorMetadata{
		Name:        "title",
		Description: "Extracts page title from <title> tag",
		Version:     "1.0.0",
		Required:    true,
	}
}
