package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/futuretea/rancher-mcp-server/pkg/rancher"
	"github.com/futuretea/rancher-mcp-server/pkg/toolsets/common"
)

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
		"created":   common.FormatTime(configMap.Created),
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
	case common.FormatYAML:
		return common.FormatAsYAML(result)
	case common.FormatJSON:
		return common.FormatAsJSON(result)
	case common.FormatTable:
		data := []map[string]string{}
		row := map[string]string{
			"id":        common.GetStringValue(result["id"]),
			"name":      common.GetStringValue(result["name"]),
			"namespace": common.GetStringValue(result["namespace"]),
			"created":   common.GetStringValue(result["created"]),
		}
		data = append(data, row)
		return common.FormatAsTable(data, []string{"id", "name", "namespace", "created"}), nil
	default:
		return "", fmt.Errorf("%w: %s", common.ErrInvalidFormat, format)
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
				"created":   common.FormatTime(cm.Created),
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
					"created":   common.FormatTime(cm.Created),
				})
			}
		}
	}

	if len(resultMaps) == 0 {
		return common.FormatEmptyResult(format)
	}

	switch format {
	case common.FormatYAML:
		return common.FormatAsYAML(resultMaps)
	case common.FormatJSON:
		return common.FormatAsJSON(resultMaps)
	case common.FormatTable:
		return common.FormatAsTable(resultMaps, []string{"id", "name", "namespace", "keys", "created"}), nil
	default:
		return "", fmt.Errorf("%w: %s", common.ErrInvalidFormat, format)
	}
}
