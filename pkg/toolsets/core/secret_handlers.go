package core

import (
	"context"
	"fmt"

	"github.com/futuretea/rancher-mcp-server/pkg/rancher"
	"github.com/futuretea/rancher-mcp-server/pkg/toolsets/common"
)

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
		"created":   common.FormatTime(secret.Created),
	}

	return formatSecretResult(result, format)
}

// formatSecretResult formats a single secret result
func formatSecretResult(result map[string]interface{}, format string) (string, error) {
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
			"type":      common.GetStringValue(result["type"]),
			"created":   common.GetStringValue(result["created"]),
		}
		data = append(data, row)
		return common.FormatAsTable(data, []string{"id", "name", "namespace", "type", "created"}), nil
	default:
		return "", fmt.Errorf("%w: %s", common.ErrInvalidFormat, format)
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
				"created":   common.FormatTime(secret.Created),
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
					"created":   common.FormatTime(secret.Created),
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
		return common.FormatAsTable(resultMaps, []string{"id", "name", "namespace", "type", "created"}), nil
	default:
		return "", fmt.Errorf("%w: %s", common.ErrInvalidFormat, format)
	}
}
