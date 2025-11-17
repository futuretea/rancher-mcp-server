package networking

import (
	"context"
	"fmt"

	"github.com/futuretea/rancher-mcp-server/pkg/logging"
	"github.com/futuretea/rancher-mcp-server/pkg/rancher"
	"github.com/futuretea/rancher-mcp-server/pkg/toolsets/common"
)

// ingressGetHandler handles the ingress_get tool for single ingress queries
// It retrieves a single ingress by name and optionally performs diagnostic checks
// on all backend paths. Unlike ingressListHandler, this fetches services/pods
// only for the specific ingress being queried.
func ingressGetHandler(client interface{}, params map[string]interface{}) (string, error) {
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

	// Get the ingress
	ingress, err := rancherClient.GetIngress(ctx, clusterID, projectID, namespace, name)
	if err != nil {
		return "", fmt.Errorf("failed to get ingress: %v", err)
	}

	// Build the result
	result := map[string]interface{}{
		"id":          ingress.ID,
		"name":        ingress.Name,
		"namespace":   ingress.NamespaceId,
		"state":       ingress.State,
		"created":     common.FormatTime(ingress.Created),
		"annotations": ingress.Annotations,
	}

	// Add optional fields
	if ingress.IngressClassName != "" {
		result["ingressClassName"] = ingress.IngressClassName
	}

	if len(ingress.Rules) > 0 {
		hosts := []string{}
		for _, rule := range ingress.Rules {
			if rule.Host != "" {
				hosts = append(hosts, rule.Host)
			}
		}
		if len(hosts) > 0 {
			result["hosts"] = hosts
		}

		paths := []string{}
		for _, rule := range ingress.Rules {
			for _, path := range rule.Paths {
				if path.Path != "" {
					paths = append(paths, path.Path)
				}
			}
		}
		if len(paths) > 0 {
			result["paths"] = paths
		}
	}

	if len(ingress.PublicEndpoints) > 0 {
		endpoints := []map[string]interface{}{}
		for _, ep := range ingress.PublicEndpoints {
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
		// Pre-fetch services and pods for this project namespace
		var serviceList []rancher.Service
		var podList []rancher.Pod

		serviceList, err = rancherClient.ListServices(ctx, clusterID, projectID)
		if err != nil {
			logging.Warn("Failed to list services: %v", err)
		}
		podList, err = rancherClient.ListPods(ctx, clusterID, projectID)
		if err != nil {
			logging.Warn("Failed to list pods: %v", err)
		}

		diagnosticStatus := IngressDiagnosticStatus{
			Ready:        true, // Start true, set to false if any backend fails
			PathStatus:   map[string]IngressPathDiagnosticStatus{},
			LoadBalancer: map[string]interface{}{},
		}

		// Process rules
		for _, rule := range ingress.Rules {
			for _, path := range rule.Paths {
				pathKey := rule.Host
				if pathKey == "" {
					pathKey = "_"
				}
				if path.Path != "" {
					pathKey = pathKey + path.Path
				}

				pathStatus := diagnoseIngressPath(ctx, rancherClient, namespace, path, serviceList, podList)
				logging.Debug("Ingress path diagnostic status: %+v", pathStatus)
				diagnosticStatus.PathStatus[pathKey] = pathStatus

				// All paths must be ready for the ingress to be ready (AND operation)
				if !pathStatus.Ready {
					diagnosticStatus.Ready = false
				}
			}
		}

		// Process default backend
		if ingress.DefaultBackend != nil {
			pathStatus := diagnoseIngressBackend(ctx, rancherClient, namespace, ingress.DefaultBackend, serviceList, podList)
			diagnosticStatus.PathStatus["default"] = pathStatus

			// Default backend must also be ready (AND operation)
			if !pathStatus.Ready {
				diagnosticStatus.Ready = false
			}
		}

		// If no backends at all, it's not configured properly
		if len(ingress.Rules) == 0 && ingress.DefaultBackend == nil {
			diagnosticStatus.Ready = false
		}

		// Add load balancer addresses
		for _, ep := range ingress.PublicEndpoints {
			if len(ep.Addresses) > 0 {
				diagnosticStatus.LoadBalancer["addresses"] = ep.Addresses
				break
			}
		}

		logging.Debug("Diagnosing ingress status: %+v", diagnosticStatus)

		result["status"] = diagnosticStatus
	}

	return formatSingleIngressResult(result, format)
}

// formatSingleIngressResult formats a single ingress result
func formatSingleIngressResult(result map[string]interface{}, format string) (string, error) {
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
			"state":     common.GetStringValue(result["state"]),
			"created":   common.GetStringValue(result["created"]),
		}

		if status, exists := result["status"]; exists {
			if diagStatus, ok := status.(IngressDiagnosticStatus); ok {
				if diagStatus.Ready {
					row["ready"] = "Yes"
				} else {
					row["ready"] = "No"
				}
			}
		} else {
			row["ready"] = "-"
		}

		data = append(data, row)
		return common.FormatAsTable(data, []string{"id", "name", "namespace", "state", "ready", "created"}), nil
	default:
		return "", fmt.Errorf("%w: %s", common.ErrInvalidFormat, format)
	}
}

// ingressListHandler handles the ingress_list tool with diagnostic capabilities
// It lists ingresses across projects and optionally performs a full diagnostic chain
// check: Ingress → Service → Pods
// The diagnostic chain is performed when getPodDetails=true using optimized API calls
// that fetch all services and pods once per project rather than for each ingress path
func ingressListHandler(client interface{}, params map[string]interface{}) (string, error) {
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

	// Collect ingresses with diagnostic information
	var ingressResults []interface{}

	// Pre-fetch services and pods for optimization when diagnostics are requested
	var serviceList []rancher.Service
	var podList []rancher.Pod

	if getPodDetails {
		// Fetch all services and pods in the project(s) for use in diagnostics
		if projectID != "" {
			// For single project, fetch once
			var err error
			serviceList, err = rancherClient.ListServices(ctx, clusterID, projectID)
			if err != nil {
				logging.Warn("Failed to list services: %v", err)
			}
			podList, err = rancherClient.ListPods(ctx, clusterID, projectID)
			if err != nil {
				logging.Warn("Failed to list pods: %v", err)
			}
		}
	}

	if projectID != "" {
		// Get ingresses for the specified project
		results, err := getIngressesWithDiagnostic(ctx, rancherClient, clusterID, projectID, namespaceName, getPodDetails, serviceList, podList)
		if err != nil {
			return "", fmt.Errorf("failed to list ingresses: %v", err)
		}
		ingressResults = results
	} else {
		// Get all projects for the cluster
		projects, err := rancherClient.ListProjects(ctx, clusterID)
		if err != nil {
			return "", fmt.Errorf("failed to list projects for cluster %s: %v", clusterID, err)
		}

		// Collect ingresses from each project
		for _, project := range projects {
			// Fetch services and pods for this specific project
			if getPodDetails {
				var err error
				serviceList, err = rancherClient.ListServices(ctx, clusterID, project.ID)
				if err != nil {
					logging.Warn("Failed to list services for project %s: %v", project.ID, err)
				}
				podList, err = rancherClient.ListPods(ctx, clusterID, project.ID)
				if err != nil {
					logging.Warn("Failed to list pods for project %s: %v", project.ID, err)
				}
			}

			results, err := getIngressesWithDiagnostic(ctx, rancherClient, clusterID, project.ID, namespaceName, getPodDetails, serviceList, podList)
			if err != nil {
				// Skip projects that fail
				continue
			}
			ingressResults = append(ingressResults, results...)
		}
	}

	if len(ingressResults) == 0 {
		return common.FormatEmptyResult(format)
	}

	return formatIngressResults(ingressResults, format, getPodDetails)
}

// getIngressesWithDiagnostic lists ingresses with diagnostic information
func getIngressesWithDiagnostic(ctx context.Context,
	rancherClient *rancher.Client,
	clusterID, projectID, namespaceName string,
	getPodDetails bool,
	serviceList []rancher.Service,
	podList []rancher.Pod) ([]interface{}, error) {

	ingresses, err := rancherClient.ListIngresses(ctx, clusterID, projectID)
	if err != nil {
		return nil, err
	}

	results := []interface{}{}
	for _, ing := range ingresses {
		// Apply namespace filter
		if namespaceName != "" && ing.NamespaceId != namespaceName {
			continue
		}

		// Build basic ingress info
		ingressInfo := map[string]interface{}{
			"id":          ing.ID,
			"name":        ing.Name,
			"namespace":   ing.NamespaceId,
			"state":       ing.State,
			"created":     common.FormatTime(ing.Created),
			"annotations": ing.Annotations,
		}

		// Add IngressClassName if present
		if ing.IngressClassName != "" {
			ingressInfo["ingressClassName"] = ing.IngressClassName
		}

		// Add basic status info
		diagnosticStatus := IngressDiagnosticStatus{
			Ready:        true, // Start true, set to false if any backend fails
			PathStatus:   map[string]IngressPathDiagnosticStatus{},
			LoadBalancer: map[string]interface{}{},
		}

		// Extract hosts information
		hosts := []string{}
		if len(ing.Rules) > 0 {
			for _, rule := range ing.Rules {
				if rule.Host != "" {
					hosts = append(hosts, rule.Host)
				}

				// Process HTTP paths for diagnostic
				if len(rule.Paths) > 0 {
					for _, path := range rule.Paths {
						pathKey := rule.Host
						if pathKey == "" {
							pathKey = "_"
						}
						if path.Path != "" {
							pathKey = pathKey + path.Path
						}

						// Perform diagnostic if requested
						if getPodDetails {
							pathStatus := diagnoseIngressPath(ctx, rancherClient, ing.NamespaceId, path, serviceList, podList)
							logging.Debug("Ingress path diagnostic status: %+v", pathStatus)
							diagnosticStatus.PathStatus[pathKey] = pathStatus

							// All paths must be ready for the ingress to be ready (AND operation)
							if !pathStatus.Ready {
								diagnosticStatus.Ready = false
							}
						}
					}
				}
			}
		}

		// Process default backend if present
		if ing.DefaultBackend != nil && getPodDetails {
			pathStatus := diagnoseIngressBackend(ctx, rancherClient, ing.NamespaceId, ing.DefaultBackend, serviceList, podList)
			diagnosticStatus.PathStatus["default"] = pathStatus

			// Default backend must also be ready (AND operation)
			if !pathStatus.Ready {
				diagnosticStatus.Ready = false
			}
		}

		// If no backends at all, it's not configured properly
		if getPodDetails && len(ing.Rules) == 0 && ing.DefaultBackend == nil {
			diagnosticStatus.Ready = false
		}

		// Add diagnostic status if requested
		if getPodDetails {
			ingressInfo["status"] = diagnosticStatus
		}

		// Add hosts and paths
		if len(hosts) > 0 {
			ingressInfo["hosts"] = hosts
		}

		// Collect paths
		paths := []string{}
		if len(ing.Rules) > 0 {
			for _, rule := range ing.Rules {
				for _, path := range rule.Paths {
					if path.Path != "" {
						paths = append(paths, path.Path)
					}
				}
			}
		}
		if len(paths) > 0 {
			ingressInfo["paths"] = paths
		}

		// Add public endpoints (load balancer addresses)
		if len(ing.PublicEndpoints) > 0 {
			endpoints := []map[string]interface{}{}
			for _, ep := range ing.PublicEndpoints {
				endpointInfo := map[string]interface{}{
					"addresses": ep.Addresses,
					"port":      ep.Port,
					"protocol":  ep.Protocol,
				}
				endpoints = append(endpoints, endpointInfo)

				// Add addresses to diagnostic status (only set once from the first endpoint)
				if getPodDetails && len(ep.Addresses) > 0 {
					if _, exists := diagnosticStatus.LoadBalancer["addresses"]; !exists {
						diagnosticStatus.LoadBalancer["addresses"] = ep.Addresses
					}
				}
			}
			ingressInfo["endpoints"] = endpoints
		}

		results = append(results, ingressInfo)
	}

	return results, nil
}
