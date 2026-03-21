package collectors

import (
	"net/url"
	"strings"
	"testing"

	"github.com/halyph/page-analyzer/internal/domain"
	"golang.org/x/net/html"
)

func TestLinksCollector(t *testing.T) {
	tests := []struct {
		name             string
		baseURL          string
		htmlInput        string
		wantInternal     []string
		wantExternal     []string
		wantTruncated    bool
	}{
		{
			name:    "no links",
			baseURL: "https://example.com",
			htmlInput: `<html><body><p>No links here</p></body></html>`,
			wantInternal: []string{},
			wantExternal: []string{},
		},
		{
			name:    "single internal link",
			baseURL: "https://example.com",
			htmlInput: `<html><body><a href="/about">About</a></body></html>`,
			wantInternal: []string{"https://example.com/about"},
			wantExternal: []string{},
		},
		{
			name:    "single external link",
			baseURL: "https://example.com",
			htmlInput: `<html><body><a href="https://other.com">Other</a></body></html>`,
			wantInternal: []string{},
			wantExternal: []string{"https://other.com"},
		},
		{
			name:    "mixed internal and external",
			baseURL: "https://example.com",
			htmlInput: `<html><body>
				<a href="/page1">Page 1</a>
				<a href="https://other.com">Other</a>
				<a href="/page2">Page 2</a>
			</body></html>`,
			wantInternal: []string{"https://example.com/page1", "https://example.com/page2"},
			wantExternal: []string{"https://other.com"},
		},
		{
			name:    "deduplication - same link twice",
			baseURL: "https://example.com",
			htmlInput: `<html><body>
				<a href="/page">Page</a>
				<a href="/page">Page Again</a>
			</body></html>`,
			wantInternal: []string{"https://example.com/page"},
			wantExternal: []string{},
		},
		{
			name:    "fragment removal",
			baseURL: "https://example.com",
			htmlInput: `<html><body>
				<a href="/page#section1">Section 1</a>
				<a href="/page#section2">Section 2</a>
			</body></html>`,
			wantInternal: []string{"https://example.com/page"},
			wantExternal: []string{},
		},
		{
			name:    "relative URLs",
			baseURL: "https://example.com/dir/",
			htmlInput: `<html><body>
				<a href="page.html">Relative</a>
				<a href="./page.html">Dot relative</a>
				<a href="../other.html">Parent</a>
			</body></html>`,
			wantInternal: []string{
				"https://example.com/dir/page.html",
				"https://example.com/other.html",
			},
			wantExternal: []string{},
		},
		{
			name:    "subdomain is external",
			baseURL: "https://example.com",
			htmlInput: `<html><body>
				<a href="https://sub.example.com">Subdomain</a>
			</body></html>`,
			wantInternal: []string{},
			wantExternal: []string{"https://sub.example.com"},
		},
		{
			name:    "different scheme is external",
			baseURL: "https://example.com",
			htmlInput: `<html><body>
				<a href="http://example.com">HTTP version</a>
			</body></html>`,
			wantInternal: []string{},
			wantExternal: []string{"http://example.com"},
		},
		{
			name:    "skip javascript links",
			baseURL: "https://example.com",
			htmlInput: `<html><body>
				<a href="javascript:void(0)">JS Link</a>
				<a href="/real">Real Link</a>
			</body></html>`,
			wantInternal: []string{"https://example.com/real"},
			wantExternal: []string{},
		},
		{
			name:    "skip mailto links",
			baseURL: "https://example.com",
			htmlInput: `<html><body>
				<a href="mailto:test@example.com">Email</a>
				<a href="/contact">Contact</a>
			</body></html>`,
			wantInternal: []string{"https://example.com/contact"},
			wantExternal: []string{},
		},
		{
			name:    "skip tel links",
			baseURL: "https://example.com",
			htmlInput: `<html><body>
				<a href="tel:+1234567890">Call</a>
			</body></html>`,
			wantInternal: []string{},
			wantExternal: []string{},
		},
		{
			name:    "skip fragment-only links",
			baseURL: "https://example.com",
			htmlInput: `<html><body>
				<a href="#section">Jump to section</a>
			</body></html>`,
			wantInternal: []string{},
			wantExternal: []string{},
		},
		{
			name:    "empty href",
			baseURL: "https://example.com",
			htmlInput: `<html><body>
				<a href="">Empty</a>
				<a>No href</a>
			</body></html>`,
			wantInternal: []string{},
			wantExternal: []string{},
		},
		{
			name:    "query parameters preserved",
			baseURL: "https://example.com",
			htmlInput: `<html><body>
				<a href="/search?q=test">Search</a>
			</body></html>`,
			wantInternal: []string{"https://example.com/search?q=test"},
			wantExternal: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector, err := NewLinksCollector(tt.baseURL, 10000)
			if err != nil {
				t.Fatalf("NewLinksCollector() error = %v", err)
			}

			result := domain.NewAnalysisResult(tt.baseURL)

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

			// Check internal links
			if len(result.Links.Internal) != len(tt.wantInternal) {
				t.Errorf("Internal links count = %d, want %d", len(result.Links.Internal), len(tt.wantInternal))
			}
			for i, want := range tt.wantInternal {
				if i >= len(result.Links.Internal) {
					break
				}
				if result.Links.Internal[i] != want {
					t.Errorf("Internal[%d] = %q, want %q", i, result.Links.Internal[i], want)
				}
			}

			// Check external links
			if len(result.Links.External) != len(tt.wantExternal) {
				t.Errorf("External links count = %d, want %d", len(result.Links.External), len(tt.wantExternal))
			}
			for i, want := range tt.wantExternal {
				if i >= len(result.Links.External) {
					break
				}
				if result.Links.External[i] != want {
					t.Errorf("External[%d] = %q, want %q", i, result.Links.External[i], want)
				}
			}

			// Check truncation
			if result.Links.Truncated != tt.wantTruncated {
				t.Errorf("Truncated = %v, want %v", result.Links.Truncated, tt.wantTruncated)
			}
		})
	}
}

func TestLinksCollector_MaxLinksLimit(t *testing.T) {
	baseURL := "https://example.com"
	maxLinks := 5

	// Generate HTML with 10 links
	var sb strings.Builder
	sb.WriteString("<html><body>")
	for i := 0; i < 10; i++ {
		sb.WriteString(`<a href="/page`)
		sb.WriteString(string(rune('0' + i)))
		sb.WriteString(`">Link</a>`)
	}
	sb.WriteString("</body></html>")

	collector, err := NewLinksCollector(baseURL, maxLinks)
	if err != nil {
		t.Fatalf("NewLinksCollector() error = %v", err)
	}

	result := domain.NewAnalysisResult(baseURL)

	tokenizer := html.NewTokenizer(strings.NewReader(sb.String()))
	for {
		tokenType := tokenizer.Next()
		if tokenType == html.ErrorToken {
			break
		}
		collector.Collect(tokenizer.Token())
	}

	collector.Apply(result)

	totalCollected := len(result.Links.Internal) + len(result.Links.External)
	if totalCollected != maxLinks {
		t.Errorf("Total collected = %d, want %d", totalCollected, maxLinks)
	}

	if !result.Links.Truncated {
		t.Error("Expected Truncated to be true")
	}

	if result.Links.TotalFound != 10 {
		t.Errorf("TotalFound = %d, want 10", result.Links.TotalFound)
	}
}

func TestIsSameOrigin(t *testing.T) {
	tests := []struct {
		name   string
		base   string
		target string
		want   bool
	}{
		{
			name:   "exact match",
			base:   "https://example.com",
			target: "https://example.com",
			want:   true,
		},
		{
			name:   "same origin different path",
			base:   "https://example.com/page1",
			target: "https://example.com/page2",
			want:   true,
		},
		{
			name:   "different scheme",
			base:   "https://example.com",
			target: "http://example.com",
			want:   false,
		},
		{
			name:   "different host",
			base:   "https://example.com",
			target: "https://other.com",
			want:   false,
		},
		{
			name:   "subdomain",
			base:   "https://example.com",
			target: "https://sub.example.com",
			want:   false,
		},
		{
			name:   "different port",
			base:   "https://example.com:443",
			target: "https://example.com:8080",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base, _ := url.Parse(tt.base)
			target, _ := url.Parse(tt.target)
			got := isSameOrigin(base, target)
			if got != tt.want {
				t.Errorf("isSameOrigin(%q, %q) = %v, want %v", tt.base, tt.target, got, tt.want)
			}
		})
	}
}

func TestIsNonHTTPScheme(t *testing.T) {
	tests := []struct {
		href string
		want bool
	}{
		{"javascript:void(0)", true},
		{"JavaScript:alert(1)", true}, // Case insensitive
		{"mailto:test@example.com", true},
		{"tel:+1234567890", true},
		{"ftp://files.example.com", true},
		{"data:text/html,<h1>Test</h1>", true},
		{"#section", true},
		{"http://example.com", false},
		{"https://example.com", false},
		{"/relative/path", false},
		{"//example.com", false}, // Protocol-relative
	}

	for _, tt := range tests {
		t.Run(tt.href, func(t *testing.T) {
			got := isNonHTTPScheme(tt.href)
			if got != tt.want {
				t.Errorf("isNonHTTPScheme(%q) = %v, want %v", tt.href, got, tt.want)
			}
		})
	}
}

func TestExtractAttr(t *testing.T) {
	tests := []struct {
		name  string
		attrs []html.Attribute
		key   string
		want  string
	}{
		{
			name:  "found",
			attrs: []html.Attribute{{Key: "href", Val: "https://example.com"}},
			key:   "href",
			want:  "https://example.com",
		},
		{
			name:  "not found",
			attrs: []html.Attribute{{Key: "class", Val: "link"}},
			key:   "href",
			want:  "",
		},
		{
			name:  "empty attrs",
			attrs: []html.Attribute{},
			key:   "href",
			want:  "",
		},
		{
			name: "multiple attrs",
			attrs: []html.Attribute{
				{Key: "id", Val: "link1"},
				{Key: "href", Val: "/page"},
				{Key: "class", Val: "nav"},
			},
			key:  "href",
			want: "/page",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractAttr(tt.attrs, tt.key)
			if got != tt.want {
				t.Errorf("extractAttr() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLinksFactory(t *testing.T) {
	factory := &LinksFactory{}

	// Test Metadata
	metadata := factory.Metadata()
	if metadata.Name != "links" {
		t.Errorf("Name = %q, want links", metadata.Name)
	}
	if !metadata.Required {
		t.Error("Expected Required to be true")
	}

	// Test Create
	config := domain.CollectorConfig{
		BaseURL:  "https://example.com",
		MaxItems: 1000,
	}
	collector, err := factory.Create(config)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if collector == nil {
		t.Fatal("Create() returned nil collector")
	}

	// Verify it's the right type
	linksCollector, ok := collector.(*LinksCollector)
	if !ok {
		t.Errorf("collector type = %T, want *LinksCollector", collector)
	}

	// Verify max links was set
	if linksCollector.maxLinks != 1000 {
		t.Errorf("maxLinks = %d, want 1000", linksCollector.maxLinks)
	}
}

func TestNewLinksCollector_InvalidBaseURL(t *testing.T) {
	_, err := NewLinksCollector("://invalid", 1000)
	if err == nil {
		t.Error("Expected error for invalid base URL")
	}
}

func TestNewLinksCollector_DefaultMaxLinks(t *testing.T) {
	collector, err := NewLinksCollector("https://example.com", 0)
	if err != nil {
		t.Fatalf("NewLinksCollector() error = %v", err)
	}

	if collector.maxLinks != 10000 {
		t.Errorf("maxLinks = %d, want 10000 (default)", collector.maxLinks)
	}
}
