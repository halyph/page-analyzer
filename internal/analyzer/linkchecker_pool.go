package analyzer

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"sync"
	"time"

	"github.com/halyph/page-analyzer/internal/cache"
	"github.com/halyph/page-analyzer/internal/domain"
)

// LinkCheckWorkerPool manages concurrent link checking with bounded resources
type LinkCheckWorkerPool struct {
	workers      int
	jobs         chan *domain.LinkCheckJob
	results      sync.Map // JobID → *LinkCheckJob
	client       *http.Client
	timeout      time.Duration
	maxAge       time.Duration
	userAgent    string
	cache        cache.Cache   // Optional cache for individual link results
	linkCacheTTL time.Duration // TTL for cached link results
	stopCleanup  chan struct{}
	ctx          context.Context
	cancel       context.CancelFunc
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
	if cfg.LinkCacheTTL <= 0 {
		cfg.LinkCacheTTL = 5 * time.Minute // Default 5 minutes for link results
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Enable cookie handling for session-based sites
	jar, _ := cookiejar.New(nil)

	// Use no-op cache if none provided
	linkCache := cfg.Cache
	if linkCache == nil {
		linkCache = cache.NewNoOpCache()
	}

	return &LinkCheckWorkerPool{
		workers:      cfg.Workers,
		jobs:         make(chan *domain.LinkCheckJob, cfg.QueueSize),
		timeout:      cfg.Timeout,
		maxAge:       cfg.JobMaxAge,
		userAgent:    cfg.UserAgent,
		cache:        linkCache,
		linkCacheTTL: cfg.LinkCacheTTL,
		stopCleanup:  make(chan struct{}),
		ctx:          ctx,
		cancel:       cancel,
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

			// Check cache first
			if cached, err := p.cache.GetCachedLink(p.ctx, u); err == nil {
				// Cache hit - use cached result
				if cached.Accessible {
					mu.Lock()
					accessible++
					mu.Unlock()
				} else {
					mu.Lock()
					inaccessible = append(inaccessible, domain.LinkError{
						URL:        u,
						StatusCode: cached.StatusCode,
						Reason:     cached.Reason,
					})
					mu.Unlock()
				}
				return
			}

			// Cache miss - perform actual check
			err := p.checkLink(p.ctx, u, job.BaseURL)

			// Store result in cache
			cachedResult := &domain.CachedLinkCheck{
				URL:        u,
				Accessible: err == nil,
				CheckedAt:  time.Now().Unix(),
			}
			if err != nil {
				cachedResult.StatusCode = extractStatusCode(err)
				cachedResult.Reason = extractReason(err)
			}
			_ = p.cache.SetCachedLink(p.ctx, u, cachedResult, p.linkCacheTTL)

			// Update job results
			if err != nil {
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
