package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()

	assert.Equal(t, LinkCheckAsync, opts.CheckLinks)
	assert.Equal(t, 10000, opts.MaxLinks)
	assert.Equal(t, 10, opts.SyncCheckLimit)
	assert.Equal(t, 30*time.Second, opts.Timeout)
	assert.True(t, opts.AllowStale)
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
			assert.Equal(t, tt.want, got)
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
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNewAnalysisResult(t *testing.T) {
	url := "https://example.com"
	result := NewAnalysisResult(url)

	assert.Equal(t, "v1", result.Version)
	assert.Equal(t, url, result.URL)
	assert.False(t, result.AnalyzedAt.IsZero())
	assert.NotNil(t, result.Extra)

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
		assert.False(t, seen[mode], "duplicate LinkCheckMode: %s", mode)
		seen[mode] = true
	}

	// Ensure they have string values
	assert.Equal(t, "sync", string(LinkCheckSync))
	assert.Equal(t, "async", string(LinkCheckAsync))
	assert.Equal(t, "hybrid", string(LinkCheckHybrid))
	assert.Equal(t, "disabled", string(LinkCheckDisabled))
}
