// Package kubernetes provides the Kubernetes toolset using Steve API.
package kubernetes

import (
	"github.com/futuretea/rancher-mcp-server/pkg/toolset"
)

// Toolset implements the Kubernetes toolset using Steve API
type Toolset struct {
	// ReadOnly disables create, patch, delete operations
	ReadOnly bool
	// DisableDestructive disables delete operations only
	DisableDestructive bool
}

var _ toolset.Toolset = (*Toolset)(nil)

// showSensitiveDataProperty is the shared schema for the showSensitiveData parameter
// used across multiple tool definitions.
var showSensitiveDataProperty = map[string]any{
	"type":        "boolean",
	"description": "Show sensitive data values (e.g., Secret data). Default is false, which masks values with '***'",
	"default":     false,
}

var apiVersionProperty = map[string]any{
	"type":        "string",
	"description": "Kubernetes API version for CRDs or ambiguous kinds, e.g. catalog.cattle.io/v1. Optional for built-in resources.",
	"default":     "",
}

var clusterIDProperty = map[string]any{
	"type":        "string",
	"description": "Cluster ID (use cluster_list tool to get available cluster IDs)",
}

// GetName returns the name of the toolset
func (t *Toolset) GetName() string {
	return "kubernetes"
}

// GetDescription returns the description of the toolset
func (t *Toolset) GetDescription() string {
	return "Generic Kubernetes operations via Steve API - supports any resource type without project requirement"
}

// GetTools returns the tools provided by this toolset
func (t *Toolset) GetTools(_ interface{}) []toolset.ServerTool {
	tools := []toolset.ServerTool{}

	tools = append(tools, resourceTools()...)
	tools = append(tools, analysisTools()...)
	tools = append(tools, aggregateTools()...)
	tools = append(tools, fileTools()...)

	// Add write operations (see toolset_write.go)
	tools = t.appendWriteTools(tools)

	return tools
}
