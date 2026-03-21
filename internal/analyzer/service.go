package analyzer

import (
	"context"
	"time"

	"github.com/halyph/page-analyzer/internal/analyzer/collectors"
	"github.com/halyph/page-analyzer/internal/cache"
	"github.com/halyph/page-analyzer/internal/domain"
)

// Service is the main analyzer that orchestrates fetching, parsing, and collecting
type Service struct {
	fetcher     *Fetcher
	walker      *Walker
	linkChecker *LinkCheckWorkerPool // Optional link checker
	cache       cache.Cache          // Optional cache
}

// ServiceConfig configures the analyzer service
type ServiceConfig struct {
	Fetcher         FetcherConfig
	Walker          WalkerConfig
	LinkChecker     *LinkCheckConfig     // Optional: config to create new link checker
	LinkCheckerPool *LinkCheckWorkerPool // Optional: use existing link checker
	Cache           cache.Cache          // Optional: nil means no caching
	CacheTTL        time.Duration        // Cache TTL (default: 1 hour)
}

// DefaultServiceConfig returns sensible defaults
func DefaultServiceConfig() ServiceConfig {
	return ServiceConfig{
		Fetcher: DefaultFetcherConfig(),
		Walker:  DefaultWalkerConfig(),
	}
}

// NewService creates a new analyzer service
func NewService(config ServiceConfig) *Service {
	s := &Service{
		fetcher: NewFetcher(config.Fetcher),
		walker:  NewWalker(config.Walker),
	}

	// Optional: use existing link checker pool or create new one
	if config.LinkCheckerPool != nil {
		s.linkChecker = config.LinkCheckerPool
	} else if config.LinkChecker != nil {
		s.linkChecker = NewLinkCheckWorkerPool(*config.LinkChecker)
		s.linkChecker.Start()
	}

	// Optional: use provided cache or default to no-op
	if config.Cache != nil {
		s.cache = config.Cache
	} else {
		s.cache = cache.NewNoOpCache()
	}

	return s
}

// Stop gracefully shuts down the service
func (s *Service) Stop() {
	if s.linkChecker != nil {
		s.linkChecker.Stop()
	}
}

// Analyze performs a complete analysis of a webpage
func (s *Service) Analyze(ctx context.Context, req domain.AnalysisRequest) (*domain.AnalysisResult, error) {
	var result *domain.AnalysisResult

	// Check cache first
	cached, err := s.cache.GetHTML(ctx, req.URL)
	if err == nil {
		// Cache hit - check if we need to do link checking
		if req.Options.CheckLinks == domain.LinkCheckDisabled {
			return cached, nil
		}
		// Use cached result and skip to link checking
		result = cached
	} else {
		// Cache miss - fetch and analyze

		// Initialize result
		result = domain.NewAnalysisResult(req.URL)

		// Fetch the webpage
		fetchResult, err := s.fetcher.Fetch(ctx, req.URL)
		if err != nil {
			return nil, err
		}

		// Update URL (in case of redirects)
		result.URL = fetchResult.URL

		// Build collectors based on configuration
		colls, err := s.buildCollectors(req)
		if err != nil {
			return nil, err
		}

		// Walk HTML and collect data
		if err := s.walker.Walk(fetchResult.Body, colls, result); err != nil {
			return nil, domain.ErrParsingFailed(req.URL, err)
		}

		// Store in cache (before link checking)
		cacheTTL := 1 * time.Hour
		_ = s.cache.SetHTML(ctx, result.URL, result, cacheTTL)
	}

	// Optional: check links
	if s.linkChecker != nil && req.Options.CheckLinks != domain.LinkCheckDisabled {
		// Combine internal and external links for checking
		allLinks := append([]string{}, result.Links.Internal...)
		allLinks = append(allLinks, result.Links.External...)

		if len(allLinks) > 0 {
			// Submit link check job
			jobID := s.linkChecker.Submit(allLinks, result.URL)
			result.Links.CheckJobID = jobID

			// For sync mode, wait for completion
			if req.Options.CheckLinks == domain.LinkCheckSync {
				// Use a reasonable timeout for link checking (30 seconds)
				timeout := 30 * time.Second
				if req.Options.Timeout > 0 {
					timeout = req.Options.Timeout
				}
				job, err := s.linkChecker.WaitForJob(jobID, timeout)
				if err != nil {
					// Don't fail the analysis, just mark check as failed
					result.Links.CheckStatus = domain.LinkCheckFailed
				} else {
					result.Links.CheckStatus = job.Status
					result.Links.CheckResult = job.Result
				}
			} else {
				// Async mode - mark as pending
				result.Links.CheckStatus = domain.LinkCheckPending
			}
		}
	}

	return result, nil
}

// buildCollectors creates the list of collectors to use for analysis
func (s *Service) buildCollectors(req domain.AnalysisRequest) ([]domain.Collector, error) {
	registry := collectors.DefaultRegistry

	// Core collectors (always enabled)
	coreCollectors := []string{"htmlversion", "title", "headings", "loginform", "links"}

	var colls []domain.Collector

	for _, name := range coreCollectors {
		config := domain.CollectorConfig{
			BaseURL:  req.URL,
			MaxItems: req.Options.MaxLinks,
		}

		collector, err := registry.Create(name, config)
		if err != nil {
			return nil, err
		}

		colls = append(colls, collector)
	}

	return colls, nil
}
