// Package core provides the core Kubernetes toolset for rancher-mcp-server.
// It implements MCP tools for managing fundamental Kubernetes resources including:
//   - Nodes (get, list)
//   - Workloads (get, list - deployments, statefulsets, daemonsets, jobs)
//   - Namespaces (get, list)
//   - ConfigMaps (list)
//   - Secrets (list - metadata only)
//   - Services (get, list with optional pod diagnostics)
//
// All tools support multiple output formats (JSON, YAML, table) and
// are marked as read-only operations.
package core
