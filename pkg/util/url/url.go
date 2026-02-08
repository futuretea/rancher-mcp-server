// Package url provides URL normalization utilities for Rancher API endpoints.
package url

import "strings"

// NormalizeRancherURL handles both URL formats:
// - "https://rancher.example.com/v3" -> strips /v3
// - "https://rancher.example.com" -> uses as-is
func NormalizeRancherURL(url string) string {
	return strings.TrimSuffix(url, "/v3")
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
