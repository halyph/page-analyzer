package collectors

import (
	"strings"
	"testing"

	"github.com/halyph/page-analyzer/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"
)

func TestHTMLVersionCollector(t *testing.T) {
	tests := []struct {
		name        string
		htmlInput   string
		wantVersion string
	}{
		{
			name:        "HTML5",
			htmlInput:   "<!DOCTYPE html><html></html>",
			wantVersion: "HTML5",
		},
		{
			name:        "HTML5 uppercase",
			htmlInput:   "<!DOCTYPE HTML><html></html>",
			wantVersion: "HTML5",
		},
		{
			name:        "HTML 4.01 Strict",
			htmlInput:   `<!DOCTYPE HTML PUBLIC "-//W3C//DTD HTML 4.01//EN" "http://www.w3.org/TR/html4/strict.dtd">`,
			wantVersion: "HTML 4.01 Strict",
		},
		{
			name:        "HTML 4.01 Transitional",
			htmlInput:   `<!DOCTYPE HTML PUBLIC "-//W3C//DTD HTML 4.01 Transitional//EN">`,
			wantVersion: "HTML 4.01 Transitional",
		},
		{
			name:        "XHTML 1.0 Strict",
			htmlInput:   `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Strict//EN">`,
			wantVersion: "XHTML 1.0 Strict",
		},
		{
			name:        "XHTML 1.0 Transitional",
			htmlInput:   `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Transitional//EN">`,
			wantVersion: "XHTML 1.0 Transitional",
		},
		{
			name:        "XHTML 1.1",
			htmlInput:   `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.1//EN">`,
			wantVersion: "XHTML 1.1",
		},
		{
			name:        "No DOCTYPE",
			htmlInput:   "<html><head><title>Test</title></head></html>",
			wantVersion: "Unknown",
		},
		{
			name:        "Empty document",
			htmlInput:   "",
			wantVersion: "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector := &HTMLVersionCollector{}
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

			assert.Equal(t, tt.wantVersion, result.HTMLVersion)
		})
	}
}

func TestDetectHTMLVersion(t *testing.T) {
	tests := []struct {
		name    string
		doctype string
		want    string
	}{
		{
			name:    "html5 lowercase",
			doctype: "html",
			want:    "HTML5",
		},
		{
			name:    "html5 uppercase",
			doctype: "HTML",
			want:    "HTML5",
		},
		{
			name:    "html5 with spaces",
			doctype: "  html  ",
			want:    "HTML5",
		},
		{
			name:    "html 4.01 strict",
			doctype: "HTML PUBLIC \"-//W3C//DTD HTML 4.01//EN\" \"http://www.w3.org/TR/html4/strict.dtd\"",
			want:    "HTML 4.01 Strict",
		},
		{
			name:    "xhtml 1.0 strict",
			doctype: "html PUBLIC \"-//W3C//DTD XHTML 1.0 Strict//EN\"",
			want:    "XHTML 1.0 Strict",
		},
		{
			name:    "unknown html",
			doctype: "html something",
			want:    "HTML (unknown version)",
		},
		{
			name:    "empty",
			doctype: "",
			want:    "Unknown",
		},
		{
			name:    "non-html",
			doctype: "xml",
			want:    "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectHTMLVersion(tt.doctype)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestHTMLVersionCollector_StopsAfterFirst(t *testing.T) {
	// Test that collector stops after finding first DOCTYPE
	collector := &HTMLVersionCollector{}

	// First DOCTYPE
	collector.Collect(html.Token{
		Type: html.DoctypeToken,
		Data: "html",
	})

	assert.Equal(t, "HTML5", collector.version)

	// Try to process another DOCTYPE (shouldn't override)
	collector.Collect(html.Token{
		Type: html.DoctypeToken,
		Data: "HTML PUBLIC \"-//W3C//DTD HTML 4.01//EN\"",
	})

	assert.Equal(t, "HTML5", collector.version, "version should not change")
}

func TestHTMLVersionFactory(t *testing.T) {
	factory := &HTMLVersionFactory{}

	// Test Metadata
	metadata := factory.Metadata()
	assert.Equal(t, "htmlversion", metadata.Name)
	assert.True(t, metadata.Required)

	// Test Create
	config := domain.CollectorConfig{}
	collector, err := factory.Create(config)
	require.NoError(t, err)
	require.NotNil(t, collector)

	// Verify it's the right type
	_, ok := collector.(*HTMLVersionCollector)
	assert.True(t, ok, "collector type = %T, want *HTMLVersionCollector", collector)
}
