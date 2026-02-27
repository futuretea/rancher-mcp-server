package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/futuretea/rancher-mcp-server/pkg/client/steve"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset/paramutil"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// getHandler handles the kubernetes_get tool
func getHandler(ctx context.Context, client interface{}, params map[string]interface{}) (string, error) {
	steveClient, err := toolset.ValidateSteveClient(client)
	if err != nil {
		return "", err
	}

	cluster, err := paramutil.ExtractRequiredString(params, paramutil.ParamCluster)
	if err != nil {
		return "", err
	}
	kind, err := paramutil.ExtractRequiredString(params, paramutil.ParamKind)
	if err != nil {
		return "", err
	}
	name, err := paramutil.ExtractRequiredString(params, paramutil.ParamName)
	if err != nil {
		return "", err
	}
	namespace := paramutil.ExtractOptionalString(params, paramutil.ParamNamespace)
	format := paramutil.ExtractFormat(params)
	filter := paramutil.NewResourceFilterFromParams(params)

	resource, err := steveClient.GetResource(ctx, cluster, kind, namespace, name)
	if err != nil {
		return "", fmt.Errorf("failed to get resource: %w", err)
	}

	// Mask sensitive data (e.g., Secret data) unless showSensitiveData is true
	if sensitiveFilter := paramutil.NewSensitiveDataFilterFromParams(params); sensitiveFilter != nil {
		resource = sensitiveFilter.Filter(resource)
	}

	return formatResource(resource, format, filter)
}

// listHandler handles the kubernetes_list tool
func listHandler(ctx context.Context, client interface{}, params map[string]interface{}) (string, error) {
	steveClient, err := toolset.ValidateSteveClient(client)
	if err != nil {
		return "", err
	}

	cluster, err := paramutil.ExtractRequiredString(params, paramutil.ParamCluster)
	if err != nil {
		return "", err
	}
	kind, err := paramutil.ExtractRequiredString(params, paramutil.ParamKind)
	if err != nil {
		return "", err
	}
	namespace := paramutil.ExtractOptionalString(params, paramutil.ParamNamespace)
	nameFilter := paramutil.ExtractOptionalString(params, paramutil.ParamName)
	labelSelector := paramutil.ExtractOptionalString(params, paramutil.ParamLabelSelector)
	limit := paramutil.ExtractInt64(params, paramutil.ParamLimit, DefaultLimit)
	page := paramutil.ExtractInt64(params, paramutil.ParamPage, DefaultPage)
	format := paramutil.ExtractFormat(params)
	filter := paramutil.NewResourceFilterFromParams(params)

	// Server-side: labelSelector (no limit here to allow client-side pagination)
	opts := &steve.ListOptions{
		LabelSelector: labelSelector,
	}

	list, err := steveClient.ListResources(ctx, cluster, kind, namespace, opts)
	if err != nil {
		return "", fmt.Errorf("failed to list resources: %w", err)
	}

	// Client-side: name filter (K8s doesn't support partial match)
	if nameFilter != "" {
		list = filterResourcesByName(list, nameFilter)
	}

	// Client-side: page pagination
	list = paginateResourceList(list, limit, page)

	// Mask sensitive data (e.g., Secret data) unless showSensitiveData is true
	if sensitiveFilter := paramutil.NewSensitiveDataFilterFromParams(params); sensitiveFilter != nil {
		list = sensitiveFilter.FilterList(list)
	}

	return formatResourceList(list, format, filter)
}

// createHandler handles the kubernetes_create tool
func createHandler(ctx context.Context, client interface{}, params map[string]interface{}) (string, error) {
	// Check read-only mode
	if readOnly, ok := params["readOnly"].(bool); ok && readOnly {
		return "", paramutil.ErrReadOnlyMode
	}

	steveClient, err := toolset.ValidateSteveClient(client)
	if err != nil {
		return "", err
	}

	cluster, err := paramutil.ExtractRequiredString(params, paramutil.ParamCluster)
	if err != nil {
		return "", err
	}
	resourceJSON, err := paramutil.ExtractRequiredString(params, paramutil.ParamResource)
	if err != nil {
		return "", err
	}
	filter := paramutil.NewResourceFilterFromParams(params)

	// Parse the resource JSON
	var resource unstructured.Unstructured
	if err := json.Unmarshal([]byte(resourceJSON), &resource.Object); err != nil {
		return "", fmt.Errorf("failed to parse resource JSON: %w", err)
	}

	created, err := steveClient.CreateResource(ctx, cluster, &resource)
	if err != nil {
		return "", fmt.Errorf("failed to create resource: %w", err)
	}

	return formatResource(created, paramutil.FormatJSON, filter)
}

// patchHandler handles the kubernetes_patch tool
func patchHandler(ctx context.Context, client interface{}, params map[string]interface{}) (string, error) {
	// Check read-only mode
	if readOnly, ok := params["readOnly"].(bool); ok && readOnly {
		return "", paramutil.ErrReadOnlyMode
	}

	steveClient, err := toolset.ValidateSteveClient(client)
	if err != nil {
		return "", err
	}

	cluster, err := paramutil.ExtractRequiredString(params, paramutil.ParamCluster)
	if err != nil {
		return "", err
	}
	kind, err := paramutil.ExtractRequiredString(params, paramutil.ParamKind)
	if err != nil {
		return "", err
	}
	name, err := paramutil.ExtractRequiredString(params, paramutil.ParamName)
	if err != nil {
		return "", err
	}
	namespace := paramutil.ExtractOptionalString(params, paramutil.ParamNamespace)
	patchStr, err := paramutil.ExtractRequiredString(params, paramutil.ParamPatch)
	if err != nil {
		return "", err
	}
	filter := paramutil.NewResourceFilterFromParams(params)

	patched, err := steveClient.PatchResource(ctx, cluster, kind, namespace, name, []byte(patchStr))
	if err != nil {
		return "", fmt.Errorf("failed to patch resource: %w", err)
	}

	return formatResource(patched, paramutil.FormatJSON, filter)
}

// deleteHandler handles the kubernetes_delete tool
func deleteHandler(ctx context.Context, client interface{}, params map[string]interface{}) (string, error) {
	// Check read-only mode
	if readOnly, ok := params["readOnly"].(bool); ok && readOnly {
		return "", paramutil.ErrReadOnlyMode
	}
	// Check destructive operations
	if disableDestructive, ok := params["disableDestructive"].(bool); ok && disableDestructive {
		return "", paramutil.ErrDestructiveDisabled
	}

	steveClient, err := toolset.ValidateSteveClient(client)
	if err != nil {
		return "", err
	}

	cluster, err := paramutil.ExtractRequiredString(params, paramutil.ParamCluster)
	if err != nil {
		return "", err
	}
	kind, err := paramutil.ExtractRequiredString(params, paramutil.ParamKind)
	if err != nil {
		return "", err
	}
	name, err := paramutil.ExtractRequiredString(params, paramutil.ParamName)
	if err != nil {
		return "", err
	}
	namespace := paramutil.ExtractOptionalString(params, paramutil.ParamNamespace)

	if err := steveClient.DeleteResource(ctx, cluster, kind, namespace, name); err != nil {
		return "", fmt.Errorf("failed to delete resource: %w", err)
	}

	return fmt.Sprintf("Successfully deleted %s/%s in namespace %s", kind, name, namespace), nil
}

// formatResource formats a single resource as JSON or YAML
func formatResource(resource *unstructured.Unstructured, format string, filter *paramutil.ResourceFilter) (string, error) {
	// Apply filter if configured
	if filter != nil {
		resource = filter.Filter(resource)
	}

	switch format {
	case paramutil.FormatYAML:
		data, err := yaml.Marshal(resource.Object)
		if err != nil {
			return "", fmt.Errorf("failed to format as YAML: %w", err)
		}
		return string(data), nil
	default: // json
		data, err := json.MarshalIndent(resource.Object, "", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to format as JSON: %w", err)
		}
		return string(data), nil
	}
}

// formatResourceList formats a resource list as JSON, YAML, or table
func formatResourceList(list *unstructured.UnstructuredList, format string, filter *paramutil.ResourceFilter) (string, error) {
	// Apply filter if configured
	if filter != nil {
		list = filter.FilterList(list)
	}

	switch format {
	case paramutil.FormatYAML:
		data, err := yaml.Marshal(list.Items)
		if err != nil {
			return "", fmt.Errorf("failed to format as YAML: %w", err)
		}
		return string(data), nil
	case paramutil.FormatTable:
		return formatAsTable(list), nil
	default: // json
		data, err := json.MarshalIndent(list.Items, "", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to format as JSON: %w", err)
		}
		return string(data), nil
	}
}

// formatAsTable formats resources as a simple table using strings.Builder
func formatAsTable(list *unstructured.UnstructuredList) string {
	if len(list.Items) == 0 {
		return "No resources found"
	}

	var b strings.Builder
	// Build table header
	fmt.Fprintf(&b, "%-40s %-20s %-15s\n", "NAME", "NAMESPACE", "KIND")
	fmt.Fprintf(&b, "%-40s %-20s %-15s\n", "----", "---------", "----")

	// Build table rows
	for _, item := range list.Items {
		namespace := item.GetNamespace()
		if namespace == "" {
			namespace = "-"
		}
		fmt.Fprintf(&b, "%-40s %-20s %-15s\n", truncate(item.GetName(), DefaultNameTruncateLen), truncate(namespace, DefaultNSTruncateLen), truncate(item.GetKind(), DefaultKindTruncateLen))
	}

	return b.String()
}

// truncate truncates a string to the specified length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// filterResourcesByName filters resources by name (partial match, case-insensitive).
func filterResourcesByName(list *unstructured.UnstructuredList, name string) *unstructured.UnstructuredList {
	var filtered []unstructured.Unstructured
	for _, item := range list.Items {
		if strings.Contains(strings.ToLower(item.GetName()), strings.ToLower(name)) {
			filtered = append(filtered, item)
		}
	}
	return &unstructured.UnstructuredList{Object: list.Object, Items: filtered}
}

// paginateResourceList applies pagination to a resource list.
func paginateResourceList(list *unstructured.UnstructuredList, limit, page int64) *unstructured.UnstructuredList {
	if limit <= 0 {
		return list
	}
	if page <= 0 {
		page = 1
	}
	total := int64(len(list.Items))
	start := (page - 1) * limit
	if start >= total {
		return &unstructured.UnstructuredList{Object: list.Object, Items: []unstructured.Unstructured{}}
	}
	end := start + limit
	if end > total {
		end = total
	}
	return &unstructured.UnstructuredList{Object: list.Object, Items: list.Items[start:end]}
}
