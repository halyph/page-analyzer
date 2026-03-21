package collectors

import (
	"strings"

	"github.com/halyph/page-analyzer/internal/domain"
	"golang.org/x/net/html"
)

// HTMLVersionCollector detects HTML version from DOCTYPE
type HTMLVersionCollector struct {
	version string
	found   bool
}

// Collect processes HTML tokens to find DOCTYPE
func (c *HTMLVersionCollector) Collect(token html.Token) {
	// Only process if we haven't found it yet
	if c.found {
		return
	}

	if token.Type == html.DoctypeToken {
		c.version = detectHTMLVersion(token.Data)
		c.found = true
	}
}

// Apply writes the detected HTML version to the result
func (c *HTMLVersionCollector) Apply(result *domain.AnalysisResult) {
	if c.version == "" {
		result.HTMLVersion = "Unknown"
	} else {
		result.HTMLVersion = c.version
	}
}

// detectHTMLVersion maps DOCTYPE declarations to version strings
func detectHTMLVersion(doctype string) string {
	doctype = strings.ToLower(strings.TrimSpace(doctype))

	// HTML5
	if doctype == "html" {
		return "HTML5"
	}

	// HTML 4.01 Strict
	if strings.Contains(doctype, "html 4.01") && strings.Contains(doctype, "strict") {
		return "HTML 4.01 Strict"
	}

	// HTML 4.01 Transitional
	if strings.Contains(doctype, "html 4.01") && strings.Contains(doctype, "transitional") {
		return "HTML 4.01 Transitional"
	}

	// HTML 4.01 Frameset
	if strings.Contains(doctype, "html 4.01") && strings.Contains(doctype, "frameset") {
		return "HTML 4.01 Frameset"
	}

	// XHTML 1.0 Strict
	if strings.Contains(doctype, "xhtml 1.0") && strings.Contains(doctype, "strict") {
		return "XHTML 1.0 Strict"
	}

	// XHTML 1.0 Transitional
	if strings.Contains(doctype, "xhtml 1.0") && strings.Contains(doctype, "transitional") {
		return "XHTML 1.0 Transitional"
	}

	// XHTML 1.1
	if strings.Contains(doctype, "xhtml 1.1") {
		return "XHTML 1.1"
	}

	// If we can't identify it specifically, but it has "html" in it
	if strings.Contains(doctype, "html") {
		return "HTML (unknown version)"
	}

	return "Unknown"
}

// HTMLVersionFactory creates HTMLVersionCollector instances
type HTMLVersionFactory struct{}

// Create instantiates a new HTMLVersionCollector
func (f *HTMLVersionFactory) Create(config domain.CollectorConfig) (domain.Collector, error) {
	return &HTMLVersionCollector{}, nil
}

// Metadata returns information about this collector
func (f *HTMLVersionFactory) Metadata() domain.CollectorMetadata {
	return domain.CollectorMetadata{
		Name:        "htmlversion",
		Description: "Detects HTML version from DOCTYPE declaration",
		Version:     "1.0.0",
		Required:    true,
	}
}
