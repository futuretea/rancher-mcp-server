// Package kubernetes provides the Kubernetes toolset using Steve API.
package kubernetes

import (
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/futuretea/rancher-mcp-server/pkg/toolset"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset/paramutil"
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
				ReadOnlyHint: paramutil.BoolPtr(true),
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
				ReadOnlyHint: paramutil.BoolPtr(true),
			},
			Handler: listHandler,
		},
		{
			Tool: mcp.Tool{
				Name:        "kubernetes_get_all",
				Description: "Get really all Kubernetes resources in the cluster (inspired by ketall). Unlike 'kubectl get all', this shows all resource types including ConfigMaps, Secrets, RBAC resources, CRDs, and other resources that are normally hidden. Supports filtering by namespace, scope, and creation time.",
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
				ReadOnlyHint: paramutil.BoolPtr(true),
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
				ReadOnlyHint: paramutil.BoolPtr(true),
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
				ReadOnlyHint: paramutil.BoolPtr(true),
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
				ReadOnlyHint: paramutil.BoolPtr(true),
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
				ReadOnlyHint: paramutil.BoolPtr(true),
			},
			Handler: rolloutHistoryHandler,
		},
		{
			Tool: mcp.Tool{
				Name:        "kubernetes_node_analysis",
				Description: "Get comprehensive node analysis including capacity, allocated resources, taints, labels, and list of pods running on the node.",
				InputSchema: mcp.ToolInputSchema{
					Type:     "object",
					Required: []string{"cluster", "name"},
					Properties: map[string]any{
						"cluster": map[string]any{
							"type":        "string",
							"description": "Cluster ID (use cluster_list tool to get available cluster IDs)",
						},
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
		},
		{
			Tool: mcp.Tool{
				Name:        "kubernetes_diff",
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
								"cluster": map[string]any{
									"type":        "string",
									"description": "Cluster ID (use cluster_list tool to get available cluster IDs)",
								},
								"namespace": map[string]any{
									"type":        "string",
									"description": "Namespace name",
									"default":     "default",
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
								"cluster": map[string]any{
									"type":        "string",
									"description": "Cluster ID (use cluster_list tool to get available cluster IDs)",
								},
								"namespace": map[string]any{
									"type":        "string",
									"description": "Namespace name",
									"default":     "default",
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
			Handler: diffHandler,
		},
		{
			Tool: mcp.Tool{
				Name:        "kubernetes_watch",
				Description: "Watch Kubernetes resources and return git-style diffs for each interval, similar to the Linux 'watch' command.",
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
							"description": "Resource kind (e.g., pod, deployment, service, or dotted resource.group form)",
						},
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
		},
		{
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
				ReadOnlyHint: paramutil.BoolPtr(true),
			},
			Handler: diffHandler,
		},
		{
			Tool: mcp.Tool{
				Name:        "kubernetes_capacity",
				Description: "Show Kubernetes cluster resource capacity, requests, limits, and utilization. Similar to kube-capacity CLI tool. Combines the best parts of kubectl top and kubectl describe into an easy to read table showing node and pod resource information.",
				InputSchema: mcp.ToolInputSchema{
					Type:     "object",
					Required: []string{"cluster"},
					Properties: map[string]any{
						"cluster": map[string]any{
							"type":        "string",
							"description": "Cluster ID (use cluster_list tool to get available cluster IDs)",
						},
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
						"showLabels": map[string]any{
							"type":        "boolean",
							"description": "Include node labels in the output",
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
						"sortBy": map[string]any{
							"type":        "string",
							"description": "Sort by field: cpu.util, mem.util, cpu.request, mem.request, cpu.limit, mem.limit, cpu.util.percentage, mem.util.percentage, pod.count, name",
							"enum":        []string{"", "cpu.util", "mem.util", "cpu.request", "mem.request", "cpu.limit", "mem.limit", "cpu.util.percentage", "mem.util.percentage", "cpu.request.percentage", "mem.request.percentage", "cpu.limit.percentage", "mem.limit.percentage", "pod.count", "name"},
							"default":     "",
						},
						"format": map[string]any{
							"type":        "string",
							"description": "Output format: table, json, yaml",
							"enum":        []string{"table", "json", "yaml"},
							"default":     "table",
						},
					},
				},
			},
			Annotations: toolset.ToolAnnotations{
				ReadOnlyHint: paramutil.BoolPtr(true),
			},
			Handler: capacityHandler,
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
					ReadOnlyHint: paramutil.BoolPtr(false),
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
					ReadOnlyHint: paramutil.BoolPtr(false),
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
					ReadOnlyHint:    paramutil.BoolPtr(false),
					DestructiveHint: paramutil.BoolPtr(true),
				},
				Handler: deleteHandler,
			})
		}
	}

	return tools
}
