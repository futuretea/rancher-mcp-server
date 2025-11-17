// Package networking provides networking-related tools for Kubernetes resources.
// It implements MCP tools for managing:
//   - Ingresses (get, list with diagnostic chain: Ingress → Service → Pods)
//
// The diagnostic feature performs health checks across the entire routing chain
// to identify connectivity issues. When enabled, it pre-fetches services and pods
// once per project for optimal performance.
//
// All tools support multiple output formats (JSON, YAML, table) and
// are marked as read-only operations.
package networking
