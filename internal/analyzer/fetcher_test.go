package analyzer

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/halyph/page-analyzer/internal/domain"
)

func TestFetcher_FetchSuccess(t *testing.T) {
	html := loadFixture(t, "fetcher_simple.html")

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(html)
	}))
	defer server.Close()

	fetcher := NewFetcher(testFetchingConfig())
	result, err := fetcher.Fetch(context.Background(), server.URL)

	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	if result.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", result.StatusCode, http.StatusOK)
	}

	if result.ContentType != "text/html" {
		t.Errorf("ContentType = %s, want text/html", result.ContentType)
	}

	if !strings.Contains(string(result.Body), "Test") {
		t.Errorf("Body does not contain expected content")
	}
}

func TestFetcher_EmptyURL(t *testing.T) {
	fetcher := NewFetcher(testFetchingConfig())
	_, err := fetcher.Fetch(context.Background(), "")

	if err != domain.ErrEmptyURL {
		t.Errorf("expected ErrEmptyURL, got %v", err)
	}
}

func TestFetcher_InvalidURL(t *testing.T) {
	fetcher := NewFetcher(testFetchingConfig())
	_, err := fetcher.Fetch(context.Background(), "://invalid")

	if err == nil {
		t.Error("expected error for invalid URL")
	}

	if !domain.IsAnalysisError(err) {
		t.Errorf("expected AnalysisError, got %T", err)
	}
}

func TestFetcher_404NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	fetcher := NewFetcher(testFetchingConfig())
	_, err := fetcher.Fetch(context.Background(), server.URL)

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

func TestFetcher_500ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	fetcher := NewFetcher(testFetchingConfig())
	_, err := fetcher.Fetch(context.Background(), server.URL)

	if err == nil {
		t.Fatal("expected error for 500")
	}
}

func TestFetcher_Redirect(t *testing.T) {
	// Create redirect chain: /start -> /middle -> /final
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/start":
			http.Redirect(w, r, "/middle", http.StatusFound)
		case "/middle":
			http.Redirect(w, r, "/final", http.StatusFound)
		case "/final":
			_, _ = w.Write([]byte("Final destination"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	fetcher := NewFetcher(testFetchingConfig())
	result, err := fetcher.Fetch(context.Background(), server.URL+"/start")

	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	// Should have followed redirects
	if !strings.Contains(result.URL, "/final") {
		t.Errorf("URL = %s, expected to contain /final", result.URL)
	}

	if string(result.Body) != "Final destination" {
		t.Errorf("Body = %s, want 'Final destination'", string(result.Body))
	}
}

func TestFetcher_TooManyRedirects(t *testing.T) {
	// Create infinite redirect loop
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/redirect", http.StatusFound)
	}))
	defer server.Close()

	fetcher := NewFetcher(testFetchingConfig())
	_, err := fetcher.Fetch(context.Background(), server.URL+"/redirect")

	if err == nil {
		t.Fatal("expected error for too many redirects")
	}
}

func TestFetcher_Timeout(t *testing.T) {
	// Create server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		_, _ = w.Write([]byte("Too slow"))
	}))
	defer server.Close()

	// Create fetcher with very short timeout
	config := testFetchingConfig()
	config.Timeout = 10 * time.Millisecond
	fetcher := NewFetcher(config)

	_, err := fetcher.Fetch(context.Background(), server.URL)

	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestFetcher_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		_, _ = w.Write([]byte("Response"))
	}))
	defer server.Close()

	fetcher := NewFetcher(testFetchingConfig())

	// Create context that cancels immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel before request

	_, err := fetcher.Fetch(ctx, server.URL)

	if err == nil {
		t.Fatal("expected error for canceled context")
	}
}

func TestFetcher_BodySizeLimit(t *testing.T) {
	// Create large response (1MB)
	largeBody := strings.Repeat("a", 1*1024*1024)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(largeBody))
	}))
	defer server.Close()

	// Create fetcher with small limit
	config := testFetchingConfig()
	config.MaxBodySize = 100 * 1024 // 100KB
	fetcher := NewFetcher(config)

	_, err := fetcher.Fetch(context.Background(), server.URL)

	if err == nil {
		t.Fatal("expected error for body too large")
	}

	if !domain.IsAnalysisError(err) {
		t.Errorf("expected AnalysisError, got %T", err)
	}
}

func TestFetcher_UserAgent(t *testing.T) {
	var receivedUA string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedUA = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte("OK"))
	}))
	defer server.Close()

	config := testFetchingConfig()
	config.UserAgent = "CustomBot/2.0"
	fetcher := NewFetcher(config)

	_, err := fetcher.Fetch(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	if receivedUA != "CustomBot/2.0" {
		t.Errorf("User-Agent = %s, want CustomBot/2.0", receivedUA)
	}
}

func TestFetcher_Headers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom-Header", "test-value")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write([]byte("OK"))
	}))
	defer server.Close()

	fetcher := NewFetcher(testFetchingConfig())
	result, err := fetcher.Fetch(context.Background(), server.URL)

	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	if result.Headers["X-Custom-Header"] != "test-value" {
		t.Errorf("X-Custom-Header = %s, want test-value", result.Headers["X-Custom-Header"])
	}

	if result.Headers["Cache-Control"] != "no-cache" {
		t.Errorf("Cache-Control = %s, want no-cache", result.Headers["Cache-Control"])
	}
}

// Test removed: TestDefaultFetcherConfig
// Config defaults are now in internal/config package and tested there
