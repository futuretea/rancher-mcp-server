package paramutil

import (
	"context"
	"fmt"

	"github.com/futuretea/rancher-mcp-server/pkg/client/norman"
)

// ResolveCluster extracts cluster parameter and resolves it via fuzzy lookup
// Supports: exact ID, exact name, or partial name match
// Returns the resolved cluster ID
func ResolveCluster(ctx context.Context, client *norman.Client, params map[string]interface{}) (string, error) {
	clusterInput, err := ExtractRequiredString(params, ParamCluster)
	if err != nil {
		return "", err
	}

	cluster, err := client.LookupCluster(ctx, clusterInput)
	if err != nil {
		return "", fmt.Errorf("failed to resolve cluster '%s': %w", clusterInput, err)
	}

	return cluster.ID, nil
}

// ResolveOptionalCluster extracts optional cluster parameter and resolves it via fuzzy lookup
// Returns empty string if cluster is not specified
// Supports: exact ID, exact name, or partial name match
func ResolveOptionalCluster(ctx context.Context, client *norman.Client, params map[string]interface{}) (string, error) {
	clusterInput := ExtractOptionalString(params, ParamCluster)
	if clusterInput == "" {
		return "", nil
	}

	cluster, err := client.LookupCluster(ctx, clusterInput)
	if err != nil {
		return "", fmt.Errorf("failed to resolve cluster '%s': %w", clusterInput, err)
	}

	return cluster.ID, nil
}

// ResolveOptionalProject extracts optional project parameter and resolves it via fuzzy lookup
// Returns empty string if project is not specified
// Supports: exact ID, exact name, or partial name match
func ResolveOptionalProject(ctx context.Context, client *norman.Client, params map[string]interface{}, clusterID string) (string, error) {
	projectInput := ExtractOptionalString(params, ParamProject)
	if projectInput == "" {
		return "", nil
	}

	project, err := client.LookupProject(ctx, clusterID, projectInput)
	if err != nil {
		return "", fmt.Errorf("failed to resolve project '%s': %w", projectInput, err)
	}

	return project.ID, nil
}
