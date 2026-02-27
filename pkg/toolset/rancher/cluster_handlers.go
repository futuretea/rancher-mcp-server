package rancher

import (
	"context"
	"fmt"
	"strings"

	"github.com/futuretea/rancher-mcp-server/pkg/client/norman"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset/paramutil"
)

// clusterToMap converts a cluster to a map with full resource details.
func clusterToMap(c norman.Cluster) map[string]string {
	version := ""
	if c.Version != nil {
		version = c.Version.GitVersion
	}
	return map[string]string{
		"id":       c.ID,
		"name":     c.Name,
		"state":    string(c.State),
		"provider": getClusterProvider(c),
		"version":  version,
		"nodes":    fmt.Sprintf("%d", c.NodeCount),
		"cpu":      getClusterCPU(c),
		"ram":      getClusterRAM(c),
		"pods":     getClusterPods(c),
	}
}

// clusterListHandler handles the cluster_list tool
func clusterListHandler(ctx context.Context, client interface{}, params map[string]interface{}) (string, error) {
	normanClient, err := toolset.ValidateNormanClient(client)
	if err != nil {
		return "", err
	}

	format, err := paramutil.ExtractAndValidateFormat(params)
	if err != nil {
		return "", err
	}

	// Extract query and pagination parameters
	nameFilter := paramutil.ExtractOptionalString(params, paramutil.ParamName)
	limit := paramutil.ExtractInt64(params, paramutil.ParamLimit, 100)
	page := paramutil.ExtractInt64(params, paramutil.ParamPage, 1)

	clusters, err := normanClient.ListClusters(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list clusters: %w", err)
	}

	// Apply name filter
	filtered := filterClustersByName(clusters, nameFilter)

	// Apply pagination
	paginated, _ := paramutil.ApplyPagination(filtered, limit, page)

	clusterMaps := make([]map[string]string, len(paginated))
	for i, c := range paginated {
		clusterMaps[i] = clusterToMap(c)
	}

	return paramutil.FormatOutput(clusterMaps, format, []string{"id", "name", "state", "provider", "version", "nodes", "cpu", "ram", "pods"}, nil)
}

// filterClustersByName filters clusters by name (partial match, case-insensitive).
func filterClustersByName(clusters []norman.Cluster, name string) []norman.Cluster {
	if name == "" {
		return clusters
	}
	var result []norman.Cluster
	for _, c := range clusters {
		if strings.Contains(strings.ToLower(c.Name), strings.ToLower(name)) {
			result = append(result, c)
		}
	}
	return result
}
