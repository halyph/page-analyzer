package analyzer

import (
	"time"

	"github.com/halyph/page-analyzer/internal/cache"
	"github.com/halyph/page-analyzer/internal/domain"
)

// LinkChecker defines the interface for link checking implementations
type LinkChecker interface {
	// Submit submits a batch of URLs for checking
	Submit(urls []string, baseURL string) string

	// GetJob retrieves a job by ID
	GetJob(jobID string) (*domain.LinkCheckJob, bool)

	// WaitForJob waits for a job to complete with timeout
	WaitForJob(jobID string, timeout time.Duration) (*domain.LinkCheckJob, error)

	// Start starts the link checker (if not already started)
	Start()

	// Stop stops the link checker
	Stop()
}

// WorkerPoolConfig configures the link checker worker pool
type WorkerPoolConfig struct {
	Workers      int           // Number of concurrent workers
	QueueSize    int           // Job queue buffer size
	Timeout      time.Duration // HTTP request timeout
	JobMaxAge    time.Duration // How long to keep completed jobs
	JobWorkers   int           // Concurrent checks within a single job
	Cache        cache.Cache   // Optional cache for individual link results
	LinkCacheTTL time.Duration // TTL for caching individual link check results
}
