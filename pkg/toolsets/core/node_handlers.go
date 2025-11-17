package core

import (
	"context"
	"fmt"

	"github.com/futuretea/rancher-mcp-server/pkg/rancher"
	"github.com/futuretea/rancher-mcp-server/pkg/toolsets/common"
)

// nodeGetHandler handles the node_get tool for single node queries
func nodeGetHandler(client interface{}, params map[string]interface{}) (string, error) {
	// Extract required parameters
	clusterID, err := common.ExtractRequiredString(params, common.ParamCluster)
	if err != nil {
		return "", err
	}

	nodeID, err := common.ExtractRequiredString(params, common.ParamNode)
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

	// Get the node
	node, err := rancherClient.GetNode(ctx, clusterID, nodeID)
	if err != nil {
		return "", fmt.Errorf("failed to get node: %v", err)
	}

	// Build the result
	result := map[string]interface{}{
		"id":                node.ID,
		"name":              node.Name,
		"state":             node.State,
		"created":           common.FormatTime(node.Created),
		"hostname":          node.Hostname,
		"externalIpAddress": node.ExternalIPAddress,
	}

	// Add kubelet version if available
	if node.Info != nil && node.Info.Kubernetes != nil {
		result["kubeletVersion"] = node.Info.Kubernetes.KubeletVersion
	}

	// Add resource information if available
	if node.Allocatable != nil {
		result["allocatable"] = node.Allocatable
	}
	if node.Capacity != nil {
		result["capacity"] = node.Capacity
	}

	return formatNodeResult(result, format)
}

// formatNodeResult formats a single node result
func formatNodeResult(result map[string]interface{}, format string) (string, error) {
	switch format {
	case common.FormatYAML:
		return common.FormatAsYAML(result)
	case common.FormatJSON:
		return common.FormatAsJSON(result)
	case common.FormatTable:
		data := []map[string]string{}
		row := map[string]string{
			"id":       common.GetStringValue(result["id"]),
			"name":     common.GetStringValue(result["name"]),
			"state":    common.GetStringValue(result["state"]),
			"hostname": common.GetStringValue(result["hostname"]),
			"version":  common.GetStringValue(result["kubeletVersion"]),
			"created":  common.GetStringValue(result["created"]),
		}
		data = append(data, row)
		return common.FormatAsTable(data, []string{"id", "name", "state", "hostname", "version", "created"}), nil
	default:
		return "", fmt.Errorf("%w: %s", common.ErrInvalidFormat, format)
	}
}

// nodeListHandler handles the node_list tool
func nodeListHandler(client interface{}, params map[string]interface{}) (string, error) {
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

	// Collect nodes from all clusters or specific cluster
	allNodes := make([]rancher.Node, 0)

	if clusterID != "" {
		// Query specific cluster only
		nodes, err := rancherClient.ListNodes(ctx, clusterID)
		if err != nil {
			return "", fmt.Errorf("failed to list nodes for cluster %s: %v", clusterID, err)
		}
		allNodes = append(allNodes, nodes...)
	} else {
		// Query all clusters
		clusters, err := rancherClient.ListClusters(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to list clusters: %v", err)
		}

		for _, c := range clusters {
			nodes, err := rancherClient.ListNodes(ctx, c.ID)
			if err != nil {
				// Skip clusters that fail to list nodes
				continue
			}
			allNodes = append(allNodes, nodes...)
		}
	}

	if len(allNodes) == 0 {
		return common.FormatEmptyResult(format)
	}

	// Convert to map format for consistent output
	nodeMaps := make([]map[string]string, len(allNodes))
	for i, node := range allNodes {
		roles := []string{}
		if node.ControlPlane {
			roles = append(roles, "control-plane")
		}
		if node.Etcd {
			roles = append(roles, "etcd")
		}
		if node.Worker {
			roles = append(roles, "worker")
		}
		rolesStr := ""
		if len(roles) > 0 {
			rolesStr = fmt.Sprintf("%v", roles)
		}

		nodeMaps[i] = map[string]string{
			"id":       node.ID,
			"name":     node.Name,
			"state":    string(node.State),
			"cluster":  node.ClusterID,
			"hostname": node.Hostname,
			"ip":       node.IPAddress,
			"roles":    rolesStr,
		}
	}

	switch format {
	case common.FormatYAML:
		return common.FormatAsYAML(nodeMaps)
	case common.FormatJSON:
		return common.FormatAsJSON(nodeMaps)
	case common.FormatTable:
		return common.FormatAsTable(nodeMaps, []string{"id", "name", "state", "cluster", "hostname", "ip", "roles"}), nil
	default:
		return "", fmt.Errorf("%w: %s", common.ErrInvalidFormat, format)
	}
}
