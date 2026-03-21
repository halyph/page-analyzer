package analyzer

import (
	"bytes"
	"fmt"
	"io"

	"github.com/halyph/page-analyzer/internal/domain"
	"golang.org/x/net/html"
)

// Walker streams HTML tokens to collectors
type Walker struct {
	maxTokens int // Safety limit to prevent infinite loops
}

// WalkerConfig configures the HTML walker
type WalkerConfig struct {
	MaxTokens int // Maximum tokens to process (0 = unlimited, but use default)
}

// DefaultWalkerConfig returns sensible defaults
func DefaultWalkerConfig() WalkerConfig {
	return WalkerConfig{
		MaxTokens: 1_000_000, // 1 million tokens should handle most pages
	}
}

// NewWalker creates a new HTML walker with the given configuration
func NewWalker(config WalkerConfig) *Walker {
	if config.MaxTokens <= 0 {
		config.MaxTokens = 1_000_000
	}

	return &Walker{
		maxTokens: config.MaxTokens,
	}
}

// Walk parses HTML and feeds tokens to all collectors
func (w *Walker) Walk(body []byte, collectors []domain.Collector, result *domain.AnalysisResult) error {
	if len(body) == 0 {
		return fmt.Errorf("empty HTML body")
	}

	// Create tokenizer
	tokenizer := html.NewTokenizer(bytes.NewReader(body))
	tokenCount := 0

	// Process tokens
	for {
		tokenType := tokenizer.Next()
		tokenCount++

		// Safety: prevent infinite loops on malformed HTML
		if tokenCount > w.maxTokens {
			return fmt.Errorf("exceeded max tokens: %d", w.maxTokens)
		}

		// Check for errors
		if tokenType == html.ErrorToken {
			err := tokenizer.Err()
			if err == io.EOF {
				break // Normal completion
			}
			return fmt.Errorf("tokenization error: %w", err)
		}

		// Get current token
		token := tokenizer.Token()

		// Feed token to all collectors
		for _, collector := range collectors {
			collector.Collect(token)
		}
	}

	// Finalize: collectors write their results
	for _, collector := range collectors {
		collector.Apply(result)
	}

	return nil
}
