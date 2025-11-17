package core

import (
	"context"
	"fmt"

	"github.com/futuretea/rancher-mcp-server/pkg/toolsets/common"
)

// namespaceGetHandler handles the namespace_get tool for single namespace queries
func namespaceGetHandler(client interface{}, params map[string]interface{}) (string, error) {
	// Extract required parameters
	clusterID, err := common.ExtractRequiredString(params, common.ParamCluster)
	if err != nil {
		return "", err
	}

	name, err := common.ExtractRequiredString(params, common.ParamName)
	if err != nil {
		return "", err
	}

	format := common.ExtractFormat(params)
	if err := common.ValidateFormat(format); err != nil {
		return "", err
	}

	rancherClient, err := common.ValidateRancherClient(client)
	if err != nil {
		return "", err
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
		"created":        common.FormatTime(namespace.Created),
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
	case common.FormatYAML:
		return common.FormatAsYAML(result)
	case common.FormatJSON:
		return common.FormatAsJSON(result)
	case common.FormatTable:
		data := []map[string]string{}
		row := map[string]string{
			"id":        common.GetStringValue(result["id"]),
			"name":      common.GetStringValue(result["name"]),
			"state":     common.GetStringValue(result["state"]),
			"projectId": common.GetStringValue(result["projectId"]),
			"created":   common.GetStringValue(result["created"]),
		}
		data = append(data, row)
		return common.FormatAsTable(data, []string{"id", "name", "state", "projectId", "created"}), nil
	default:
		return "", fmt.Errorf("%w: %s", common.ErrInvalidFormat, format)
	}
}

// namespaceListHandler handles the namespace_list tool
func namespaceListHandler(client interface{}, params map[string]interface{}) (string, error) {
	clusterID, err := common.ExtractRequiredString(params, common.ParamCluster)
	if err != nil {
		return "", err
	}

	projectID := common.ExtractOptionalString(params, common.ParamProject, "")
	format := common.ExtractFormat(params)
	if err := common.ValidateFormat(format); err != nil {
		return "", err
	}

	rancherClient, err := common.ValidateRancherClient(client)
	if err != nil {
		return "", err
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
			"created":     common.FormatTime(ns.Created),
		})
	}

	if len(namespaceMaps) == 0 {
		return common.FormatEmptyResult(format)
	}

	switch format {
	case common.FormatYAML:
		return common.FormatAsYAML(namespaceMaps)
	case common.FormatJSON:
		return common.FormatAsJSON(namespaceMaps)
	case common.FormatTable:
		return common.FormatAsTable(namespaceMaps, []string{"id", "name", "state", "project", "description", "created"}), nil
	default:
		return "", fmt.Errorf("%w: %s", common.ErrInvalidFormat, format)
	}
}
