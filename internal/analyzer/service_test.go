package analyzer

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/halyph/page-analyzer/internal/domain"
)

func TestService_AnalyzeSuccess(t *testing.T) {
	html := `<!DOCTYPE html>
<html>
<head><title>Example Domain</title></head>
<body>
	<h1>Example Domain</h1>
	<h2>Section 1</h2>
	<p>This domain is for use in illustrative examples.</p>
	<a href="/more">More information...</a>
	<a href="https://iana.org">IANA</a>
</body>
</html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(html))
	}))
	defer server.Close()

	service := NewService(DefaultServiceConfig())
	req := domain.AnalysisRequest{
		URL:     server.URL,
		Options: domain.DefaultOptions(),
	}

	result, err := service.Analyze(context.Background(), req)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	// Verify HTML version
	if result.HTMLVersion != "HTML5" {
		t.Errorf("HTMLVersion = %s, want HTML5", result.HTMLVersion)
	}

	// Verify title
	if result.Title != "Example Domain" {
		t.Errorf("Title = %s, want 'Example Domain'", result.Title)
	}

	// Verify headings
	if result.Headings.H1 != 1 {
		t.Errorf("H1 = %d, want 1", result.Headings.H1)
	}
	if result.Headings.H2 != 1 {
		t.Errorf("H2 = %d, want 1", result.Headings.H2)
	}

	// Verify links
	if result.Links.InternalCount() != 1 {
		t.Errorf("Internal links = %d, want 1", result.Links.InternalCount())
	}
	if result.Links.ExternalCount() != 1 {
		t.Errorf("External links = %d, want 1", result.Links.ExternalCount())
	}

	// Verify login form
	if result.HasLoginForm {
		t.Error("HasLoginForm = true, want false")
	}

	// Verify metadata
	if result.Version != "v1" {
		t.Errorf("Version = %s, want v1", result.Version)
	}
	if result.AnalyzedAt.IsZero() {
		t.Error("AnalyzedAt should be set")
	}
}

func TestService_AnalyzeWithLoginForm(t *testing.T) {
	html := `<!DOCTYPE html>
<html>
<head><title>Login Page</title></head>
<body>
	<h1>Login</h1>
	<form action="/login" method="post">
		<input type="text" name="username" />
		<input type="password" name="password" />
		<button type="submit">Login</button>
	</form>
</body>
</html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(html))
	}))
	defer server.Close()

	service := NewService(DefaultServiceConfig())
	req := domain.AnalysisRequest{
		URL:     server.URL,
		Options: domain.DefaultOptions(),
	}

	result, err := service.Analyze(context.Background(), req)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	if !result.HasLoginForm {
		t.Error("HasLoginForm = false, want true")
	}
}

func TestService_AnalyzeComplexPage(t *testing.T) {
	html := `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Transitional//EN">
<html>
<head><title>Complex Page</title></head>
<body>
	<h1>Main</h1>
	<h2>Section 1</h2>
	<h3>Subsection 1.1</h3>
	<h3>Subsection 1.2</h3>
	<h2>Section 2</h2>
	<h3>Subsection 2.1</h3>
	<h4>Details</h4>
	<h5>Fine print</h5>
	<h6>Very fine print</h6>

	<a href="/page1">Page 1</a>
	<a href="/page2">Page 2</a>
	<a href="/page3">Page 3</a>
	<a href="https://external.com">External</a>
	<a href="https://another.com">Another</a>

	<form>
		<input type="email" name="email" />
		<input type="password" name="pwd" />
	</form>
</body>
</html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(html))
	}))
	defer server.Close()

	service := NewService(DefaultServiceConfig())
	req := domain.AnalysisRequest{
		URL:     server.URL,
		Options: domain.DefaultOptions(),
	}

	result, err := service.Analyze(context.Background(), req)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	// Verify HTML version
	if result.HTMLVersion != "XHTML 1.0 Transitional" {
		t.Errorf("HTMLVersion = %s, want XHTML 1.0 Transitional", result.HTMLVersion)
	}

	// Verify all heading levels
	expected := domain.HeadingCounts{H1: 1, H2: 2, H3: 3, H4: 1, H5: 1, H6: 1}
	if result.Headings != expected {
		t.Errorf("Headings = %+v, want %+v", result.Headings, expected)
	}

	// Verify total headings
	if result.Headings.Total() != 9 {
		t.Errorf("Total headings = %d, want 9", result.Headings.Total())
	}

	// Verify links
	if result.Links.InternalCount() != 3 {
		t.Errorf("Internal links = %d, want 3", result.Links.InternalCount())
	}
	if result.Links.ExternalCount() != 2 {
		t.Errorf("External links = %d, want 2", result.Links.ExternalCount())
	}

	// Verify login form
	if !result.HasLoginForm {
		t.Error("HasLoginForm = false, want true")
	}
}

func TestService_AnalyzeRedirect(t *testing.T) {
	finalHTML := `<html><head><title>Final Page</title></head><body>Final content</body></html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "/final", http.StatusFound)
			return
		}
		if r.URL.Path == "/final" {
			_, _ = w.Write([]byte(finalHTML))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	service := NewService(DefaultServiceConfig())
	req := domain.AnalysisRequest{
		URL:     server.URL + "/start",
		Options: domain.DefaultOptions(),
	}

	result, err := service.Analyze(context.Background(), req)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	// URL should be updated to final destination
	if result.URL != server.URL+"/final" {
		t.Errorf("URL = %s, want %s", result.URL, server.URL+"/final")
	}

	if result.Title != "Final Page" {
		t.Errorf("Title = %s, want 'Final Page'", result.Title)
	}
}

func TestService_Analyze404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	service := NewService(DefaultServiceConfig())
	req := domain.AnalysisRequest{
		URL:     server.URL,
		Options: domain.DefaultOptions(),
	}

	_, err := service.Analyze(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for 404")
	}

	if !domain.IsAnalysisError(err) {
		t.Errorf("expected AnalysisError, got %T", err)
	}

	statusCode := domain.GetStatusCode(err)
	if statusCode != http.StatusBadGateway {
		t.Errorf("StatusCode = %d, want %d", statusCode, http.StatusBadGateway)
	}
}

func TestService_AnalyzeInvalidURL(t *testing.T) {
	service := NewService(DefaultServiceConfig())
	req := domain.AnalysisRequest{
		URL:     "://invalid",
		Options: domain.DefaultOptions(),
	}

	_, err := service.Analyze(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}

	if !domain.IsAnalysisError(err) {
		t.Errorf("expected AnalysisError, got %T", err)
	}
}

func TestService_AnalyzeEmptyURL(t *testing.T) {
	service := NewService(DefaultServiceConfig())
	req := domain.AnalysisRequest{
		URL:     "",
		Options: domain.DefaultOptions(),
	}

	_, err := service.Analyze(context.Background(), req)
	if err != domain.ErrEmptyURL {
		t.Errorf("expected ErrEmptyURL, got %v", err)
	}
}

func TestService_AnalyzeMalformedHTML(t *testing.T) {
	html := `<html><head><title>Broken</title></head><h1>Unclosed<p>Tags everywhere`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(html))
	}))
	defer server.Close()

	service := NewService(DefaultServiceConfig())
	req := domain.AnalysisRequest{
		URL:     server.URL,
		Options: domain.DefaultOptions(),
	}

	result, err := service.Analyze(context.Background(), req)
	if err != nil {
		t.Fatalf("Analyze() error = %v (parser should be forgiving)", err)
	}

	// HTML parser should extract title even from malformed HTML
	if result.Title != "Broken" {
		t.Errorf("Title = %q, want 'Broken'", result.Title)
	}

	// Should still count H1
	if result.Headings.H1 != 1 {
		t.Errorf("H1 = %d, want 1", result.Headings.H1)
	}
}

func TestService_AnalyzeEmptyPage(t *testing.T) {
	html := ``

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(html))
	}))
	defer server.Close()

	service := NewService(DefaultServiceConfig())
	req := domain.AnalysisRequest{
		URL:     server.URL,
		Options: domain.DefaultOptions(),
	}

	_, err := service.Analyze(context.Background(), req)
	if err == nil {
		t.Error("expected error for empty HTML")
	}
}

func TestService_AnalyzeWithMaxLinks(t *testing.T) {
	// Generate page with many unique links
	html := `<html><body>`
	for i := 0; i < 50; i++ {
		html += `<a href="/page` + fmt.Sprintf("%d", i) + `">Link</a>`
	}
	html += `</body></html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(html))
	}))
	defer server.Close()

	service := NewService(DefaultServiceConfig())
	req := domain.AnalysisRequest{
		URL: server.URL,
		Options: domain.AnalysisOptions{
			MaxLinks: 10, // Limit to 10 links
		},
	}

	result, err := service.Analyze(context.Background(), req)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	totalCollected := result.Links.InternalCount() + result.Links.ExternalCount()
	if totalCollected != 10 {
		t.Errorf("Total collected links = %d, want 10", totalCollected)
	}

	if !result.Links.Truncated {
		t.Error("Expected Truncated to be true")
	}

	// TotalFound should be higher (all unique links discovered)
	if result.Links.TotalFound != 50 {
		t.Errorf("TotalFound = %d, expected 50", result.Links.TotalFound)
	}
}

func TestDefaultServiceConfig(t *testing.T) {
	config := DefaultServiceConfig()

	// Should include default fetcher config
	if config.Fetcher.Timeout == 0 {
		t.Error("Fetcher timeout should be set")
	}

	// Should include default walker config
	if config.Walker.MaxTokens == 0 {
		t.Error("Walker MaxTokens should be set")
	}
}

func TestService_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html><title>Test</title></html>"))
	}))
	defer server.Close()

	service := NewService(DefaultServiceConfig())

	// Create canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	req := domain.AnalysisRequest{
		URL:     server.URL,
		Options: domain.DefaultOptions(),
	}

	_, err := service.Analyze(ctx, req)
	if err == nil {
		t.Error("expected error for canceled context")
	}
}
