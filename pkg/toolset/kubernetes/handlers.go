package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/futuretea/rancher-mcp-server/pkg/client/steve"
	"github.com/futuretea/rancher-mcp-server/pkg/dep"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset/handler"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// getHandler handles the kubernetes_get tool
func getHandler(client interface{}, params map[string]interface{}) (string, error) {
	steveClient, err := toolset.ValidateSteveClient(client)
	if err != nil {
		return "", err
	}

	cluster, err := handler.ExtractRequiredString(params, handler.ParamCluster)
	if err != nil {
		return "", err
	}
	kind, err := handler.ExtractRequiredString(params, handler.ParamKind)
	if err != nil {
		return "", err
	}
	name, err := handler.ExtractRequiredString(params, handler.ParamName)
	if err != nil {
		return "", err
	}
	namespace := handler.ExtractOptionalString(params, handler.ParamNamespace)
	format := handler.ExtractFormat(params)
	filter := handler.NewResourceFilterFromParams(params)

	ctx := context.Background()
	resource, err := steveClient.GetResource(ctx, cluster, kind, namespace, name)
	if err != nil {
		return "", fmt.Errorf("failed to get resource: %w", err)
	}

	return formatResource(resource, format, filter)
}

// listHandler handles the kubernetes_list tool
func listHandler(client interface{}, params map[string]interface{}) (string, error) {
	steveClient, err := toolset.ValidateSteveClient(client)
	if err != nil {
		return "", err
	}

	cluster, err := handler.ExtractRequiredString(params, handler.ParamCluster)
	if err != nil {
		return "", err
	}
	kind, err := handler.ExtractRequiredString(params, handler.ParamKind)
	if err != nil {
		return "", err
	}
	namespace := handler.ExtractOptionalString(params, handler.ParamNamespace)
	nameFilter := handler.ExtractOptionalString(params, handler.ParamName)
	labelSelector := handler.ExtractOptionalString(params, handler.ParamLabelSelector)
	limit := handler.ExtractInt64(params, handler.ParamLimit, 100)
	page := handler.ExtractInt64(params, handler.ParamPage, 1)
	format := handler.ExtractFormat(params)
	filter := handler.NewResourceFilterFromParams(params)

	ctx := context.Background()
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

	return formatResourceList(list, format, filter)
}

// logsHandler handles the kubernetes_logs tool
func logsHandler(client interface{}, params map[string]interface{}) (string, error) {
	steveClient, err := toolset.ValidateSteveClient(client)
	if err != nil {
		return "", err
	}

	cluster, err := handler.ExtractRequiredString(params, handler.ParamCluster)
	if err != nil {
		return "", err
	}
	namespace, err := handler.ExtractRequiredString(params, handler.ParamNamespace)
	if err != nil {
		return "", err
	}
	name, err := handler.ExtractRequiredString(params, handler.ParamName)
	if err != nil {
		return "", err
	}
	container := handler.ExtractOptionalString(params, handler.ParamContainer)
	tailLines := handler.ExtractInt64(params, handler.ParamTailLines, 100)
	sinceSeconds := handler.ExtractOptionalInt64(params, handler.ParamSinceSeconds)
	timestamps := handler.ExtractBool(params, handler.ParamTimestamps, false)
	previous := handler.ExtractBool(params, handler.ParamPrevious, false)
	keyword := handler.ExtractOptionalString(params, handler.ParamKeyword)

	ctx := context.Background()

	if container != "" {
		// Get logs for specific container
		opts := &steve.PodLogOptions{
			Container:    container,
			TailLines:    &tailLines,
			SinceSeconds: sinceSeconds,
			Timestamps:   timestamps,
			Previous:     previous,
		}
		logs, err := steveClient.GetPodLogs(ctx, cluster, namespace, name, opts)
		if err != nil {
			return "", fmt.Errorf("failed to get pod logs: %w", err)
		}
		return filterLogsByKeyword(logs, keyword), nil
	}

	// Get logs for all containers
	logs, err := steveClient.GetAllContainerLogs(ctx, cluster, namespace, name, tailLines)
	if err != nil {
		return "", fmt.Errorf("failed to get pod logs: %w", err)
	}

	// Apply keyword filter to each container's logs
	for containerName, containerLogs := range logs {
		logs[containerName] = filterLogsByKeyword(containerLogs, keyword)
	}

	result, err := json.MarshalIndent(logs, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to format logs: %w", err)
	}
	return string(result), nil
}

// inspectPodHandler handles the kubernetes_inspect_pod tool
func inspectPodHandler(client interface{}, params map[string]interface{}) (string, error) {
	steveClient, err := toolset.ValidateSteveClient(client)
	if err != nil {
		return "", err
	}

	cluster, err := handler.ExtractRequiredString(params, handler.ParamCluster)
	if err != nil {
		return "", err
	}
	namespace, err := handler.ExtractRequiredString(params, handler.ParamNamespace)
	if err != nil {
		return "", err
	}
	name, err := handler.ExtractRequiredString(params, handler.ParamName)
	if err != nil {
		return "", err
	}

	ctx := context.Background()
	result, err := steveClient.InspectPod(ctx, cluster, namespace, name)
	if err != nil {
		return "", fmt.Errorf("failed to inspect pod: %w", err)
	}

	return result.ToJSON()
}

// describeHandler handles the kubernetes_describe tool
func describeHandler(client interface{}, params map[string]interface{}) (string, error) {
	steveClient, err := toolset.ValidateSteveClient(client)
	if err != nil {
		return "", err
	}

	cluster, err := handler.ExtractRequiredString(params, handler.ParamCluster)
	if err != nil {
		return "", err
	}
	kind, err := handler.ExtractRequiredString(params, handler.ParamKind)
	if err != nil {
		return "", err
	}
	name, err := handler.ExtractRequiredString(params, handler.ParamName)
	if err != nil {
		return "", err
	}
	namespace := handler.ExtractOptionalString(params, handler.ParamNamespace)
	format := handler.ExtractFormat(params)

	ctx := context.Background()
	result, err := steveClient.DescribeResource(ctx, cluster, kind, namespace, name)
	if err != nil {
		return "", fmt.Errorf("failed to describe resource: %w", err)
	}

	switch format {
	case handler.FormatYAML:
		data, err := yaml.Marshal(result)
		if err != nil {
			return "", fmt.Errorf("failed to format as YAML: %w", err)
		}
		return string(data), nil
	default: // json
		return result.ToJSON()
	}
}

// eventsHandler handles the kubernetes_events tool
func eventsHandler(client interface{}, params map[string]interface{}) (string, error) {
	steveClient, err := toolset.ValidateSteveClient(client)
	if err != nil {
		return "", err
	}

	cluster, err := handler.ExtractRequiredString(params, handler.ParamCluster)
	if err != nil {
		return "", err
	}
	namespace := handler.ExtractOptionalString(params, handler.ParamNamespace)
	nameFilter := handler.ExtractOptionalString(params, handler.ParamName)
	kindFilter := handler.ExtractOptionalString(params, handler.ParamKind)
	limit := handler.ExtractInt64(params, handler.ParamLimit, 50)
	page := handler.ExtractInt64(params, handler.ParamPage, 1)
	format := handler.ExtractOptionalStringWithDefault(params, handler.ParamFormat, handler.FormatTable)

	ctx := context.Background()
	events, err := steveClient.GetEvents(ctx, cluster, namespace, nameFilter, kindFilter)
	if err != nil {
		return "", fmt.Errorf("failed to get events: %w", err)
	}

	sortEventsByTime(events)

	events, _ = handler.ApplyPagination(events, limit, page)

	if len(events) == 0 {
		return "No events found", nil
	}

	switch format {
	case handler.FormatYAML:
		data, err := yaml.Marshal(events)
		if err != nil {
			return "", fmt.Errorf("failed to format as YAML: %w", err)
		}
		return string(data), nil
	case handler.FormatTable:
		return formatEventsAsTable(events), nil
	default: // json
		data, err := json.MarshalIndent(events, "", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to format as JSON: %w", err)
		}
		return string(data), nil
	}
}

// eventTime returns the most relevant timestamp for an event,
// preferring LastTimestamp over EventTime.
func eventTime(e corev1.Event) time.Time {
	if !e.LastTimestamp.IsZero() {
		return e.LastTimestamp.Time
	}
	return e.EventTime.Time
}

// sortEventsByTime sorts events by timestamp, most recent first.
func sortEventsByTime(events []corev1.Event) {
	sort.Slice(events, func(i, j int) bool {
		return eventTime(events[i]).After(eventTime(events[j]))
	})
}

// formatEventsAsTable formats events as a human-readable table
func formatEventsAsTable(events []corev1.Event) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%-8s %-25s %-40s %-6s %-s\n", "TYPE", "REASON", "OBJECT", "COUNT", "MESSAGE")
	fmt.Fprintf(&b, "%-8s %-25s %-40s %-6s %-s\n", "----", "------", "------", "-----", "-------")

	for _, event := range events {
		object := fmt.Sprintf("%s/%s", event.InvolvedObject.Kind, event.InvolvedObject.Name)
		message := event.Message
		if len(message) > 100 {
			message = message[:97] + "..."
		}
		fmt.Fprintf(&b, "%-8s %-25s %-40s %-6d %s\n",
			truncate(event.Type, 8),
			truncate(event.Reason, 25),
			truncate(object, 40),
			event.Count,
			message,
		)
	}

	return b.String()
}

// createHandler handles the kubernetes_create tool
func createHandler(client interface{}, params map[string]interface{}) (string, error) {
	// Check read-only mode
	if readOnly, ok := params["readOnly"].(bool); ok && readOnly {
		return "", handler.ErrReadOnlyMode
	}

	steveClient, err := toolset.ValidateSteveClient(client)
	if err != nil {
		return "", err
	}

	cluster, err := handler.ExtractRequiredString(params, handler.ParamCluster)
	if err != nil {
		return "", err
	}
	resourceJSON, err := handler.ExtractRequiredString(params, handler.ParamResource)
	if err != nil {
		return "", err
	}
	filter := handler.NewResourceFilterFromParams(params)

	// Parse the resource JSON
	var resource unstructured.Unstructured
	if err := json.Unmarshal([]byte(resourceJSON), &resource.Object); err != nil {
		return "", fmt.Errorf("failed to parse resource JSON: %w", err)
	}

	ctx := context.Background()
	created, err := steveClient.CreateResource(ctx, cluster, &resource)
	if err != nil {
		return "", fmt.Errorf("failed to create resource: %w", err)
	}

	return formatResource(created, handler.FormatJSON, filter)
}

// patchHandler handles the kubernetes_patch tool
func patchHandler(client interface{}, params map[string]interface{}) (string, error) {
	// Check read-only mode
	if readOnly, ok := params["readOnly"].(bool); ok && readOnly {
		return "", handler.ErrReadOnlyMode
	}

	steveClient, err := toolset.ValidateSteveClient(client)
	if err != nil {
		return "", err
	}

	cluster, err := handler.ExtractRequiredString(params, handler.ParamCluster)
	if err != nil {
		return "", err
	}
	kind, err := handler.ExtractRequiredString(params, handler.ParamKind)
	if err != nil {
		return "", err
	}
	name, err := handler.ExtractRequiredString(params, handler.ParamName)
	if err != nil {
		return "", err
	}
	namespace := handler.ExtractOptionalString(params, handler.ParamNamespace)
	patchStr, err := handler.ExtractRequiredString(params, handler.ParamPatch)
	if err != nil {
		return "", err
	}
	filter := handler.NewResourceFilterFromParams(params)

	ctx := context.Background()
	patched, err := steveClient.PatchResource(ctx, cluster, kind, namespace, name, []byte(patchStr))
	if err != nil {
		return "", fmt.Errorf("failed to patch resource: %w", err)
	}

	return formatResource(patched, handler.FormatJSON, filter)
}

// deleteHandler handles the kubernetes_delete tool
func deleteHandler(client interface{}, params map[string]interface{}) (string, error) {
	// Check read-only mode
	if readOnly, ok := params["readOnly"].(bool); ok && readOnly {
		return "", handler.ErrReadOnlyMode
	}
	// Check destructive operations
	if disableDestructive, ok := params["disableDestructive"].(bool); ok && disableDestructive {
		return "", handler.ErrDestructiveDisabled
	}

	steveClient, err := toolset.ValidateSteveClient(client)
	if err != nil {
		return "", err
	}

	cluster, err := handler.ExtractRequiredString(params, handler.ParamCluster)
	if err != nil {
		return "", err
	}
	kind, err := handler.ExtractRequiredString(params, handler.ParamKind)
	if err != nil {
		return "", err
	}
	name, err := handler.ExtractRequiredString(params, handler.ParamName)
	if err != nil {
		return "", err
	}
	namespace := handler.ExtractOptionalString(params, handler.ParamNamespace)

	ctx := context.Background()
	if err := steveClient.DeleteResource(ctx, cluster, kind, namespace, name); err != nil {
		return "", fmt.Errorf("failed to delete resource: %w", err)
	}

	return fmt.Sprintf("Successfully deleted %s/%s in namespace %s", kind, name, namespace), nil
}

// formatResource formats a single resource as JSON or YAML
func formatResource(resource *unstructured.Unstructured, format string, filter *handler.ResourceFilter) (string, error) {
	// Apply filter if configured
	if filter != nil {
		resource = filter.Filter(resource)
	}

	switch format {
	case handler.FormatYAML:
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
func formatResourceList(list *unstructured.UnstructuredList, format string, filter *handler.ResourceFilter) (string, error) {
	// Apply filter if configured
	if filter != nil {
		list = filter.FilterList(list)
	}

	switch format {
	case handler.FormatYAML:
		data, err := yaml.Marshal(list.Items)
		if err != nil {
			return "", fmt.Errorf("failed to format as YAML: %w", err)
		}
		return string(data), nil
	case handler.FormatTable:
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
		fmt.Fprintf(&b, "%-40s %-20s %-15s\n", truncate(item.GetName(), 40), truncate(namespace, 20), truncate(item.GetKind(), 15))
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

// filterLogsByKeyword filters log lines by keyword (case-insensitive).
// Returns the original logs if keyword is empty.
func filterLogsByKeyword(logs, keyword string) string {
	if keyword == "" {
		return logs
	}
	keywordLower := strings.ToLower(keyword)
	lines := strings.Split(logs, "\n")
	var filtered []string
	for _, line := range lines {
		if strings.Contains(strings.ToLower(line), keywordLower) {
			filtered = append(filtered, line)
		}
	}
	if len(filtered) == 0 {
		return fmt.Sprintf("No log lines matching keyword %q", keyword)
	}
	return strings.Join(filtered, "\n")
}

// depHandler handles the kubernetes_dep tool
func depHandler(client interface{}, params map[string]interface{}) (string, error) {
	steveClient, err := toolset.ValidateSteveClient(client)
	if err != nil {
		return "", err
	}

	cluster, err := handler.ExtractRequiredString(params, handler.ParamCluster)
	if err != nil {
		return "", err
	}
	kind, err := handler.ExtractRequiredString(params, handler.ParamKind)
	if err != nil {
		return "", err
	}
	name, err := handler.ExtractRequiredString(params, handler.ParamName)
	if err != nil {
		return "", err
	}
	namespace := handler.ExtractOptionalString(params, handler.ParamNamespace)
	direction := handler.ExtractOptionalStringWithDefault(params, handler.ParamDirection, "dependents")
	maxDepth := int(handler.ExtractInt64(params, handler.ParamDepth, 10))
	format := handler.ExtractOptionalStringWithDefault(params, handler.ParamFormat, "tree")

	if direction != "dependents" && direction != "dependencies" {
		return "", fmt.Errorf("%w: direction must be 'dependents' or 'dependencies'", handler.ErrMissingParameter)
	}
	if maxDepth < 1 || maxDepth > 20 {
		maxDepth = 10
	}

	ctx := context.Background()
	result, err := dep.Resolve(ctx, steveClient, cluster, kind, namespace, name, direction, maxDepth)
	if err != nil {
		return "", fmt.Errorf("failed to resolve dependencies: %w", err)
	}

	depsIsDependencies := direction == "dependencies"

	switch format {
	case "json":
		return dep.FormatJSON(result, depsIsDependencies)
	default: // tree
		return dep.FormatTree(result, depsIsDependencies), nil
	}
}
