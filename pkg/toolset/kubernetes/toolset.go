// Package kubernetes provides the Kubernetes toolset using Steve API.
package kubernetes

import (
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/futuretea/rancher-mcp-server/pkg/toolset"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset/handler"
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

// GetName returns the name of the toolset
func (t *Toolset) GetName() string {
	return "kubernetes"
}

// GetDescription returns the description of the toolset
func (t *Toolset) GetDescription() string {
	return "Generic Kubernetes operations via Steve API - supports any resource type without project requirement"
}

// GetTools returns the tools provided by this toolset
func (t *Toolset) GetTools(client interface{}) []toolset.ServerTool {
	tools := []toolset.ServerTool{
		// Read-only tools - always enabled
		{
			Tool: mcp.Tool{
				Name:        "kubernetes_get",
				Description: "Get any Kubernetes resource by kind, namespace, and name. Works with any resource type including CRDs.",
				InputSchema: mcp.ToolInputSchema{
					Type:     "object",
					Required: []string{"cluster", "kind", "name"},
					Properties: map[string]any{
						"cluster": map[string]any{
							"type":        "string",
							"description": "Cluster ID (use cluster_list tool to get available cluster IDs)",
						},
						"kind": map[string]any{
							"type":        "string",
							"description": "Resource kind (e.g., pod, deployment, service, configmap, secret, ingress, etc.)",
						},
						"namespace": map[string]any{
							"type":        "string",
							"description": "Namespace name (optional for cluster-scoped resources)",
							"default":     "",
						},
						"name": map[string]any{
							"type":        "string",
							"description": "Resource name",
						},
						"format": map[string]any{
							"type":        "string",
							"description": "Output format: json or yaml",
							"enum":        []string{"json", "yaml"},
							"default":     "json",
						},
						"showSensitiveData": showSensitiveDataProperty,
					},
				},
			},
			Annotations: toolset.ToolAnnotations{
				ReadOnlyHint: handler.BoolPtr(true),
			},
			Handler: getHandler,
		},
		{
			Tool: mcp.Tool{
				Name:        "kubernetes_list",
				Description: "List Kubernetes resources by kind and optional namespace. Supports label selectors for filtering.",
				InputSchema: mcp.ToolInputSchema{
					Type:     "object",
					Required: []string{"cluster", "kind"},
					Properties: map[string]any{
						"cluster": map[string]any{
							"type":        "string",
							"description": "Cluster ID (use cluster_list tool to get available cluster IDs)",
						},
						"kind": map[string]any{
							"type":        "string",
							"description": "Resource kind (e.g., pod, deployment, service, configmap, secret, ingress, etc.)",
						},
						"namespace": map[string]any{
							"type":        "string",
							"description": "Namespace name (optional, empty for all namespaces or cluster-scoped resources)",
							"default":     "",
						},
						"name": map[string]any{
							"type":        "string",
							"description": "Filter by resource name (partial match, client-side)",
							"default":     "",
						},
						"labelSelector": map[string]any{
							"type":        "string",
							"description": "Label selector for filtering (e.g., 'app=nginx,env=prod')",
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
						"showSensitiveData": showSensitiveDataProperty,
					},
				},
			},
			Annotations: toolset.ToolAnnotations{
				ReadOnlyHint: handler.BoolPtr(true),
			},
			Handler: listHandler,
		},
		{
			Tool: mcp.Tool{
				Name:        "kubernetes_logs",
				Description: "Get logs from a pod or specific container. Supports tail lines, time range filtering, keyword search, and multi-pod log aggregation via label selector. Use 'name' for single pod logs, or 'labelSelector' to aggregate logs from multiple pods (e.g., all pods of a deployment).",
				InputSchema: mcp.ToolInputSchema{
					Type:     "object",
					Required: []string{"cluster", "namespace"},
					Properties: map[string]any{
						"cluster": map[string]any{
							"type":        "string",
							"description": "Cluster ID (use cluster_list tool to get available cluster IDs)",
						},
						"namespace": map[string]any{
							"type":        "string",
							"description": "Namespace name",
						},
						"name": map[string]any{
							"type":        "string",
							"description": "Pod name (optional, use for single pod logs. Either 'name' or 'labelSelector' is required)",
							"default":     "",
						},
						"labelSelector": map[string]any{
							"type":        "string",
							"description": "Label selector for filtering pods (optional, use for multi-pod log aggregation. Either 'name' or 'labelSelector' is required). Example: 'app=nginx,env=prod'",
							"default":     "",
						},
						"container": map[string]any{
							"type":        "string",
							"description": "Container name (optional, fetches all containers if not specified)",
							"default":     "",
						},
						"tailLines": map[string]any{
							"type":        "integer",
							"description": "Number of lines from the end to show",
							"default":     100,
						},
						"sinceSeconds": map[string]any{
							"type":        "integer",
							"description": "Show logs from last N seconds (optional)",
						},
						"timestamps": map[string]any{
							"type":        "boolean",
							"description": "Include timestamps in log output",
							"default":     false,
						},
						"previous": map[string]any{
							"type":        "boolean",
							"description": "Get logs from previous container instance",
							"default":     false,
						},
						"keyword": map[string]any{
							"type":        "string",
							"description": "Filter log lines containing this keyword (case-insensitive)",
							"default":     "",
						},
					},
				},
			},
			Annotations: toolset.ToolAnnotations{
				ReadOnlyHint: handler.BoolPtr(true),
			},
			Handler: logsHandler,
		},
		{
			Tool: mcp.Tool{
				Name:        "kubernetes_inspect_pod",
				Description: "Get comprehensive pod diagnostics: pod details, parent workload (Deployment/StatefulSet/DaemonSet), metrics, and container logs.",
				InputSchema: mcp.ToolInputSchema{
					Type:     "object",
					Required: []string{"cluster", "namespace", "name"},
					Properties: map[string]any{
						"cluster": map[string]any{
							"type":        "string",
							"description": "Cluster ID (use cluster_list tool to get available cluster IDs)",
						},
						"namespace": map[string]any{
							"type":        "string",
							"description": "Namespace name",
						},
						"name": map[string]any{
							"type":        "string",
							"description": "Pod name",
						},
					},
				},
			},
			Annotations: toolset.ToolAnnotations{
				ReadOnlyHint: handler.BoolPtr(true),
			},
			Handler: inspectPodHandler,
		},
		{
			Tool: mcp.Tool{
				Name:        "kubernetes_describe",
				Description: "Describe a Kubernetes resource with its related events. Similar to 'kubectl describe', returns resource details and associated events.",
				InputSchema: mcp.ToolInputSchema{
					Type:     "object",
					Required: []string{"cluster", "kind", "name"},
					Properties: map[string]any{
						"cluster": map[string]any{
							"type":        "string",
							"description": "Cluster ID (use cluster_list tool to get available cluster IDs)",
						},
						"kind": map[string]any{
							"type":        "string",
							"description": "Resource kind (e.g., pod, deployment, service, node, etc.)",
						},
						"namespace": map[string]any{
							"type":        "string",
							"description": "Namespace name (optional for cluster-scoped resources)",
							"default":     "",
						},
						"name": map[string]any{
							"type":        "string",
							"description": "Resource name",
						},
						"format": map[string]any{
							"type":        "string",
							"description": "Output format: json or yaml",
							"enum":        []string{"json", "yaml"},
							"default":     "json",
						},
						"showSensitiveData": showSensitiveDataProperty,
					},
				},
			},
			Annotations: toolset.ToolAnnotations{
				ReadOnlyHint: handler.BoolPtr(true),
			},
			Handler: describeHandler,
		},
		{
			Tool: mcp.Tool{
				Name:        "kubernetes_events",
				Description: "List Kubernetes events. Supports filtering by namespace, involved object name, and involved object kind.",
				InputSchema: mcp.ToolInputSchema{
					Type:     "object",
					Required: []string{"cluster"},
					Properties: map[string]any{
						"cluster": map[string]any{
							"type":        "string",
							"description": "Cluster ID (use cluster_list tool to get available cluster IDs)",
						},
						"namespace": map[string]any{
							"type":        "string",
							"description": "Namespace name (optional, empty for all namespaces)",
							"default":     "",
						},
						"name": map[string]any{
							"type":        "string",
							"description": "Filter by involved object name (optional)",
							"default":     "",
						},
						"kind": map[string]any{
							"type":        "string",
							"description": "Filter by involved object kind, e.g., Pod, Deployment, Node (optional)",
							"default":     "",
						},
						"limit": map[string]any{
							"type":        "integer",
							"description": "Number of events per page",
							"default":     50,
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
							"default":     "table",
						},
					},
				},
			},
			Annotations: toolset.ToolAnnotations{
				ReadOnlyHint: handler.BoolPtr(true),
			},
			Handler: eventsHandler,
		},
		{
			Tool: mcp.Tool{
				Name:        "kubernetes_dep",
				Description: "Show all dependencies or dependents of any Kubernetes resource as a tree. Covers OwnerReference chains, Pod->Node/SA/ConfigMap/Secret/PVC, Service->Pod (label selector), Ingress->IngressClass/Service/TLS Secret, PVC<->PV->StorageClass, RBAC bindings, PDB->Pod, and Events.",
				InputSchema: mcp.ToolInputSchema{
					Type:     "object",
					Required: []string{"cluster", "kind", "name"},
					Properties: map[string]any{
						"cluster": map[string]any{
							"type":        "string",
							"description": "Cluster ID (use cluster_list tool to get available cluster IDs)",
						},
						"kind": map[string]any{
							"type":        "string",
							"description": "Resource kind (e.g., deployment, pod, service, ingress, node, etc.)",
						},
						"namespace": map[string]any{
							"type":        "string",
							"description": "Namespace name (optional for cluster-scoped resources)",
							"default":     "",
						},
						"name": map[string]any{
							"type":        "string",
							"description": "Resource name",
						},
						"direction": map[string]any{
							"type":        "string",
							"description": "Traversal direction: 'dependents' shows resources that depend on this resource, 'dependencies' shows resources this resource depends on",
							"enum":        []string{"dependents", "dependencies"},
							"default":     "dependents",
						},
						"depth": map[string]any{
							"type":        "integer",
							"description": "Maximum traversal depth (1-20)",
							"default":     10,
						},
						"format": map[string]any{
							"type":        "string",
							"description": "Output format: tree (human-readable) or json (structured)",
							"enum":        []string{"tree", "json"},
							"default":     "tree",
						},
					},
				},
			},
			Annotations: toolset.ToolAnnotations{
				ReadOnlyHint: handler.BoolPtr(true),
			},
			Handler: depHandler,
		},
		{
			Tool: mcp.Tool{
				Name:        "kubernetes_rollout_history",
				Description: "Get rollout history for a Deployment, including revision versions and change causes. Similar to 'kubectl rollout history deployment'.",
				InputSchema: mcp.ToolInputSchema{
					Type:     "object",
					Required: []string{"cluster", "namespace", "name"},
					Properties: map[string]any{
						"cluster": map[string]any{
							"type":        "string",
							"description": "Cluster ID (use cluster_list tool to get available cluster IDs)",
						},
						"namespace": map[string]any{
							"type":        "string",
							"description": "Namespace name",
						},
						"name": map[string]any{
							"type":        "string",
							"description": "Deployment name",
						},
						"format": map[string]any{
							"type":        "string",
							"description": "Output format: json or table",
							"enum":        []string{"json", "table"},
							"default":     "table",
						},
					},
				},
			},
			Annotations: toolset.ToolAnnotations{
				ReadOnlyHint: handler.BoolPtr(true),
			},
			Handler: rolloutHistoryHandler,
		},
	}

	// Add write operations if not in read-only mode
	if !t.ReadOnly {
		tools = append(tools,
			toolset.ServerTool{
				Tool: mcp.Tool{
					Name:        "kubernetes_create",
					Description: "Create a Kubernetes resource from a JSON manifest.",
					InputSchema: mcp.ToolInputSchema{
						Type:     "object",
						Required: []string{"cluster", "resource"},
						Properties: map[string]any{
							"cluster": map[string]any{
								"type":        "string",
								"description": "Cluster ID (use cluster_list tool to get available cluster IDs)",
							},
							"resource": map[string]any{
								"type":        "string",
								"description": "Resource manifest as JSON string (must include apiVersion, kind, metadata, and spec)",
							},
						},
					},
				},
				Annotations: toolset.ToolAnnotations{
					ReadOnlyHint: handler.BoolPtr(false),
				},
				Handler: createHandler,
			},
			toolset.ServerTool{
				Tool: mcp.Tool{
					Name:        "kubernetes_patch",
					Description: "Patch a Kubernetes resource using JSON Patch (RFC 6902).",
					InputSchema: mcp.ToolInputSchema{
						Type:     "object",
						Required: []string{"cluster", "kind", "name", "patch"},
						Properties: map[string]any{
							"cluster": map[string]any{
								"type":        "string",
								"description": "Cluster ID (use cluster_list tool to get available cluster IDs)",
							},
							"kind": map[string]any{
								"type":        "string",
								"description": "Resource kind (e.g., deployment, service)",
							},
							"namespace": map[string]any{
								"type":        "string",
								"description": "Namespace name (optional for cluster-scoped resources)",
								"default":     "",
							},
							"name": map[string]any{
								"type":        "string",
								"description": "Resource name",
							},
							"patch": map[string]any{
								"type":        "string",
								"description": "JSON Patch array as string, e.g., '[{\"op\":\"replace\",\"path\":\"/spec/replicas\",\"value\":3}]'",
							},
						},
					},
				},
				Annotations: toolset.ToolAnnotations{
					ReadOnlyHint: handler.BoolPtr(false),
				},
				Handler: patchHandler,
			},
		)

		// Add delete only if destructive operations are enabled
		if !t.DisableDestructive {
			tools = append(tools, toolset.ServerTool{
				Tool: mcp.Tool{
					Name:        "kubernetes_delete",
					Description: "Delete a Kubernetes resource.",
					InputSchema: mcp.ToolInputSchema{
						Type:     "object",
						Required: []string{"cluster", "kind", "name"},
						Properties: map[string]any{
							"cluster": map[string]any{
								"type":        "string",
								"description": "Cluster ID (use cluster_list tool to get available cluster IDs)",
							},
							"kind": map[string]any{
								"type":        "string",
								"description": "Resource kind (e.g., deployment, service)",
							},
							"namespace": map[string]any{
								"type":        "string",
								"description": "Namespace name (optional for cluster-scoped resources)",
								"default":     "",
							},
							"name": map[string]any{
								"type":        "string",
								"description": "Resource name",
							},
						},
					},
				},
				Annotations: toolset.ToolAnnotations{
					ReadOnlyHint:    handler.BoolPtr(false),
					DestructiveHint: handler.BoolPtr(true),
				},
				Handler: deleteHandler,
			})
		}
	}

	return tools
}
