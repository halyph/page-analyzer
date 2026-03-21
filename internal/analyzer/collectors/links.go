package collectors

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/oivasiv/page-analyzer/internal/domain"
	"golang.org/x/net/html"
)

// LinksCollector collects and classifies links from a webpage
type LinksCollector struct {
	baseURL   *url.URL
	maxLinks  int
	internal  []string
	external  []string
	seen      map[string]bool
	truncated bool
}

// NewLinksCollector creates a new links collector with the given base URL and max links
func NewLinksCollector(baseURL string, maxLinks int) (*LinksCollector, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	if maxLinks <= 0 {
		maxLinks = 10000 // Default
	}

	return &LinksCollector{
		baseURL:  u,
		maxLinks: maxLinks,
		seen:     make(map[string]bool),
	}, nil
}

// Collect processes HTML tokens to extract links
func (c *LinksCollector) Collect(token html.Token) {
	if token.Type != html.StartTagToken {
		return
	}

	// Only process <a> tags
	if token.Data != "a" {
		return
	}

	href := extractAttr(token.Attr, "href")
	if href == "" {
		return
	}

	// Skip non-HTTP(S) links (javascript:, mailto:, tel:, etc.)
	if isNonHTTPScheme(href) {
		return
	}

	// Resolve relative URLs
	absolute, err := c.baseURL.Parse(href)
	if err != nil {
		// Invalid URL, skip
		return
	}

	// Remove fragment (#section) for deduplication
	absolute.Fragment = ""

	normalized := absolute.String()

	// Deduplicate
	if c.seen[normalized] {
		return
	}

	// Mark as seen for counting
	c.seen[normalized] = true

	// Check if we've hit the collection limit
	totalCollected := len(c.internal) + len(c.external)
	if totalCollected >= c.maxLinks {
		c.truncated = true
		return // Don't add to arrays, but we've already counted it in seen
	}

	// Classify: internal vs external
	if isSameOrigin(c.baseURL, absolute) {
		c.internal = append(c.internal, normalized)
	} else {
		c.external = append(c.external, normalized)
	}
}

// Apply writes the collected links to the result
func (c *LinksCollector) Apply(result *domain.AnalysisResult) {
	result.Links.Internal = c.internal
	result.Links.External = c.external
	result.Links.TotalFound = len(c.seen)
	result.Links.Truncated = c.truncated

	// Initialize with no link checking by default
	result.Links.CheckStatus = domain.LinkCheckCompleted
}

// extractAttr extracts an attribute value from a token's attributes
func extractAttr(attrs []html.Attribute, key string) string {
	for _, attr := range attrs {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

// isSameOrigin checks if two URLs have the same scheme and host
func isSameOrigin(base, target *url.URL) bool {
	return base.Scheme == target.Scheme && base.Host == target.Host
}

// isNonHTTPScheme checks if a URL uses a non-HTTP(S) scheme
func isNonHTTPScheme(href string) bool {
	lower := strings.ToLower(strings.TrimSpace(href))

	// Check for explicit non-HTTP schemes
	nonHTTPSchemes := []string{
		"javascript:",
		"mailto:",
		"tel:",
		"ftp:",
		"data:",
		"file:",
		"#", // Fragment-only links
	}

	for _, scheme := range nonHTTPSchemes {
		if strings.HasPrefix(lower, scheme) {
			return true
		}
	}

	return false
}

// LinksFactory creates LinksCollector instances
type LinksFactory struct{}

// Create instantiates a new LinksCollector
func (f *LinksFactory) Create(config domain.CollectorConfig) (domain.Collector, error) {
	maxLinks := config.MaxItems
	if maxLinks == 0 {
		maxLinks = 10000 // Default
	}

	return NewLinksCollector(config.BaseURL, maxLinks)
}

// Metadata returns information about this collector
func (f *LinksFactory) Metadata() domain.CollectorMetadata {
	return domain.CollectorMetadata{
		Name:        "links",
		Description: "Collects and classifies links (internal vs external)",
		Version:     "1.0.0",
		Required:    true,
	}
}
