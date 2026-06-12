package kubernetes

import (
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/futuretea/rancher-mcp-server/pkg/toolset"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset/paramutil"
)

// resourceTools returns the core resource-oriented read-only tool definitions.
func resourceTools() []toolset.ServerTool {
	return []toolset.ServerTool{
		getTool(),
		listTool(),
		getAllTool(),
		logsTool(),
		inspectPodTool(),
		describeTool(),
		eventsTool(),
		rolloutHistoryTool(),
	}
}

func getTool() toolset.ServerTool {
	return toolset.ServerTool{
		Tool: mcp.Tool{
			Name:        "kubernetes_get",
			Description: "Get any Kubernetes resource by kind, namespace, and name. Works with any resource type including CRDs.",
			InputSchema: mcp.ToolInputSchema{
				Type:     "object",
				Required: []string{"cluster", "kind", "name"},
				Properties: map[string]any{
					"cluster": clusterIDProperty,
					"kind": map[string]any{
						"type":        "string",
						"description": "Resource kind (e.g., pod, deployment, service, App). For CRDs, pass the manifest kind and optionally apiVersion.",
					},
					"apiVersion": apiVersionProperty,
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
			ReadOnlyHint: paramutil.BoolPtr(true),
		},
		Handler: getHandler,
	}
}

func listTool() toolset.ServerTool {
	return toolset.ServerTool{
		Tool: mcp.Tool{
			Name:        "kubernetes_list",
			Description: "List Kubernetes resources by kind and optional namespace. Supports label selectors for filtering.",
			InputSchema: mcp.ToolInputSchema{
				Type:     "object",
				Required: []string{"cluster", "kind"},
				Properties: map[string]any{
					"cluster": clusterIDProperty,
					"kind": map[string]any{
						"type":        "string",
						"description": "Resource kind (e.g., pod, deployment, service, App). For CRDs, pass the manifest kind and optionally apiVersion.",
					},
					"apiVersion": apiVersionProperty,
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
			ReadOnlyHint: paramutil.BoolPtr(true),
		},
		Handler: listHandler,
	}
}

func getAllTool() toolset.ServerTool {
	return toolset.ServerTool{
		Tool: mcp.Tool{
			Name:        "kubernetes_get_all",
			Description: "Get really all Kubernetes resources in the cluster (inspired by ketall). Unlike 'kubectl get all', this shows all resource types including ConfigMaps, Secrets, RBAC resources, CRDs, and other resources that are normally hidden. Supports filtering by namespace, scope, and creation time.",
			InputSchema: mcp.ToolInputSchema{
				Type:     "object",
				Required: []string{"cluster"},
				Properties: map[string]any{
					"cluster": clusterIDProperty,
					"namespace": map[string]any{
						"type":        "string",
						"description": "Filter by namespace (optional, empty for all namespaces)",
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
					"excludeEvents": map[string]any{
						"type":        "boolean",
						"description": "Exclude events from output (default true, as events are often noisy)",
						"default":     true,
					},
					"scope": map[string]any{
						"type":        "string",
						"description": "Filter by scope: 'namespaced' for namespaced resources only, 'cluster' for cluster-scoped resources only, or empty for all",
						"enum":        []string{"", "namespaced", "cluster"},
						"default":     "",
					},
					"since": map[string]any{
						"type":        "string",
						"description": "Only show resources created since this duration (e.g., '1h30m', '2d', '1w')",
						"default":     "",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Limit number of resources per API call (0 for no limit)",
						"default":     0,
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
			ReadOnlyHint: paramutil.BoolPtr(true),
		},
		Handler: getAllHandler,
	}
}

func logsTool() toolset.ServerTool {
	return toolset.ServerTool{
		Tool: mcp.Tool{
			Name:        "kubernetes_logs",
			Description: "Get logs from a pod or specific container. Supports tail lines, time range filtering, keyword search, and multi-pod log aggregation via label selector. Use 'name' for single pod logs, or 'labelSelector' to aggregate logs from multiple pods (e.g., all pods of a deployment).",
			InputSchema: mcp.ToolInputSchema{
				Type:     "object",
				Required: []string{"cluster", "namespace"},
				Properties: map[string]any{
					"cluster": clusterIDProperty,
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
						"default":     true,
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
			ReadOnlyHint: paramutil.BoolPtr(true),
		},
		Handler: logsHandler,
	}
}

func inspectPodTool() toolset.ServerTool {
	return toolset.ServerTool{
		Tool: mcp.Tool{
			Name:        "kubernetes_inspect_pod",
			Description: "Get comprehensive pod diagnostics: pod details, parent workload (Deployment/StatefulSet/DaemonSet), metrics, and container logs.",
			InputSchema: mcp.ToolInputSchema{
				Type:     "object",
				Required: []string{"cluster", "namespace", "name"},
				Properties: map[string]any{
					"cluster": clusterIDProperty,
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
			ReadOnlyHint: paramutil.BoolPtr(true),
		},
		Handler: inspectPodHandler,
	}
}

func describeTool() toolset.ServerTool {
	return toolset.ServerTool{
		Tool: mcp.Tool{
			Name:        "kubernetes_describe",
			Description: "Describe a Kubernetes resource with its related events. Similar to 'kubectl describe', returns resource details and associated events.",
			InputSchema: mcp.ToolInputSchema{
				Type:     "object",
				Required: []string{"cluster", "kind", "name"},
				Properties: map[string]any{
					"cluster": clusterIDProperty,
					"kind": map[string]any{
						"type":        "string",
						"description": "Resource kind (e.g., pod, deployment, service, App). For CRDs, pass the manifest kind and optionally apiVersion.",
					},
					"apiVersion": apiVersionProperty,
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
			ReadOnlyHint: paramutil.BoolPtr(true),
		},
		Handler: describeHandler,
	}
}

func eventsTool() toolset.ServerTool {
	return toolset.ServerTool{
		Tool: mcp.Tool{
			Name:        "kubernetes_events",
			Description: "List Kubernetes events. Supports filtering by namespace, involved object name, and involved object kind.",
			InputSchema: mcp.ToolInputSchema{
				Type:     "object",
				Required: []string{"cluster"},
				Properties: map[string]any{
					"cluster": clusterIDProperty,
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
			ReadOnlyHint: paramutil.BoolPtr(true),
		},
		Handler: eventsHandler,
	}
}

func rolloutHistoryTool() toolset.ServerTool {
	return toolset.ServerTool{
		Tool: mcp.Tool{
			Name:        "kubernetes_rollout_history",
			Description: "Get rollout history for a Deployment, including revision versions and change causes. Similar to 'kubectl rollout history deployment'.",
			InputSchema: mcp.ToolInputSchema{
				Type:     "object",
				Required: []string{"cluster", "namespace", "name"},
				Properties: map[string]any{
					"cluster": clusterIDProperty,
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
			ReadOnlyHint: paramutil.BoolPtr(true),
		},
		Handler: rolloutHistoryHandler,
	}
}
