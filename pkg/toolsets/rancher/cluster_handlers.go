package rancher

import (
	"context"
	"fmt"

	"github.com/futuretea/rancher-mcp-server/pkg/toolsets/common"
)

// clusterListHandler handles the cluster_list tool
func clusterListHandler(client interface{}, params map[string]interface{}) (string, error) {
	format := common.ExtractFormat(params)
	if err := common.ValidateFormat(format); err != nil {
		return "", err
	}

	rancherClient, err := common.ValidateRancherClient(client)
	if err != nil {
		return "", err
	}
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
	case common.FormatYAML:
		return common.FormatAsYAML(clusterMaps)
	case common.FormatJSON:
		return common.FormatAsJSON(clusterMaps)
	case common.FormatTable:
		return common.FormatAsTable(clusterMaps, []string{"id", "name", "state", "provider", "version", "nodes", "cpu", "ram", "pods"}), nil
	default:
		return "", fmt.Errorf("%w: %s", common.ErrInvalidFormat, format)
	}
}

// clusterHealthHandler handles the cluster_health tool
func clusterHealthHandler(client interface{}, params map[string]interface{}) (string, error) {
	clusterID := common.ExtractOptionalString(params, common.ParamCluster, "")
	format := common.ExtractFormat(params)
	if err := common.ValidateFormat(format); err != nil {
		return "", err
	}

	rancherClient, err := common.ValidateRancherClient(client)
	if err != nil {
		return "", err
	}
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
	case common.FormatYAML:
		return common.FormatAsYAML(clusterMaps)
	case common.FormatJSON:
		return common.FormatAsJSON(clusterMaps)
	case common.FormatTable:
		return common.FormatAsTable(clusterMaps, []string{"id", "name", "state", "provider", "version", "nodes"}), nil
	default:
		return "", fmt.Errorf("%w: %s", common.ErrInvalidFormat, format)
	}
}
