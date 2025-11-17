// Package rancher provides Rancher-specific toolset for multi-cluster management.
// It implements MCP tools for managing Rancher resources including:
//   - Clusters (list, health checks)
//   - Projects (get, list, access permissions)
//   - Users (get, list)
//
// This toolset enables AI agents to interact with Rancher's multi-cluster
// management features and access control systems.
//
// All tools support multiple output formats (JSON, YAML, table) and
// are marked as read-only operations.
package rancher
