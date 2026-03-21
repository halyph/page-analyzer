package analyzer

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/halyph/page-analyzer/internal/config"
)

// loadFixture loads an HTML fixture from testdata directory
func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("testdata", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to load fixture %s: %v", name, err)
	}
	return data
}

// testFetchingConfig returns a default FetchingConfig for tests
func testFetchingConfig() config.FetchingConfig {
	return config.FetchingConfig{
		Timeout:     15 * time.Second,
		MaxBodySize: 10 * 1024 * 1024, // 10MB
		UserAgent:   "PageAnalyzer/1.0",
	}
}

// testProcessingConfig returns a default ProcessingConfig for tests
func testProcessingConfig() config.ProcessingConfig {
	return config.ProcessingConfig{
		MaxTokens: 1_000_000,
	}
}

// testServiceConfig returns a default ServiceConfig for tests
func testServiceConfig() ServiceConfig {
	return ServiceConfig{
		Fetcher:  testFetchingConfig(),
		Walker:   testProcessingConfig(),
		Cache:    nil, // Will use NoOpCache
		CacheTTL: time.Hour,
	}
}

// testLinkCheckConfig returns a default LinkCheckConfig for tests
func testLinkCheckConfig() LinkCheckConfig {
	return LinkCheckConfig{
		Workers:    20,
		QueueSize:  100,
		Timeout:    5 * time.Second,
		JobMaxAge:  10 * time.Minute,
		UserAgent:  "PageAnalyzer/1.0",
		JobWorkers: 10,
	}
}
