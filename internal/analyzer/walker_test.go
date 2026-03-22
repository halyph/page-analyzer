package analyzer

import (
	"strings"
	"testing"

	"github.com/halyph/page-analyzer/internal/analyzer/collectors"
	"github.com/halyph/page-analyzer/internal/config"
	"github.com/halyph/page-analyzer/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWalker_Simple(t *testing.T) {
	html := loadFixture(t, "walker_simple.html")

	walker := NewWalker(testProcessingConfig())
	result := domain.NewAnalysisResult("https://example.com")

	// Create collectors
	colls := []domain.Collector{
		&collectors.HTMLVersionCollector{},
		&collectors.TitleCollector{},
		&collectors.HeadingsCollector{},
	}

	err := walker.Walk(html, colls, result)
	require.NoError(t, err)

	assert.Equal(t, "HTML5", result.HTMLVersion)
	assert.Equal(t, "Test Page", result.Title)
	assert.Equal(t, 1, result.Headings.H1)
}

func TestWalker_CompleteAnalysis(t *testing.T) {
	html := loadFixture(t, "walker_complete.html")

	walker := NewWalker(testProcessingConfig())
	result := domain.NewAnalysisResult("https://example.com")

	linksCollector, _ := collectors.NewLinksCollector("https://example.com", 10000)

	colls := []domain.Collector{
		&collectors.HTMLVersionCollector{},
		&collectors.TitleCollector{},
		&collectors.HeadingsCollector{},
		linksCollector,
		&collectors.LoginFormCollector{},
	}

	err := walker.Walk(html, colls, result)
	require.NoError(t, err)

	assert.Equal(t, "HTML5", result.HTMLVersion)
	assert.Equal(t, "Complete Test", result.Title)
	assert.Equal(t, 1, result.Headings.H1)
	assert.Equal(t, 2, result.Headings.H2)
	assert.Equal(t, 1, result.Headings.H3)
	assert.Len(t, result.Links.Internal, 1)
	assert.Len(t, result.Links.External, 1)
	assert.True(t, result.HasLoginForm)
}

func TestWalker_EmptyBody(t *testing.T) {
	walker := NewWalker(testProcessingConfig())
	result := domain.NewAnalysisResult("https://example.com")

	err := walker.Walk([]byte{}, []domain.Collector{}, result)
	require.Error(t, err)
}

func TestWalker_MalformedHTML(t *testing.T) {
	// Malformed HTML should still be parseable (HTML parser is forgiving)
	html := loadFixture(t, "walker_malformed.html")

	walker := NewWalker(testProcessingConfig())
	result := domain.NewAnalysisResult("https://example.com")

	colls := []domain.Collector{
		&collectors.TitleCollector{},
		&collectors.HeadingsCollector{},
	}

	err := walker.Walk(html, colls, result)
	require.NoError(t, err, "parser should be forgiving")

	assert.Equal(t, "Test", result.Title)
	assert.Equal(t, 1, result.Headings.H1)
}

func TestWalker_LargeDocument(t *testing.T) {
	// Generate large HTML with many elements
	var sb strings.Builder
	sb.WriteString("<html><body>")
	for i := 0; i < 1000; i++ {
		sb.WriteString("<h2>Heading ")
		sb.WriteString(string(rune('0' + (i % 10))))
		sb.WriteString("</h2>")
		sb.WriteString("<p>Paragraph content</p>")
	}
	sb.WriteString("</body></html>")

	walker := NewWalker(testProcessingConfig())
	result := domain.NewAnalysisResult("https://example.com")

	colls := []domain.Collector{
		&collectors.HeadingsCollector{},
	}

	err := walker.Walk([]byte(sb.String()), colls, result)
	require.NoError(t, err)

	assert.Equal(t, 1000, result.Headings.H2)
}

func TestWalker_MaxTokensExceeded(t *testing.T) {
	// Generate HTML that exceeds token limit
	var sb strings.Builder
	sb.WriteString("<html><body>")
	for i := 0; i < 100000; i++ {
		sb.WriteString("<div>")
	}
	sb.WriteString("</body></html>")

	cfg := config.ProcessingConfig{MaxTokens: 1000}
	walker := NewWalker(cfg)
	result := domain.NewAnalysisResult("https://example.com")

	err := walker.Walk([]byte(sb.String()), []domain.Collector{}, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeded max tokens")
}

func TestWalker_NoCollectors(t *testing.T) {
	html := `<html><head><title>Test</title></head></html>`

	walker := NewWalker(testProcessingConfig())
	result := domain.NewAnalysisResult("https://example.com")

	// Walk with no collectors should complete without error
	err := walker.Walk([]byte(html), []domain.Collector{}, result)
	require.NoError(t, err)

	// Result should have default values
	assert.Empty(t, result.Title, "no title collector")
}

func TestWalker_SingleCollector(t *testing.T) {
	html := `<html><head><title>Single</title></head></html>`

	walker := NewWalker(testProcessingConfig())
	result := domain.NewAnalysisResult("https://example.com")

	colls := []domain.Collector{
		&collectors.TitleCollector{},
	}

	err := walker.Walk([]byte(html), colls, result)
	require.NoError(t, err)

	assert.Equal(t, "Single", result.Title)
}

func TestDefaultWalkerConfig(t *testing.T) {
	config := testProcessingConfig()

	assert.Equal(t, 1_000_000, config.MaxTokens)
}

func TestNewWalker_ZeroMaxTokens(t *testing.T) {
	processingCfg := config.ProcessingConfig{MaxTokens: 0}
	walker := NewWalker(processingCfg)

	assert.Equal(t, 1_000_000, walker.maxTokens, "should use default")
}

func TestNewWalker_NegativeMaxTokens(t *testing.T) {
	cfg := config.ProcessingConfig{MaxTokens: -100}
	walker := NewWalker(cfg)

	assert.Equal(t, 1_000_000, walker.maxTokens, "should use default")
}
