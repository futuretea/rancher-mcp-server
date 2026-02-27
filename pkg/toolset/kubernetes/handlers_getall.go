package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/futuretea/rancher-mcp-server/pkg/client/steve"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset/paramutil"
	"gopkg.in/yaml.v3"
)

// getAllHandler handles the kubernetes_get_all tool (inspired by ketall)
func getAllHandler(ctx context.Context, client interface{}, params map[string]interface{}) (string, error) {
	steveClient, err := toolset.ValidateSteveClient(client)
	if err != nil {
		return "", err
	}

	cluster, err := paramutil.ExtractRequiredString(params, paramutil.ParamCluster)
	if err != nil {
		return "", err
	}

	// Extract optional parameters
	namespace := paramutil.ExtractOptionalString(params, paramutil.ParamNamespace)
	format := paramutil.ExtractFormat(params)
	nameFilter := paramutil.ExtractOptionalString(params, paramutil.ParamName)
	labelSelector := paramutil.ExtractOptionalString(params, paramutil.ParamLabelSelector)
	excludeEvents := paramutil.ExtractBool(params, "excludeEvents", true)
	scope := paramutil.ExtractOptionalString(params, "scope")
	since := paramutil.ExtractOptionalString(params, "since")
	limit := paramutil.ExtractInt64(params, paramutil.ParamLimit, 0)

	// Validate scope parameter
	if scope != "" && scope != "namespaced" && scope != "cluster" {
		return "", fmt.Errorf("invalid scope: %s (must be 'namespaced', 'cluster', or empty)", scope)
	}

	// Parse since duration if provided
	var sinceTime *time.Time
	if since != "" {
		duration, err := parseDuration(since)
		if err != nil {
			return "", fmt.Errorf("invalid since duration: %w", err)
		}
		t := time.Now().Add(-duration)
		sinceTime = &t
	}

	// Use the Steve client's GetAllResources method
	opts := &steve.GetAllOptions{
		Namespace:     namespace,
		ExcludeEvents: excludeEvents,
		Scope:         scope,
		Limit:         limit,
	}

	result, err := steveClient.GetAllResources(ctx, cluster, opts)
	if err != nil {
		return "", fmt.Errorf("failed to get all resources: %w", err)
	}

	// Apply filters
	filteredItems := filterAllResources(result.Items, nameFilter, labelSelector, sinceTime)

	// Format and return result
	return formatAllResources(filteredItems, format)
}

// filterAllResources applies client-side filters to the resource list.
func filterAllResources(items []steve.AllResourceItem, nameFilter, labelSelector string, sinceTime *time.Time) []steve.AllResourceItem {
	if nameFilter == "" && labelSelector == "" && sinceTime == nil {
		return items
	}

	var filtered []steve.AllResourceItem

	for _, item := range items {
		// Filter by name (partial match, case-insensitive)
		if nameFilter != "" {
			if !strings.Contains(strings.ToLower(item.Name), strings.ToLower(nameFilter)) {
				continue
			}
		}

		// Filter by label selector (simple implementation)
		if labelSelector != "" {
			if item.Resource == nil {
				continue
			}
			labels := item.Resource.GetLabels()
			if !matchesLabelSelector(labels, labelSelector) {
				continue
			}
		}

		// Filter by creation time (since)
		if sinceTime != nil && item.Resource != nil {
			creationTime := item.Resource.GetCreationTimestamp()
			if creationTime.IsZero() || creationTime.Time.Before(*sinceTime) {
				continue
			}
		}

		filtered = append(filtered, item)
	}

	return filtered
}

// matchesLabelSelector performs simple label selector matching.
// Supports format: "key=value,key2=value2" or "key=value" (single).
func matchesLabelSelector(labels map[string]string, selector string) bool {
	if selector == "" {
		return true
	}

	// Parse label selector (simple implementation)
	pairs := strings.Split(selector, ",")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		// Check for existence operator: "key"
		if !strings.Contains(pair, "=") {
			if _, exists := labels[pair]; !exists {
				return false
			}
			continue
		}

		// Check for equality operator: "key=value"
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if labels[key] != value {
			return false
		}
	}

	return true
}

// formatAllResources formats the all resources result in the requested format.
func formatAllResources(items []steve.AllResourceItem, format string) (string, error) {
	switch format {
	case paramutil.FormatTable:
		return formatAllResourcesAsTable(items), nil
	case paramutil.FormatYAML:
		return formatAllResourcesAsYAML(items)
	default: // json
		return formatAllResourcesAsJSON(items)
	}
}

// formatAllResourcesAsTable formats all resources as a table.
func formatAllResourcesAsTable(items []steve.AllResourceItem) string {
	if len(items) == 0 {
		return "No resources found"
	}

	var b strings.Builder
	// Build table header
	fmt.Fprintf(&b, "%-40s %-20s %-20s %-30s\n", "NAME", "NAMESPACE", "KIND", "APIVERSION")
	fmt.Fprintf(&b, "%-40s %-20s %-20s %-30s\n", "----", "---------", "----", "----------")

	// Build table rows
	for _, item := range items {
		namespace := item.Namespace
		if namespace == "" {
			namespace = "-"
		}
		fmt.Fprintf(&b, "%-40s %-20s %-20s %-30s\n",
			truncate(item.Name, 40),
			truncate(namespace, 20),
			truncate(item.Kind, 20),
			truncate(item.APIVersion, 30))
	}

	fmt.Fprintf(&b, "\nTotal: %d resources\n", len(items))
	return b.String()
}

// formatAllResourcesAsJSON formats all resources as JSON.
func formatAllResourcesAsJSON(items []steve.AllResourceItem) (string, error) {
	// Create a simplified structure for output
	type simpleItem struct {
		Name       string                 `json:"name"`
		Namespace  string                 `json:"namespace,omitempty"`
		Kind       string                 `json:"kind"`
		APIVersion string                 `json:"apiVersion"`
		Resource   map[string]interface{} `json:"resource,omitempty"`
	}

	output := make([]simpleItem, 0, len(items))
	for _, item := range items {
		si := simpleItem{
			Name:       item.Name,
			Namespace:  item.Namespace,
			Kind:       item.Kind,
			APIVersion: item.APIVersion,
		}
		if item.Resource != nil {
			si.Resource = item.Resource.Object
		}
		output = append(output, si)
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to format as JSON: %w", err)
	}
	return string(data), nil
}

// formatAllResourcesAsYAML formats all resources as YAML.
func formatAllResourcesAsYAML(items []steve.AllResourceItem) (string, error) {
	// Create a simplified structure for output
	type simpleItem struct {
		Name       string                 `yaml:"name"`
		Namespace  string                 `yaml:"namespace,omitempty"`
		Kind       string                 `yaml:"kind"`
		APIVersion string                 `yaml:"apiVersion"`
		Resource   map[string]interface{} `yaml:"resource,omitempty"`
	}

	output := make([]simpleItem, 0, len(items))
	for _, item := range items {
		si := simpleItem{
			Name:       item.Name,
			Namespace:  item.Namespace,
			Kind:       item.Kind,
			APIVersion: item.APIVersion,
		}
		if item.Resource != nil {
			si.Resource = item.Resource.Object
		}
		output = append(output, si)
	}

	data, err := yaml.Marshal(output)
	if err != nil {
		return "", fmt.Errorf("failed to format as YAML: %w", err)
	}
	return string(data), nil
}

// parseDuration parses a human-readable duration string.
// Supports formats like: "1h30m", "2d", "1w", etc.
func parseDuration(s string) (time.Duration, error) {
	// Try standard Go duration parsing first
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}

	// Parse custom formats like "1d" (day), "1w" (week)
	var total time.Duration
	var num int
	var unit string

	// Simple parser for patterns like "1h30m", "2d", "1w"
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= '0' && c <= '9' {
			num = num*10 + int(c-'0')
		} else {
			unit = string(c)
			switch unit {
			case "s":
				total += time.Duration(num) * time.Second
			case "m":
				total += time.Duration(num) * time.Minute
			case "h":
				total += time.Duration(num) * time.Hour
			case "d":
				total += time.Duration(num) * 24 * time.Hour
			case "w":
				total += time.Duration(num) * 7 * 24 * time.Hour
			default:
				return 0, fmt.Errorf("unknown time unit: %s", unit)
			}
			num = 0
		}
	}

	if total == 0 {
		return 0, fmt.Errorf("could not parse duration: %s", s)
	}

	return total, nil
}
