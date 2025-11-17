package config

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/futuretea/rancher-mcp-server/pkg/api"
	"github.com/futuretea/rancher-mcp-server/pkg/rancher"
	"github.com/futuretea/rancher-mcp-server/pkg/toolsets/common"
)

// Toolset implements the config toolset
type Toolset struct{}

var _ api.Toolset = (*Toolset)(nil)

// GetName returns the name of the toolset
func (t *Toolset) GetName() string {
	return "config"
}

// GetDescription returns the description of the toolset
func (t *Toolset) GetDescription() string {
	return "Generate and view Kubernetes configuration (kubeconfig) for Rancher clusters"
}

// GetTools returns the tools provided by this toolset
func (t *Toolset) GetTools(client interface{}) []api.ServerTool {
	return []api.ServerTool{
		{
			Tool: mcp.Tool{
				Name:        "configuration_view",
				Description: "Generate and view Kubernetes configuration (kubeconfig) for a Rancher cluster",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]any{
						"cluster": map[string]any{
							"type":        "string",
							"description": "Cluster ID to generate kubeconfig for",
						},
					},
				},
			},
			Annotations: api.ToolAnnotations{
				ReadOnlyHint: common.BoolPtr(true),
			},
			Handler: configurationViewHandler,
		},
	}
}

// configurationViewHandler handles the configuration_view tool
func configurationViewHandler(client interface{}, params map[string]interface{}) (string, error) {
	clusterID := ""
	if clusterParam, ok := params["cluster"].(string); ok && clusterParam != "" {
		clusterID = clusterParam
	}

	// Try to use real Rancher client if available
	if rancherClient, ok := client.(*rancher.Client); ok && rancherClient != nil && rancherClient.IsConfigured() {
		ctx := context.Background()

		// If cluster is specified, generate kubeconfig for that specific cluster
		if clusterID != "" {
			// Try to generate kubeconfig directly for the specified cluster
			kubeconfig, err := rancherClient.GenerateKubeconfig(ctx, clusterID)
			if err != nil {
				// If direct generation fails, check if cluster exists
				clusters, err := rancherClient.ListClusters(ctx)
				if err != nil {
					return "", fmt.Errorf("failed to list clusters: %v", err)
				}

				var clusterExists bool
				for _, c := range clusters {
					if c.ID == clusterID {
						clusterExists = true
						break
					}
				}

				if !clusterExists {
					return "", fmt.Errorf("cluster with ID '%s' not found", clusterID)
				}
				return "", fmt.Errorf("failed to generate kubeconfig for cluster %s: %v", clusterID, err)
			}

			return kubeconfig, nil
		}

		// If no cluster specified, return information about available clusters
		clusters, err := rancherClient.ListClusters(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to list clusters: %v", err)
		}

		if len(clusters) == 0 {
			return "No clusters available. Please specify a cluster name or ID to generate kubeconfig.", nil
		}

		// Format cluster list for user to choose from
		clusterInfo := "Available clusters:\n"
		for _, c := range clusters {
			clusterInfo += fmt.Sprintf("  - %s (ID: %s, State: %s)\n", c.Name, c.ID, c.State)
		}
		clusterInfo += "\nUse the 'cluster' parameter to specify which cluster to generate kubeconfig for."
		return clusterInfo, nil
	}

	// No Rancher client available
	return "", common.ErrRancherNotConfigured
}
