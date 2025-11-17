package rancher

import (
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/futuretea/rancher-mcp-server/pkg/api"
	"github.com/futuretea/rancher-mcp-server/pkg/toolsets/common"
)

// Toolset implements the Rancher-specific toolset
type Toolset struct{}

var _ api.Toolset = (*Toolset)(nil)

// GetName returns the name of the toolset
func (t *Toolset) GetName() string {
	return "rancher"
}

// GetDescription returns the description of the toolset
func (t *Toolset) GetDescription() string {
	return "Rancher-specific operations for managing projects, users, and multi-cluster resources"
}

// GetTools returns the tools provided by this toolset
func (t *Toolset) GetTools(client interface{}) []api.ServerTool {
	return []api.ServerTool{
		{
			Tool: mcp.Tool{
				Name:        "cluster_list",
				Description: "List all available Kubernetes clusters",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]any{
						"format": map[string]any{
							"type":        "string",
							"description": "Output format: json, table, or yaml",
							"enum":        []string{"json", "table", "yaml"},
							"default":     "json",
						},
					},
				},
			},
			Annotations: api.ToolAnnotations{
				ReadOnlyHint: common.BoolPtr(true),
			},
			Handler: clusterListHandler,
		},
		{
			Tool: mcp.Tool{
				Name:        "project_get",
				Description: "Get a single Rancher project by ID, more efficient than list",
				InputSchema: mcp.ToolInputSchema{
					Type:     "object",
					Required: []string{"project"},
					Properties: map[string]any{
						"project": map[string]any{
							"type":        "string",
							"description": "Project ID to get",
						},
						"cluster": map[string]any{
							"type":        "string",
							"description": "Cluster ID (required for verification)",
						},
						"format": map[string]any{
							"type":        "string",
							"description": "Output format: json or yaml",
							"enum":        []string{"json", "yaml"},
							"default":     "json",
						},
					},
				},
			},
			Annotations: api.ToolAnnotations{
				ReadOnlyHint: common.BoolPtr(true),
			},
			Handler: projectGetHandler,
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
							"description": "Filter projects by cluster ID",
							"default":     "",
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
			Annotations: api.ToolAnnotations{
				ReadOnlyHint: common.BoolPtr(true),
			},
			Handler: projectListHandler,
		},
		{
			Tool: mcp.Tool{
				Name:        "user_get",
				Description: "Get a single Rancher user by ID, more efficient than list",
				InputSchema: mcp.ToolInputSchema{
					Type:     "object",
					Required: []string{"user"},
					Properties: map[string]any{
						"user": map[string]any{
							"type":        "string",
							"description": "User ID to get",
						},
						"format": map[string]any{
							"type":        "string",
							"description": "Output format: json or yaml",
							"enum":        []string{"json", "yaml"},
							"default":     "json",
						},
					},
				},
			},
			Annotations: api.ToolAnnotations{
				ReadOnlyHint: common.BoolPtr(true),
			},
			Handler: userGetHandler,
		},
		{
			Tool: mcp.Tool{
				Name:        "user_list",
				Description: "List all Rancher users",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]any{
						"format": map[string]any{
							"type":        "string",
							"description": "Output format: json, table, or yaml",
							"enum":        []string{"json", "table", "yaml"},
							"default":     "json",
						},
					},
				},
			},
			Annotations: api.ToolAnnotations{
				ReadOnlyHint: common.BoolPtr(true),
			},
			Handler: userListHandler,
		},
		{
			Tool: mcp.Tool{
				Name:        "cluster_health",
				Description: "Get health status of Rancher clusters",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]any{
						"cluster": map[string]any{
							"type":        "string",
							"description": "Specific cluster ID to check health",
							"default":     "",
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
			Annotations: api.ToolAnnotations{
				ReadOnlyHint: common.BoolPtr(true),
			},
			Handler: clusterHealthHandler,
		},
		{
			Tool: mcp.Tool{
				Name:        "project_access",
				Description: "List user access permissions for Rancher projects",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]any{
						"project": map[string]any{
							"type":        "string",
							"description": "Project ID to check access",
							"default":     "",
						},
						"cluster": map[string]any{
							"type":        "string",
							"description": "Cluster ID",
							"default":     "",
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
			Annotations: api.ToolAnnotations{
				ReadOnlyHint: common.BoolPtr(true),
			},
			Handler: projectAccessHandler,
		},
	}
}
