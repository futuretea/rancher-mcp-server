package networking

import (
	"context"
	"fmt"
	"strings"

	"github.com/futuretea/rancher-mcp-server/pkg/api"
	"github.com/futuretea/rancher-mcp-server/pkg/output"
	"github.com/futuretea/rancher-mcp-server/pkg/rancher"
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
				Name:        "ingress_list",
				Description: "List all ingresses in a cluster",
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
						"format": map[string]any{
							"type":        "string",
							"description": "Output format: table, yaml, or json",
							"enum":        []string{"table", "yaml", "json"},
							"default":     "table",
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

// ingressListHandler handles the ingress_list tool
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

	format := "table"
	if formatParam, ok := params["format"].(string); ok {
		format = formatParam
	}

	rancherClient, ok := client.(*rancher.Client)
	if !ok || rancherClient == nil || !rancherClient.IsConfigured() {
		return "", fmt.Errorf("Rancher client not configured. Please configure Rancher credentials to use this tool")
	}

	ctx := context.Background()

	// Collect ingresses from all projects or specific project
	resultMaps := make([]map[string]string, 0)

	if projectID != "" {
		// Get ingresses for the specified project
		ingresses, err := rancherClient.ListIngresses(ctx, clusterID, projectID)
		if err != nil {
			return "", fmt.Errorf("failed to list ingresses for cluster %s, project %s: %v", clusterID, projectID, err)
		}

		for _, ing := range ingresses {
			// Apply namespace filter
			if namespaceName != "" && ing.NamespaceId != namespaceName {
				continue
			}

			// Extract hosts and paths from rules
			hosts := make([]string, 0)
			paths := make([]string, 0)
			if len(ing.Rules) > 0 {
				for _, rule := range ing.Rules {
					if rule.Host != "" {
						hosts = append(hosts, rule.Host)
					}
					if len(rule.Paths) > 0 {
						for _, path := range rule.Paths {
							paths = append(paths, path.Path)
						}
					}
				}
			}

			hostsStr := "-"
			if len(hosts) > 0 {
				hostsStr = strings.Join(hosts, ",")
			}

			pathsStr := "-"
			if len(paths) > 0 {
				pathsStr = strings.Join(paths, ",")
			}

			// Get address from status
			addresses := "-"
			if len(ing.PublicEndpoints) > 0 {
				addrs := make([]string, 0, len(ing.PublicEndpoints))
				for _, ep := range ing.PublicEndpoints {
					if len(ep.Addresses) > 0 {
						addrs = append(addrs, ep.Addresses...)
					}
				}
				if len(addrs) > 0 {
					addresses = strings.Join(addrs, ",")
				}
			}

			resultMaps = append(resultMaps, map[string]string{
				"id":        ing.ID,
				"name":      ing.Name,
				"namespace": ing.NamespaceId,
				"hosts":     hostsStr,
				"paths":     pathsStr,
				"addresses": addresses,
				"created":   formatTime(ing.Created),
			})
		}
	} else {
		// Get all projects for the cluster
		projects, err := rancherClient.ListProjects(ctx, clusterID)
		if err != nil {
			return "", fmt.Errorf("failed to list projects for cluster %s: %v", clusterID, err)
		}

		// Collect ingresses from each project
		for _, project := range projects {
			ingresses, err := rancherClient.ListIngresses(ctx, clusterID, project.ID)
			if err != nil {
				// Skip projects that fail
				continue
			}

			for _, ing := range ingresses {
				// Apply namespace filter
				if namespaceName != "" && ing.NamespaceId != namespaceName {
					continue
				}

				// Extract hosts and paths from rules
				hosts := make([]string, 0)
				paths := make([]string, 0)
				if len(ing.Rules) > 0 {
					for _, rule := range ing.Rules {
						if rule.Host != "" {
							hosts = append(hosts, rule.Host)
						}
						if len(rule.Paths) > 0 {
							for _, path := range rule.Paths {
								paths = append(paths, path.Path)
							}
						}
					}
				}

				hostsStr := "-"
				if len(hosts) > 0 {
					hostsStr = strings.Join(hosts, ",")
				}

				pathsStr := "-"
				if len(paths) > 0 {
					pathsStr = strings.Join(paths, ",")
				}

				// Get address from status
				addresses := "-"
				if len(ing.PublicEndpoints) > 0 {
					addrs := make([]string, 0, len(ing.PublicEndpoints))
					for _, ep := range ing.PublicEndpoints {
						if len(ep.Addresses) > 0 {
							addrs = append(addrs, ep.Addresses...)
						}
					}
					if len(addrs) > 0 {
						addresses = strings.Join(addrs, ",")
					}
				}

				resultMaps = append(resultMaps, map[string]string{
					"id":        ing.ID,
					"name":      ing.Name,
					"namespace": ing.NamespaceId,
					"hosts":     hostsStr,
					"paths":     pathsStr,
					"addresses": addresses,
					"created":   formatTime(ing.Created),
				})
			}
		}
	}

	if len(resultMaps) == 0 {
		return "No ingresses found", nil
	}

	switch format {
	case "yaml":
		return formatAsYAML(resultMaps), nil
	case "json":
		return formatAsJSON(resultMaps), nil
	default:
		return formatAsTable(resultMaps, []string{"id", "name", "namespace", "hosts", "paths", "addresses", "created"}), nil
	}
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
