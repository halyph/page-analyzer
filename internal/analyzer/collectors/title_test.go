package collectors

import (
	"strings"
	"testing"

	"github.com/halyph/page-analyzer/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"
)

func TestTitleCollector(t *testing.T) {
	tests := []struct {
		name      string
		htmlInput string
		wantTitle string
	}{
		{
			name:      "simple title",
			htmlInput: "<html><head><title>My Page</title></head></html>",
			wantTitle: "My Page",
		},
		{
			name:      "title with whitespace",
			htmlInput: "<html><head><title>  Spaces Around  </title></head></html>",
			wantTitle: "Spaces Around",
		},
		{
			name:      "title with newlines",
			htmlInput: "<html><head><title>\n  Multi\n  Line\n  </title></head></html>",
			wantTitle: "Multi\n  Line",
		},
		{
			name:      "empty title",
			htmlInput: "<html><head><title></title></head></html>",
			wantTitle: "",
		},
		{
			name:      "no title tag",
			htmlInput: "<html><head></head><body>Content</body></html>",
			wantTitle: "",
		},
		{
			name:      "multiple titles - takes first",
			htmlInput: "<html><head><title>First</title><title>Second</title></head></html>",
			wantTitle: "First",
		},
		{
			name:      "title in body (should still work)",
			htmlInput: "<html><body><title>Body Title</title></body></html>",
			wantTitle: "Body Title",
		},
		{
			name:      "title with special chars",
			htmlInput: "<html><head><title>Test & Example - Page</title></head></html>",
			wantTitle: "Test & Example - Page",
		},
		{
			name:      "title with unicode",
			htmlInput: "<html><head><title>Hello 世界 🌍</title></head></html>",
			wantTitle: "Hello 世界 🌍",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector := &TitleCollector{}
			result := domain.NewAnalysisResult("https://example.com")

			// Parse and collect
			tokenizer := html.NewTokenizer(strings.NewReader(tt.htmlInput))
			for {
				tokenType := tokenizer.Next()
				if tokenType == html.ErrorToken {
					break
				}
				collector.Collect(tokenizer.Token())
			}

			// Apply result
			collector.Apply(result)

			assert.Equal(t, tt.wantTitle, result.Title)
		})
	}
}

func TestTitleCollector_StopsAfterFirst(t *testing.T) {
	collector := &TitleCollector{}
	result := domain.NewAnalysisResult("https://example.com")

	// Simulate tokens for: <title>First</title><title>Second</title>
	tokens := []html.Token{
		{Type: html.StartTagToken, Data: "title"},
		{Type: html.TextToken, Data: "First"},
		{Type: html.EndTagToken, Data: "title"},
		{Type: html.StartTagToken, Data: "title"},
		{Type: html.TextToken, Data: "Second"},
		{Type: html.EndTagToken, Data: "title"},
	}

	for _, token := range tokens {
		collector.Collect(token)
	}

	collector.Apply(result)

	assert.Equal(t, "First", result.Title)
}

func TestTitleCollector_MultipleTextNodes(t *testing.T) {
	// Test that title collector concatenates multiple text nodes
	collector := &TitleCollector{}
	result := domain.NewAnalysisResult("https://example.com")

	tokens := []html.Token{
		{Type: html.StartTagToken, Data: "title"},
		{Type: html.TextToken, Data: "Part "},
		{Type: html.TextToken, Data: "One "},
		{Type: html.TextToken, Data: "Part "},
		{Type: html.TextToken, Data: "Two"},
		{Type: html.EndTagToken, Data: "title"},
	}

	for _, token := range tokens {
		collector.Collect(token)
	}

	collector.Apply(result)

	assert.Equal(t, "Part One Part Two", result.Title)
}

func TestTitleCollector_NestedTags(t *testing.T) {
	// HTML like: <title>Hello <span>World</span></title>
	// Should extract text from all nested elements
	htmlInput := "<html><head><title>Hello <span>World</span></title></head></html>"

	collector := &TitleCollector{}
	result := domain.NewAnalysisResult("https://example.com")

	tokenizer := html.NewTokenizer(strings.NewReader(htmlInput))
	for {
		tokenType := tokenizer.Next()
		if tokenType == html.ErrorToken {
			break
		}
		collector.Collect(tokenizer.Token())
	}

	collector.Apply(result)

	// Note: The tokenizer will give us "Hello " and "World" as separate text tokens
	assert.Contains(t, result.Title, "Hello")
	assert.Contains(t, result.Title, "World")
}

func TestTitleFactory(t *testing.T) {
	factory := &TitleFactory{}

	// Test Metadata
	metadata := factory.Metadata()
	assert.Equal(t, "title", metadata.Name)
	assert.True(t, metadata.Required)

	// Test Create
	config := domain.CollectorConfig{}
	collector, err := factory.Create(config)
	require.NoError(t, err)
	require.NotNil(t, collector)

	// Verify it's the right type
	_, ok := collector.(*TitleCollector)
	assert.True(t, ok, "collector type = %T, want *TitleCollector", collector)
}
