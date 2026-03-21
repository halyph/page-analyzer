package domain

import (
	"testing"
	"time"
)

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()

	if opts.CheckLinks != LinkCheckAsync {
		t.Errorf("expected CheckLinks to be %s, got %s", LinkCheckAsync, opts.CheckLinks)
	}

	if opts.MaxLinks != 10000 {
		t.Errorf("expected MaxLinks to be 10000, got %d", opts.MaxLinks)
	}

	if opts.SyncCheckLimit != 10 {
		t.Errorf("expected SyncCheckLimit to be 10, got %d", opts.SyncCheckLimit)
	}

	if opts.Timeout != 30*time.Second {
		t.Errorf("expected Timeout to be 30s, got %v", opts.Timeout)
	}

	if !opts.AllowStale {
		t.Error("expected AllowStale to be true")
	}
}

func TestHeadingCounts_Total(t *testing.T) {
	tests := []struct {
		name     string
		headings HeadingCounts
		want     int
	}{
		{
			name:     "empty",
			headings: HeadingCounts{},
			want:     0,
		},
		{
			name:     "single heading",
			headings: HeadingCounts{H1: 1},
			want:     1,
		},
		{
			name:     "multiple headings",
			headings: HeadingCounts{H1: 2, H2: 3, H3: 1},
			want:     6,
		},
		{
			name:     "all levels",
			headings: HeadingCounts{H1: 1, H2: 2, H3: 3, H4: 4, H5: 5, H6: 6},
			want:     21,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.headings.Total()
			if got != tt.want {
				t.Errorf("Total() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestHeadingCounts_IsEmpty(t *testing.T) {
	tests := []struct {
		name     string
		headings HeadingCounts
		want     bool
	}{
		{
			name:     "empty",
			headings: HeadingCounts{},
			want:     true,
		},
		{
			name:     "has h1",
			headings: HeadingCounts{H1: 1},
			want:     false,
		},
		{
			name:     "has multiple",
			headings: HeadingCounts{H2: 5, H3: 2},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.headings.IsEmpty()
			if got != tt.want {
				t.Errorf("IsEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewAnalysisResult(t *testing.T) {
	url := "https://example.com"
	result := NewAnalysisResult(url)

	if result.Version != "v1" {
		t.Errorf("expected Version to be v1, got %s", result.Version)
	}

	if result.URL != url {
		t.Errorf("expected URL to be %s, got %s", url, result.URL)
	}

	if result.AnalyzedAt.IsZero() {
		t.Error("expected AnalyzedAt to be set")
	}

	if result.Extra == nil {
		t.Error("expected Extra map to be initialized")
	}

	// Should be able to add to Extra without panic
	result.Extra["test"] = []byte(`"value"`)
}

func TestLinkCheckModeConstants(t *testing.T) {
	modes := []LinkCheckMode{
		LinkCheckSync,
		LinkCheckAsync,
		LinkCheckHybrid,
		LinkCheckDisabled,
	}

	// Ensure all modes are unique
	seen := make(map[LinkCheckMode]bool)
	for _, mode := range modes {
		if seen[mode] {
			t.Errorf("duplicate LinkCheckMode: %s", mode)
		}
		seen[mode] = true
	}

	// Ensure they have string values
	if string(LinkCheckSync) != "sync" {
		t.Errorf("expected LinkCheckSync to be 'sync', got %s", LinkCheckSync)
	}
	if string(LinkCheckAsync) != "async" {
		t.Errorf("expected LinkCheckAsync to be 'async', got %s", LinkCheckAsync)
	}
	if string(LinkCheckHybrid) != "hybrid" {
		t.Errorf("expected LinkCheckHybrid to be 'hybrid', got %s", LinkCheckHybrid)
	}
	if string(LinkCheckDisabled) != "disabled" {
		t.Errorf("expected LinkCheckDisabled to be 'disabled', got %s", LinkCheckDisabled)
	}
}
