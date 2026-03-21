package cache

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net"
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

	// Remove default ports (use proper host:port parsing)
	host, port, err := net.SplitHostPort(parsed.Host)
	if err == nil {
		// Port was specified - check if it's default
		if (parsed.Scheme == "http" && port == "80") ||
			(parsed.Scheme == "https" && port == "443") {
			// Restore IPv6 brackets if needed
			if strings.Contains(host, ":") {
				parsed.Host = "[" + host + "]"
			} else {
				parsed.Host = host
			}
		}
	}
	// If no port was specified, err != nil and we keep Host as-is

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

// GenerateCachedLinkKey generates a cache key for an individual cached link check
func GenerateCachedLinkKey(url string) string {
	// Use hash of URL to keep key size reasonable
	h := sha256.Sum256([]byte(url))
	return "link:" + base64.RawURLEncoding.EncodeToString(h[:])
}
