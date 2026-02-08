package rancher

import (
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/futuretea/rancher-mcp-server/pkg/toolset"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset/handler"
)

// Toolset implements the Rancher-specific toolset
type Toolset struct{}

var _ toolset.Toolset = (*Toolset)(nil)

// GetName returns the name of the toolset
func (t *Toolset) GetName() string {
	return "rancher"
}

// GetDescription returns the description of the toolset
func (t *Toolset) GetDescription() string {
	return "Rancher-specific operations for managing clusters and projects"
}

// GetTools returns the tools provided by this toolset
func (t *Toolset) GetTools(client interface{}) []toolset.ServerTool {
	return []toolset.ServerTool{
		{
			Tool: mcp.Tool{
				Name:        "cluster_list",
				Description: "List all available Rancher clusters",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]any{
						"name": map[string]any{
							"type":        "string",
							"description": "Filter by cluster name (partial match)",
							"default":     "",
						},
						"limit": map[string]any{
							"type":        "integer",
							"description": "Number of items per page",
							"default":     100,
						},
						"page": map[string]any{
							"type":        "integer",
							"description": "Page number (starting from 1)",
							"default":     1,
						},
						"format": map[string]any{
							"type":        "string",
							"description": "Output format: json, table, or yaml",
							"enum":        []string{"json", "table", "yaml"},
							"default":     "json",
						},
					},
				},
			},
			Annotations: toolset.ToolAnnotations{
				ReadOnlyHint: handler.BoolPtr(true),
			},
			Handler: clusterListHandler,
		},
		{
			Tool: mcp.Tool{
				Name:        "project_list",
				Description: "List all Rancher projects across clusters",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]any{
						"cluster": map[string]any{
							"type":        "string",
							"description": "Filter by cluster ID (use cluster_list to get available cluster IDs)",
							"default":     "",
						},
						"name": map[string]any{
							"type":        "string",
							"description": "Filter by project name (partial match)",
							"default":     "",
						},
						"limit": map[string]any{
							"type":        "integer",
							"description": "Number of items per page",
							"default":     100,
						},
						"page": map[string]any{
							"type":        "integer",
							"description": "Page number (starting from 1)",
							"default":     1,
						},
						"format": map[string]any{
							"type":        "string",
							"description": "Output format: json, table, or yaml",
							"enum":        []string{"json", "table", "yaml"},
							"default":     "json",
						},
					},
				},
			},
			Annotations: toolset.ToolAnnotations{
				ReadOnlyHint: handler.BoolPtr(true),
			},
			Handler: projectListHandler,
		},
	}
}
