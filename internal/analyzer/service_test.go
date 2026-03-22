package analyzer

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/halyph/page-analyzer/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_AnalyzeSuccess(t *testing.T) {
	html := loadFixture(t, "simple.html")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(html)
	}))
	defer server.Close()

	service := NewService(testServiceConfig())
	req := domain.AnalysisRequest{
		URL:     server.URL,
		Options: domain.DefaultOptions(),
	}

	result, err := service.Analyze(context.Background(), req)
	require.NoError(t, err)

	assert.Equal(t, "HTML5", result.HTMLVersion)
	assert.Equal(t, "Example Domain", result.Title)
	assert.Equal(t, 1, result.Headings.H1)
	assert.Equal(t, 1, result.Headings.H2)
	assert.Equal(t, 1, result.Links.InternalCount())
	assert.Equal(t, 1, result.Links.ExternalCount())
	assert.False(t, result.HasLoginForm)
	assert.Equal(t, "v1", result.Version)
	assert.False(t, result.AnalyzedAt.IsZero())
}

func TestService_AnalyzeWithLoginForm(t *testing.T) {
	html := loadFixture(t, "login_form.html")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(html)
	}))
	defer server.Close()

	service := NewService(testServiceConfig())
	req := domain.AnalysisRequest{
		URL:     server.URL,
		Options: domain.DefaultOptions(),
	}

	result, err := service.Analyze(context.Background(), req)
	require.NoError(t, err)

	assert.True(t, result.HasLoginForm)
}

func TestService_AnalyzeComplexPage(t *testing.T) {
	html := loadFixture(t, "complex.html")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(html)
	}))
	defer server.Close()

	service := NewService(testServiceConfig())
	req := domain.AnalysisRequest{
		URL:     server.URL,
		Options: domain.DefaultOptions(),
	}

	result, err := service.Analyze(context.Background(), req)
	require.NoError(t, err)

	assert.Equal(t, "XHTML 1.0 Transitional", result.HTMLVersion)

	expected := domain.HeadingCounts{H1: 1, H2: 2, H3: 3, H4: 1, H5: 1, H6: 1}
	assert.Equal(t, expected, result.Headings)
	assert.Equal(t, 9, result.Headings.Total())
	assert.Equal(t, 3, result.Links.InternalCount())
	assert.Equal(t, 2, result.Links.ExternalCount())
	assert.True(t, result.HasLoginForm)
}

func TestService_AnalyzeRedirect(t *testing.T) {
	finalHTML := loadFixture(t, "redirect_final.html")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "/final", http.StatusFound)
			return
		}
		if r.URL.Path == "/final" {
			_, _ = w.Write(finalHTML)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	service := NewService(testServiceConfig())
	req := domain.AnalysisRequest{
		URL:     server.URL + "/start",
		Options: domain.DefaultOptions(),
	}

	result, err := service.Analyze(context.Background(), req)
	require.NoError(t, err)

	assert.Equal(t, server.URL+"/final", result.URL)
	assert.Equal(t, "Final Page", result.Title)
}

func TestService_Analyze404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	service := NewService(testServiceConfig())
	req := domain.AnalysisRequest{
		URL:     server.URL,
		Options: domain.DefaultOptions(),
	}

	_, err := service.Analyze(context.Background(), req)
	require.Error(t, err)
	assert.True(t, domain.IsAnalysisError(err))

	statusCode := domain.GetStatusCode(err)
	assert.Equal(t, http.StatusBadGateway, statusCode)
}

func TestService_AnalyzeInvalidURL(t *testing.T) {
	service := NewService(testServiceConfig())
	req := domain.AnalysisRequest{
		URL:     "://invalid",
		Options: domain.DefaultOptions(),
	}

	_, err := service.Analyze(context.Background(), req)
	require.Error(t, err)
	assert.True(t, domain.IsAnalysisError(err))
}

func TestService_AnalyzeEmptyURL(t *testing.T) {
	service := NewService(testServiceConfig())
	req := domain.AnalysisRequest{
		URL:     "",
		Options: domain.DefaultOptions(),
	}

	_, err := service.Analyze(context.Background(), req)
	assert.ErrorIs(t, err, domain.ErrEmptyURL)
}

func TestService_AnalyzeMalformedHTML(t *testing.T) {
	html := loadFixture(t, "malformed.html")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(html)
	}))
	defer server.Close()

	service := NewService(testServiceConfig())
	req := domain.AnalysisRequest{
		URL:     server.URL,
		Options: domain.DefaultOptions(),
	}

	result, err := service.Analyze(context.Background(), req)
	require.NoError(t, err, "parser should be forgiving")

	assert.Equal(t, "Broken", result.Title)
	assert.Equal(t, 1, result.Headings.H1)
}

func TestService_AnalyzeEmptyPage(t *testing.T) {
	html := ``

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(html))
	}))
	defer server.Close()

	service := NewService(testServiceConfig())
	req := domain.AnalysisRequest{
		URL:     server.URL,
		Options: domain.DefaultOptions(),
	}

	_, err := service.Analyze(context.Background(), req)
	require.Error(t, err)
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

	service := NewService(testServiceConfig())
	req := domain.AnalysisRequest{
		URL: server.URL,
		Options: domain.AnalysisOptions{
			MaxLinks: 10, // Limit to 10 links
		},
	}

	result, err := service.Analyze(context.Background(), req)
	require.NoError(t, err)

	totalCollected := result.Links.InternalCount() + result.Links.ExternalCount()
	assert.Equal(t, 10, totalCollected)
	assert.True(t, result.Links.Truncated)
	assert.Equal(t, 50, result.Links.TotalFound)
}

func TestDefaultServiceConfig(t *testing.T) {
	config := testServiceConfig()

	assert.NotZero(t, config.Fetcher.Timeout)
	assert.NotZero(t, config.Walker.MaxTokens)
}

func TestService_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html><title>Test</title></html>"))
	}))
	defer server.Close()

	service := NewService(testServiceConfig())

	// Create canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	req := domain.AnalysisRequest{
		URL:     server.URL,
		Options: domain.DefaultOptions(),
	}

	_, err := service.Analyze(ctx, req)
	require.Error(t, err)
}
