package kubernetes

import (
	"maps"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/futuretea/rancher-mcp-server/pkg/toolset"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset/paramutil"
)

// analysisTools returns the analysis, diff, watch, and capacity tool definitions.
func analysisTools() []toolset.ServerTool {
	return []toolset.ServerTool{
		depTool(),
		nodeAnalysisTool(),
		resourceDiffTool(),
		watchTool(),
		diffTool(),
		capacityTool(),
	}
}

func depTool() toolset.ServerTool {
	return toolset.ServerTool{
		Tool: mcp.Tool{
			Name:        "kubernetes_dep",
			Description: "Show all dependencies or dependents of any Kubernetes resource as a tree. Covers OwnerReference chains, Pod->Node/SA/ConfigMap/Secret/PVC, Service->Pod (label selector), Ingress->IngressClass/Service/TLS Secret, PVC<->PV->StorageClass, RBAC bindings, PDB->Pod, and Events. Cluster-scoped roots can narrow auxiliary namespaced scans with scanNamespace, and maxScannedObjects enables fail-fast scan budgeting.",
			InputSchema: mcp.ToolInputSchema{
				Type:     "object",
				Required: []string{"cluster", "kind", "name"},
				Properties: map[string]any{
					"cluster": clusterIDProperty,
					"kind": map[string]any{
						"type":        "string",
						"description": "Resource kind (e.g., deployment, pod, service, App). For CRDs, pass the manifest kind and optionally apiVersion.",
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
					"scanNamespace": map[string]any{
						"type":        "string",
						"description": "Optional namespace override for auxiliary namespaced scans. Use this only with cluster-scoped roots; namespaced roots must match their own namespace.",
						"default":     "",
					},
					"maxScannedObjects": map[string]any{
						"type":        "integer",
						"description": "Optional fail-fast budget for total scanned objects. When set to a value greater than 0, kubernetes_dep aborts instead of building a partial graph after the budget is exceeded.",
						"default":     0,
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
			ReadOnlyHint: paramutil.BoolPtr(true),
		},
		Handler: depHandler,
	}
}

func nodeAnalysisTool() toolset.ServerTool {
	return toolset.ServerTool{
		Tool: mcp.Tool{
			Name:        "kubernetes_node_analysis",
			Description: "Get comprehensive node analysis including capacity, allocated resources, taints, labels, and list of pods running on the node.",
			InputSchema: mcp.ToolInputSchema{
				Type:     "object",
				Required: []string{"cluster", "name"},
				Properties: map[string]any{
					"cluster": clusterIDProperty,
					"name": map[string]any{
						"type":        "string",
						"description": "Node name",
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
		Annotations: toolset.ToolAnnotations{
			ReadOnlyHint: paramutil.BoolPtr(true),
		},
		Handler: nodeAnalysisHandler,
	}
}

func resourceDiffTool() toolset.ServerTool {
	return toolset.ServerTool{
		Tool: mcp.Tool{
			Name:        "kubernetes_resource_diff",
			Description: "Compare two Kubernetes resources (e.g., two deployments). Returns a git-style diff showing differences between the specified resources. Can compare resources across different clusters and namespaces.",
			InputSchema: mcp.ToolInputSchema{
				Type:     "object",
				Required: []string{"kind", "left", "right"},
				Properties: map[string]any{
					"kind": map[string]any{
						"type":        "string",
						"description": "Resource kind (e.g., deployment, daemonset, statefulset)",
					},
					"apiVersion": map[string]any{
						"type":        "string",
						"description": "API version (e.g., apps/v1)",
						"default":     "",
					},
					"left": map[string]any{
						"type":        "object",
						"description": "Left side of the comparison",
						"properties": map[string]any{
							"cluster": clusterIDProperty,
							"namespace": map[string]any{
								"type":        "string",
								"description": "Namespace name (optional, empty for cluster-scoped resources)",
							},
							"name": map[string]any{
								"type":        "string",
								"description": "Resource name",
							},
						},
						"required": []string{"cluster", "name"},
					},
					"right": map[string]any{
						"type":        "object",
						"description": "Right side of the comparison",
						"properties": map[string]any{
							"cluster": clusterIDProperty,
							"namespace": map[string]any{
								"type":        "string",
								"description": "Namespace name (optional, empty for cluster-scoped resources)",
							},
							"name": map[string]any{
								"type":        "string",
								"description": "Resource name",
							},
						},
						"required": []string{"cluster", "name"},
					},
					"ignoreStatus": map[string]any{
						"type":        "boolean",
						"description": "Ignore changes under the status field when computing diffs",
						"default":     false,
					},
					"ignoreMeta": map[string]any{
						"type":        "boolean",
						"description": "Ignore non-essential metadata differences (managedFields, resourceVersion, uid, etc.)",
						"default":     true,
					},
				},
			},
		},
		Annotations: toolset.ToolAnnotations{
			ReadOnlyHint: paramutil.BoolPtr(true),
		},
		Handler: resourceDiffHandler,
	}
}

func watchTool() toolset.ServerTool {
	return toolset.ServerTool{
		Tool: mcp.Tool{
			Name:        "kubernetes_watch",
			Description: "Watch Kubernetes resources with polling and return git-style diffs for each interval, including deletion diffs. The polling path fails fast when per-iteration resource or output-size guards are exceeded.",
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
					"labelSelector": map[string]any{
						"type":        "string",
						"description": "Label selector for filtering (e.g., 'app=nginx,env=prod')",
						"default":     "",
					},
					"fieldSelector": map[string]any{
						"type":        "string",
						"description": "Field selector for filtering",
						"default":     "",
					},
					"ignoreStatus": map[string]any{
						"type":        "boolean",
						"description": "Ignore changes under the status field when computing diffs (similar to --no-status)",
						"default":     false,
					},
					"ignoreMeta": map[string]any{
						"type":        "boolean",
						"description": "Ignore non-essential metadata differences (similar to --no-meta)",
						"default":     false,
					},
					"intervalSeconds": map[string]any{
						"type":        "integer",
						"description": "Interval in seconds between evaluations, like the Linux 'watch' command",
						"default":     10,
					},
					"iterations": map[string]any{
						"type":        "integer",
						"description": "Number of times to re-evaluate and diff before returning. Use a small number to avoid very large outputs.",
						"default":     6,
					},
				},
			},
		},
		Annotations: toolset.ToolAnnotations{
			ReadOnlyHint: paramutil.BoolPtr(true),
		},
		Handler: watchDiffHandler,
	}
}

func diffTool() toolset.ServerTool {
	return toolset.ServerTool{
		Tool: mcp.Tool{
			Name:        "kubernetes_diff",
			Description: "Compare two Kubernetes resource versions and show the differences as a git-style diff. Useful for comparing current vs desired state, or before/after changes.",
			InputSchema: mcp.ToolInputSchema{
				Type:     "object",
				Required: []string{"resource1", "resource2"},
				Properties: map[string]any{
					"resource1": map[string]any{
						"type":        "string",
						"description": "First resource version as JSON string (the 'before' or 'old' version). Use kubernetes_get to retrieve the resource.",
					},
					"resource2": map[string]any{
						"type":        "string",
						"description": "Second resource version as JSON string (the 'after' or 'new' version). Use kubernetes_get to retrieve the resource.",
					},
					"ignoreStatus": map[string]any{
						"type":        "boolean",
						"description": "Ignore changes under the status field when computing diffs",
						"default":     false,
					},
					"ignoreMeta": map[string]any{
						"type":        "boolean",
						"description": "Ignore non-essential metadata differences (managedFields, resourceVersion, etc.)",
						"default":     false,
					},
				},
			},
		},
		Annotations: toolset.ToolAnnotations{
			ReadOnlyHint:       paramutil.BoolPtr(true),
			RequiresKubernetes: paramutil.BoolPtr(false),
		},
		Handler: diffHandler,
	}
}

func capacityTool() toolset.ServerTool {
	return toolset.ServerTool{
		Tool: mcp.Tool{
			Name:        "kubernetes_capacity",
			Description: "Show Kubernetes cluster resource capacity, requests, limits, and utilization. Similar to kube-capacity CLI tool. Combines the best parts of kubectl top and kubectl describe into an easy to read table showing node and pod resource information.",
			InputSchema: mcp.ToolInputSchema{
				Type:       "object",
				Required:   []string{"cluster"},
				Properties: capacityToolProperties(),
			},
		},
		Annotations: toolset.ToolAnnotations{
			ReadOnlyHint: paramutil.BoolPtr(true),
		},
		Handler: capacityHandler,
	}
}

func capacityToolProperties() map[string]any {
	props := capacityToolResourceProperties()
	maps.Copy(props, capacityToolFilterProperties())
	maps.Copy(props, capacityToolFormatProperties())
	return props
}

func capacityToolResourceProperties() map[string]any {
	return map[string]any{
		"pods": map[string]any{
			"type":        "boolean",
			"description": "Include individual pod resources in the output",
			"default":     false,
		},
		"util": map[string]any{
			"type":        "boolean",
			"description": "Include actual resource utilization from metrics-server (requires metrics-server to be installed)",
			"default":     false,
		},
		"available": map[string]any{
			"type":        "boolean",
			"description": "Show raw available capacity instead of percentages",
			"default":     false,
		},
		"containers": map[string]any{
			"type":        "boolean",
			"description": "Include individual container resources in the output (implies pods=true)",
			"default":     false,
		},
		"podCount": map[string]any{
			"type":        "boolean",
			"description": "Include pod counts for each node and the whole cluster",
			"default":     false,
		},
		"hideRequests": map[string]any{
			"type":        "boolean",
			"description": "Hide request columns from output",
			"default":     false,
		},
		"hideLimits": map[string]any{
			"type":        "boolean",
			"description": "Hide limit columns from output",
			"default":     false,
		},
	}
}

func capacityToolFilterProperties() map[string]any {
	return map[string]any{
		"cluster": clusterIDProperty,
		"namespace": map[string]any{
			"type":        "string",
			"description": "Filter by namespace (optional, empty for all namespaces)",
			"default":     "",
		},
		"labelSelector": map[string]any{
			"type":        "string",
			"description": "Filter pods by label selector (e.g., 'app=nginx,env=prod')",
			"default":     "",
		},
		"nodeLabelSelector": map[string]any{
			"type":        "string",
			"description": "Filter nodes by label selector (e.g., 'node-role.kubernetes.io/worker=true')",
			"default":     "",
		},
		"namespaceLabelSelector": map[string]any{
			"type":        "string",
			"description": "Filter namespaces by label selector (e.g., 'env=production')",
			"default":     "",
		},
		"nodeTaints": map[string]any{
			"type":        "string",
			"description": "Filter nodes by taints. Use 'key=value:effect' to include, 'key=value:effect-' to exclude. Multiple taints can be separated by comma",
			"default":     "",
		},
		"noTaint": map[string]any{
			"type":        "boolean",
			"description": "Exclude nodes with any taints",
			"default":     false,
		},
		"showLabels": map[string]any{
			"type":        "boolean",
			"description": "Include node labels in the output",
			"default":     false,
		},
	}
}

func capacityToolFormatProperties() map[string]any {
	return map[string]any{
		"sortBy": map[string]any{
			"type":        "string",
			"description": "Sort by field: cpu.util, mem.util, cpu.request, mem.request, cpu.limit, mem.limit, cpu.util.percentage, mem.util.percentage, cpu.request.percentage, mem.request.percentage, cpu.limit.percentage, mem.limit.percentage, pod.count, name",
			"enum":        []string{"", "cpu.util", "mem.util", "cpu.request", "mem.request", "cpu.limit", "mem.limit", "cpu.util.percentage", "mem.util.percentage", "cpu.request.percentage", "mem.request.percentage", "cpu.limit.percentage", "mem.limit.percentage", "pod.count", "name"},
			"default":     "",
		},
		"format": map[string]any{
			"type":        "string",
			"description": "Output format: table, json, yaml",
			"enum":        []string{"table", "json", "yaml"},
			"default":     "table",
		},
	}
}
