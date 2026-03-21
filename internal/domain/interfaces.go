package domain

import (
	"context"
	"time"

	"golang.org/x/net/html"
)

// Analyzer is the main service interface for webpage analysis
type Analyzer interface {
	// Analyze performs a complete analysis of a webpage
	Analyze(ctx context.Context, req AnalysisRequest) (*AnalysisResult, error)
}

// Collector processes HTML tokens in a single pass
// Each collector is responsible for one analysis concern
type Collector interface {
	// Collect processes a single HTML token
	// Called once per token during HTML parsing
	Collect(token html.Token)

	// Apply writes collected data into AnalysisResult
	// Called after all tokens have been processed
	Apply(result *AnalysisResult)
}

// CollectorFactory creates configured collectors
type CollectorFactory interface {
	// Create instantiates a new collector with the given configuration
	Create(config CollectorConfig) (Collector, error)

	// Metadata returns information about this collector
	Metadata() CollectorMetadata
}

// CollectorMetadata describes a collector's purpose and requirements
type CollectorMetadata struct {
	Name        string // Unique identifier (e.g., "title", "headings")
	Description string // Human-readable description
	Version     string // Collector version
	Required    bool   // Is this a core collector or optional?
}

// CollectorConfig provides configuration for collector creation
type CollectorConfig struct {
	BaseURL  string                 // Base URL for resolving relative links
	MaxItems int                    // Maximum items to collect (0 = unlimited)
	Params   map[string]interface{} // Collector-specific parameters
}

// LinkChecker verifies link accessibility
type LinkChecker interface {
	// Submit enqueues a link check job and returns a job ID
	// Non-blocking: returns immediately
	Submit(ctx context.Context, urls []string, baseURL string) string

	// GetJob retrieves the status and results of a link check job
	GetJob(jobID string) (*LinkCheckJob, bool)

	// CheckSync synchronously checks links (blocking)
	// Used for CLI mode or when immediate results are required
	CheckSync(ctx context.Context, urls []string) (*LinkCheckResult, error)

	// Start initializes the worker pool
	// Must be called before Submit
	Start(ctx context.Context)

	// Stop gracefully shuts down the worker pool
	Stop()
}

// Cache stores analysis results with TTL
type Cache interface {
	// GetHTML retrieves cached HTML analysis
	GetHTML(ctx context.Context, url string) (*AnalysisResult, error)

	// SetHTML stores HTML analysis (excludes link check results)
	SetHTML(ctx context.Context, url string, result *AnalysisResult, ttl time.Duration) error

	// GetLinkCheck retrieves cached link check results
	GetLinkCheck(ctx context.Context, url string) (*LinkCheckResult, error)

	// SetLinkCheck stores link check results (shared across users)
	SetLinkCheck(ctx context.Context, url string, result *LinkCheckResult, ttl time.Duration) error

	// Delete removes all cached data for a URL
	Delete(ctx context.Context, url string) error

	// Health checks cache connectivity and availability
	Health(ctx context.Context) error

	// Close releases cache resources
	Close() error
}

// Fetcher retrieves webpage content via HTTP
type Fetcher interface {
	// Fetch retrieves the content of a URL
	// Returns an error if the URL is unreachable or returns non-2xx status
	Fetch(ctx context.Context, url string) (*FetchResult, error)
}

// FetchResult represents the result of fetching a URL
type FetchResult struct {
	URL         string            // Final URL (after redirects)
	StatusCode  int               // HTTP status code
	ContentType string            // Content-Type header
	Body        []byte            // Response body
	Headers     map[string]string // Response headers
}

// Walker streams HTML tokens to collectors
type Walker interface {
	// Walk parses HTML and feeds tokens to all collectors
	// Calls Collector.Collect() for each token
	// Calls Collector.Apply() after all tokens processed
	Walk(body []byte, collectors []Collector, result *AnalysisResult) error
}
