package analyzer

import (
	"bytes"
	"fmt"
	"io"

	"github.com/halyph/page-analyzer/internal/config"
	"github.com/halyph/page-analyzer/internal/domain"
	"golang.org/x/net/html"
)

// Walker streams HTML tokens to collectors
type Walker struct {
	maxTokens int // Safety limit to prevent infinite loops
}

// NewWalker creates a new HTML walker with the given configuration
func NewWalker(cfg config.ProcessingConfig) *Walker {
	maxTokens := cfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1_000_000
	}

	return &Walker{
		maxTokens: maxTokens,
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
