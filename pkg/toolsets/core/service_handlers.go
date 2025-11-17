package core

import (
	"context"
	"fmt"

	"github.com/futuretea/rancher-mcp-server/pkg/logging"
	"github.com/futuretea/rancher-mcp-server/pkg/rancher"
	"github.com/futuretea/rancher-mcp-server/pkg/toolsets/common"
)

// serviceGetHandler handles the service_get tool for single service queries
func serviceGetHandler(client interface{}, params map[string]interface{}) (string, error) {
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
	getPodDetails := common.ExtractBool(params, "getPodDetails", false)
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

	// Get the service
	service, err := rancherClient.GetService(ctx, clusterID, projectID, namespace, name)
	if err != nil {
		return "", fmt.Errorf("failed to get service: %v", err)
	}

	// Build the result
	result := map[string]interface{}{
		"id":              service.ID,
		"name":            service.Name,
		"namespace":       service.NamespaceId,
		"state":           service.State,
		"created":         common.FormatTime(service.Created),
		"type":            service.Type,
		"clusterIp":       service.ClusterIp,
		"sessionAffinity": service.SessionAffinity,
	}

	if service.Selector != nil {
		result["selector"] = service.Selector
	}

	if len(service.PublicEndpoints) > 0 {
		endpoints := []map[string]interface{}{}
		for _, ep := range service.PublicEndpoints {
			endpointInfo := map[string]interface{}{
				"addresses": ep.Addresses,
				"port":      ep.Port,
				"protocol":  ep.Protocol,
			}
			endpoints = append(endpoints, endpointInfo)
		}
		result["endpoints"] = endpoints
	}

	// Perform diagnostic if requested
	if getPodDetails {
		// Fetch pods for this project namespace
		podList, err := rancherClient.ListPods(ctx, clusterID, projectID)
		if err != nil {
			logging.Warn("Failed to list pods: %v", err)
		}

		status := diagnoseService(ctx, rancherClient, clusterID, projectID, *service, podList)
		result["status"] = status
	}

	return formatSingleServiceResult(result, format)
}

// formatSingleServiceResult formats a single service result
func formatSingleServiceResult(result map[string]interface{}, format string) (string, error) {
	switch format {
	case common.FormatYAML:
		return common.FormatAsYAML(result)
	case common.FormatJSON:
		return common.FormatAsJSON(result)
	case common.FormatTable:
		// For table format (though not recommended for single objects), we still support it
		data := []map[string]string{}
		row := map[string]string{
			"id":        common.GetStringValue(result["id"]),
			"name":      common.GetStringValue(result["name"]),
			"namespace": common.GetStringValue(result["namespace"]),
			"type":      common.GetStringValue(result["type"]),
			"clusterIp": common.GetStringValue(result["clusterIp"]),
			"created":   common.GetStringValue(result["created"]),
		}

		if status, exists := result["status"]; exists {
			if diagStatus, ok := status.(ServiceDiagnosticStatus); ok {
				if diagStatus.Ready {
					row["ready"] = "Yes"
				} else {
					row["ready"] = "No"
				}

				if diagStatus.Degraded {
					row["degraded"] = "Yes"
				} else {
					row["degraded"] = "No"
				}
			}
		} else {
			row["ready"] = "-"
			row["degraded"] = "-"
		}

		data = append(data, row)
		return common.FormatAsTable(data, []string{"id", "name", "namespace", "type", "clusterIp", "ready", "degraded", "created"}), nil
	default:
		return "", fmt.Errorf("%w: %s", common.ErrInvalidFormat, format)
	}
}

// serviceListHandler handles the service_list tool with diagnostic capabilities
// It lists services across projects and optionally performs a full diagnostic chain
// check: Service → Pods
// The diagnostic chain is performed when getPodDetails=true using optimized API calls
// that fetch all pods once per project rather than for each service
func serviceListHandler(client interface{}, params map[string]interface{}) (string, error) {
	clusterID, err := common.ExtractRequiredString(params, "cluster")
	if err != nil {
		return "", err
	}

	projectID := common.ExtractOptionalString(params, "project", "")
	namespaceName := common.ExtractOptionalString(params, "namespace", "")
	getPodDetails := common.ExtractBool(params, "getPodDetails", false)
	format := common.ExtractFormat(params)

	rancherClient, err := common.ValidateRancherClient(client)
	if err != nil {
		return "", err
	}

	ctx := context.Background()

	// Pre-fetch pods for optimization when diagnostics are requested
	var podList []rancher.Pod

	if getPodDetails {
		if projectID != "" {
			// For single project, fetch once
			var err error
			podList, err = rancherClient.ListPods(ctx, clusterID, projectID)
			if err != nil {
				logging.Warn("Failed to list pods: %v", err)
			}
		}
	}

	// Collect services with diagnostic information
	serviceResults := []interface{}{}

	if projectID != "" {
		// Get services for the specified project
		results, err := getServicesWithDiagnostic(ctx, rancherClient, clusterID, projectID, namespaceName, getPodDetails, podList)
		if err != nil {
			return "", fmt.Errorf("failed to list services: %v", err)
		}
		serviceResults = results
	} else {
		// Get all projects for the cluster
		projects, err := rancherClient.ListProjects(ctx, clusterID)
		if err != nil {
			return "", fmt.Errorf("failed to list projects for cluster %s: %v", clusterID, err)
		}

		// Collect services from each project
		for _, project := range projects {
			// Fetch pods for this specific project when diagnostics enabled
			if getPodDetails {
				var err error
				podList, err = rancherClient.ListPods(ctx, clusterID, project.ID)
				if err != nil {
					logging.Warn("Failed to list pods for project %s: %v", project.ID, err)
				}
			}

			results, err := getServicesWithDiagnostic(ctx, rancherClient, clusterID, project.ID, namespaceName, getPodDetails, podList)
			if err != nil {
				// Skip projects that fail
				continue
			}
			serviceResults = append(serviceResults, results...)
		}
	}

	if len(serviceResults) == 0 {
		return common.FormatEmptyResult(format)
	}

	return formatServiceResults(serviceResults, format)
}

// getServicesWithDiagnostic lists services with diagnostic information
// When getPodDetails is true, it performs diagnostic checks using the provided podList
// to avoid redundant API calls for each service
func getServicesWithDiagnostic(ctx context.Context,
	rancherClient *rancher.Client,
	clusterID, projectID, namespaceName string,
	getPodDetails bool,
	podList []rancher.Pod) ([]interface{}, error) {

	services, err := rancherClient.ListServices(ctx, clusterID, projectID)
	if err != nil {
		return nil, err
	}

	results := []interface{}{}
	for _, svc := range services {
		// Apply namespace filter
		if namespaceName != "" && svc.NamespaceId != namespaceName {
			continue
		}

		// Build basic service info
		serviceInfo := map[string]interface{}{
			"id":          svc.ID,
			"name":        svc.Name,
			"namespace":   svc.NamespaceId,
			"state":       svc.State,
			"created":     common.FormatTime(svc.Created),
			"annotations": svc.Annotations,
			"labels":      svc.Labels,
		}

		// Add service type
		svcType := svc.Kind
		if svcType == "" {
			svcType = "ClusterIP"
		}
		serviceInfo["type"] = svcType

		// Add cluster IP
		clusterIP := svc.ClusterIp
		if clusterIP == "" {
			clusterIP = "-"
		}
		serviceInfo["cluster_ip"] = clusterIP

		// Add ports
		if len(svc.PublicEndpoints) > 0 {
			endpoints := []map[string]interface{}{}
			for _, ep := range svc.PublicEndpoints {
				endpointInfo := map[string]interface{}{
					"addresses": ep.Addresses,
					"port":      ep.Port,
					"protocol":  ep.Protocol,
				}
				endpoints = append(endpoints, endpointInfo)
			}
			serviceInfo["endpoints"] = endpoints
		}

		// Perform diagnostic if requested
		if getPodDetails {
			diagnosticStatus := diagnoseService(ctx, rancherClient, clusterID, projectID, svc, podList)
			serviceInfo["status"] = diagnosticStatus
		}

		results = append(results, serviceInfo)
	}

	return results, nil
}

// formatServiceResults formats the service results based on output format
func formatServiceResults(results []interface{}, format string) (string, error) {
	if len(results) == 0 {
		return common.FormatEmptyResult(format)
	}

	switch format {
	case common.FormatYAML:
		return common.FormatAsYAML(results)
	case common.FormatJSON:
		return common.FormatAsJSON(results)
	case common.FormatTable:
		// For table format, extract basic information
		tableData := []map[string]string{}
		for _, result := range results {
			if svc, ok := result.(map[string]interface{}); ok {
				row := map[string]string{
					"id":         common.GetStringValue(svc["id"]),
					"name":       common.GetStringValue(svc["name"]),
					"namespace":  common.GetStringValue(svc["namespace"]),
					"type":       common.GetStringValue(svc["type"]),
					"cluster_ip": common.GetStringValue(svc["cluster_ip"]),
					"state":      common.GetStringValue(svc["state"]),
					"created":    common.GetStringValue(svc["created"]),
				}

				// Determine ready status
				if status, exists := svc["status"]; exists {
					if diagStatus, ok := status.(ServiceDiagnosticStatus); ok {
						if diagStatus.Ready {
							row["ready"] = "Yes"
						} else {
							row["ready"] = "No"
						}
					}
				} else {
					row["ready"] = "-"
				}

				tableData = append(tableData, row)
			}
		}

		return common.FormatAsTable(tableData, []string{"id", "name", "namespace", "type", "cluster_ip", "state", "ready", "created"}), nil
	default:
		return "", fmt.Errorf("%w: %s", common.ErrInvalidFormat, format)
	}
}
