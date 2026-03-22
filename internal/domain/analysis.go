package domain

import (
	"encoding/json"
	"time"
)

// AnalysisRequest represents a request to analyze a webpage
type AnalysisRequest struct {
	URL     string
	Options AnalysisOptions
}

// AnalysisOptions configures how the analysis should be performed
type AnalysisOptions struct {
	CheckLinks     LinkCheckMode // "sync", "async", "hybrid", "disabled"
	MaxLinks       int           // Upper bound (default: 10000)
	SyncCheckLimit int           // For hybrid mode (default: 10)
	Timeout        time.Duration // Overall timeout (default: 30s)
	AllowStale     bool          // Allow stale cache under high load
}

// LinkCheckMode defines how link checking should be performed
type LinkCheckMode string

const (
	LinkCheckSync     LinkCheckMode = "sync"
	LinkCheckAsync    LinkCheckMode = "async"
	LinkCheckHybrid   LinkCheckMode = "hybrid"
	LinkCheckDisabled LinkCheckMode = "disabled"
)

// DefaultOptions returns sensible default analysis options
func DefaultOptions() AnalysisOptions {
	return AnalysisOptions{
		CheckLinks:     LinkCheckAsync,
		MaxLinks:       10000,
		SyncCheckLimit: 10,
		Timeout:        30 * time.Second,
		AllowStale:     true,
	}
}

// HeadingCounts tracks the count of each heading level in an HTML document
type HeadingCounts struct {
	H1 int `json:"h1"`
	H2 int `json:"h2"`
	H3 int `json:"h3"`
	H4 int `json:"h4"`
	H5 int `json:"h5"`
	H6 int `json:"h6"`
}

// Total returns the total number of headings across all levels
func (h HeadingCounts) Total() int {
	return h.H1 + h.H2 + h.H3 + h.H4 + h.H5 + h.H6
}

// IsEmpty returns true if there are no headings
func (h HeadingCounts) IsEmpty() bool {
	return h.Total() == 0
}

// AnalysisResult represents the complete analysis of a webpage
type AnalysisResult struct {
	Version      string        `json:"version"`        // API version: "v1"
	URL          string        `json:"url"`            // Analyzed URL
	HTMLVersion  string        `json:"html_version"`   // Detected HTML version
	Title        string        `json:"title"`          // Page title
	Headings     HeadingCounts `json:"headings"`       // Heading counts
	Links        LinkAnalysis  `json:"links"`          // Link analysis
	HasLoginForm bool          `json:"has_login_form"` // Login form detected
	AnalyzedAt   time.Time     `json:"analyzed_at"`    // Analysis timestamp
	CacheHit     bool          `json:"cache_hit"`      // Was result from cache
	Stale        bool          `json:"stale"`          // Is result stale (degraded mode)

	// Extension point for future collectors
	Extra map[string]json.RawMessage `json:"extra,omitempty"`
}

// NewAnalysisResult creates a new result with default values
func NewAnalysisResult(url string) *AnalysisResult {
	return &AnalysisResult{
		Version:    "v1",
		URL:        url,
		AnalyzedAt: time.Now(),
		Links:      LinkAnalysis{},
		Extra:      make(map[string]json.RawMessage),
	}
}
