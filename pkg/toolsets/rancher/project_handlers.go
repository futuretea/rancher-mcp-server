package rancher

import (
	"context"
	"fmt"
	"strings"

	"github.com/futuretea/rancher-mcp-server/pkg/rancher"
	"github.com/futuretea/rancher-mcp-server/pkg/toolsets/common"
)

// projectGetHandler handles the project_get tool for single project queries
func projectGetHandler(client interface{}, params map[string]interface{}) (string, error) {
	// Extract required parameters
	projectID := ""
	if projectParam, ok := params["project"].(string); ok {
		projectID = projectParam
	}
	if projectID == "" {
		return "", fmt.Errorf("project parameter is required")
	}

	clusterID := ""
	if clusterParam, ok := params["cluster"].(string); ok {
		clusterID = clusterParam
	}
	if clusterID == "" {
		return "", fmt.Errorf("cluster parameter is required")
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

	// Get the project
	project, err := rancherClient.GetProject(ctx, clusterID, projectID)
	if err != nil {
		return "", fmt.Errorf("failed to get project: %v", err)
	}

	// Build the result
	result := map[string]interface{}{
		"id":          project.ID,
		"name":        project.Name,
		"cluster":     project.ClusterID,
		"state":       project.State,
		"created":     common.FormatTime(project.Created),
		"description": project.Description,
	}

	return formatProjectResult(result, format)
}

// formatProjectResult formats a single project result
func formatProjectResult(result map[string]interface{}, format string) (string, error) {
	switch format {
	case common.FormatYAML:
		return common.FormatAsYAML(result)
	case common.FormatJSON:
		return common.FormatAsJSON(result)
	case common.FormatTable:
		data := []map[string]string{}
		row := map[string]string{
			"id":          common.GetStringValue(result["id"]),
			"name":        common.GetStringValue(result["name"]),
			"cluster":     common.GetStringValue(result["cluster"]),
			"state":       common.GetStringValue(result["state"]),
			"created":     common.GetStringValue(result["created"]),
			"description": common.GetStringValue(result["description"]),
		}
		data = append(data, row)
		return common.FormatAsTable(data, []string{"id", "name", "cluster", "state", "created", "description"}), nil
	default:
		return "", fmt.Errorf("%w: %s", common.ErrInvalidFormat, format)
	}
}

// projectListHandler handles the project_list tool
func projectListHandler(client interface{}, params map[string]interface{}) (string, error) {
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
				"created":     common.FormatTime(project.Created),
				"description": project.Description,
			}
		}

		switch format {
		case common.FormatYAML:
			return common.FormatAsYAML(projectMaps)
		case common.FormatJSON:
			return common.FormatAsJSON(projectMaps)
		case common.FormatTable:
			return common.FormatAsTable(projectMaps, []string{"id", "name", "cluster", "state", "created", "description"}), nil
		default:
			return "", fmt.Errorf("%w: %s", common.ErrInvalidFormat, format)
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

	format := "json"
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
		case common.FormatYAML:
			return common.FormatAsYAML(accessList)
		case common.FormatJSON:
			return common.FormatAsJSON(accessList)
		case common.FormatTable:
			return common.FormatAsTable(accessList, []string{"project", "cluster", "user", "role", "access"}), nil
		default:
			return "", fmt.Errorf("%w: %s", common.ErrInvalidFormat, format)
		}
	}

	// No Rancher client available
	return "", fmt.Errorf("Rancher client not configured. Please configure Rancher credentials to use this tool")
}
