package analyzer

import (
	"strings"
	"testing"

	"github.com/oivasiv/page-analyzer/internal/analyzer/collectors"
	"github.com/oivasiv/page-analyzer/internal/domain"
)

func TestWalker_Simple(t *testing.T) {
	html := `<!DOCTYPE html>
<html>
<head><title>Test Page</title></head>
<body>
	<h1>Main Title</h1>
	<p>Content here</p>
</body>
</html>`

	walker := NewWalker(DefaultWalkerConfig())
	result := domain.NewAnalysisResult("https://example.com")

	// Create collectors
	colls := []domain.Collector{
		&collectors.HTMLVersionCollector{},
		&collectors.TitleCollector{},
		&collectors.HeadingsCollector{},
	}

	err := walker.Walk([]byte(html), colls, result)
	if err != nil {
		t.Fatalf("Walk() error = %v", err)
	}

	// Verify results
	if result.HTMLVersion != "HTML5" {
		t.Errorf("HTMLVersion = %s, want HTML5", result.HTMLVersion)
	}

	if result.Title != "Test Page" {
		t.Errorf("Title = %s, want 'Test Page'", result.Title)
	}

	if result.Headings.H1 != 1 {
		t.Errorf("H1 count = %d, want 1", result.Headings.H1)
	}
}

func TestWalker_CompleteAnalysis(t *testing.T) {
	html := `<!DOCTYPE html>
<html>
<head><title>Complete Test</title></head>
<body>
	<h1>Title</h1>
	<h2>Section 1</h2>
	<h2>Section 2</h2>
	<h3>Subsection</h3>
	<a href="https://example.com/page1">Internal</a>
	<a href="https://other.com">External</a>
	<form>
		<input type="text" name="username">
		<input type="password" name="password">
	</form>
</body>
</html>`

	walker := NewWalker(DefaultWalkerConfig())
	result := domain.NewAnalysisResult("https://example.com")

	linksCollector, _ := collectors.NewLinksCollector("https://example.com", 10000)

	colls := []domain.Collector{
		&collectors.HTMLVersionCollector{},
		&collectors.TitleCollector{},
		&collectors.HeadingsCollector{},
		linksCollector,
		&collectors.LoginFormCollector{},
	}

	err := walker.Walk([]byte(html), colls, result)
	if err != nil {
		t.Fatalf("Walk() error = %v", err)
	}

	// Verify all results
	if result.HTMLVersion != "HTML5" {
		t.Errorf("HTMLVersion = %s, want HTML5", result.HTMLVersion)
	}

	if result.Title != "Complete Test" {
		t.Errorf("Title = %s, want 'Complete Test'", result.Title)
	}

	if result.Headings.H1 != 1 || result.Headings.H2 != 2 || result.Headings.H3 != 1 {
		t.Errorf("Headings = %+v, want H1:1 H2:2 H3:1", result.Headings)
	}

	if len(result.Links.Internal) != 1 {
		t.Errorf("Internal links = %d, want 1", len(result.Links.Internal))
	}

	if len(result.Links.External) != 1 {
		t.Errorf("External links = %d, want 1", len(result.Links.External))
	}

	if !result.HasLoginForm {
		t.Error("HasLoginForm = false, want true")
	}
}

func TestWalker_EmptyBody(t *testing.T) {
	walker := NewWalker(DefaultWalkerConfig())
	result := domain.NewAnalysisResult("https://example.com")

	err := walker.Walk([]byte{}, []domain.Collector{}, result)
	if err == nil {
		t.Error("expected error for empty body")
	}
}

func TestWalker_MalformedHTML(t *testing.T) {
	// Malformed HTML should still be parseable (HTML parser is forgiving)
	html := `<html><head><title>Test</title></head><h1>Unclosed tags`

	walker := NewWalker(DefaultWalkerConfig())
	result := domain.NewAnalysisResult("https://example.com")

	colls := []domain.Collector{
		&collectors.TitleCollector{},
		&collectors.HeadingsCollector{},
	}

	err := walker.Walk([]byte(html), colls, result)
	if err != nil {
		t.Fatalf("Walk() error = %v (parser should be forgiving)", err)
	}

	// Parser should have extracted title
	if result.Title != "Test" {
		t.Errorf("Title = %q, want 'Test'", result.Title)
	}

	// Should still count H1 even though unclosed
	if result.Headings.H1 != 1 {
		t.Errorf("H1 = %d, want 1", result.Headings.H1)
	}
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

	walker := NewWalker(DefaultWalkerConfig())
	result := domain.NewAnalysisResult("https://example.com")

	colls := []domain.Collector{
		&collectors.HeadingsCollector{},
	}

	err := walker.Walk([]byte(sb.String()), colls, result)
	if err != nil {
		t.Fatalf("Walk() error = %v", err)
	}

	if result.Headings.H2 != 1000 {
		t.Errorf("H2 count = %d, want 1000", result.Headings.H2)
	}
}

func TestWalker_MaxTokensExceeded(t *testing.T) {
	// Generate HTML that exceeds token limit
	var sb strings.Builder
	sb.WriteString("<html><body>")
	for i := 0; i < 100000; i++ {
		sb.WriteString("<div>")
	}
	sb.WriteString("</body></html>")

	config := WalkerConfig{MaxTokens: 1000}
	walker := NewWalker(config)
	result := domain.NewAnalysisResult("https://example.com")

	err := walker.Walk([]byte(sb.String()), []domain.Collector{}, result)
	if err == nil {
		t.Error("expected error for exceeding max tokens")
	}

	if !strings.Contains(err.Error(), "exceeded max tokens") {
		t.Errorf("error = %v, want 'exceeded max tokens'", err)
	}
}

func TestWalker_NoCollectors(t *testing.T) {
	html := `<html><head><title>Test</title></head></html>`

	walker := NewWalker(DefaultWalkerConfig())
	result := domain.NewAnalysisResult("https://example.com")

	// Walk with no collectors should complete without error
	err := walker.Walk([]byte(html), []domain.Collector{}, result)
	if err != nil {
		t.Fatalf("Walk() error = %v", err)
	}

	// Result should have default values
	if result.Title != "" {
		t.Errorf("Title = %s, want empty (no title collector)", result.Title)
	}
}

func TestWalker_SingleCollector(t *testing.T) {
	html := `<html><head><title>Single</title></head></html>`

	walker := NewWalker(DefaultWalkerConfig())
	result := domain.NewAnalysisResult("https://example.com")

	colls := []domain.Collector{
		&collectors.TitleCollector{},
	}

	err := walker.Walk([]byte(html), colls, result)
	if err != nil {
		t.Fatalf("Walk() error = %v", err)
	}

	if result.Title != "Single" {
		t.Errorf("Title = %s, want 'Single'", result.Title)
	}
}

func TestDefaultWalkerConfig(t *testing.T) {
	config := DefaultWalkerConfig()

	if config.MaxTokens != 1_000_000 {
		t.Errorf("MaxTokens = %d, want 1000000", config.MaxTokens)
	}
}

func TestNewWalker_ZeroMaxTokens(t *testing.T) {
	config := WalkerConfig{MaxTokens: 0}
	walker := NewWalker(config)

	if walker.maxTokens != 1_000_000 {
		t.Errorf("maxTokens = %d, want 1000000 (default)", walker.maxTokens)
	}
}

func TestNewWalker_NegativeMaxTokens(t *testing.T) {
	config := WalkerConfig{MaxTokens: -100}
	walker := NewWalker(config)

	if walker.maxTokens != 1_000_000 {
		t.Errorf("maxTokens = %d, want 1000000 (default)", walker.maxTokens)
	}
}
