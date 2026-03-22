package analyzer

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/halyph/page-analyzer/internal/config"
	"github.com/halyph/page-analyzer/internal/domain"
	"github.com/halyph/page-analyzer/internal/observability"
	"go.opentelemetry.io/otel/attribute"
)

// Fetcher retrieves webpage content via HTTP
type Fetcher struct {
	client      *http.Client
	maxBodySize int64
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
	}
}

// Fetch retrieves the content of a URL
func (f *Fetcher) Fetch(ctx context.Context, url string) (*domain.FetchResult, error) {
	ctx, span := observability.StartSpan(ctx, "fetcher.Fetch",
		observability.AttrHTTPURL.String(url),
		observability.AttrHTTPMethod.String("GET"),
	)
	defer span.End()

	// Validate URL
	if url == "" {
		err := domain.ErrEmptyURL
		observability.RecordError(span, err)
		return nil, err
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		observability.RecordError(span, err)
		return nil, domain.ErrInvalidURLWithReason(url, err)
	}

	// Set headers
	req.Header.Set("User-Agent", "PageAnalyzer/1.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	// Execute request
	resp, err := f.client.Do(req)
	if err != nil {
		// Check if it's a timeout
		if ctx.Err() == context.DeadlineExceeded {
			err := domain.ErrTimeoutWithContext(url, f.client.Timeout.String())
			observability.RecordError(span, err)
			return nil, err
		}
		err := domain.ErrConnectionFailed(url, err)
		observability.RecordError(span, err)
		return nil, err
	}
	defer resp.Body.Close()

	// Record status code
	observability.SetSpanAttributes(span, observability.AttrHTTPStatusCode.Int(resp.StatusCode))

	// Check status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		err := domain.ErrFetchFailedWithStatus(url, resp.StatusCode, resp.Status)
		observability.RecordError(span, err)
		return nil, err
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
		err := domain.ErrBodyTooLargeWithSize(url, int64(len(body)), f.maxBodySize)
		observability.RecordError(span, err)
		return nil, err
	}

	// Record body size
	observability.SetSpanAttributes(span,
		attribute.Int("http.response.body.size", len(body)),
		attribute.String("http.response.content_type", resp.Header.Get("Content-Type")),
	)

	// Extract headers
	headers := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	finalURL := resp.Request.URL.String()
	if finalURL != url {
		// Record redirect
		observability.SetSpanAttributes(span, attribute.String("http.redirect.final_url", finalURL))
	}

	return &domain.FetchResult{
		URL:         finalURL, // Final URL after redirects
		StatusCode:  resp.StatusCode,
		ContentType: resp.Header.Get("Content-Type"),
		Body:        body,
		Headers:     headers,
	}, nil
}
