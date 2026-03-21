package analyzer

import (
	"context"

	"github.com/oivasiv/page-analyzer/internal/analyzer/collectors"
	"github.com/oivasiv/page-analyzer/internal/domain"
)

// Service is the main analyzer that orchestrates fetching, parsing, and collecting
type Service struct {
	fetcher *Fetcher
	walker  *Walker
}

// ServiceConfig configures the analyzer service
type ServiceConfig struct {
	Fetcher FetcherConfig
	Walker  WalkerConfig
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
	return &Service{
		fetcher: NewFetcher(config.Fetcher),
		walker:  NewWalker(config.Walker),
	}
}

// Analyze performs a complete analysis of a webpage
func (s *Service) Analyze(ctx context.Context, req domain.AnalysisRequest) (*domain.AnalysisResult, error) {
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
