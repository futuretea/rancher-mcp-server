package networking

import (
	"fmt"
	"strings"

	"github.com/futuretea/rancher-mcp-server/pkg/toolsets/common"
)

// formatIngressResults formats the ingress results based on output format
func formatIngressResults(results []interface{}, format string, getPodDetails bool) (string, error) {
	if len(results) == 0 {
		return common.FormatEmptyResult(format)
	}

	switch format {
	case common.FormatYAML:
		return common.FormatAsYAML(results)
	case common.FormatJSON:
		return common.FormatAsJSON(results)
	case common.FormatTable:
		// For table format, extract comprehensive information
		tableData := []map[string]string{}
		for _, result := range results {
			if ingress, ok := result.(map[string]interface{}); ok {
				row := map[string]string{
					"name":      common.GetStringValue(ingress["name"]),
					"namespace": common.GetStringValue(ingress["namespace"]),
					"state":     common.GetStringValue(ingress["state"]),
					"created":   common.GetStringValue(ingress["created"]),
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

		return common.FormatAsTable(tableData, []string{"name", "namespace", "state", "ready", "health", "hosts", "paths", "addresses", "created"}), nil
	default:
		return "", fmt.Errorf("%w: %s", common.ErrInvalidFormat, format)
	}
}
