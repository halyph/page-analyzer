package analyzer

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/halyph/page-analyzer/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, result.StatusCode)
	assert.Equal(t, "text/html", result.ContentType)
	assert.Contains(t, string(result.Body), "Test")
}

func TestFetcher_EmptyURL(t *testing.T) {
	fetcher := NewFetcher(testFetchingConfig())
	_, err := fetcher.Fetch(context.Background(), "")

	assert.ErrorIs(t, err, domain.ErrEmptyURL)
}

func TestFetcher_InvalidURL(t *testing.T) {
	fetcher := NewFetcher(testFetchingConfig())
	_, err := fetcher.Fetch(context.Background(), "://invalid")

	require.Error(t, err)
	assert.True(t, domain.IsAnalysisError(err))
}

func TestFetcher_404NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	fetcher := NewFetcher(testFetchingConfig())
	_, err := fetcher.Fetch(context.Background(), server.URL)

	require.Error(t, err)
	assert.True(t, domain.IsAnalysisError(err))

	statusCode := domain.GetStatusCode(err)
	assert.Equal(t, http.StatusBadGateway, statusCode)
}

func TestFetcher_500ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	fetcher := NewFetcher(testFetchingConfig())
	_, err := fetcher.Fetch(context.Background(), server.URL)

	require.Error(t, err)
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

	require.NoError(t, err)
	assert.Contains(t, result.URL, "/final")
	assert.Equal(t, "Final destination", string(result.Body))
}

func TestFetcher_TooManyRedirects(t *testing.T) {
	// Create infinite redirect loop
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/redirect", http.StatusFound)
	}))
	defer server.Close()

	fetcher := NewFetcher(testFetchingConfig())
	_, err := fetcher.Fetch(context.Background(), server.URL+"/redirect")

	require.Error(t, err)
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

	require.Error(t, err)
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

	require.Error(t, err)
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

	require.Error(t, err)
	assert.True(t, domain.IsAnalysisError(err))
}

func TestFetcher_UserAgent(t *testing.T) {
	var receivedUA string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedUA = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte("OK"))
	}))
	defer server.Close()

	fetcher := NewFetcher(testFetchingConfig())

	_, err := fetcher.Fetch(context.Background(), server.URL)
	require.NoError(t, err)

	assert.Equal(t, "PageAnalyzer/1.0", receivedUA)
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

	require.NoError(t, err)
	assert.Equal(t, "test-value", result.Headers["X-Custom-Header"])
	assert.Equal(t, "no-cache", result.Headers["Cache-Control"])
}
