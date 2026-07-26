package rancher

import (
	"context"
	"fmt"
	"strings"

	"github.com/futuretea/rancher-mcp-server/pkg/client/norman"
	"github.com/futuretea/rancher-mcp-server/pkg/client/steve"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset/paramutil"
)

// clusterToMap converts a Rancher cluster to a source-agnostic output row.
func clusterToMap(c norman.Cluster) map[string]string {
	version := ""
	if c.Version != nil {
		version = c.Version.GitVersion
	}
	return map[string]string{
		"id":       c.ID,
		"name":     c.Name,
		"source":   "rancher",
		"state":    string(c.State),
		"provider": getClusterProvider(c),
		"version":  version,
		"nodes":    fmt.Sprintf("%d", c.NodeCount),
		"cpu":      getClusterCPU(c),
		"ram":      getClusterRAM(c),
		"pods":     getClusterPods(c),
	}
}

func kubeconfigClusterToMap(contextName string) map[string]string {
	return map[string]string{
		"id":       "kubeconfig:" + contextName,
		"name":     contextName,
		"source":   "kubeconfig",
		"state":    "",
		"provider": "",
		"version":  "",
		"nodes":    "",
		"cpu":      "",
		"ram":      "",
		"pods":     "",
	}
}

// clusterListHandler handles the cluster_list tool
func clusterListHandler(ctx context.Context, client interface{}, params map[string]interface{}) (string, error) {
	format, err := paramutil.ExtractAndValidateFormat(params)
	if err != nil {
		return "", err
	}

	// Extract query and pagination parameters
	nameFilter := paramutil.ExtractOptionalString(params, paramutil.ParamName)
	limit := paramutil.ExtractInt64(params, paramutil.ParamLimit, 100)
	page := paramutil.ExtractInt64(params, paramutil.ParamPage, 1)

	clusterMaps := make([]map[string]string, 0)
	hasClusterSource := false
	if normanClient, err := toolset.ValidateNormanClient(client); err == nil {
		hasClusterSource = true
		clusters, err := normanClient.ListClusters(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to list Rancher clusters: %w", err)
		}
		for _, cluster := range clusters {
			clusterMaps = append(clusterMaps, clusterToMap(cluster))
		}
	}
	if steveClient, err := toolset.ValidateSteveClient(client); err == nil && len(steveClient.KubeconfigContexts()) > 0 {
		hasClusterSource = true
		clusterMaps = appendKubeconfigClusters(clusterMaps, steveClient)
	}
	if !hasClusterSource {
		return "", paramutil.ErrClusterSourcesNotConfigured
	}

	filtered := filterClusterMapsByName(clusterMaps, nameFilter)
	paginated, _ := paramutil.ApplyPagination(filtered, limit, page)

	return paramutil.FormatOutput(paginated, format, []string{"id", "name", "source", "state", "provider", "version", "nodes", "cpu", "ram", "pods"}, nil)
}

func appendKubeconfigClusters(rows []map[string]string, client *steve.Client) []map[string]string {
	for _, contextName := range client.KubeconfigContexts() {
		rows = append(rows, kubeconfigClusterToMap(contextName))
	}
	return rows
}

func filterClusterMapsByName(clusters []map[string]string, name string) []map[string]string {
	if name == "" {
		return clusters
	}
	result := make([]map[string]string, 0, len(clusters))
	for _, cluster := range clusters {
		if strings.Contains(strings.ToLower(cluster["name"]), strings.ToLower(name)) {
			result = append(result, cluster)
		}
	}
	return result
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
