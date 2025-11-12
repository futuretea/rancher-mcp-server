package core

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/futuretea/rancher-mcp-server/pkg/api"
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
							"description": "Output format: table, yaml, or json",
							"enum":        []string{"table", "yaml", "json"},
							"default":     "table",
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
							"description": "Output format: table, yaml, or json",
							"enum":        []string{"table", "yaml", "json"},
							"default":     "table",
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
							"description": "Output format: table, yaml, or json",
							"enum":        []string{"table", "yaml", "json"},
							"default":     "table",
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
							"description": "Output format: table, yaml, or json",
							"enum":        []string{"table", "yaml", "json"},
							"default":     "table",
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
							"description": "Output format: table, yaml, or json",
							"enum":        []string{"table", "yaml", "json"},
							"default":     "table",
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
				Name:        "service_list",
				Description: "List all services in a cluster",
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
						"format": map[string]any{
							"type":        "string",
							"description": "Output format: table, yaml, or json",
							"enum":        []string{"table", "yaml", "json"},
							"default":     "table",
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

// nodeListHandler handles the node_list tool
func nodeListHandler(client interface{}, params map[string]interface{}) (string, error) {
	clusterID := ""
	if clusterParam, ok := params["cluster"].(string); ok {
		clusterID = clusterParam
	}

	format := "table"
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

	format := "table"
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

	format := "table"
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

	format := "table"
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

	format := "table"
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

// serviceListHandler handles the service_list tool
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

	format := "table"
	if formatParam, ok := params["format"].(string); ok {
		format = formatParam
	}

	rancherClient, ok := client.(*rancher.Client)
	if !ok || rancherClient == nil || !rancherClient.IsConfigured() {
		return "", fmt.Errorf("Rancher client not configured. Please configure Rancher credentials to use this tool")
	}

	ctx := context.Background()

	// Collect services from all projects or specific project
	resultMaps := make([]map[string]string, 0)

	if projectID != "" {
		// Get services for the specified project
		services, err := rancherClient.ListServices(ctx, clusterID, projectID)
		if err != nil {
			return "", fmt.Errorf("failed to list services for cluster %s, project %s: %v", clusterID, projectID, err)
		}

		for _, svc := range services {
			// Apply namespace filter
			if namespaceName != "" && svc.NamespaceId != namespaceName {
				continue
			}

			svcType := svc.Kind
			if svcType == "" {
				svcType = "ClusterIP"
			}

			clusterIP := svc.ClusterIp
			if clusterIP == "" {
				clusterIP = "-"
			}

			ports := "-"
			if len(svc.PublicEndpoints) > 0 {
				portStrs := make([]string, 0, len(svc.PublicEndpoints))
				for _, ep := range svc.PublicEndpoints {
					portStrs = append(portStrs, fmt.Sprintf("%d", ep.Port))
				}
				ports = strings.Join(portStrs, ",")
			}

			resultMaps = append(resultMaps, map[string]string{
				"id":         svc.ID,
				"name":       svc.Name,
				"namespace":  svc.NamespaceId,
				"type":       svcType,
				"cluster_ip": clusterIP,
				"ports":      ports,
				"created":    formatTime(svc.Created),
			})
		}
	} else {
		// Get all projects for the cluster
		projects, err := rancherClient.ListProjects(ctx, clusterID)
		if err != nil {
			return "", fmt.Errorf("failed to list projects for cluster %s: %v", clusterID, err)
		}

		// Collect services from each project
		for _, project := range projects {
			services, err := rancherClient.ListServices(ctx, clusterID, project.ID)
			if err != nil {
				// Skip projects that fail
				continue
			}

			for _, svc := range services {
				// Apply namespace filter
				if namespaceName != "" && svc.NamespaceId != namespaceName {
					continue
				}

				svcType := svc.Kind
				if svcType == "" {
					svcType = "ClusterIP"
				}

				clusterIP := svc.ClusterIp
				if clusterIP == "" {
					clusterIP = "-"
				}

				ports := "-"
				if len(svc.PublicEndpoints) > 0 {
					portStrs := make([]string, 0, len(svc.PublicEndpoints))
					for _, ep := range svc.PublicEndpoints {
						portStrs = append(portStrs, fmt.Sprintf("%d", ep.Port))
					}
					ports = strings.Join(portStrs, ",")
				}

				resultMaps = append(resultMaps, map[string]string{
					"id":         svc.ID,
					"name":       svc.Name,
					"namespace":  svc.NamespaceId,
					"type":       svcType,
					"cluster_ip": clusterIP,
					"ports":      ports,
					"created":    formatTime(svc.Created),
				})
			}
		}
	}

	if len(resultMaps) == 0 {
		return "No services found", nil
	}

	switch format {
	case "yaml":
		return formatAsYAML(resultMaps), nil
	case "json":
		return formatAsJSON(resultMaps), nil
	default:
		return formatAsTable(resultMaps, []string{"id", "name", "namespace", "type", "cluster_ip", "ports", "created"}), nil
	}
}

func init() {
	// Register this toolset
	// This will be implemented when we have the toolsets registry
}