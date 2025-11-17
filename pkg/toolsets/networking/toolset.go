package networking

import (
	"context"
	"fmt"
	"strings"

	"github.com/futuretea/rancher-mcp-server/pkg/api"
	"github.com/futuretea/rancher-mcp-server/pkg/output"
	"github.com/futuretea/rancher-mcp-server/pkg/rancher"
	"github.com/futuretea/rancher-mcp-server/pkg/logging"
	"github.com/mark3labs/mcp-go/mcp"
)

// Toolset implements the networking toolset
type Toolset struct{}

var _ api.Toolset = (*Toolset)(nil)

// GetName returns the name of the toolset
func (t *Toolset) GetName() string {
	return "networking"
}

// GetDescription returns the description of the toolset
func (t *Toolset) GetDescription() string {
	return "Networking operations for managing ingresses and network policies"
}

// GetTools returns the tools provided by this toolset
func (t *Toolset) GetTools(client interface{}) []api.ServerTool {
	return []api.ServerTool{
		{
			Tool: mcp.Tool{
				Name:        "ingress_get",
				Description: "Get a single ingress by name with diagnostic chain check (Ingress → Service → Pods), more efficient than list",
				InputSchema: mcp.ToolInputSchema{
					Type:     "object",
					Required: []string{"cluster", "namespace", "name"},
					Properties: map[string]any{
						"cluster": map[string]any{
							"type":        "string",
							"description": "Cluster ID",
						},
						"namespace": map[string]any{
							"type":        "string",
							"description": "Namespace name",
						},
						"name": map[string]any{
							"type":        "string",
							"description": "Ingress name to get",
						},
						"project": map[string]any{
							"type":        "string",
							"description": "Project ID (optional, will auto-detect if not provided)",
							"default":     "",
						},
						"getPodDetails": map[string]any{
							"type":        "boolean",
							"description": "Get detailed pod information and perform health checks",
							"default":     false,
						},
						"format": map[string]any{
							"type":        "string",
							"description": "Output format: json or yaml",
							"enum":        []string{"json", "yaml"},
							"default":     "json",
						},
					},
				},
			},
			Annotations: api.ToolAnnotations{
				ReadOnlyHint: boolPtr(true),
			},
			Handler: ingressGetHandler,
		},
		{
			Tool: mcp.Tool{
				Name:        "ingress_list",
				Description: "List ingresses with full diagnostic chain check (Ingress → Service → Pods)",
				InputSchema: mcp.ToolInputSchema{
					Type:     "object",
					Required: []string{"cluster"},
					Properties: map[string]any{
						"cluster": map[string]any{
							"type":        "string",
							"description": "Cluster ID",
						},
						"project": map[string]any{
							"type":        "string",
							"description": "Project ID to filter ingresses (optional)",
							"default":     "",
						},
						"namespace": map[string]any{
							"type":        "string",
							"description": "Namespace name to filter ingresses (optional)",
							"default":     "",
						},
						"getPodDetails": map[string]any{
							"type":        "boolean",
							"description": "Get detailed pod information and perform health checks",
							"default":     false,
						},
						"format": map[string]any{
							"type":        "string",
							"description": "Output format: json, table, or yaml",
							"enum":        []string{"json", "table", "yaml"},
							"default":     "json",
						},
					},
				},
			},
			Annotations: api.ToolAnnotations{
				ReadOnlyHint: boolPtr(true),
			},
			Handler: ingressListHandler,
		},
	}
}

// ingressListHandler handles the ingress_list tool with diagnostic capabilities
// It lists ingresses across projects and optionally performs a full diagnostic chain
// check: Ingress → Service → Pods
// The diagnostic chain is performed when getPodDetails=true using optimized API calls
// that fetch all services and pods once per project rather than for each ingress path
func ingressListHandler(client interface{}, params map[string]interface{}) (string, error) {
	clusterID := ""
	if clusterParam, ok := params["cluster"].(string); ok {
		clusterID = clusterParam
	}

	if clusterID == "" {
		return "", fmt.Errorf("cluster parameter is required")
	}

	projectID := ""
	if projectParam, ok := params["project"].(string); ok {
		projectID = projectParam
	}

	namespaceName := ""
	if namespaceParam, ok := params["namespace"].(string); ok {
		namespaceName = namespaceParam
	}

	getPodDetails := false
	if detailsParam, ok := params["getPodDetails"].(bool); ok {
		getPodDetails = detailsParam
	}

	format := "json"
	if formatParam, ok := params["format"].(string); ok {
		format = formatParam
	}

	rancherClient, ok := client.(*rancher.Client)
	if !ok || rancherClient == nil || !rancherClient.IsConfigured() {
		return "", fmt.Errorf("Rancher client not configured. Please configure Rancher credentials to use this tool")
	}

	ctx := context.Background()

	// Collect ingresses with diagnostic information
	var ingressResults []interface{}

	// Pre-fetch services and pods for optimization when diagnostics are requested
	var serviceList []rancher.Service
	var podList []rancher.Pod
	var err error

	if getPodDetails {
		// Fetch all services and pods in the project(s) for use in diagnostics
		if projectID != "" {
			// For single project, fetch once
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
		return "No ingresses found", nil
	}

	return formatIngressResults(ingressResults, format, getPodDetails)
}

// ingressGetHandler handles the ingress_get tool for single ingress queries
// It retrieves a single ingress by name and optionally performs diagnostic checks
// on all backend paths. Unlike ingressListHandler, this fetches services/pods
// only for the specific ingress being queried.
func ingressGetHandler(client interface{}, params map[string]interface{}) (string, error) {
	// Extract required parameters
	clusterID := ""
	if clusterParam, ok := params["cluster"].(string); ok {
		clusterID = clusterParam
	}
	if clusterID == "" {
		return "", fmt.Errorf("cluster parameter is required")
	}

	namespace := ""
	if namespaceParam, ok := params["namespace"].(string); ok {
		namespace = namespaceParam
	}
	if namespace == "" {
		return "", fmt.Errorf("namespace parameter is required")
	}

	name := ""
	if nameParam, ok := params["name"].(string); ok {
		name = nameParam
	}
	if name == "" {
		return "", fmt.Errorf("name parameter is required")
	}

	// Extract optional parameters
	projectID := ""
	if projectParam, ok := params["project"].(string); ok {
		projectID = projectParam
	}

	getPodDetails := false
	if detailsParam, ok := params["getPodDetails"].(bool); ok {
		getPodDetails = detailsParam
	}

	format := "json"
	if formatParam, ok := params["format"].(string); ok {
		format = formatParam
	}

	rancherClient, ok := client.(*rancher.Client)
	if !ok || rancherClient == nil || !rancherClient.IsConfigured() {
		return "", fmt.Errorf("Rancher client not configured. Please configure Rancher credentials to use this tool")
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
		"created":     formatTime(ingress.Created),
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
	case "yaml":
		return formatAsYAML(result), nil
	case "json":
		return formatAsJSON(result), nil
	default:
		// For table format (though not recommended for single objects), we still support it
		data := []map[string]string{}
		row := map[string]string{
			"id":        getStringValue(result["id"]),
			"name":      getStringValue(result["name"]),
			"namespace": getStringValue(result["namespace"]),
			"state":     getStringValue(result["state"]),
			"created":   getStringValue(result["created"]),
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
		return formatAsTable(data, []string{"id", "name", "namespace", "state", "ready", "created"}), nil
	}
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
			"created":     formatTime(ing.Created),
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

// formatIngressResults formats the ingress results based on output format
func formatIngressResults(results []interface{}, format string, getPodDetails bool) (string, error) {
	if len(results) == 0 {
		return "No ingresses found", nil
	}

	switch format {
	case "yaml":
		return formatAsYAML(results), nil
	case "json":
		return formatAsJSON(results), nil
	default:
		// For table format, extract comprehensive information
		tableData := []map[string]string{}
		for _, result := range results {
			if ingress, ok := result.(map[string]interface{}); ok {
				row := map[string]string{
					"name":      getStringValue(ingress["name"]),
					"namespace": getStringValue(ingress["namespace"]),
					"state":     getStringValue(ingress["state"]),
					"created":   getStringValue(ingress["created"]),
				}

				// Get hosts info
				if hosts, exists := ingress["hosts"]; exists {
					if hostList, ok := hosts.([]string); ok && len(hostList) > 0 {
						// Show first host, truncate if too many
						hostsStr := strings.Join(hostList, ",")
						if len(hostsStr) > 40 {
							hostsStr = hostsStr[:37] + "..."
						}
						row["hosts"] = hostsStr
					} else {
						row["hosts"] = "-"
					}
				} else {
					row["hosts"] = "-"
				}

				// Get paths info
				if paths, exists := ingress["paths"]; exists {
					if pathList, ok := paths.([]string); ok && len(pathList) > 0 {
						// Show paths, truncate if too many
						pathsStr := strings.Join(pathList, ",")
						if len(pathsStr) > 30 {
							pathsStr = pathsStr[:27] + "..."
						}
						row["paths"] = pathsStr
					} else {
						row["paths"] = "-"
					}
				} else {
					row["paths"] = "-"
				}

				// Get addresses (load balancer)
				if endpoints, exists := ingress["endpoints"]; exists {
					if endpointList, ok := endpoints.([]map[string]interface{}); ok && len(endpointList) > 0 {
						if firstEndpoint := endpointList[0]; firstEndpoint != nil {
							if addrs, ok := firstEndpoint["addresses"].([]string); ok && len(addrs) > 0 {
								// Show first address, indicate more if multiple
								addrStr := strings.Join(addrs, ",")
								if len(addrStr) > 30 {
									addrStr = addrs[0]
									if len(addrs) > 1 {
										addrStr = addrStr + "..."
									}
								}
								row["addresses"] = addrStr
							} else {
								row["addresses"] = "-"
							}
						} else {
							row["addresses"] = "-"
						}
					} else {
						row["addresses"] = "-"
					}
				} else {
					row["addresses"] = "-"
				}

				// Determine ready status
				if status, exists := ingress["status"]; exists {
					if diagStatus, ok := status.(IngressDiagnosticStatus); ok {
						if diagStatus.Ready {
							row["ready"] = "✓"
						} else {
							row["ready"] = "✗"
						}
					} else {
						row["ready"] = "?"
					}
				} else {
					row["ready"] = "-"
				}

				// Show health summary if diagnostic was run
				if getPodDetails {
					if status, exists := ingress["status"]; exists {
						if diagStatus, ok := status.(IngressDiagnosticStatus); ok && len(diagStatus.PathStatus) > 0 {
							// Count healthy/unhealthy paths
							healthyPaths := 0
							totalPaths := 0
							for _, ps := range diagStatus.PathStatus {
								totalPaths++
								if ps.Ready {
									healthyPaths++
								}
							}
							row["health"] = fmt.Sprintf("%d/%d", healthyPaths, totalPaths)
						} else {
							row["health"] = "-"
						}
					} else {
						row["health"] = "-"
					}
				} else {
					row["health"] = "-"
				}

				tableData = append(tableData, row)
			}
		}

		return formatAsTable(tableData, []string{"name", "namespace", "state", "ready", "health", "hosts", "paths", "addresses", "created"}), nil
	}
}

// Helper function to get string value from interface
func getStringValue(v interface{}) string {
	if str, ok := v.(string); ok {
		return str
	}
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("%v", v)
}

// Helper functions for formatting
func formatAsTable(data []map[string]string, headers []string) string {
	formatter := output.NewFormatter()
	return formatter.FormatTableWithHeaders(data, headers)
}

func formatAsYAML(data interface{}) string {
	formatter := output.NewFormatter()
	result, err := formatter.FormatYAML(data)
	if err != nil {
		return fmt.Sprintf("Error formatting YAML: %v", err)
	}
	return result
}

func formatAsJSON(data interface{}) string {
	formatter := output.NewFormatter()
	result, err := formatter.FormatJSON(data)
	if err != nil {
		return fmt.Sprintf("Error formatting JSON: %v", err)
	}
	return result
}

func boolPtr(b bool) *bool {
	return &b
}

// formatTime formats time for display
func formatTime(timestamp string) string {
	if timestamp == "" {
		return "-"
	}
	// For now, just return the timestamp as-is
	// In a real implementation, you might want to parse and format it
	return timestamp
}

func init() {
	// Register this toolset
	// This will be implemented when we have the toolsets registry
}
