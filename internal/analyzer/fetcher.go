package analyzer

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/halyph/page-analyzer/internal/config"
	"github.com/halyph/page-analyzer/internal/domain"
)

// Fetcher retrieves webpage content via HTTP
type Fetcher struct {
	client      *http.Client
	maxBodySize int64
	userAgent   string
}

// NewFetcher creates a new HTTP fetcher with the given configuration
func NewFetcher(cfg config.FetchingConfig) *Fetcher {
	return &Fetcher{
		client: &http.Client{
			Timeout: cfg.Timeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
			// Follow redirects (up to 10)
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return fmt.Errorf("stopped after 10 redirects")
				}
				return nil
			},
		},
		maxBodySize: cfg.MaxBodySize,
		userAgent:   cfg.UserAgent,
	}
}

// Fetch retrieves the content of a URL
func (f *Fetcher) Fetch(ctx context.Context, url string) (*domain.FetchResult, error) {
	// Validate URL
	if url == "" {
		return nil, domain.ErrEmptyURL
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, domain.ErrInvalidURLWithReason(url, err)
	}

	// Set headers
	req.Header.Set("User-Agent", f.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	// Execute request
	resp, err := f.client.Do(req)
	if err != nil {
		// Check if it's a timeout
		if ctx.Err() == context.DeadlineExceeded {
			return nil, domain.ErrTimeoutWithContext(url, f.client.Timeout.String())
		}
		return nil, domain.ErrConnectionFailed(url, err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, domain.ErrFetchFailedWithStatus(url, resp.StatusCode, resp.Status)
	}

	// Limit body size
	limitedReader := io.LimitReader(resp.Body, f.maxBodySize+1)

	// Read body
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, domain.WrapAnalysisError(
			http.StatusBadGateway,
			fmt.Sprintf("failed to read response body: %s", url),
			err,
		)
	}

	// Check if body exceeded limit
	if int64(len(body)) > f.maxBodySize {
		return nil, domain.ErrBodyTooLargeWithSize(url, int64(len(body)), f.maxBodySize)
	}

	// Extract headers
	headers := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	return &domain.FetchResult{
		URL:         resp.Request.URL.String(), // Final URL after redirects
		StatusCode:  resp.StatusCode,
		ContentType: resp.Header.Get("Content-Type"),
		Body:        body,
		Headers:     headers,
	}, nil
}
