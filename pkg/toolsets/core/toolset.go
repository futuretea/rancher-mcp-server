package core

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/futuretea/rancher-mcp-server/pkg/api"
	"github.com/futuretea/rancher-mcp-server/pkg/logging"
	"github.com/futuretea/rancher-mcp-server/pkg/output"
	"github.com/futuretea/rancher-mcp-server/pkg/rancher"
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
				ReadOnlyHint: boolPtr(true),
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
				ReadOnlyHint: boolPtr(true),
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
				ReadOnlyHint: boolPtr(true),
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
				ReadOnlyHint: boolPtr(true),
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
				ReadOnlyHint: boolPtr(true),
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
				ReadOnlyHint: boolPtr(true),
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
				ReadOnlyHint: boolPtr(true),
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
				ReadOnlyHint: boolPtr(true),
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
				ReadOnlyHint: boolPtr(true),
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
				ReadOnlyHint: boolPtr(true),
			},
			Handler: serviceListHandler,
		},
	}
}

// nodeGetHandler handles the node_get tool for single node queries
func nodeGetHandler(client interface{}, params map[string]interface{}) (string, error) {
	// Extract required parameters
	clusterID := ""
	if clusterParam, ok := params["cluster"].(string); ok {
		clusterID = clusterParam
	}
	if clusterID == "" {
		return "", fmt.Errorf("cluster parameter is required")
	}

	nodeID := ""
	if nodeParam, ok := params["node"].(string); ok {
		nodeID = nodeParam
	}
	if nodeID == "" {
		return "", fmt.Errorf("node parameter is required")
	}

	format := "json"
	if formatParam, ok := params["format"].(string); ok {
		format = formatParam
	}

	rancherClient, ok := client.(*rancher.Client)
	if !ok || rancherClient == nil || !rancherClient.IsConfigured() {
		return "", fmt.Errorf("Rancher client not configured. Please configure Rancher credentials to use this tool")
	}

	ctx := context.Background()

	// Get the node
	node, err := rancherClient.GetNode(ctx, clusterID, nodeID)
	if err != nil {
		return "", fmt.Errorf("failed to get node: %v", err)
	}

	// Build the result
	result := map[string]interface{}{
		"id":                node.ID,
		"name":              node.Name,
		"state":             node.State,
		"created":           formatTime(node.Created),
		"hostname":          node.Hostname,
		"externalIpAddress": node.ExternalIPAddress,
	}

	// Add kubelet version if available
	if node.Info != nil && node.Info.Kubernetes != nil {
		result["kubeletVersion"] = node.Info.Kubernetes.KubeletVersion
	}

	// Add resource information if available
	if node.Allocatable != nil {
		result["allocatable"] = node.Allocatable
	}
	if node.Capacity != nil {
		result["capacity"] = node.Capacity
	}

	return formatNodeResult(result, format)
}

// formatNodeResult formats a single node result
func formatNodeResult(result map[string]interface{}, format string) (string, error) {
	switch format {
	case "yaml":
		return formatAsYAML(result), nil
	case "json":
		return formatAsJSON(result), nil
	default:
		data := []map[string]string{}
		row := map[string]string{
			"id":       getStringValue(result["id"]),
			"name":     getStringValue(result["name"]),
			"state":    getStringValue(result["state"]),
			"hostname": getStringValue(result["hostname"]),
			"version":  getStringValue(result["kubeletVersion"]),
			"created":  getStringValue(result["created"]),
		}
		data = append(data, row)
		return formatAsTable(data, []string{"id", "name", "state", "hostname", "version", "created"}), nil
	}
}

// nodeListHandler handles the node_list tool
func nodeListHandler(client interface{}, params map[string]interface{}) (string, error) {
	clusterID := ""
	if clusterParam, ok := params["cluster"].(string); ok {
		clusterID = clusterParam
	}

	format := "json"
	if formatParam, ok := params["format"].(string); ok {
		format = formatParam
	}

	// Try to use real Rancher client if available
	if rancherClient, ok := client.(*rancher.Client); ok && rancherClient != nil && rancherClient.IsConfigured() {
		ctx := context.Background()

		// Collect nodes from all clusters or specific cluster
		allNodes := make([]rancher.Node, 0)

		if clusterID != "" {
			// Query specific cluster only
			nodes, err := rancherClient.ListNodes(ctx, clusterID)
			if err != nil {
				return "", fmt.Errorf("failed to list nodes for cluster %s: %v", clusterID, err)
			}
			allNodes = append(allNodes, nodes...)
		} else {
			// Query all clusters
			clusters, err := rancherClient.ListClusters(ctx)
			if err != nil {
				return "", fmt.Errorf("failed to list clusters: %v", err)
			}

			for _, c := range clusters {
				nodes, err := rancherClient.ListNodes(ctx, c.ID)
				if err != nil {
					// Skip clusters that fail to list nodes
					continue
				}
				allNodes = append(allNodes, nodes...)
			}
		}

		// Convert to map format for consistent output
		nodeMaps := make([]map[string]string, len(allNodes))
		for i, node := range allNodes {
			roles := []string{}
			if node.ControlPlane {
				roles = append(roles, "control-plane")
			}
			if node.Etcd {
				roles = append(roles, "etcd")
			}
			if node.Worker {
				roles = append(roles, "worker")
			}
			rolesStr := ""
			if len(roles) > 0 {
				rolesStr = fmt.Sprintf("%v", roles)
			}

			nodeMaps[i] = map[string]string{
				"id":       node.ID,
				"name":     node.Name,
				"state":    string(node.State),
				"cluster":  node.ClusterID,
				"hostname": node.Hostname,
				"ip":       node.IPAddress,
				"roles":    rolesStr,
			}
		}

		switch format {
		case "yaml":
			return formatAsYAML(nodeMaps), nil
		case "json":
			return formatAsJSON(nodeMaps), nil
		default:
			return formatAsTable(nodeMaps, []string{"id", "name", "state", "cluster", "hostname", "ip", "roles"}), nil
		}
	}

	// No Rancher client available
	return "", fmt.Errorf("Rancher client not configured. Please configure Rancher credentials to use this tool")
}

// workloadGetHandler handles the workload_get tool for single workload queries
func workloadGetHandler(client interface{}, params map[string]interface{}) (string, error) {
	// Extract required parameters
	clusterID := ""
	if clusterParam, ok := params["cluster"].(string); ok {
		clusterID = clusterParam
	}
	if clusterID == "" {
		return "", fmt.Errorf("cluster parameter is required")
	}

	namespace := ""
	if namespaceParam, ok := params["namespace"].(string); ok {
		namespace = namespaceParam
	}
	if namespace == "" {
		return "", fmt.Errorf("namespace parameter is required")
	}

	name := ""
	if nameParam, ok := params["name"].(string); ok {
		name = nameParam
	}
	if name == "" {
		return "", fmt.Errorf("name parameter is required")
	}

	// Extract optional parameters
	projectID := ""
	if projectParam, ok := params["project"].(string); ok {
		projectID = projectParam
	}

	format := "json"
	if formatParam, ok := params["format"].(string); ok {
		format = formatParam
	}

	rancherClient, ok := client.(*rancher.Client)
	if !ok || rancherClient == nil || !rancherClient.IsConfigured() {
		return "", fmt.Errorf("Rancher client not configured. Please configure Rancher credentials to use this tool")
	}

	ctx := context.Background()

	// If project ID is not provided, auto-detect it
	if projectID == "" {
		// Use optimized label-based auto-detection
		autoDetectedProjectID, err := rancherClient.AutoDetectProjectID(ctx, clusterID, namespace)
		if err != nil {
			return "", fmt.Errorf("failed to auto-detect project for namespace %s: %v", namespace, err)
		}
		projectID = autoDetectedProjectID
	}

	// Get the workload
	workload, err := rancherClient.GetWorkload(ctx, clusterID, projectID, namespace, name)
	if err != nil {
		return "", fmt.Errorf("failed to get workload: %v", err)
	}

	// Build the result
	result := map[string]interface{}{
		"id":        workload.ID,
		"name":      workload.Name,
		"namespace": workload.NamespaceId,
		"type":      workload.Type,
		"state":     workload.State,
		"created":   formatTime(workload.Created),
	}

	// Add scale information if available
	if workload.Scale != nil {
		result["scale"] = workload.Scale
	}

	return formatWorkloadResult(result, format)
}

// formatWorkloadResult formats a single workload result
func formatWorkloadResult(result map[string]interface{}, format string) (string, error) {
	switch format {
	case "yaml":
		return formatAsYAML(result), nil
	case "json":
		return formatAsJSON(result), nil
	default:
		data := []map[string]string{}
		row := map[string]string{
			"id":        getStringValue(result["id"]),
			"name":      getStringValue(result["name"]),
			"namespace": getStringValue(result["namespace"]),
			"type":      getStringValue(result["type"]),
			"state":     getStringValue(result["state"]),
			"created":   getStringValue(result["created"]),
		}
		data = append(data, row)
		return formatAsTable(data, []string{"id", "name", "namespace", "type", "state", "created"}), nil
	}
}

// workloadListHandler handles the workload_list tool - shows workloads and orphan pods like Rancher CLI
func workloadListHandler(client interface{}, params map[string]interface{}) (string, error) {
	clusterID := ""
	if clusterParam, ok := params["cluster"].(string); ok {
		clusterID = clusterParam
	}

	if clusterID == "" {
		return "", fmt.Errorf("cluster parameter is required")
	}

	projectID := ""
	if projectParam, ok := params["project"].(string); ok {
		projectID = projectParam
	}

	namespaceName := ""
	if namespaceParam, ok := params["namespace"].(string); ok {
		namespaceName = namespaceParam
	}

	nodeName := ""
	if nodeParam, ok := params["node"].(string); ok {
		nodeName = nodeParam
	}

	format := "json"
	if formatParam, ok := params["format"].(string); ok {
		format = formatParam
	}

	rancherClient, ok := client.(*rancher.Client)
	if !ok || rancherClient == nil || !rancherClient.IsConfigured() {
		return "", fmt.Errorf("Rancher client not configured. Please configure Rancher credentials to use this tool")
	}

	ctx := context.Background()

	// Get namespace mapping (ID to name) for filtering
	namespaceMapping := make(map[string]string)
	if namespaceName != "" {
		namespaces, err := rancherClient.ListNamespaces(ctx, clusterID)
		if err != nil {
			return "", fmt.Errorf("failed to list namespaces for cluster %s: %v", clusterID, err)
		}
		for _, ns := range namespaces {
			namespaceMapping[ns.ID] = ns.Name
		}
	}

	// Collect workloads and orphan pods
	resultMaps := make([]map[string]string, 0)

	// If project is specified, only query that project
	if projectID != "" {
		// Get workloads for the specified project
		workloads, err := rancherClient.ListWorkloads(ctx, clusterID, projectID)
		if err != nil {
			return "", fmt.Errorf("failed to list workloads for cluster %s, project %s: %v", clusterID, projectID, err)
		}

		// Add workloads to results
		for _, workload := range workloads {
			// Apply filters
			if namespaceName != "" {
				nsName, exists := namespaceMapping[workload.NamespaceId]
				if !exists || nsName != namespaceName {
					continue
				}
			}
			if nodeName != "" && workload.NodeID != nodeName {
				continue
			}

			image := ""
			if len(workload.Containers) > 0 {
				image = workload.Containers[0].Image
			}

			scale := "-"
			if workload.Scale != nil {
				scale = fmt.Sprintf("%d", *workload.Scale)
			}

			// Title case the type
			workloadType := titleCase(workload.Type)

			// Get namespace name for display
			nsName := workload.NamespaceId
			if namespaceMapping != nil {
				if mappedName, exists := namespaceMapping[workload.NamespaceId]; exists {
					nsName = mappedName
				}
			}

			resultMaps = append(resultMaps, map[string]string{
				"namespace": nsName,
				"name":      workload.Name,
				"type":      workloadType,
				"state":     workload.State,
				"image":     image,
				"scale":     scale,
			})
		}

		// Get orphan pods for the specified project
		orphanPods, err := rancherClient.ListOrphanPods(ctx, clusterID, projectID)
		if err != nil {
			return "", fmt.Errorf("failed to list orphan pods for cluster %s, project %s: %v", clusterID, projectID, err)
		}

		// Add orphan pods to results
		for _, pod := range orphanPods {
			// Apply filters
			if namespaceName != "" {
				nsName, exists := namespaceMapping[pod.NamespaceId]
				if !exists || nsName != namespaceName {
					continue
				}
			}
			if nodeName != "" && pod.NodeID != nodeName {
				continue
			}

			image := ""
			if len(pod.Containers) > 0 {
				image = pod.Containers[0].Image
			}

			// Title case the type
			podType := titleCase(pod.Type)

			// Get namespace name for display
			nsName := pod.NamespaceId
			if namespaceMapping != nil {
				if mappedName, exists := namespaceMapping[pod.NamespaceId]; exists {
					nsName = mappedName
				}
			}

			resultMaps = append(resultMaps, map[string]string{
				"namespace": nsName,
				"name":      pod.Name,
				"type":      podType,
				"state":     pod.State,
				"image":     image,
				"scale":     "Standalone",
			})
		}
	} else {
		// Get all projects for the cluster
		projects, err := rancherClient.ListProjects(ctx, clusterID)
		if err != nil {
			return "", fmt.Errorf("failed to list projects for cluster %s: %v", clusterID, err)
		}

		// Collect workloads and orphan pods from each project
		for _, project := range projects {
			// Get workloads for this project
			workloads, err := rancherClient.ListWorkloads(ctx, clusterID, project.ID)
			if err != nil {
				// Skip projects that fail
				continue
			}

			// Add workloads to results
			for _, workload := range workloads {
				// Apply filters
				if namespaceName != "" {
					nsName, exists := namespaceMapping[workload.NamespaceId]
					if !exists || nsName != namespaceName {
						continue
					}
				}
				if nodeName != "" && workload.NodeID != nodeName {
					continue
				}

				image := ""
				if len(workload.Containers) > 0 {
					image = workload.Containers[0].Image
				}

				scale := "-"
				if workload.Scale != nil {
					scale = fmt.Sprintf("%d", *workload.Scale)
				}

				// Title case the type
				workloadType := titleCase(workload.Type)

				// Get namespace name for display
				nsName := workload.NamespaceId
				if namespaceMapping != nil {
					if mappedName, exists := namespaceMapping[workload.NamespaceId]; exists {
						nsName = mappedName
					}
				}

				resultMaps = append(resultMaps, map[string]string{
					"namespace": nsName,
					"name":      workload.Name,
					"type":      workloadType,
					"state":     workload.State,
					"image":     image,
					"scale":     scale,
				})
			}

			// Get orphan pods (pods without workloads) for this project
			orphanPods, err := rancherClient.ListOrphanPods(ctx, clusterID, project.ID)
			if err != nil {
				// Skip if orphan pods fail
				continue
			}

			// Add orphan pods to results
			for _, pod := range orphanPods {
				// Apply filters
				if namespaceName != "" {
					nsName, exists := namespaceMapping[pod.NamespaceId]
					if !exists || nsName != namespaceName {
						continue
					}
				}
				if nodeName != "" && pod.NodeID != nodeName {
					continue
				}

				image := ""
				if len(pod.Containers) > 0 {
					image = pod.Containers[0].Image
				}

				// Title case the type
				podType := titleCase(pod.Type)

				// Get namespace name for display
				nsName := pod.NamespaceId
				if namespaceMapping != nil {
					if mappedName, exists := namespaceMapping[pod.NamespaceId]; exists {
						nsName = mappedName
					}
				}

				resultMaps = append(resultMaps, map[string]string{
					"namespace": nsName,
					"name":      pod.Name,
					"type":      podType,
					"state":     pod.State,
					"image":     image,
					"scale":     "Standalone",
				})
			}
		}
	}

	if len(resultMaps) == 0 {
		return "No workloads found", nil
	}

	switch format {
	case "yaml":
		return formatAsYAML(resultMaps), nil
	case "json":
		return formatAsJSON(resultMaps), nil
	default:
		return formatAsTable(resultMaps, []string{"namespace", "name", "type", "state", "image", "scale"}), nil
	}
}

// namespaceGetHandler handles the namespace_get tool for single namespace queries
func namespaceGetHandler(client interface{}, params map[string]interface{}) (string, error) {
	// Extract required parameters
	clusterID := ""
	if clusterParam, ok := params["cluster"].(string); ok {
		clusterID = clusterParam
	}
	if clusterID == "" {
		return "", fmt.Errorf("cluster parameter is required")
	}

	name := ""
	if nameParam, ok := params["name"].(string); ok {
		name = nameParam
	}
	if name == "" {
		return "", fmt.Errorf("name parameter is required")
	}

	format := "json"
	if formatParam, ok := params["format"].(string); ok {
		format = formatParam
	}

	rancherClient, ok := client.(*rancher.Client)
	if !ok || rancherClient == nil || !rancherClient.IsConfigured() {
		return "", fmt.Errorf("Rancher client not configured. Please configure Rancher credentials to use this tool")
	}

	ctx := context.Background()

	// Get the namespace
	namespace, err := rancherClient.GetNamespace(ctx, clusterID, name)
	if err != nil {
		return "", fmt.Errorf("failed to get namespace: %v", err)
	}

	// Build the result
	result := map[string]interface{}{
		"id":             namespace.ID,
		"name":           namespace.Name,
		"state":          namespace.State,
		"created":        formatTime(namespace.Created),
		"resourceQuota":  namespace.ResourceQuota,
		"containerLimit": namespace.ContainerDefaultResourceLimit,
	}

	// Add project ID if available
	if namespace.ProjectID != "" {
		result["projectId"] = namespace.ProjectID
	}

	return formatNamespaceResult(result, format)
}

// formatNamespaceResult formats a single namespace result
func formatNamespaceResult(result map[string]interface{}, format string) (string, error) {
	switch format {
	case "yaml":
		return formatAsYAML(result), nil
	case "json":
		return formatAsJSON(result), nil
	default:
		data := []map[string]string{}
		row := map[string]string{
			"id":        getStringValue(result["id"]),
			"name":      getStringValue(result["name"]),
			"state":     getStringValue(result["state"]),
			"projectId": getStringValue(result["projectId"]),
			"created":   getStringValue(result["created"]),
		}
		data = append(data, row)
		return formatAsTable(data, []string{"id", "name", "state", "projectId", "created"}), nil
	}
}

// namespaceListHandler handles the namespace_list tool
func namespaceListHandler(client interface{}, params map[string]interface{}) (string, error) {
	clusterID := ""
	if clusterParam, ok := params["cluster"].(string); ok {
		clusterID = clusterParam
	}

	if clusterID == "" {
		return "", fmt.Errorf("cluster parameter is required")
	}

	projectID := ""
	if projectParam, ok := params["project"].(string); ok {
		projectID = projectParam
	}

	format := "json"
	if formatParam, ok := params["format"].(string); ok {
		format = formatParam
	}

	rancherClient, ok := client.(*rancher.Client)
	if !ok || rancherClient == nil || !rancherClient.IsConfigured() {
		return "", fmt.Errorf("Rancher client not configured. Please configure Rancher credentials to use this tool")
	}

	ctx := context.Background()

	// Get namespaces for the cluster
	namespaces, err := rancherClient.ListNamespaces(ctx, clusterID)
	if err != nil {
		return "", fmt.Errorf("failed to list namespaces for cluster %s: %v", clusterID, err)
	}

	// Format and return namespaces with richer information
	namespaceMaps := make([]map[string]string, 0)
	for _, ns := range namespaces {
		// Filter by project ID if specified
		if projectID != "" && ns.ProjectID != projectID {
			continue
		}

		namespaceMaps = append(namespaceMaps, map[string]string{
			"id":          ns.ID,
			"name":        ns.Name,
			"state":       ns.State,
			"cluster":     clusterID,
			"project":     ns.ProjectID,
			"description": ns.Description,
			"created":     formatTime(ns.Created),
		})
	}

	switch format {
	case "yaml":
		return formatAsYAML(namespaceMaps), nil
	case "json":
		return formatAsJSON(namespaceMaps), nil
	default:
		return formatAsTable(namespaceMaps, []string{"id", "name", "state", "project", "description", "created"}), nil
	}
}

// Helper functions for formatting
func formatAsTable(data []map[string]string, headers []string) string {
	formatter := output.NewFormatter()
	return formatter.FormatTableWithHeaders(data, headers)
}

func formatAsYAML(data interface{}) string {
	formatter := output.NewFormatter()
	result, err := formatter.FormatYAML(data)
	if err != nil {
		return fmt.Sprintf("Error formatting YAML: %v", err)
	}
	return result
}

func formatAsJSON(data interface{}) string {
	formatter := output.NewFormatter()
	result, err := formatter.FormatJSON(data)
	if err != nil {
		return fmt.Sprintf("Error formatting JSON: %v", err)
	}
	return result
}

func boolPtr(b bool) *bool {
	return &b
}

// titleCase converts a string to title case using the modern cases package
func titleCase(s string) string {
	caser := cases.Title(language.English)
	return caser.String(strings.ToLower(s))
}

// formatTime formats time for display
func formatTime(timestamp string) string {
	if timestamp == "" {
		return "-"
	}
	// For now, just return the timestamp as-is
	// In a real implementation, you might want to parse and format it
	return timestamp
}

// configMapGetHandler handles the configmap_get tool for single configmap queries
func configMapGetHandler(client interface{}, params map[string]interface{}) (string, error) {
	// Extract required parameters
	clusterID := ""
	if clusterParam, ok := params["cluster"].(string); ok {
		clusterID = clusterParam
	}
	if clusterID == "" {
		return "", fmt.Errorf("cluster parameter is required")
	}

	namespace := ""
	if namespaceParam, ok := params["namespace"].(string); ok {
		namespace = namespaceParam
	}
	if namespace == "" {
		return "", fmt.Errorf("namespace parameter is required")
	}

	name := ""
	if nameParam, ok := params["name"].(string); ok {
		name = nameParam
	}
	if name == "" {
		return "", fmt.Errorf("name parameter is required")
	}

	// Extract optional parameters
	projectID := ""
	if projectParam, ok := params["project"].(string); ok {
		projectID = projectParam
	}

	format := "json"
	if formatParam, ok := params["format"].(string); ok {
		format = formatParam
	}

	rancherClient, ok := client.(*rancher.Client)
	if !ok || rancherClient == nil || !rancherClient.IsConfigured() {
		return "", fmt.Errorf("Rancher client not configured. Please configure Rancher credentials to use this tool")
	}

	ctx := context.Background()

	// If project ID is not provided, auto-detect it
	if projectID == "" {
		// Use optimized label-based auto-detection
		autoDetectedProjectID, err := rancherClient.AutoDetectProjectID(ctx, clusterID, namespace)
		if err != nil {
			return "", fmt.Errorf("failed to auto-detect project for namespace %s: %v", namespace, err)
		}
		projectID = autoDetectedProjectID
	}

	// Get the configmap
	configMap, err := rancherClient.GetConfigMap(ctx, clusterID, projectID, namespace, name)
	if err != nil {
		return "", fmt.Errorf("failed to get configmap: %v", err)
	}

	// Build the result
	result := map[string]interface{}{
		"id":        configMap.ID,
		"name":      configMap.Name,
		"namespace": configMap.NamespaceId,
		"created":   formatTime(configMap.Created),
	}

	// Add data if available (for ConfigMaps, we include the data for inspection)
	if configMap.Data != nil {
		result["data"] = configMap.Data
	}

	return formatConfigMapResult(result, format)
}

// formatConfigMapResult formats a single configmap result
func formatConfigMapResult(result map[string]interface{}, format string) (string, error) {
	switch format {
	case "yaml":
		return formatAsYAML(result), nil
	case "json":
		return formatAsJSON(result), nil
	default:
		data := []map[string]string{}
		row := map[string]string{
			"id":        getStringValue(result["id"]),
			"name":      getStringValue(result["name"]),
			"namespace": getStringValue(result["namespace"]),
			"created":   getStringValue(result["created"]),
		}
		data = append(data, row)
		return formatAsTable(data, []string{"id", "name", "namespace", "created"}), nil
	}
}

// configMapListHandler handles the configmap_list tool
func configMapListHandler(client interface{}, params map[string]interface{}) (string, error) {
	clusterID := ""
	if clusterParam, ok := params["cluster"].(string); ok {
		clusterID = clusterParam
	}

	if clusterID == "" {
		return "", fmt.Errorf("cluster parameter is required")
	}

	projectID := ""
	if projectParam, ok := params["project"].(string); ok {
		projectID = projectParam
	}

	namespaceName := ""
	if namespaceParam, ok := params["namespace"].(string); ok {
		namespaceName = namespaceParam
	}

	format := "json"
	if formatParam, ok := params["format"].(string); ok {
		format = formatParam
	}

	rancherClient, ok := client.(*rancher.Client)
	if !ok || rancherClient == nil || !rancherClient.IsConfigured() {
		return "", fmt.Errorf("Rancher client not configured. Please configure Rancher credentials to use this tool")
	}

	ctx := context.Background()

	// Collect configmaps from all projects or specific project
	resultMaps := make([]map[string]string, 0)

	if projectID != "" {
		// Get configmaps for the specified project
		configMaps, err := rancherClient.ListConfigMaps(ctx, clusterID, projectID)
		if err != nil {
			return "", fmt.Errorf("failed to list configmaps for cluster %s, project %s: %v", clusterID, projectID, err)
		}

		for _, cm := range configMaps {
			// Apply namespace filter
			if namespaceName != "" && cm.NamespaceId != namespaceName {
				continue
			}

			dataKeys := "-"
			if len(cm.Data) > 0 {
				keys := make([]string, 0, len(cm.Data))
				for k := range cm.Data {
					keys = append(keys, k)
				}
				dataKeys = strings.Join(keys, ",")
			}

			resultMaps = append(resultMaps, map[string]string{
				"id":        cm.ID,
				"name":      cm.Name,
				"namespace": cm.NamespaceId,
				"keys":      dataKeys,
				"created":   formatTime(cm.Created),
			})
		}
	} else {
		// Get all projects for the cluster
		projects, err := rancherClient.ListProjects(ctx, clusterID)
		if err != nil {
			return "", fmt.Errorf("failed to list projects for cluster %s: %v", clusterID, err)
		}

		// Collect configmaps from each project
		for _, project := range projects {
			configMaps, err := rancherClient.ListConfigMaps(ctx, clusterID, project.ID)
			if err != nil {
				// Skip projects that fail
				continue
			}

			for _, cm := range configMaps {
				// Apply namespace filter
				if namespaceName != "" && cm.NamespaceId != namespaceName {
					continue
				}

				dataKeys := "-"
				if len(cm.Data) > 0 {
					keys := make([]string, 0, len(cm.Data))
					for k := range cm.Data {
						keys = append(keys, k)
					}
					dataKeys = strings.Join(keys, ",")
				}

				resultMaps = append(resultMaps, map[string]string{
					"id":        cm.ID,
					"name":      cm.Name,
					"namespace": cm.NamespaceId,
					"keys":      dataKeys,
					"created":   formatTime(cm.Created),
				})
			}
		}
	}

	if len(resultMaps) == 0 {
		return "No configmaps found", nil
	}

	switch format {
	case "yaml":
		return formatAsYAML(resultMaps), nil
	case "json":
		return formatAsJSON(resultMaps), nil
	default:
		return formatAsTable(resultMaps, []string{"id", "name", "namespace", "keys", "created"}), nil
	}
}

// secretGetHandler handles the secret_get tool for single secret queries
// Note: This handler ONLY returns metadata (id, name, namespace, type, created)
// and does NOT expose sensitive secret data (Data or StringData fields)
func secretGetHandler(client interface{}, params map[string]interface{}) (string, error) {
	// Extract required parameters
	clusterID := ""
	if clusterParam, ok := params["cluster"].(string); ok {
		clusterID = clusterParam
	}
	if clusterID == "" {
		return "", fmt.Errorf("cluster parameter is required")
	}

	namespace := ""
	if namespaceParam, ok := params["namespace"].(string); ok {
		namespace = namespaceParam
	}
	if namespace == "" {
		return "", fmt.Errorf("namespace parameter is required")
	}

	name := ""
	if nameParam, ok := params["name"].(string); ok {
		name = nameParam
	}
	if name == "" {
		return "", fmt.Errorf("name parameter is required")
	}

	// Extract optional parameters
	projectID := ""
	if projectParam, ok := params["project"].(string); ok {
		projectID = projectParam
	}

	format := "json"
	if formatParam, ok := params["format"].(string); ok {
		format = formatParam
	}

	rancherClient, ok := client.(*rancher.Client)
	if !ok || rancherClient == nil || !rancherClient.IsConfigured() {
		return "", fmt.Errorf("Rancher client not configured. Please configure Rancher credentials to use this tool")
	}

	ctx := context.Background()

	// If project ID is not provided, auto-detect it
	if projectID == "" {
		// Use optimized label-based auto-detection
		autoDetectedProjectID, err := rancherClient.AutoDetectProjectID(ctx, clusterID, namespace)
		if err != nil {
			return "", fmt.Errorf("failed to auto-detect project for namespace %s: %v", namespace, err)
		}
		projectID = autoDetectedProjectID
	}

	// Get the secret
	secret, err := rancherClient.GetSecret(ctx, clusterID, projectID, namespace, name)
	if err != nil {
		return "", fmt.Errorf("failed to get secret: %v", err)
	}

	// Build the result - ONLY metadata, no sensitive data
	result := map[string]interface{}{
		"id":        secret.ID,
		"name":      secret.Name,
		"namespace": secret.NamespaceId,
		"type":      secret.Type,
		"created":   formatTime(secret.Created),
	}

	return formatSecretResult(result, format)
}

// formatSecretResult formats a single secret result
func formatSecretResult(result map[string]interface{}, format string) (string, error) {
	switch format {
	case "yaml":
		return formatAsYAML(result), nil
	case "json":
		return formatAsJSON(result), nil
	default:
		data := []map[string]string{}
		row := map[string]string{
			"id":        getStringValue(result["id"]),
			"name":      getStringValue(result["name"]),
			"namespace": getStringValue(result["namespace"]),
			"type":      getStringValue(result["type"]),
			"created":   getStringValue(result["created"]),
		}
		data = append(data, row)
		return formatAsTable(data, []string{"id", "name", "namespace", "type", "created"}), nil
	}
}

// secretListHandler handles the secret_list tool
// Note: This handler ONLY returns metadata (id, name, namespace, type, created)
// and does NOT expose sensitive secret data (Data or StringData fields)
func secretListHandler(client interface{}, params map[string]interface{}) (string, error) {
	clusterID := ""
	if clusterParam, ok := params["cluster"].(string); ok {
		clusterID = clusterParam
	}

	if clusterID == "" {
		return "", fmt.Errorf("cluster parameter is required")
	}

	projectID := ""
	if projectParam, ok := params["project"].(string); ok {
		projectID = projectParam
	}

	namespaceName := ""
	if namespaceParam, ok := params["namespace"].(string); ok {
		namespaceName = namespaceParam
	}

	format := "json"
	if formatParam, ok := params["format"].(string); ok {
		format = formatParam
	}

	rancherClient, ok := client.(*rancher.Client)
	if !ok || rancherClient == nil || !rancherClient.IsConfigured() {
		return "", fmt.Errorf("Rancher client not configured. Please configure Rancher credentials to use this tool")
	}

	ctx := context.Background()

	// Collect secrets from all projects or specific project
	resultMaps := make([]map[string]string, 0)

	if projectID != "" {
		// Get secrets for the specified project
		secrets, err := rancherClient.ListSecrets(ctx, clusterID, projectID)
		if err != nil {
			return "", fmt.Errorf("failed to list secrets for cluster %s, project %s: %v", clusterID, projectID, err)
		}

		for _, secret := range secrets {
			// Apply namespace filter
			if namespaceName != "" && secret.NamespaceId != namespaceName {
				continue
			}

			secretType := secret.Type
			if secretType == "" {
				secretType = "Opaque"
			}

			// Only include metadata, never include secret.Data or secret.StringData
			resultMaps = append(resultMaps, map[string]string{
				"id":        secret.ID,
				"name":      secret.Name,
				"namespace": secret.NamespaceId,
				"type":      secretType,
				"created":   formatTime(secret.Created),
			})
		}
	} else {
		// Get all projects for the cluster
		projects, err := rancherClient.ListProjects(ctx, clusterID)
		if err != nil {
			return "", fmt.Errorf("failed to list projects for cluster %s: %v", clusterID, err)
		}

		// Collect secrets from each project
		for _, project := range projects {
			secrets, err := rancherClient.ListSecrets(ctx, clusterID, project.ID)
			if err != nil {
				// Skip projects that fail
				continue
			}

			for _, secret := range secrets {
				// Apply namespace filter
				if namespaceName != "" && secret.NamespaceId != namespaceName {
					continue
				}

				secretType := secret.Type
				if secretType == "" {
					secretType = "Opaque"
				}

				// Only include metadata, never include secret.Data or secret.StringData
				resultMaps = append(resultMaps, map[string]string{
					"id":        secret.ID,
					"name":      secret.Name,
					"namespace": secret.NamespaceId,
					"type":      secretType,
					"created":   formatTime(secret.Created),
				})
			}
		}
	}

	if len(resultMaps) == 0 {
		return "No secrets found", nil
	}

	switch format {
	case "yaml":
		return formatAsYAML(resultMaps), nil
	case "json":
		return formatAsJSON(resultMaps), nil
	default:
		return formatAsTable(resultMaps, []string{"id", "name", "namespace", "type", "created"}), nil
	}
}

// serviceGetHandler handles the service_get tool for single service queries
func serviceGetHandler(client interface{}, params map[string]interface{}) (string, error) {
	// Extract required parameters
	clusterID := ""
	if clusterParam, ok := params["cluster"].(string); ok {
		clusterID = clusterParam
	}
	if clusterID == "" {
		return "", fmt.Errorf("cluster parameter is required")
	}

	namespace := ""
	if namespaceParam, ok := params["namespace"].(string); ok {
		namespace = namespaceParam
	}
	if namespace == "" {
		return "", fmt.Errorf("namespace parameter is required")
	}

	name := ""
	if nameParam, ok := params["name"].(string); ok {
		name = nameParam
	}
	if name == "" {
		return "", fmt.Errorf("name parameter is required")
	}

	// Extract optional parameters
	projectID := ""
	if projectParam, ok := params["project"].(string); ok {
		projectID = projectParam
	}

	getPodDetails := false
	if detailsParam, ok := params["getPodDetails"].(bool); ok {
		getPodDetails = detailsParam
	}

	format := "json"
	if formatParam, ok := params["format"].(string); ok {
		format = formatParam
	}

	rancherClient, ok := client.(*rancher.Client)
	if !ok || rancherClient == nil || !rancherClient.IsConfigured() {
		return "", fmt.Errorf("Rancher client not configured. Please configure Rancher credentials to use this tool")
	}

	ctx := context.Background()

	// If project ID is not provided, auto-detect it
	if projectID == "" {
		// Use optimized label-based auto-detection
		autoDetectedProjectID, err := rancherClient.AutoDetectProjectID(ctx, clusterID, namespace)
		if err != nil {
			return "", fmt.Errorf("failed to auto-detect project for namespace %s: %v", namespace, err)
		}
		projectID = autoDetectedProjectID
	}

	// Get the service
	service, err := rancherClient.GetService(ctx, clusterID, projectID, namespace, name)
	if err != nil {
		return "", fmt.Errorf("failed to get service: %v", err)
	}

	// Build the result
	result := map[string]interface{}{
		"id":              service.ID,
		"name":            service.Name,
		"namespace":       service.NamespaceId,
		"state":           service.State,
		"created":         formatTime(service.Created),
		"type":            service.Type,
		"clusterIp":       service.ClusterIp,
		"sessionAffinity": service.SessionAffinity,
	}

	if service.Selector != nil {
		result["selector"] = service.Selector
	}

	if len(service.PublicEndpoints) > 0 {
		endpoints := []map[string]interface{}{}
		for _, ep := range service.PublicEndpoints {
			endpointInfo := map[string]interface{}{
				"addresses": ep.Addresses,
				"port":      ep.Port,
				"protocol":  ep.Protocol,
			}
			endpoints = append(endpoints, endpointInfo)
		}
		result["endpoints"] = endpoints
	}

	// Perform diagnostic if requested
	if getPodDetails {
		// Fetch pods for this project namespace
		podList, err := rancherClient.ListPods(ctx, clusterID, projectID)
		if err != nil {
			logging.Warn("Failed to list pods: %v", err)
		}

		status := diagnoseService(ctx, rancherClient, clusterID, projectID, *service, podList)
		result["status"] = status
	}

	return formatSingleServiceResult(result, format)
}

// formatSingleServiceResult formats a single service result
func formatSingleServiceResult(result map[string]interface{}, format string) (string, error) {
	switch format {
	case "yaml":
		return formatAsYAML(result), nil
	case "json":
		return formatAsJSON(result), nil
	default:
		// For table format (though not recommended for single objects), we still support it
		data := []map[string]string{}
		row := map[string]string{
			"id":          getStringValue(result["id"]),
			"name":        getStringValue(result["name"]),
			"namespace":   getStringValue(result["namespace"]),
			"type":        getStringValue(result["type"]),
			"clusterIp":   getStringValue(result["clusterIp"]),
			"created":     getStringValue(result["created"]),
		}

		if status, exists := result["status"]; exists {
			if diagStatus, ok := status.(ServiceDiagnosticStatus); ok {
				if diagStatus.Ready {
					row["ready"] = "Yes"
				} else {
					row["ready"] = "No"
				}

				if diagStatus.Degraded {
					row["degraded"] = "Yes"
				} else {
					row["degraded"] = "No"
				}
			}
		} else {
			row["ready"] = "-"
			row["degraded"] = "-"
		}

		data = append(data, row)
		return formatAsTable(data, []string{"id", "name", "namespace", "type", "clusterIp", "ready", "degraded", "created"}), nil
	}
}

// serviceListHandler handles the service_list tool with diagnostic capabilities
// It lists services across projects and optionally performs a full diagnostic chain
// check: Service → Pods
// The diagnostic chain is performed when getPodDetails=true using optimized API calls
// that fetch all pods once per project rather than for each service
func serviceListHandler(client interface{}, params map[string]interface{}) (string, error) {
	clusterID := ""
	if clusterParam, ok := params["cluster"].(string); ok {
		clusterID = clusterParam
	}

	if clusterID == "" {
		return "", fmt.Errorf("cluster parameter is required")
	}

	projectID := ""
	if projectParam, ok := params["project"].(string); ok {
		projectID = projectParam
	}

	namespaceName := ""
	if namespaceParam, ok := params["namespace"].(string); ok {
		namespaceName = namespaceParam
	}

	getPodDetails := false
	if detailsParam, ok := params["getPodDetails"].(bool); ok {
		getPodDetails = detailsParam
	}

	format := "json"
	if formatParam, ok := params["format"].(string); ok {
		format = formatParam
	}

	rancherClient, ok := client.(*rancher.Client)
	if !ok || rancherClient == nil || !rancherClient.IsConfigured() {
		return "", fmt.Errorf("Rancher client not configured. Please configure Rancher credentials to use this tool")
	}

	ctx := context.Background()

	// Pre-fetch pods for optimization when diagnostics are requested
	var podList []rancher.Pod
	var err error

	if getPodDetails {
		if projectID != "" {
			// For single project, fetch once
			podList, err = rancherClient.ListPods(ctx, clusterID, projectID)
			if err != nil {
				logging.Warn("Failed to list pods: %v", err)
			}
		}
	}

	// Collect services with diagnostic information
	serviceResults := []interface{}{}

	if projectID != "" {
		// Get services for the specified project
		results, err := getServicesWithDiagnostic(ctx, rancherClient, clusterID, projectID, namespaceName, getPodDetails, podList)
		if err != nil {
			return "", fmt.Errorf("failed to list services: %v", err)
		}
		serviceResults = results
	} else {
		// Get all projects for the cluster
		projects, err := rancherClient.ListProjects(ctx, clusterID)
		if err != nil {
			return "", fmt.Errorf("failed to list projects for cluster %s: %v", clusterID, err)
		}

		// Collect services from each project
		for _, project := range projects {
			// Fetch pods for this specific project when diagnostics enabled
			if getPodDetails {
				podList, err = rancherClient.ListPods(ctx, clusterID, project.ID)
				if err != nil {
					logging.Warn("Failed to list pods for project %s: %v", project.ID, err)
				}
			}

			results, err := getServicesWithDiagnostic(ctx, rancherClient, clusterID, project.ID, namespaceName, getPodDetails, podList)
			if err != nil {
				// Skip projects that fail
				continue
			}
			serviceResults = append(serviceResults, results...)
		}
	}

	if len(serviceResults) == 0 {
		return "No services found", nil
	}

	return formatServiceResults(serviceResults, format)
}

// getServicesWithDiagnostic lists services with diagnostic information
// When getPodDetails is true, it performs diagnostic checks using the provided podList
// to avoid redundant API calls for each service
func getServicesWithDiagnostic(ctx context.Context,
	rancherClient *rancher.Client,
	clusterID, projectID, namespaceName string,
	getPodDetails bool,
	podList []rancher.Pod) ([]interface{}, error) {

	services, err := rancherClient.ListServices(ctx, clusterID, projectID)
	if err != nil {
		return nil, err
	}

	results := []interface{}{}
	for _, svc := range services {
		// Apply namespace filter
		if namespaceName != "" && svc.NamespaceId != namespaceName {
			continue
		}

		// Build basic service info
		serviceInfo := map[string]interface{}{
			"id":          svc.ID,
			"name":        svc.Name,
			"namespace":   svc.NamespaceId,
			"state":       svc.State,
			"created":     formatTime(svc.Created),
			"annotations": svc.Annotations,
			"labels":      svc.Labels,
		}

		// Add service type
		svcType := svc.Kind
		if svcType == "" {
			svcType = "ClusterIP"
		}
		serviceInfo["type"] = svcType

		// Add cluster IP
		clusterIP := svc.ClusterIp
		if clusterIP == "" {
			clusterIP = "-"
		}
		serviceInfo["cluster_ip"] = clusterIP

		// Add ports
		if len(svc.PublicEndpoints) > 0 {
			endpoints := []map[string]interface{}{}
			for _, ep := range svc.PublicEndpoints {
				endpointInfo := map[string]interface{}{
					"addresses": ep.Addresses,
					"port":      ep.Port,
					"protocol":  ep.Protocol,
				}
				endpoints = append(endpoints, endpointInfo)
			}
			serviceInfo["endpoints"] = endpoints
		}

		// Perform diagnostic if requested
		if getPodDetails {
			diagnosticStatus := diagnoseService(ctx, rancherClient, clusterID, projectID, svc, podList)
			serviceInfo["status"] = diagnosticStatus
		}

		results = append(results, serviceInfo)
	}

	return results, nil
}

// formatServiceResults formats the service results based on output format
func formatServiceResults(results []interface{}, format string) (string, error) {
	if len(results) == 0 {
		return "No services found", nil
	}

	switch format {
	case "yaml":
		return formatAsYAML(results), nil
	case "json":
		return formatAsJSON(results), nil
	default:
		// For table format, extract basic information
		tableData := []map[string]string{}
		for _, result := range results {
			if svc, ok := result.(map[string]interface{}); ok {
				row := map[string]string{
					"id":        getStringValue(svc["id"]),
					"name":      getStringValue(svc["name"]),
					"namespace": getStringValue(svc["namespace"]),
					"type":      getStringValue(svc["type"]),
					"cluster_ip": getStringValue(svc["cluster_ip"]),
					"state":     getStringValue(svc["state"]),
					"created":   getStringValue(svc["created"]),
				}

				// Determine ready status
				if status, exists := svc["status"]; exists {
					if diagStatus, ok := status.(ServiceDiagnosticStatus); ok {
						if diagStatus.Ready {
							row["ready"] = "Yes"
						} else {
							row["ready"] = "No"
						}
					}
				} else {
					row["ready"] = "-"
				}

				tableData = append(tableData, row)
			}
		}

		return formatAsTable(tableData, []string{"id", "name", "namespace", "type", "cluster_ip", "state", "ready", "created"}), nil
	}
}

// Helper function to get string value from interface
func getStringValue(v interface{}) string {
	if str, ok := v.(string); ok {
		return str
	}
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("%v", v)
}

func init() {
	// Register this toolset
	// This will be implemented when we have the toolsets registry
}