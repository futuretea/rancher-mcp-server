package rancher

import (
	"context"
	"fmt"
	"strings"
)

const (
	// ProjectIDLabel is the label key that contains the project ID in namespace
	ProjectIDLabel = "field.cattle.io/projectId"
)

// GetProjectIDFromNamespace gets the project ID from namespace labels
// It returns empty string if the namespace is not in a project
func GetProjectIDFromNamespace(namespace map[string]interface{}) string {
	if namespace == nil {
		return ""
	}

	labels, ok := namespace["labels"].(map[string]interface{})
	if !ok || labels == nil {
		return ""
	}

	projectID, ok := labels[ProjectIDLabel].(string)
	if !ok || projectID == "" {
		return ""
	}

	return projectID
}

// GetProjectIDForNamespace retrieves the project ID for a given namespace
// Returns empty string if namespace is not in a project
func (c *Client) GetProjectIDForNamespace(ctx context.Context, clusterID, namespaceName string) (string, error) {
	if !c.configured {
		return "", fmt.Errorf("Rancher client not configured")
	}

	// Get the namespace
	namespace, err := c.GetNamespace(ctx, clusterID, namespaceName)
	if err != nil {
		return "", fmt.Errorf("failed to get namespace %s: %w", namespaceName, err)
	}

	if namespace == nil {
		return "", fmt.Errorf("namespace %s not found", namespaceName)
	}

	// Convert namespace to map to extract labels
	namespaceMap := map[string]interface{}{
		"labels": namespace.Labels,
	}

	projectID := GetProjectIDFromNamespace(namespaceMap)

	return projectID, nil
}

// AutoDetectProjectID attempts to auto-detect the project ID for a namespace
// Uses label-based approach for optimal performance
func (c *Client) AutoDetectProjectID(ctx context.Context, clusterID, namespaceName string) (string, error) {
	// Try label-based approach first
	projectID, err := c.GetProjectIDForNamespace(ctx, clusterID, namespaceName)
	if err != nil {
		return "", fmt.Errorf("failed to auto-detect project: %w", err)
	}

	if projectID == "" {
		return "", fmt.Errorf("namespace %s is not in a project (missing label %s)", namespaceName, ProjectIDLabel)
	}

	return projectID, nil
}

// ExtractProjectNameFromID extracts the project name from a project ID
// Project ID format: "cluster-xxxxx:project-yyyyy"
// Returns the "project-yyyyy" part
func ExtractProjectNameFromID(projectID string) string {
	if projectID == "" {
		return ""
	}

	parts := strings.Split(projectID, ":")
	if len(parts) < 2 {
		return projectID
	}

	return parts[1]
}
