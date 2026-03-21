package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"sort"
	"strings"
)

// NormalizeURL normalizes a URL for cache key generation
// - Lowercases scheme and host
// - Removes default ports (80 for http, 443 for https)
// - Sorts query parameters
// - Removes fragment
func NormalizeURL(rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	// Lowercase scheme and host
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)

	// Normalize path - remove trailing slash except for root
	if parsed.Path != "/" && strings.HasSuffix(parsed.Path, "/") {
		parsed.Path = strings.TrimSuffix(parsed.Path, "/")
	}
	// Ensure root path has slash
	if parsed.Path == "" {
		parsed.Path = "/"
	}

	// Remove default ports
	if parsed.Scheme == "http" && strings.HasSuffix(parsed.Host, ":80") {
		parsed.Host = strings.TrimSuffix(parsed.Host, ":80")
	}
	if parsed.Scheme == "https" && strings.HasSuffix(parsed.Host, ":443") {
		parsed.Host = strings.TrimSuffix(parsed.Host, ":443")
	}

	// Sort query parameters for consistency
	if parsed.RawQuery != "" {
		query := parsed.Query()
		parsed.RawQuery = sortQueryParams(query)
	}

	// Remove fragment
	parsed.Fragment = ""

	return parsed.String(), nil
}

// sortQueryParams sorts query parameters alphabetically
func sortQueryParams(query url.Values) string {
	keys := make([]string, 0, len(query))
	for k := range query {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		values := query[k]
		sort.Strings(values)
		for _, v := range values {
			parts = append(parts, url.QueryEscape(k)+"="+url.QueryEscape(v))
		}
	}

	return strings.Join(parts, "&")
}

// GenerateKey generates a cache key from a normalized URL
func GenerateKey(prefix string, rawURL string) (string, error) {
	normalized, err := NormalizeURL(rawURL)
	if err != nil {
		return "", err
	}

	// Generate SHA256 hash
	hash := sha256.Sum256([]byte(normalized))
	hashStr := hex.EncodeToString(hash[:])

	return prefix + ":" + hashStr, nil
}

// GenerateHTMLKey generates a cache key for HTML analysis results
func GenerateHTMLKey(rawURL string) (string, error) {
	return GenerateKey("html", rawURL)
}

// GenerateLinkCheckKey generates a cache key for link check results
func GenerateLinkCheckKey(jobID string) string {
	return "links:" + jobID
}
