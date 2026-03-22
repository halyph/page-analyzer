package analyzer

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// Realistic browser user agents to avoid bot detection
var browserUserAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Safari/605.1.15",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
}

// getRandomUserAgent returns a random browser user agent
func getRandomUserAgent() string {
	if len(browserUserAgents) == 0 {
		return "Mozilla/5.0 (compatible)"
	}
	// Use crypto/rand for random selection
	b := make([]byte, 1)
	_, _ = rand.Read(b)
	idx := int(b[0]) % len(browserUserAgents)
	return browserUserAgents[idx]
}

// checkLink performs an HTTP HEAD request to check link accessibility
// Falls back to GET if HEAD is not allowed (405) or forbidden (403)
func (p *LinkCheckWorkerPool) checkLink(ctx context.Context, urlStr, baseURL string) error {
	// Try HEAD first (faster, less bandwidth)
	err := p.doRequest(ctx, http.MethodHead, urlStr, baseURL)
	if err == nil {
		return nil
	}

	// If HEAD fails with 403, 405, or 406, try GET as fallback
	if httpErr, ok := err.(*httpError); ok {
		if httpErr.StatusCode == 403 || httpErr.StatusCode == 405 || httpErr.StatusCode == 406 {
			return p.doRequest(ctx, http.MethodGet, urlStr, baseURL)
		}
	}

	return err
}

// doRequest performs the actual HTTP request
func (p *LinkCheckWorkerPool) doRequest(ctx context.Context, method, urlStr, baseURL string) error {
	req, err := http.NewRequestWithContext(ctx, method, urlStr, nil)
	if err != nil {
		return fmt.Errorf("invalid_url: %w", err)
	}

	// Set comprehensive browser-like headers to avoid bot detection
	req.Header.Set("User-Agent", getRandomUserAgent())
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("DNT", "1")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Cache-Control", "max-age=0")

	// Set Referer to make it look like we navigated from the analyzed page
	if baseURL != "" {
		req.Header.Set("Referer", baseURL)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return fmt.Errorf("timeout: %w", err)
		}
		if strings.Contains(err.Error(), "connection refused") {
			return fmt.Errorf("connection_refused: %w", err)
		}
		return fmt.Errorf("network_error: %w", err)
	}
	defer resp.Body.Close()

	// Accept 2xx and 3xx as accessible
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return nil
	}

	return &httpError{StatusCode: resp.StatusCode}
}

// httpError wraps HTTP status codes
type httpError struct {
	StatusCode int
}

func (e *httpError) Error() string {
	return fmt.Sprintf("http_%d", e.StatusCode)
}

// extractStatusCode extracts HTTP status code from error
func extractStatusCode(err error) int {
	if err == nil {
		return 0
	}
	var httpErr *httpError
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode
	}
	return 0
}

// extractReason extracts a human-readable reason from error
func extractReason(err error) string {
	if err == nil {
		return ""
	}

	errStr := err.Error()

	if strings.Contains(errStr, "timeout") {
		return "timeout"
	}
	if strings.Contains(errStr, "connection_refused") {
		return "connection refused"
	}
	if strings.Contains(errStr, "network_error") {
		return "network error"
	}
	if strings.HasPrefix(errStr, "http_") {
		code := strings.TrimPrefix(errStr, "http_")
		return fmt.Sprintf("HTTP %s", code)
	}
	if strings.Contains(errStr, "invalid_url") {
		return "invalid URL"
	}

	return "unknown error"
}
