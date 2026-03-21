package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple http url",
			input:    "http://example.com",
			expected: "http://example.com/",
		},
		{
			name:     "https url",
			input:    "https://example.com",
			expected: "https://example.com/",
		},
		{
			name:     "uppercase scheme and host",
			input:    "HTTP://EXAMPLE.COM/path",
			expected: "http://example.com/path",
		},
		{
			name:     "remove default http port",
			input:    "http://example.com:80/path",
			expected: "http://example.com/path",
		},
		{
			name:     "remove default https port",
			input:    "https://example.com:443/path",
			expected: "https://example.com/path",
		},
		{
			name:     "keep non-default port",
			input:    "http://example.com:8080/path",
			expected: "http://example.com:8080/path",
		},
		{
			name:     "remove fragment",
			input:    "https://example.com/path#section",
			expected: "https://example.com/path",
		},
		{
			name:     "sort query params",
			input:    "https://example.com/path?z=1&a=2&m=3",
			expected: "https://example.com/path?a=2&m=3&z=1",
		},
		{
			name:     "multiple values same param",
			input:    "https://example.com/path?tag=b&tag=a",
			expected: "https://example.com/path?tag=a&tag=b",
		},
		{
			name:     "complex url",
			input:    "HTTPS://Example.COM:443/Path?z=1&a=2#frag",
			expected: "https://example.com/Path?a=2&z=1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := NormalizeURL(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNormalizeURL_InvalidURL(t *testing.T) {
	_, err := NormalizeURL("ht!tp://invalid url")
	assert.Error(t, err)
}

func TestGenerateHTMLKey(t *testing.T) {
	url1 := "https://example.com"
	url2 := "https://example.com/"
	url3 := "https://different.com"

	key1, err := GenerateHTMLKey(url1)
	require.NoError(t, err)

	key2, err := GenerateHTMLKey(url2)
	require.NoError(t, err)

	key3, err := GenerateHTMLKey(url3)
	require.NoError(t, err)

	// Same URL should generate same key
	assert.Equal(t, key1, key2)

	// Different URLs should generate different keys
	assert.NotEqual(t, key1, key3)

	// Keys should have prefix
	assert.Contains(t, key1, "html:")
}

func TestGenerateHTMLKey_Consistency(t *testing.T) {
	// Same URL with different query param order should generate same key
	url1 := "https://example.com?a=1&b=2"
	url2 := "https://example.com?b=2&a=1"

	key1, err := GenerateHTMLKey(url1)
	require.NoError(t, err)

	key2, err := GenerateHTMLKey(url2)
	require.NoError(t, err)

	assert.Equal(t, key1, key2)
}

func TestGenerateLinkCheckKey(t *testing.T) {
	jobID := "test-job-123"
	key := GenerateLinkCheckKey(jobID)

	assert.Equal(t, "links:test-job-123", key)
}

func TestSortQueryParams(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single param",
			input:    "a=1",
			expected: "a=1",
		},
		{
			name:     "multiple params",
			input:    "z=1&a=2&m=3",
			expected: "a=2&m=3&z=1",
		},
		{
			name:     "special characters",
			input:    "key=hello%20world",
			expected: "key=hello+world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the URL with query
			fullURL := "https://example.com?" + tt.input
			normalized, err := NormalizeURL(fullURL)
			require.NoError(t, err)
			assert.Contains(t, normalized, tt.expected)
		})
	}
}
