package collectors

import (
	"github.com/halyph/page-analyzer/internal/domain"
	"golang.org/x/net/html"
)

// LoginFormCollector detects login forms by looking for password inputs inside forms
type LoginFormCollector struct {
	hasLoginForm bool
	inForm       bool
	formDepth    int
	currentDepth int
}

// Collect processes HTML tokens to detect login forms
func (c *LoginFormCollector) Collect(token html.Token) {
	// Once found, stop processing
	if c.hasLoginForm {
		return
	}

	switch token.Type {
	case html.StartTagToken, html.SelfClosingTagToken:
		c.currentDepth++

		if token.Data == "form" {
			c.inForm = true
			c.formDepth = c.currentDepth
		}

		// Check for password input inside a form
		if c.inForm && token.Data == "input" {
			if hasPasswordType(token.Attr) {
				c.hasLoginForm = true
			}
		}

		// Self-closing tags don't have matching end tags, so adjust depth
		if token.Type == html.SelfClosingTagToken {
			c.currentDepth--
		}

	case html.EndTagToken:
		if token.Data == "form" && c.inForm && c.currentDepth == c.formDepth {
			c.inForm = false
			c.formDepth = 0
		}
		c.currentDepth--
	}
}

// Apply writes the login form detection result
func (c *LoginFormCollector) Apply(result *domain.AnalysisResult) {
	result.HasLoginForm = c.hasLoginForm
}

// hasPasswordType checks if an input element has type="password"
func hasPasswordType(attrs []html.Attribute) bool {
	for _, attr := range attrs {
		if attr.Key == "type" && attr.Val == "password" {
			return true
		}
	}
	return false
}

// LoginFormFactory creates LoginFormCollector instances
type LoginFormFactory struct{}

// Create instantiates a new LoginFormCollector
func (f *LoginFormFactory) Create(config domain.CollectorConfig) (domain.Collector, error) {
	return &LoginFormCollector{}, nil
}

// Metadata returns information about this collector
func (f *LoginFormFactory) Metadata() domain.CollectorMetadata {
	return domain.CollectorMetadata{
		Name:        "loginform",
		Description: "Detects login forms by finding password input fields within forms",
		Version:     "1.0.0",
		Required:    true,
	}
}
