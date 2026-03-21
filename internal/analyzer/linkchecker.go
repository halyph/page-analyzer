package analyzer

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"sync"
	"time"

	"github.com/halyph/page-analyzer/internal/config"
	"github.com/halyph/page-analyzer/internal/domain"
)

// Realistic browser user agents to avoid bot detection
var browserUserAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Safari/605.1.15",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
}

// LinkCheckWorkerPool manages concurrent link checking with bounded resources
type LinkCheckWorkerPool struct {
	workers     int
	jobs        chan *domain.LinkCheckJob
	results     sync.Map // JobID → *LinkCheckJob
	client      *http.Client
	timeout     time.Duration
	maxAge      time.Duration
	userAgent   string
	stopCleanup chan struct{}
	ctx         context.Context
	cancel      context.CancelFunc
}

// LinkCheckConfig configures the link checker worker pool
type LinkCheckConfig struct {
	Workers    int           // Number of concurrent workers
	QueueSize  int           // Job queue buffer size
	Timeout    time.Duration // HTTP request timeout
	JobMaxAge  time.Duration // How long to keep completed jobs
	UserAgent  string        // User agent for HTTP requests
	JobWorkers int           // Concurrent checks within a single job
}

// NewLinkCheckConfigFromGlobal converts global config to LinkCheckConfig
func NewLinkCheckConfigFromGlobal(cfg config.LinkCheckingConfig, userAgent string) LinkCheckConfig {
	return LinkCheckConfig{
		Workers:    cfg.Workers,
		QueueSize:  cfg.QueueSize,
		Timeout:    cfg.CheckTimeout,
		JobMaxAge:  10 * time.Minute, // Use reasonable default
		UserAgent:  userAgent,
		JobWorkers: cfg.JobWorkers,
	}
}

// NewLinkCheckWorkerPool creates a new worker pool for link checking
func NewLinkCheckWorkerPool(cfg LinkCheckConfig) *LinkCheckWorkerPool {
	if cfg.Workers <= 0 {
		cfg.Workers = 20
	}
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = 100
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Second
	}
	if cfg.JobMaxAge <= 0 {
		cfg.JobMaxAge = 10 * time.Minute
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = "PageAnalyzer/1.0"
	}
	if cfg.JobWorkers <= 0 {
		cfg.JobWorkers = 10
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Enable cookie handling for session-based sites
	jar, _ := cookiejar.New(nil)

	return &LinkCheckWorkerPool{
		workers:     cfg.Workers,
		jobs:        make(chan *domain.LinkCheckJob, cfg.QueueSize),
		timeout:     cfg.Timeout,
		maxAge:      cfg.JobMaxAge,
		userAgent:   cfg.UserAgent,
		stopCleanup: make(chan struct{}),
		ctx:         ctx,
		cancel:      cancel,
		client: &http.Client{
			Timeout: cfg.Timeout,
			Jar:     jar,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// Follow up to 5 redirects
				if len(via) >= 5 {
					return fmt.Errorf("stopped after 5 redirects")
				}
				return nil
			},
		},
	}
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

// Start launches the worker pool and cleanup goroutine
func (p *LinkCheckWorkerPool) Start() {
	// Start worker goroutines
	for i := 0; i < p.workers; i++ {
		go p.worker(i)
	}

	// Start cleanup goroutine
	go p.cleanup()
}

// Stop gracefully shuts down the worker pool
func (p *LinkCheckWorkerPool) Stop() {
	p.cancel()
	close(p.stopCleanup)
}

// worker processes jobs from the queue
func (p *LinkCheckWorkerPool) worker(_ int) {
	for {
		select {
		case <-p.ctx.Done():
			return
		case job := <-p.jobs:
			p.processJob(job)
		}
	}
}

// processJob checks all links in a job
func (p *LinkCheckWorkerPool) processJob(job *domain.LinkCheckJob) {
	// Update job status with lock
	job.Mu.Lock()
	job.Status = domain.LinkCheckInProgress
	now := time.Now()
	job.StartedAt = &now
	job.Mu.Unlock()

	var inaccessible []domain.LinkError
	accessible := 0

	// Bounded concurrency within job (prevent resource exhaustion)
	sem := make(chan struct{}, 10) // Max 10 concurrent checks per job
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, urlStr := range job.URLs {
		wg.Add(1)
		sem <- struct{}{} // Acquire

		go func(u string) {
			defer wg.Done()
			defer func() { <-sem }() // Release

			if err := p.checkLink(p.ctx, u, job.BaseURL); err != nil {
				mu.Lock()
				inaccessible = append(inaccessible, domain.LinkError{
					URL:        u,
					StatusCode: extractStatusCode(err),
					Reason:     extractReason(err),
				})
				mu.Unlock()
			} else {
				mu.Lock()
				accessible++
				mu.Unlock()
			}
		}(urlStr)
	}

	wg.Wait()

	// Finalize job
	completed := time.Now()
	job.Mu.Lock()
	job.CompletedAt = &completed
	job.Status = domain.LinkCheckCompleted
	job.Result = &domain.LinkCheckResult{
		Checked:      len(job.URLs),
		Accessible:   accessible,
		Inaccessible: inaccessible,
		Duration:     completed.Sub(*job.StartedAt).String(),
		CompletedAt:  completed,
	}
	job.Mu.Unlock()

	p.results.Store(job.ID, job)
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

	// Use realistic browser user agent or custom if not bot-like
	userAgent := p.userAgent
	if strings.Contains(userAgent, "PageAnalyzer") || strings.Contains(userAgent, "bot") || strings.Contains(userAgent, "Bot") {
		userAgent = getRandomUserAgent()
	}

	// Set comprehensive browser-like headers to avoid bot detection
	req.Header.Set("User-Agent", userAgent)
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

// Submit queues a link check job and returns a job ID
func (p *LinkCheckWorkerPool) Submit(urls []string, baseURL string) string {
	jobID := generateJobID()

	job := &domain.LinkCheckJob{
		ID:        jobID,
		URLs:      urls,
		BaseURL:   baseURL,
		Status:    domain.LinkCheckPending,
		CreatedAt: time.Now(),
	}

	p.results.Store(jobID, job)

	select {
	case p.jobs <- job:
		// Queued successfully
	default:
		// Queue full - mark as failed
		job.Status = domain.LinkCheckFailed
		job.Result = &domain.LinkCheckResult{
			Checked:      0,
			Accessible:   0,
			Inaccessible: []domain.LinkError{{Reason: "queue_full"}},
		}
	}

	return jobID
}

// GetJob retrieves a job by ID
func (p *LinkCheckWorkerPool) GetJob(jobID string) (*domain.LinkCheckJob, bool) {
	val, ok := p.results.Load(jobID)
	if !ok {
		return nil, false
	}
	return val.(*domain.LinkCheckJob), true
}

// WaitForJob blocks until a job completes or times out
func (p *LinkCheckWorkerPool) WaitForJob(jobID string, timeout time.Duration) (*domain.LinkCheckJob, error) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		job, ok := p.GetJob(jobID)
		if !ok {
			return nil, fmt.Errorf("job not found: %s", jobID)
		}

		if job.IsComplete() {
			return job, nil
		}

		time.Sleep(100 * time.Millisecond)
	}

	return nil, fmt.Errorf("timeout waiting for job: %s", jobID)
}

// cleanup periodically removes old jobs
func (p *LinkCheckWorkerPool) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-p.stopCleanup:
			return
		case <-ticker.C:
			p.gcOldJobs()
		}
	}
}

// gcOldJobs removes jobs older than maxAge
func (p *LinkCheckWorkerPool) gcOldJobs() {
	cutoff := time.Now().Add(-p.maxAge)
	p.results.Range(func(key, value interface{}) bool {
		job := value.(*domain.LinkCheckJob)
		if job.CreatedAt.Before(cutoff) {
			p.results.Delete(key)
		}
		return true
	})
}

// generateJobID creates a random job ID
func generateJobID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
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
