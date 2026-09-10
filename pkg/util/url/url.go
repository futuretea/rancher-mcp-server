// Package url provides URL normalization utilities for Rancher API endpoints.
package url

import (
	"net/url"
	"strings"
)

// NormalizeRancherURL handles both URL formats:
// - "https://rancher.example.com/v3" -> strips /v3
// - "https://rancher.example.com" -> uses as-is
func NormalizeRancherURL(rawURL string) string {
	return strings.TrimSuffix(rawURL, "/v3")
}

// GetNormanURL returns URL with /v3 for Norman API
func GetNormanURL(baseURL string) string {
	normalized := NormalizeRancherURL(baseURL)
	return normalized + "/v3"
}

// GetSteveURL returns URL for Steve API cluster access
func GetSteveURL(baseURL string, clusterID string) string {
	normalized := NormalizeRancherURL(baseURL)
	return normalized + "/k8s/clusters/" + clusterID
}

// RedactCredentials removes credentials from the userinfo segment of a URL,
// keeping scheme, host and path so the value stays diagnosable. A value that
// does not parse as a URL is masked conservatively up to its last "@", because
// an invalid URL cannot be split into authority and path.
func RedactCredentials(rawURL string) string {
	parsed, parseErr := url.Parse(rawURL)
	if parseErr == nil && parsed.User == nil {
		return rawURL
	}

	const separator = "://"
	schemeEnd := strings.Index(rawURL, separator)
	if schemeEnd < 0 {
		return rawURL
	}
	prefixEnd := schemeEnd + len(separator)
	rest := rawURL[prefixEnd:]

	authorityEnd := len(rest)
	if i := strings.IndexAny(rest, "/?#"); i >= 0 {
		authorityEnd = i
	}

	// The userinfo ends at the last "@" of the authority, matching net/url.
	searchEnd := authorityEnd
	if parseErr != nil {
		searchEnd = len(rest)
	}
	if at := strings.LastIndex(rest[:searchEnd], "@"); at >= 0 {
		return rawURL[:prefixEnd] + "***@" + rest[at+1:]
	}
	return rawURL
}
