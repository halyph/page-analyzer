package collectors

import (
	"strings"
	"testing"

	"github.com/halyph/page-analyzer/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"
)

func TestHeadingsCollector(t *testing.T) {
	tests := []struct {
		name         string
		htmlInput    string
		wantHeadings domain.HeadingCounts
	}{
		{
			name:         "no headings",
			htmlInput:    "<html><body><p>text</p></body></html>",
			wantHeadings: domain.HeadingCounts{},
		},
		{
			name:         "single h1",
			htmlInput:    "<html><body><h1>Title</h1></body></html>",
			wantHeadings: domain.HeadingCounts{H1: 1},
		},
		{
			name:         "all heading levels",
			htmlInput:    "<html><body><h1>H1</h1><h2>H2</h2><h3>H3</h3><h4>H4</h4><h5>H5</h5><h6>H6</h6></body></html>",
			wantHeadings: domain.HeadingCounts{H1: 1, H2: 1, H3: 1, H4: 1, H5: 1, H6: 1},
		},
		{
			name:         "multiple h2",
			htmlInput:    "<html><body><h1>Title</h1><h2>Section 1</h2><h2>Section 2</h2><h2>Section 3</h2></body></html>",
			wantHeadings: domain.HeadingCounts{H1: 1, H2: 3},
		},
		{
			name: "nested structure",
			htmlInput: `<html><body>
				<h1>Main</h1>
				<div>
					<h2>Section 1</h2>
					<h3>Subsection 1.1</h3>
					<h3>Subsection 1.2</h3>
				</div>
				<div>
					<h2>Section 2</h2>
					<h3>Subsection 2.1</h3>
				</div>
			</body></html>`,
			wantHeadings: domain.HeadingCounts{H1: 1, H2: 2, H3: 3},
		},
		{
			name:         "empty headings still count",
			htmlInput:    "<html><body><h1></h1><h2></h2><h3></h3></body></html>",
			wantHeadings: domain.HeadingCounts{H1: 1, H2: 1, H3: 1},
		},
		{
			name:         "headings with attributes",
			htmlInput:    `<html><body><h1 class="main">Title</h1><h2 id="section1">Section</h2></body></html>`,
			wantHeadings: domain.HeadingCounts{H1: 1, H2: 1},
		},
		{
			name:         "only h6",
			htmlInput:    "<html><body><h6>Small</h6><h6>Headers</h6><h6>Only</h6></body></html>",
			wantHeadings: domain.HeadingCounts{H6: 3},
		},
		{
			name:         "uppercase tags also counted",
			htmlInput:    "<html><body><H1>First</H1><h1>Second</h1></body></html>",
			wantHeadings: domain.HeadingCounts{H1: 2}, // HTML tokenizer lowercases tags, so both count
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector := &HeadingsCollector{}
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

			assert.Equal(t, tt.wantHeadings, result.Headings)
		})
	}
}

func TestHeadingsCollector_IgnoresEndTags(t *testing.T) {
	collector := &HeadingsCollector{}

	// Process start tag
	collector.Collect(html.Token{
		Type: html.StartTagToken,
		Data: "h1",
	})

	// Process end tag (should not increment)
	collector.Collect(html.Token{
		Type: html.EndTagToken,
		Data: "h1",
	})

	assert.Equal(t, 1, collector.counts.H1)
}

func TestHeadingsCollector_IgnoresTextTokens(t *testing.T) {
	collector := &HeadingsCollector{}

	// Text tokens should be ignored
	collector.Collect(html.Token{
		Type: html.TextToken,
		Data: "h1",
	})

	assert.Equal(t, 0, collector.counts.H1)
}

func TestHeadingsCollector_IgnoresNonHeadings(t *testing.T) {
	collector := &HeadingsCollector{}

	// Non-heading tags
	nonHeadings := []string{"div", "p", "span", "header", "h7", "h0", "heading"}

	for _, tag := range nonHeadings {
		collector.Collect(html.Token{
			Type: html.StartTagToken,
			Data: tag,
		})
	}

	// All counts should be zero
	assert.Equal(t, 0, collector.counts.Total())
}

func TestHeadingsCollector_LargeCount(t *testing.T) {
	collector := &HeadingsCollector{}

	// Simulate 100 h2 tags
	for i := 0; i < 100; i++ {
		collector.Collect(html.Token{
			Type: html.StartTagToken,
			Data: "h2",
		})
	}

	assert.Equal(t, 100, collector.counts.H2)
}

func TestHeadingsFactory(t *testing.T) {
	factory := &HeadingsFactory{}

	// Test Metadata
	metadata := factory.Metadata()
	assert.Equal(t, "headings", metadata.Name)
	assert.True(t, metadata.Required)

	// Test Create
	config := domain.CollectorConfig{}
	collector, err := factory.Create(config)
	require.NoError(t, err)
	require.NotNil(t, collector)

	// Verify it's the right type
	_, ok := collector.(*HeadingsCollector)
	assert.True(t, ok, "collector type = %T, want *HeadingsCollector", collector)
}
