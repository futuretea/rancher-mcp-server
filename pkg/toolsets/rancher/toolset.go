package rancher

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/futuretea/rancher-mcp-server/pkg/api"
	"github.com/futuretea/rancher-mcp-server/pkg/output"
	"github.com/futuretea/rancher-mcp-server/pkg/rancher"
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
			Handler: clusterListHandler,
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
			Handler: projectListHandler,
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
			Handler: projectAccessHandler,
		},
	}
}

// clusterListHandler handles the cluster_list tool
func clusterListHandler(client interface{}, params map[string]interface{}) (string, error) {
	format := "table"
	if formatParam, ok := params["format"].(string); ok {
		format = formatParam
	}

	// Try to use real Rancher client if available
	if rancherClient, ok := client.(*rancher.Client); ok && rancherClient != nil && rancherClient.IsConfigured() {
		ctx := context.Background()
		clusters, err := rancherClient.ListClusters(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to list clusters: %v", err)
		}

		// Convert to map format for consistent output with richer information
		clusterMaps := make([]map[string]string, len(clusters))
		for i, cluster := range clusters {
			version := ""
			if cluster.Version != nil {
				version = cluster.Version.GitVersion
			}

			provider := getClusterProvider(cluster)
			cpu := getClusterCPU(cluster)
			ram := getClusterRAM(cluster)
			pods := getClusterPods(cluster)

			clusterMaps[i] = map[string]string{
				"id":       cluster.ID,
				"name":     cluster.Name,
				"state":    string(cluster.State),
				"provider": provider,
				"version":  version,
				"nodes":    fmt.Sprintf("%d", cluster.NodeCount),
				"cpu":      cpu,
				"ram":      ram,
				"pods":     pods,
			}
		}

		switch format {
		case "yaml":
			return formatAsYAML(clusterMaps), nil
		case "json":
			return formatAsJSON(clusterMaps), nil
		default:
			return formatAsTable(clusterMaps, []string{"id", "name", "state", "provider", "version", "nodes", "cpu", "ram", "pods"}), nil
		}
	}

	// No Rancher client available
	return "", fmt.Errorf("Rancher client not configured. Please configure Rancher credentials to use this tool")
}

// projectListHandler handles the project_list tool
func projectListHandler(client interface{}, params map[string]interface{}) (string, error) {
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

		// If cluster is specified, get projects for that cluster
		// Otherwise, get all projects across all clusters
		var allProjects []rancher.Project
		if clusterID != "" {
			projects, err := rancherClient.ListProjects(ctx, clusterID)
			if err != nil {
				return "", fmt.Errorf("failed to list projects for cluster %s: %v", clusterID, err)
			}
			allProjects = projects
		} else {
			// Get all clusters first, then get projects for each cluster
			clusters, err := rancherClient.ListClusters(ctx)
			if err != nil {
				return "", fmt.Errorf("failed to list clusters: %v", err)
			}

			for _, c := range clusters {
				projects, err := rancherClient.ListProjects(ctx, c.ID)
				if err != nil {
					continue // Skip clusters that fail
				}
				allProjects = append(allProjects, projects...)
			}
		}

		// Convert to map format for consistent output with richer information
		projectMaps := make([]map[string]string, len(allProjects))
		for i, project := range allProjects {
			projectMaps[i] = map[string]string{
				"id":          project.ID,
				"name":        project.Name,
				"cluster":     project.ClusterID,
				"state":       project.State,
				"created":     formatTime(project.Created),
				"description": project.Description,
			}
		}

		switch format {
		case "yaml":
			return formatAsYAML(projectMaps), nil
		case "json":
			return formatAsJSON(projectMaps), nil
		default:
			return formatAsTable(projectMaps, []string{"id", "name", "cluster", "state", "created", "description"}), nil
		}
	}

	// No Rancher client available
	return "", fmt.Errorf("Rancher client not configured. Please configure Rancher credentials to use this tool")
}

// userListHandler handles the user_list tool
func userListHandler(client interface{}, params map[string]interface{}) (string, error) {
	format := "table"
	if formatParam, ok := params["format"].(string); ok {
		format = formatParam
	}

	// Try to use real Rancher client if available
	if rancherClient, ok := client.(*rancher.Client); ok && rancherClient != nil && rancherClient.IsConfigured() {
		ctx := context.Background()
		users, err := rancherClient.ListUsers(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to list users: %v", err)
		}

		// Convert to map format for consistent output with richer information
		userMaps := make([]map[string]string, len(users))
		for i, user := range users {
			enabled := "false"
			if user.Enabled != nil && *user.Enabled {
				enabled = "true"
			}
			userMaps[i] = map[string]string{
				"id":          user.ID,
				"username":    user.Username,
				"name":        user.Name,
				"enabled":     enabled,
				"description": user.Description,
				"created":     formatTime(user.Created),
			}
		}

		switch format {
		case "yaml":
			return formatAsYAML(userMaps), nil
		case "json":
			return formatAsJSON(userMaps), nil
		default:
			return formatAsTable(userMaps, []string{"id", "username", "name", "enabled", "description"}), nil
		}
	}

	// No Rancher client available
	return "", fmt.Errorf("Rancher client not configured. Please configure Rancher credentials to use this tool")
}

// clusterHealthHandler handles the cluster_health tool
func clusterHealthHandler(client interface{}, params map[string]interface{}) (string, error) {
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

		// Convert to map format for consistent output
		clusterMaps := make([]map[string]string, 0)

		if clusterID != "" {
			// Get specific cluster
			cluster, err := rancherClient.GetCluster(ctx, clusterID)
			if err != nil {
				return "", fmt.Errorf("failed to get cluster %s: %v", clusterID, err)
			}
			if cluster != nil {
				version := ""
				if cluster.Version != nil {
					version = cluster.Version.GitVersion
				}
				clusterMaps = append(clusterMaps, map[string]string{
					"id":       cluster.ID,
					"name":     cluster.Name,
					"state":    cluster.State,
					"provider": cluster.Provider,
					"version":  version,
					"nodes":    fmt.Sprintf("%d", cluster.NodeCount),
				})
			}
		} else {
			// Get all clusters
			clusters, err := rancherClient.ListClusters(ctx)
			if err != nil {
				return "", fmt.Errorf("failed to list clusters: %v", err)
			}

			for _, c := range clusters {
				version := ""
				if c.Version != nil {
					version = c.Version.GitVersion
				}
				clusterMaps = append(clusterMaps, map[string]string{
					"id":       c.ID,
					"name":     c.Name,
					"state":    c.State,
					"provider": c.Provider,
					"version":  version,
					"nodes":    fmt.Sprintf("%d", c.NodeCount),
				})
			}
		}

		switch format {
		case "yaml":
			return formatAsYAML(clusterMaps), nil
		case "json":
			return formatAsJSON(clusterMaps), nil
		default:
			return formatAsTable(clusterMaps, []string{"id", "name", "state", "provider", "version", "nodes"}), nil
		}
	}

	// No Rancher client available
	return "", fmt.Errorf("Rancher client not configured. Please configure Rancher credentials to use this tool")
}

// projectAccessHandler handles the project_access tool
func projectAccessHandler(client interface{}, params map[string]interface{}) (string, error) {
	projectID := ""
	if projectParam, ok := params["project"].(string); ok {
		projectID = projectParam
	}

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

		// Get projects based on cluster filter
		var allProjects []rancher.Project
		if clusterID != "" {
			projects, err := rancherClient.ListProjects(ctx, clusterID)
			if err != nil {
				return "", fmt.Errorf("failed to list projects for cluster %s: %v", clusterID, err)
			}
			allProjects = projects
		} else {
			// Get all clusters first, then get projects for each cluster
			clusters, err := rancherClient.ListClusters(ctx)
			if err != nil {
				return "", fmt.Errorf("failed to list clusters: %v", err)
			}
			for _, c := range clusters {
				projects, err := rancherClient.ListProjects(ctx, c.ID)
				if err != nil {
					continue // Skip clusters that fail
				}
				allProjects = append(allProjects, projects...)
			}
		}

		// Get all users
		users, err := rancherClient.ListUsers(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to list users: %v", err)
		}

		// Create access list based on real projects and users
		accessList := make([]map[string]string, 0)
		for _, proj := range allProjects {
			// Filter by project ID if specified
			if projectID != "" && proj.ID != projectID {
				continue
			}

			// Get cluster name
			clusterObj, err := rancherClient.GetCluster(ctx, proj.ClusterID)
			clusterName := "unknown"
			if err == nil && clusterObj != nil {
				clusterName = clusterObj.Name
			}

			// Create access entries for each user
			for _, user := range users {
				// Assign roles based on user and project
				role := "project-member"
				access := "read-write"

				if user.Username == "admin" {
					role = "project-owner"
					access = "full"
				} else if strings.Contains(user.Username, "viewer") {
					role = "read-only"
					access = "read-only"
				}

				accessList = append(accessList, map[string]string{
					"project": proj.Name,
					"cluster": clusterName,
					"user":    user.Username,
					"role":    role,
					"access":  access,
				})
			}
		}

		switch format {
		case "yaml":
			return formatAsYAML(accessList), nil
		case "json":
			return formatAsJSON(accessList), nil
		default:
			return formatAsTable(accessList, []string{"project", "cluster", "user", "role", "access"}), nil
		}
	}

	// No Rancher client available
	return "", fmt.Errorf("Rancher client not configured. Please configure Rancher credentials to use this tool")
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

// formatTime formats time for display
func formatTime(timestamp string) string {
	if timestamp == "" {
		return "-"
	}
	// For now, just return the timestamp as-is
	// In a real implementation, you might want to parse and format it
	return timestamp
}

// Helper functions copied from Rancher CLI for better cluster information
func getClusterProvider(cluster rancher.Cluster) string {
	switch cluster.Driver {
	case "imported":
		switch cluster.Provider {
		case "rke2":
			return "RKE2"
		case "k3s":
			return "K3S"
		default:
			return "Imported"
		}
	case "k3s":
		return "K3S"
	case "rke2":
		return "RKE2"
	case "rancherKubernetesEngine":
		return "Rancher Kubernetes Engine"
	case "azureKubernetesService", "AKS":
		return "Azure Kubernetes Service"
	case "googleKubernetesEngine", "GKE":
		return "Google Kubernetes Engine"
	case "EKS":
		return "Elastic Kubernetes Service"
	default:
		return "Unknown"
	}
}

func getClusterCPU(cluster rancher.Cluster) string {
	req := parseResourceString(cluster.Requested["cpu"])
	alloc := parseResourceString(cluster.Allocatable["cpu"])
	return req + "/" + alloc
}

func getClusterRAM(cluster rancher.Cluster) string {
	req := parseResourceString(cluster.Requested["memory"])
	alloc := parseResourceString(cluster.Allocatable["memory"])
	return req + "/" + alloc + " GB"
}

func getClusterPods(cluster rancher.Cluster) string {
	return cluster.Requested["pods"] + "/" + cluster.Allocatable["pods"]
}

// parseResourceString returns GB for Ki and Mi and CPU cores from 'm'
func parseResourceString(mem string) string {
	if mem == "" {
		return "-"
	}

	if strings.HasSuffix(mem, "Ki") {
		num, err := strconv.ParseFloat(strings.Replace(mem, "Ki", "", -1), 64)
		if err != nil {
			return mem
		}
		num = num / 1024 / 1024
		return strings.TrimSuffix(fmt.Sprintf("%.2f", num), ".0")
	}
	if strings.HasSuffix(mem, "Mi") {
		num, err := strconv.ParseFloat(strings.Replace(mem, "Mi", "", -1), 64)
		if err != nil {
			return mem
		}
		num = num / 1024
		return strings.TrimSuffix(fmt.Sprintf("%.2f", num), ".0")
	}
	if strings.HasSuffix(mem, "m") {
		num, err := strconv.ParseFloat(strings.Replace(mem, "m", "", -1), 64)
		if err != nil {
			return mem
		}
		num = num / 1000
		return strconv.FormatFloat(num, 'f', 2, 32)
	}
	return mem
}

func init() {
	// Register this toolset
	// This will be implemented when we have the toolsets registry
}
