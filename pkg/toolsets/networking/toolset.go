package networking

import (
	"github.com/futuretea/rancher-mcp-server/pkg/api"
	"github.com/futuretea/rancher-mcp-server/pkg/toolsets/common"
	"github.com/mark3labs/mcp-go/mcp"
)

// Toolset implements the networking toolset
type Toolset struct{}

var _ api.Toolset = (*Toolset)(nil)

// GetName returns the name of the toolset
func (t *Toolset) GetName() string {
	return "networking"
}

// GetDescription returns the description of the toolset
func (t *Toolset) GetDescription() string {
	return "Networking operations for managing ingresses and network policies"
}

// GetTools returns the tools provided by this toolset
func (t *Toolset) GetTools(client interface{}) []api.ServerTool {
	return []api.ServerTool{
		{
			Tool: mcp.Tool{
				Name:        "ingress_get",
				Description: "Get a single ingress by name with diagnostic chain check (Ingress → Service → Pods), more efficient than list",
				InputSchema: mcp.ToolInputSchema{
					Type:     "object",
					Required: []string{"cluster", "namespace", "name"},
					Properties: map[string]any{
						"cluster": map[string]any{
							"type":        "string",
							"description": "Cluster ID",
						},
						"namespace": map[string]any{
							"type":        "string",
							"description": "Namespace name",
						},
						"name": map[string]any{
							"type":        "string",
							"description": "Ingress name to get",
						},
						"project": map[string]any{
							"type":        "string",
							"description": "Project ID (optional, will auto-detect if not provided)",
							"default":     "",
						},
						"getPodDetails": map[string]any{
							"type":        "boolean",
							"description": "Get detailed pod information and perform health checks",
							"default":     false,
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
			Handler: ingressGetHandler,
		},
		{
			Tool: mcp.Tool{
				Name:        "ingress_list",
				Description: "List ingresses with full diagnostic chain check (Ingress → Service → Pods)",
				InputSchema: mcp.ToolInputSchema{
					Type:     "object",
					Required: []string{"cluster"},
					Properties: map[string]any{
						"cluster": map[string]any{
							"type":        "string",
							"description": "Cluster ID",
						},
						"project": map[string]any{
							"type":        "string",
							"description": "Project ID to filter ingresses (optional)",
							"default":     "",
						},
						"namespace": map[string]any{
							"type":        "string",
							"description": "Namespace name to filter ingresses (optional)",
							"default":     "",
						},
						"getPodDetails": map[string]any{
							"type":        "boolean",
							"description": "Get detailed pod information and perform health checks",
							"default":     false,
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
			Handler: ingressListHandler,
		},
	}
}
