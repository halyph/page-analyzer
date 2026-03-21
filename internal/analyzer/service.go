package analyzer

import (
	"context"
	"log/slog"
	"time"

	"github.com/halyph/page-analyzer/internal/analyzer/collectors"
	"github.com/halyph/page-analyzer/internal/cache"
	"github.com/halyph/page-analyzer/internal/config"
	"github.com/halyph/page-analyzer/internal/domain"
)

// Service is the main analyzer that orchestrates fetching, parsing, and collecting
type Service struct {
	fetcher     *Fetcher
	walker      *Walker
	linkChecker LinkChecker   // Optional link checker
	cache       cache.Cache   // Optional cache
	cacheTTL    time.Duration // Cache TTL for HTML results
	logger      *slog.Logger  // Optional logger (nil = no logging)
	collectors  []string      // List of collectors to run
}

// ServiceConfig configures the analyzer service
type ServiceConfig struct {
	Fetcher         config.FetchingConfig
	Walker          config.ProcessingConfig
	LinkChecker     *LinkCheckConfig // Optional: config to create new link checker
	LinkCheckerPool LinkChecker      // Optional: use existing link checker
	Cache           cache.Cache      // Optional: nil means no caching
	CacheTTL        time.Duration    // Cache TTL (default: 1 hour)
	Logger          *slog.Logger     // Optional: logger (nil = no logging)
}

// NewService creates a new analyzer service
func NewService(cfg ServiceConfig) *Service {
	// Set default TTL if not specified
	cacheTTL := cfg.CacheTTL
	if cacheTTL == 0 {
		cacheTTL = 1 * time.Hour
	}

	// Set default collectors if not specified
	collectors := cfg.Walker.Collectors
	if len(collectors) == 0 {
		collectors = config.DefaultCollectors
	}

	s := &Service{
		fetcher:    NewFetcher(cfg.Fetcher),
		walker:     NewWalker(cfg.Walker),
		cacheTTL:   cacheTTL,
		logger:     cfg.Logger,
		collectors: collectors,
	}

	// Optional: use existing link checker pool or create new one
	if cfg.LinkCheckerPool != nil {
		s.linkChecker = cfg.LinkCheckerPool
	} else if cfg.LinkChecker != nil {
		s.linkChecker = NewLinkCheckWorkerPool(*cfg.LinkChecker)
		s.linkChecker.Start()
	}

	// Optional: use provided cache or default to no-op
	if cfg.Cache != nil {
		s.cache = cfg.Cache
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
	// Try cache first
	if result, found := s.tryCache(ctx, req.URL); found {
		// Cache hit with no link checking needed
		if req.Options.CheckLinks == domain.LinkCheckDisabled {
			return result, nil
		}
		// Use cached result but perform link check
		s.performLinkCheck(result, req)
		return result, nil
	}

	// Cache miss - fetch and analyze
	result, err := s.fetchAndAnalyze(ctx, req)
	if err != nil {
		return nil, err
	}

	// Optional: check links
	s.performLinkCheck(result, req)

	return result, nil
}

// tryCache attempts to retrieve a cached result
func (s *Service) tryCache(ctx context.Context, url string) (*domain.AnalysisResult, bool) {
	cached, err := s.cache.GetHTML(ctx, url)
	if err != nil {
		return nil, false
	}
	return cached, true
}

// fetchAndAnalyze fetches a webpage and performs HTML analysis
func (s *Service) fetchAndAnalyze(ctx context.Context, req domain.AnalysisRequest) (*domain.AnalysisResult, error) {
	// Initialize result
	result := domain.NewAnalysisResult(req.URL)

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
	if err := s.cache.SetHTML(ctx, result.URL, result, s.cacheTTL); err != nil && s.logger != nil {
		s.logger.Warn("failed to cache HTML result",
			"url", result.URL,
			"error", err)
	}

	return result, nil
}

// performLinkCheck performs link checking if configured
func (s *Service) performLinkCheck(result *domain.AnalysisResult, req domain.AnalysisRequest) {
	if s.linkChecker == nil || req.Options.CheckLinks == domain.LinkCheckDisabled {
		return
	}

	// Combine internal and external links for checking
	allLinks := append([]string{}, result.Links.Internal...)
	allLinks = append(allLinks, result.Links.External...)

	if len(allLinks) == 0 {
		return
	}

	// Submit link check job
	jobID := s.linkChecker.Submit(allLinks, result.URL)
	result.Links.CheckJobID = jobID

	// For sync mode, wait for completion
	if req.Options.CheckLinks == domain.LinkCheckSync {
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

// buildCollectors creates the list of collectors to use for analysis
func (s *Service) buildCollectors(req domain.AnalysisRequest) ([]domain.Collector, error) {
	registry := collectors.DefaultRegistry

	var colls []domain.Collector

	for _, name := range s.collectors {
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
