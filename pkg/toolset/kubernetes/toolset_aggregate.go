package kubernetes

import (
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/futuretea/rancher-mcp-server/pkg/toolset"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset/paramutil"
)

// aggregateTools returns the aggregate/ranking tool definitions.
func aggregateTools() []toolset.ServerTool {
	return []toolset.ServerTool{
		topTool(),
		workloadHealthTool(),
		resourceSummaryTool(),
		eventSummaryTool(),
	}
}

func topTool() toolset.ServerTool {
	return toolset.ServerTool{
		Tool: mcp.Tool{
			Name:        "kubernetes_top",
			Description: "Rank pods or nodes by resource usage, requests, limits, or restart count. Supports metrics-server utilization data with fallback to requests/limits when metrics are unavailable.",
			InputSchema: mcp.ToolInputSchema{
				Type:     "object",
				Required: []string{"cluster"},
				Properties: map[string]any{
					"cluster": clusterIDProperty,
					"kind": map[string]any{
						"type":        "string",
						"description": "Resource kind to rank: 'pod' or 'node'",
						"enum":        []string{"pod", "node"},
						"default":     "pod",
					},
					"namespace": map[string]any{
						"type":        "string",
						"description": "Namespace name (optional, empty for all namespaces)",
						"default":     "",
					},
					"labelSelector": map[string]any{
						"type":        "string",
						"description": "Label selector for filtering (e.g., 'app=nginx,env=prod')",
						"default":     "",
					},
					"sortBy": map[string]any{
						"type":        "string",
						"description": "Sort by field. For pods: cpu.util, mem.util, cpu.request, mem.request, cpu.limit, mem.limit, restart.count. For nodes: cpu.util, mem.util, cpu.util.percentage, mem.util.percentage, pod.count",
						"enum":        []string{"", "cpu.util", "mem.util", "cpu.request", "mem.request", "cpu.limit", "mem.limit", "cpu.util.percentage", "mem.util.percentage", "restart.count", "pod.count", "name"},
						"default":     "",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Maximum number of results to return (Top-N)",
						"default":     50,
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
		Handler: topHandler,
	}
}

func workloadHealthTool() toolset.ServerTool {
	return toolset.ServerTool{
		Tool: mcp.Tool{
			Name:        "kubernetes_workload_health",
			Description: "Get a health summary for Deployments, StatefulSets, and DaemonSets. Shows ready vs desired replicas, unavailable count, update progress, and derived status.",
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
					"kind": map[string]any{
						"type":        "string",
						"description": "Workload kind to check: 'deployment', 'statefulset', 'daemonset', or 'all'",
						"enum":        []string{"deployment", "statefulset", "daemonset", "all"},
						"default":     "all",
					},
					"labelSelector": map[string]any{
						"type":        "string",
						"description": "Label selector for filtering (e.g., 'app=nginx,env=prod')",
						"default":     "",
					},
					"sortBy": map[string]any{
						"type":        "string",
						"description": "Sort by field: unready.count, ready.ratio, name",
						"enum":        []string{"", "unready.count", "ready.ratio", "name"},
						"default":     "",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Maximum number of results to return",
						"default":     50,
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
		Handler: workloadHealthHandler,
	}
}

func resourceSummaryTool() toolset.ServerTool {
	return toolset.ServerTool{
		Tool: mcp.Tool{
			Name:        "kubernetes_resource_summary",
			Description: "Aggregate pod/container resources by namespace or label key. Returns total requests, limits, and pod counts per group.",
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
					"labelSelector": map[string]any{
						"type":        "string",
						"description": "Label selector for filtering pods (e.g., 'app=nginx,env=prod')",
						"default":     "",
					},
					"groupBy": map[string]any{
						"type":        "string",
						"description": "Group by: 'namespace' or 'label'",
						"enum":        []string{"namespace", "label"},
						"default":     "namespace",
					},
					"groupByKey": map[string]any{
						"type":        "string",
						"description": "Label key to group by (required when groupBy=label)",
						"default":     "",
					},
					"sortBy": map[string]any{
						"type":        "string",
						"description": "Sort by field: cpu.request, mem.request, cpu.limit, mem.limit, pod.count, name",
						"enum":        []string{"", "cpu.request", "mem.request", "cpu.limit", "mem.limit", "pod.count", "name"},
						"default":     "",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Maximum number of results to return",
						"default":     50,
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
		Handler: resourceSummaryHandler,
	}
}

func eventSummaryTool() toolset.ServerTool {
	return toolset.ServerTool{
		Tool: mcp.Tool{
			Name:        "kubernetes_event_summary",
			Description: "Group and rank Kubernetes events by reason, kind, and frequency. Useful for identifying recurring issues and patterns.",
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
					"kind": map[string]any{
						"type":        "string",
						"description": "Filter by involved object kind (e.g., Pod, Deployment, Node)",
						"default":     "",
					},
					"type": map[string]any{
						"type":        "string",
						"description": "Filter by event type: 'Warning' or 'Normal'",
						"enum":        []string{"", "Warning", "Normal"},
						"default":     "",
					},
					"since": map[string]any{
						"type":        "string",
						"description": "Only include events newer than this duration (e.g., '1h30m', '2h')",
						"default":     "",
					},
					"sortBy": map[string]any{
						"type":        "string",
						"description": "Sort by field: count, lastSeen, name",
						"enum":        []string{"", "count", "lastSeen", "name"},
						"default":     "",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Maximum number of results to return",
						"default":     50,
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
		Handler: eventSummaryHandler,
	}
}
