package core

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/futuretea/rancher-mcp-server/pkg/toolsets/common"
)

// workloadGetHandler handles the workload_get tool for single workload queries
func workloadGetHandler(client interface{}, params map[string]interface{}) (string, error) {
	// Extract required parameters
	clusterID, err := common.ExtractRequiredString(params, "cluster")
	if err != nil {
		return "", err
	}

	namespace, err := common.ExtractRequiredString(params, "namespace")
	if err != nil {
		return "", err
	}

	name, err := common.ExtractRequiredString(params, "name")
	if err != nil {
		return "", err
	}

	// Extract optional parameters
	projectID := common.ExtractOptionalString(params, "project", "")
	format := common.ExtractFormat(params)

	rancherClient, err := common.ValidateRancherClient(client)
	if err != nil {
		return "", err
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

	// Get the workload
	workload, err := rancherClient.GetWorkload(ctx, clusterID, projectID, namespace, name)
	if err != nil {
		return "", fmt.Errorf("failed to get workload: %v", err)
	}

	// Build the result
	result := map[string]interface{}{
		"id":        workload.ID,
		"name":      workload.Name,
		"namespace": workload.NamespaceId,
		"type":      workload.Type,
		"state":     workload.State,
		"created":   common.FormatTime(workload.Created),
	}

	// Add scale information if available
	if workload.Scale != nil {
		result["scale"] = workload.Scale
	}

	return formatWorkloadResult(result, format)
}

// formatWorkloadResult formats a single workload result
func formatWorkloadResult(result map[string]interface{}, format string) (string, error) {
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
			"state":     common.GetStringValue(result["state"]),
			"created":   common.GetStringValue(result["created"]),
		}
		data = append(data, row)
		return common.FormatAsTable(data, []string{"id", "name", "namespace", "type", "state", "created"}), nil
	default:
		return "", fmt.Errorf("%w: %s", common.ErrInvalidFormat, format)
	}
}

// workloadListHandler handles the workload_list tool - shows workloads and orphan pods like Rancher CLI
func workloadListHandler(client interface{}, params map[string]interface{}) (string, error) {
	clusterID, err := common.ExtractRequiredString(params, "cluster")
	if err != nil {
		return "", err
	}

	projectID := common.ExtractOptionalString(params, "project", "")
	namespaceName := common.ExtractOptionalString(params, "namespace", "")
	nodeName := common.ExtractOptionalString(params, "node", "")
	format := common.ExtractFormat(params)

	rancherClient, err := common.ValidateRancherClient(client)
	if err != nil {
		return "", err
	}

	ctx := context.Background()

	// Get namespace mapping (ID to name) for filtering
	namespaceMapping := make(map[string]string)
	if namespaceName != "" {
		namespaces, err := rancherClient.ListNamespaces(ctx, clusterID)
		if err != nil {
			return "", fmt.Errorf("failed to list namespaces for cluster %s: %v", clusterID, err)
		}
		for _, ns := range namespaces {
			namespaceMapping[ns.ID] = ns.Name
		}
	}

	// Collect workloads and orphan pods
	resultMaps := make([]map[string]string, 0)

	// If project is specified, only query that project
	if projectID != "" {
		// Get workloads for the specified project
		workloads, err := rancherClient.ListWorkloads(ctx, clusterID, projectID)
		if err != nil {
			return "", fmt.Errorf("failed to list workloads for cluster %s, project %s: %v", clusterID, projectID, err)
		}

		// Add workloads to results
		for _, workload := range workloads {
			// Apply filters
			if namespaceName != "" {
				nsName, exists := namespaceMapping[workload.NamespaceId]
				if !exists || nsName != namespaceName {
					continue
				}
			}
			if nodeName != "" && workload.NodeID != nodeName {
				continue
			}

			image := ""
			if len(workload.Containers) > 0 {
				image = workload.Containers[0].Image
			}

			scale := "-"
			if workload.Scale != nil {
				scale = fmt.Sprintf("%d", *workload.Scale)
			}

			// Title case the type
			workloadType := titleCase(workload.Type)

			// Get namespace name for display
			nsName := workload.NamespaceId
			if namespaceMapping != nil {
				if mappedName, exists := namespaceMapping[workload.NamespaceId]; exists {
					nsName = mappedName
				}
			}

			resultMaps = append(resultMaps, map[string]string{
				"namespace": nsName,
				"name":      workload.Name,
				"type":      workloadType,
				"state":     workload.State,
				"image":     image,
				"scale":     scale,
			})
		}

		// Get orphan pods for the specified project
		orphanPods, err := rancherClient.ListOrphanPods(ctx, clusterID, projectID)
		if err != nil {
			return "", fmt.Errorf("failed to list orphan pods for cluster %s, project %s: %v", clusterID, projectID, err)
		}

		// Add orphan pods to results
		for _, pod := range orphanPods {
			// Apply filters
			if namespaceName != "" {
				nsName, exists := namespaceMapping[pod.NamespaceId]
				if !exists || nsName != namespaceName {
					continue
				}
			}
			if nodeName != "" && pod.NodeID != nodeName {
				continue
			}

			image := ""
			if len(pod.Containers) > 0 {
				image = pod.Containers[0].Image
			}

			// Title case the type
			podType := titleCase(pod.Type)

			// Get namespace name for display
			nsName := pod.NamespaceId
			if namespaceMapping != nil {
				if mappedName, exists := namespaceMapping[pod.NamespaceId]; exists {
					nsName = mappedName
				}
			}

			resultMaps = append(resultMaps, map[string]string{
				"namespace": nsName,
				"name":      pod.Name,
				"type":      podType,
				"state":     pod.State,
				"image":     image,
				"scale":     "Standalone",
			})
		}
	} else {
		// Get all projects for the cluster
		projects, err := rancherClient.ListProjects(ctx, clusterID)
		if err != nil {
			return "", fmt.Errorf("failed to list projects for cluster %s: %v", clusterID, err)
		}

		// Collect workloads and orphan pods from each project
		for _, project := range projects {
			// Get workloads for this project
			workloads, err := rancherClient.ListWorkloads(ctx, clusterID, project.ID)
			if err != nil {
				// Skip projects that fail
				continue
			}

			// Add workloads to results
			for _, workload := range workloads {
				// Apply filters
				if namespaceName != "" {
					nsName, exists := namespaceMapping[workload.NamespaceId]
					if !exists || nsName != namespaceName {
						continue
					}
				}
				if nodeName != "" && workload.NodeID != nodeName {
					continue
				}

				image := ""
				if len(workload.Containers) > 0 {
					image = workload.Containers[0].Image
				}

				scale := "-"
				if workload.Scale != nil {
					scale = fmt.Sprintf("%d", *workload.Scale)
				}

				// Title case the type
				workloadType := titleCase(workload.Type)

				// Get namespace name for display
				nsName := workload.NamespaceId
				if namespaceMapping != nil {
					if mappedName, exists := namespaceMapping[workload.NamespaceId]; exists {
						nsName = mappedName
					}
				}

				resultMaps = append(resultMaps, map[string]string{
					"namespace": nsName,
					"name":      workload.Name,
					"type":      workloadType,
					"state":     workload.State,
					"image":     image,
					"scale":     scale,
				})
			}

			// Get orphan pods (pods without workloads) for this project
			orphanPods, err := rancherClient.ListOrphanPods(ctx, clusterID, project.ID)
			if err != nil {
				// Skip if orphan pods fail
				continue
			}

			// Add orphan pods to results
			for _, pod := range orphanPods {
				// Apply filters
				if namespaceName != "" {
					nsName, exists := namespaceMapping[pod.NamespaceId]
					if !exists || nsName != namespaceName {
						continue
					}
				}
				if nodeName != "" && pod.NodeID != nodeName {
					continue
				}

				image := ""
				if len(pod.Containers) > 0 {
					image = pod.Containers[0].Image
				}

				// Title case the type
				podType := titleCase(pod.Type)

				// Get namespace name for display
				nsName := pod.NamespaceId
				if namespaceMapping != nil {
					if mappedName, exists := namespaceMapping[pod.NamespaceId]; exists {
						nsName = mappedName
					}
				}

				resultMaps = append(resultMaps, map[string]string{
					"namespace": nsName,
					"name":      pod.Name,
					"type":      podType,
					"state":     pod.State,
					"image":     image,
					"scale":     "Standalone",
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
		return common.FormatAsTable(resultMaps, []string{"namespace", "name", "type", "state", "image", "scale"}), nil
	default:
		return "", fmt.Errorf("%w: %s", common.ErrInvalidFormat, format)
	}
}

// titleCase converts a string to title case using the modern cases package
func titleCase(s string) string {
	caser := cases.Title(language.English)
	return caser.String(strings.ToLower(s))
}
