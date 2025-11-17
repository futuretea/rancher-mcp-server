package core

import (
	"github.com/futuretea/rancher-mcp-server/pkg/api"
	"github.com/futuretea/rancher-mcp-server/pkg/toolsets/common"
	"github.com/mark3labs/mcp-go/mcp"
)

// Toolset implements the core Kubernetes toolset
type Toolset struct{}

var _ api.Toolset = (*Toolset)(nil)

// GetName returns the name of the toolset
func (t *Toolset) GetName() string {
	return "core"
}

// GetDescription returns the description of the toolset
func (t *Toolset) GetDescription() string {
	return "Core Kubernetes operations for managing clusters, nodes, pods, and other resources"
}

// GetTools returns the tools provided by this toolset
func (t *Toolset) GetTools(client interface{}) []api.ServerTool {
	return []api.ServerTool{
		{
			Tool: mcp.Tool{
				Name:        "node_get",
				Description: "Get a single node by ID, more efficient than list",
				InputSchema: mcp.ToolInputSchema{
					Type:     "object",
					Required: []string{"cluster", "node"},
					Properties: map[string]any{
						"cluster": map[string]any{
							"type":        "string",
							"description": "Cluster ID",
						},
						"node": map[string]any{
							"type":        "string",
							"description": "Node ID to get",
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
			Handler: nodeGetHandler,
		},
		{
			Tool: mcp.Tool{
				Name:        "node_list",
				Description: "List nodes in specified cluster or all clusters",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]any{
						"cluster": map[string]any{
							"type":        "string",
							"description": "Cluster ID to list nodes from (optional)",
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
			Handler: nodeListHandler,
		},
		{
			Tool: mcp.Tool{
				Name:        "workload_get",
				Description: "Get a single workload by name and namespace, more efficient than list",
				InputSchema: mcp.ToolInputSchema{
					Type:     "object",
					Required: []string{"cluster", "namespace", "name"},
					Properties: map[string]any{
						"cluster": map[string]any{
							"type":        "string",
							"description": "Cluster ID",
						},
						"project": map[string]any{
							"type":        "string",
							"description": "Project ID (optional, will auto-detect if not provided)",
							"default":     "",
						},
						"namespace": map[string]any{
							"type":        "string",
							"description": "Namespace name",
						},
						"name": map[string]any{
							"type":        "string",
							"description": "Workload name to get",
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
			Handler: workloadGetHandler,
		},
		{
			Tool: mcp.Tool{
				Name:        "workload_list",
				Description: "List workloads (deployments, statefulsets, daemonsets, jobs) and orphan pods in a cluster",
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
							"description": "Project ID to filter workloads (optional)",
							"default":     "",
						},
						"namespace": map[string]any{
							"type":        "string",
							"description": "Namespace name to filter workloads (optional)",
							"default":     "",
						},
						"node": map[string]any{
							"type":        "string",
							"description": "Node name to filter workloads (optional)",
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
			Handler: workloadListHandler,
		},
		{
			Tool: mcp.Tool{
				Name:        "namespace_get",
				Description: "Get a single namespace by name, more efficient than list",
				InputSchema: mcp.ToolInputSchema{
					Type:     "object",
					Required: []string{"cluster", "name"},
					Properties: map[string]any{
						"cluster": map[string]any{
							"type":        "string",
							"description": "Cluster ID",
						},
						"name": map[string]any{
							"type":        "string",
							"description": "Namespace name to get",
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
			Handler: namespaceGetHandler,
		},
		{
			Tool: mcp.Tool{
				Name:        "namespace_list",
				Description: "List all namespaces in a cluster",
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
							"description": "Project ID to filter namespaces (optional)",
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
			Handler: namespaceListHandler,
		},
		{
			Tool: mcp.Tool{
				Name:        "configmap_list",
				Description: "List all configmaps in a cluster",
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
							"description": "Project ID to filter configmaps (optional)",
							"default":     "",
						},
						"namespace": map[string]any{
							"type":        "string",
							"description": "Namespace name to filter configmaps (optional)",
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
			Handler: configMapListHandler,
		},
		{
			Tool: mcp.Tool{
				Name:        "secret_list",
				Description: "List all secrets in a cluster (metadata only, does not expose secret data)",
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
							"description": "Project ID to filter secrets (optional)",
							"default":     "",
						},
						"namespace": map[string]any{
							"type":        "string",
							"description": "Namespace name to filter secrets (optional)",
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
			Handler: secretListHandler,
		},
		{
			Tool: mcp.Tool{
				Name:        "service_get",
				Description: "Get a single service by name with optional pod diagnostic check (Service → Pods), more efficient than list",
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
							"description": "Service name to get",
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
			Handler: serviceGetHandler,
		},
		{
			Tool: mcp.Tool{
				Name:        "service_list",
				Description: "List services with optional pod diagnostic check (Service → Pods)",
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
							"description": "Project ID to filter services (optional)",
							"default":     "",
						},
						"namespace": map[string]any{
							"type":        "string",
							"description": "Namespace name to filter services (optional)",
							"default":     "",
						},
						"getPodDetails": map[string]any{
							"type":        "boolean",
							"description": "Get pod information and perform health checks for services",
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
			Handler: serviceListHandler,
		},
	}
}
