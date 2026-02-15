package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
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

	// Mask sensitive data (e.g., Secret data) unless showSensitiveData is true
	if sensitiveFilter := handler.NewSensitiveDataFilterFromParams(params); sensitiveFilter != nil {
		resource = sensitiveFilter.Filter(resource)
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

	// Mask sensitive data (e.g., Secret data) unless showSensitiveData is true
	if sensitiveFilter := handler.NewSensitiveDataFilterFromParams(params); sensitiveFilter != nil {
		list = sensitiveFilter.FilterList(list)
	}

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
	name := handler.ExtractOptionalString(params, handler.ParamName)
	labelSelector := handler.ExtractOptionalString(params, handler.ParamLabelSelector)
	container := handler.ExtractOptionalString(params, handler.ParamContainer)
	tailLines := handler.ExtractInt64(params, handler.ParamTailLines, 100)
	sinceSeconds := handler.ExtractOptionalInt64(params, handler.ParamSinceSeconds)
	timestamps := handler.ExtractBool(params, handler.ParamTimestamps, false)
	previous := handler.ExtractBool(params, handler.ParamPrevious, false)
	keyword := handler.ExtractOptionalString(params, handler.ParamKeyword)

	ctx := context.Background()

	// If labelSelector is provided, get logs from multiple pods
	if labelSelector != "" {
		return getMultiPodLogs(ctx, steveClient, cluster, namespace, labelSelector, container, tailLines, keyword, timestamps)
	}

	// If name is not provided and no labelSelector, return error
	if name == "" {
		return "", fmt.Errorf("either 'name' (pod name) or 'labelSelector' must be specified")
	}

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
		logs = filterLogsByKeyword(logs, keyword)
		// Sort logs by timestamp when timestamps are enabled
		logs = sortLogsByTime(logs, timestamps)
		return logs, nil
	}

	// Get logs for all containers
	logs, err := steveClient.GetAllContainerLogs(ctx, cluster, namespace, name, &steve.PodLogOptions{
		TailLines:  &tailLines,
		Timestamps: timestamps,
	})
	if err != nil {
		return "", fmt.Errorf("failed to get pod logs: %w", err)
	}

	// Apply keyword filter to each container's logs and collect all entries for sorting
	var allEntries []LogEntry
	for containerName, containerLogs := range logs {
		filteredLogs := filterLogsByKeyword(containerLogs, keyword)
		lines := strings.Split(filteredLogs, "\n")
		for _, line := range lines {
			if line == "" {
				continue
			}
			ts, content := parseLogTimestamp(line)
			allEntries = append(allEntries, LogEntry{
				Timestamp: ts,
				Content:   content,
				Container: containerName,
			})
		}
	}

	// Sort by timestamp (oldest first)
	sort.Slice(allEntries, func(i, j int) bool {
		if allEntries[i].Timestamp.IsZero() || allEntries[j].Timestamp.IsZero() {
			return allEntries[i].Container < allEntries[j].Container
		}
		return allEntries[i].Timestamp.Before(allEntries[j].Timestamp)
	})

	// Format output
	var resultLines []string
	for _, entry := range allEntries {
		if timestamps && !entry.Timestamp.IsZero() {
			resultLines = append(resultLines, fmt.Sprintf("[%s] %s %s", entry.Container, entry.Timestamp.Format(time.RFC3339Nano), entry.Content))
		} else {
			resultLines = append(resultLines, fmt.Sprintf("[%s] %s", entry.Container, entry.Content))
		}
	}

	return strings.Join(resultLines, "\n"), nil
}

// getMultiPodLogs retrieves and merges logs from multiple pods matching the label selector
// Logs are sorted by timestamp when timestamps is true.
func getMultiPodLogs(ctx context.Context, steveClient *steve.Client, cluster, namespace, labelSelector, container string, tailLines int64, keyword string, timestamps bool) (string, error) {
	opts := &steve.PodLogOptions{
		TailLines:  &tailLines,
		Timestamps: timestamps,
	}

	results, err := steveClient.GetMultiPodLogs(ctx, cluster, namespace, labelSelector, opts)
	if err != nil {
		return "", fmt.Errorf("failed to get multi pod logs: %w", err)
	}

	if len(results) == 0 {
		return "No pods found matching the label selector", nil
	}

	// Collect all log entries from all pods and containers
	var allEntries []LogEntry

	for _, result := range results {
		// If a specific container is requested, filter to that container only
		if container != "" {
			if containerLogs, ok := result.Logs[container]; ok {
				filteredLogs := filterLogsByKeyword(containerLogs, keyword)
				lines := strings.Split(filteredLogs, "\n")
				for _, line := range lines {
					if line == "" {
						continue
					}
					ts, content := parseLogTimestamp(line)
					allEntries = append(allEntries, LogEntry{
						Timestamp: ts,
						Content:   content,
						Pod:       result.Pod,
						Container: container,
					})
				}
			}
		} else {
			// Process all containers in this pod
			for containerName, containerLogs := range result.Logs {
				filteredLogs := containerLogs
				if keyword != "" {
					filteredLogs = filterLogsByKeyword(containerLogs, keyword)
				}
				lines := strings.Split(filteredLogs, "\n")
				for _, line := range lines {
					if line == "" {
						continue
					}
					ts, content := parseLogTimestamp(line)
					allEntries = append(allEntries, LogEntry{
						Timestamp: ts,
						Content:   content,
						Pod:       result.Pod,
						Container: containerName,
					})
				}
			}
		}
	}

	if len(allEntries) == 0 {
		return "No log entries found", nil
	}

	// Sort by timestamp (oldest first)
	sort.Slice(allEntries, func(i, j int) bool {
		if allEntries[i].Timestamp.IsZero() || allEntries[j].Timestamp.IsZero() {
			// If no timestamp, sort by pod name then container name
			if allEntries[i].Pod != allEntries[j].Pod {
				return allEntries[i].Pod < allEntries[j].Pod
			}
			return allEntries[i].Container < allEntries[j].Container
		}
		return allEntries[i].Timestamp.Before(allEntries[j].Timestamp)
	})

	// Format output
	var resultLines []string
	for _, entry := range allEntries {
		if timestamps && !entry.Timestamp.IsZero() {
			resultLines = append(resultLines, fmt.Sprintf("[%s/%s] %s %s", entry.Pod, entry.Container, entry.Timestamp.Format(time.RFC3339Nano), entry.Content))
		} else {
			resultLines = append(resultLines, fmt.Sprintf("[%s/%s] %s", entry.Pod, entry.Container, entry.Content))
		}
	}

	return strings.Join(resultLines, "\n"), nil
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

	// Mask sensitive data (e.g., Secret data) unless showSensitiveData is true
	if sensitiveFilter := handler.NewSensitiveDataFilterFromParams(params); sensitiveFilter != nil {
		result.Resource = sensitiveFilter.Filter(result.Resource)
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

// LogEntry represents a single log line with its timestamp
type LogEntry struct {
	Timestamp time.Time
	Content   string
	Pod       string
	Container string
}

// parseLogTimestamp extracts timestamp from a log line.
// Kubernetes log format: 2024-01-15T10:30:00.123456789Z log message
// Returns the timestamp and the remaining log content.
func parseLogTimestamp(line string) (time.Time, string) {
	if len(line) < 30 {
		return time.Time{}, line
	}
	// Try to parse ISO 8601 timestamp at the beginning of the line
	timestampStr := line[:30]
	if t, err := time.Parse(time.RFC3339Nano, timestampStr); err == nil {
		return t, line[31:]
	}
	// Try without nanoseconds
	if len(line) >= 20 {
		timestampStr = line[:20]
		if t, err := time.Parse(time.RFC3339, timestampStr); err == nil {
			return t, line[21:]
		}
	}
	return time.Time{}, line
}

// sortLogsByTime sorts log lines by timestamp (oldest first).
// If timestamps is true, log lines start with timestamps.
func sortLogsByTime(logs string, timestamps bool) string {
	if !timestamps || logs == "" {
		return logs
	}
	lines := strings.Split(logs, "\n")
	var entries []LogEntry
	for _, line := range lines {
		if line == "" {
			continue
		}
		ts, content := parseLogTimestamp(line)
		entries = append(entries, LogEntry{
			Timestamp: ts,
			Content:   content,
		})
	}
	// Sort by timestamp (oldest first)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.Before(entries[j].Timestamp)
	})
	// Reconstruct log lines
	var result []string
	for _, entry := range entries {
		if !entry.Timestamp.IsZero() {
			result = append(result, entry.Timestamp.Format(time.RFC3339Nano)+" "+entry.Content)
		} else {
			result = append(result, entry.Content)
		}
	}
	return strings.Join(result, "\n")
}

// sortMultiPodLogsByTime sorts logs from multiple pods by timestamp.
// Returns a merged, time-sorted list of log entries with pod and container info.
func sortMultiPodLogsByTime(results []steve.MultiPodLogResult, timestamps bool) []LogEntry {
	if !timestamps {
		return nil
	}
	var allEntries []LogEntry
	for _, result := range results {
		for containerName, logs := range result.Logs {
			lines := strings.Split(logs, "\n")
			for _, line := range lines {
				if line == "" {
					continue
				}
				ts, content := parseLogTimestamp(line)
				allEntries = append(allEntries, LogEntry{
					Timestamp: ts,
					Content:   content,
					Pod:       result.Pod,
					Container: containerName,
				})
			}
		}
	}
	// Sort by timestamp (oldest first)
	sort.Slice(allEntries, func(i, j int) bool {
		if allEntries[i].Timestamp.IsZero() || allEntries[j].Timestamp.IsZero() {
			return allEntries[i].Pod < allEntries[j].Pod
		}
		return allEntries[i].Timestamp.Before(allEntries[j].Timestamp)
	})
	return allEntries
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

// RevisionInfo represents a single revision in the rollout history
type RevisionInfo struct {
	Revision    string `json:"revision"`
	ChangeCause string `json:"change_cause"`
	Created     string `json:"created"`
	Name        string `json:"name"`
}

// rolloutHistoryHandler handles the kubernetes_rollout_history tool
func rolloutHistoryHandler(client interface{}, params map[string]interface{}) (string, error) {
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
	format := handler.ExtractOptionalStringWithDefault(params, handler.ParamFormat, handler.FormatTable)

	ctx := context.Background()

	// Get the Deployment
	deployment, err := steveClient.GetResource(ctx, cluster, "deployment", namespace, name)
	if err != nil {
		return "", fmt.Errorf("failed to get deployment: %w", err)
	}

	// Get ReplicaSets owned by this Deployment
	// Use label selector to find ReplicaSets with the deployment's selector
	selector, found, err := unstructured.NestedStringMap(deployment.Object, "spec", "selector", "matchLabels")
	if err != nil || !found || len(selector) == 0 {
		// Try to get the selector from the deployment's spec.selector
		selector = make(map[string]string)
	}

	// Build label selector string
	var labelSelectors []string
	for k, v := range selector {
		labelSelectors = append(labelSelectors, fmt.Sprintf("%s=%s", k, v))
	}

	var rsList *unstructured.UnstructuredList
	if len(labelSelectors) > 0 {
		rsList, err = steveClient.ListResources(ctx, cluster, "replicaset", namespace, &steve.ListOptions{
			LabelSelector: strings.Join(labelSelectors, ","),
		})
		if err != nil {
			return "", fmt.Errorf("failed to list replicasets: %w", err)
		}
	} else {
		// If no selector, list all replicasets in namespace and filter by owner
		rsList, err = steveClient.ListResources(ctx, cluster, "replicaset", namespace, nil)
		if err != nil {
			return "", fmt.Errorf("failed to list replicasets: %w", err)
		}
	}

	// Extract revision history from ReplicaSets
	var history []RevisionInfo
	for _, rs := range rsList.Items {
		// Check if this RS is owned by the deployment
		ownerRefs, found, _ := unstructured.NestedSlice(rs.Object, "metadata", "ownerReferences")
		if !found {
			continue
		}

		isOwned := false
		for _, ref := range ownerRefs {
			ownerRef, ok := ref.(map[string]interface{})
			if !ok {
				continue
			}
			kind, _ := ownerRef["kind"].(string)
			ownerName, _ := ownerRef["name"].(string)
			if kind == "Deployment" && ownerName == name {
				isOwned = true
				break
			}
		}

		if !isOwned {
			continue
		}

		// Extract revision number and change cause
		revision, _, _ := unstructured.NestedString(rs.Object, "metadata", "annotations", "deployment.kubernetes.io/revision")
		changeCause, _, _ := unstructured.NestedString(rs.Object, "metadata", "annotations", "deployment.kubernetes.io/change-cause")

		// Get creation timestamp
		created := rs.GetCreationTimestamp()

		history = append(history, RevisionInfo{
			Revision:    revision,
			ChangeCause: changeCause,
			Created:     created.Format(time.RFC3339),
			Name:        rs.GetName(),
		})
	}

	// Sort by revision number (descending)
	sort.Slice(history, func(i, j int) bool {
		revI, _ := strconv.Atoi(history[i].Revision)
		revJ, _ := strconv.Atoi(history[j].Revision)
		return revI > revJ
	})

	// Format output
	switch format {
	case handler.FormatTable:
		return formatRolloutHistoryAsTable(history), nil
	default: // json
		data, err := json.MarshalIndent(history, "", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to format as JSON: %w", err)
		}
		return string(data), nil
	}
}

// formatRolloutHistoryAsTable formats rollout history as a human-readable table
func formatRolloutHistoryAsTable(history []RevisionInfo) string {
	if len(history) == 0 {
		return "No rollout history found"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%-10s %-30s %-25s %s\n", "REVISION", "NAME", "CREATED", "CHANGE_CAUSE")
	fmt.Fprintf(&b, "%-10s %-30s %-25s %s\n", "--------", "----", "-------", "------------")

	for _, rev := range history {
		changeCause := rev.ChangeCause
		if changeCause == "" {
			changeCause = "-"
		}
		fmt.Fprintf(&b, "%-10s %-30s %-25s %s\n",
			rev.Revision,
			truncate(rev.Name, 30),
			truncate(rev.Created, 25),
			changeCause,
		)
	}

	return b.String()
}

// NodeAnalysisResult contains the comprehensive analysis of a node.
type NodeAnalysisResult struct {
	Node      *unstructured.Unstructured `json:"node"`
	Capacity  map[string]string          `json:"capacity"`
	Allocated map[string]string          `json:"allocated"`
	Taints    []corev1.Taint             `json:"taints"`
	Labels    map[string]string          `json:"labels"`
	Pods      []NodePodInfo              `json:"pods"`
}

// NodePodInfo contains summary information about a pod running on the node.
type NodePodInfo struct {
	Namespace     string `json:"namespace"`
	Name          string `json:"name"`
	Phase         string `json:"phase"`
	CPURequest    string `json:"cpuRequest,omitempty"`
	MemoryRequest string `json:"memoryRequest,omitempty"`
	CPULimit      string `json:"cpuLimit,omitempty"`
	MemoryLimit   string `json:"memoryLimit,omitempty"`
}

// nodeAnalysisHandler handles the kubernetes_node_analysis tool
func nodeAnalysisHandler(client interface{}, params map[string]interface{}) (string, error) {
	steveClient, err := toolset.ValidateSteveClient(client)
	if err != nil {
		return "", err
	}

	cluster, err := handler.ExtractRequiredString(params, handler.ParamCluster)
	if err != nil {
		return "", err
	}
	name, err := handler.ExtractRequiredString(params, handler.ParamName)
	if err != nil {
		return "", err
	}
	format := handler.ExtractFormat(params)

	ctx := context.Background()

	// Get node details
	node, err := steveClient.GetResource(ctx, cluster, "node", "", name)
	if err != nil {
		return "", fmt.Errorf("failed to get node: %w", err)
	}

	result := &NodeAnalysisResult{
		Node:      node,
		Capacity:  make(map[string]string),
		Allocated: make(map[string]string),
		Taints:    []corev1.Taint{},
		Labels:    make(map[string]string),
		Pods:      []NodePodInfo{},
	}

	// Extract capacity
	if capacity, found, _ := unstructured.NestedMap(node.Object, "status", "capacity"); found {
		for k, v := range capacity {
			if s, ok := v.(string); ok {
				result.Capacity[k] = s
			}
		}
	}

	// Extract allocatable (as allocated capacity)
	if allocatable, found, _ := unstructured.NestedMap(node.Object, "status", "allocatable"); found {
		for k, v := range allocatable {
			if s, ok := v.(string); ok {
				result.Allocated[k] = s
			}
		}
	}

	// Extract taints
	if taints, found, _ := unstructured.NestedSlice(node.Object, "spec", "taints"); found {
		for _, t := range taints {
			taintMap, ok := t.(map[string]interface{})
			if !ok {
				continue
			}
			taint := corev1.Taint{}
			if key, ok := taintMap["key"].(string); ok {
				taint.Key = key
			}
			if value, ok := taintMap["value"].(string); ok {
				taint.Value = value
			}
			if effect, ok := taintMap["effect"].(string); ok {
				taint.Effect = corev1.TaintEffect(effect)
			}
			result.Taints = append(result.Taints, taint)
		}
	}

	// Extract labels
	result.Labels = node.GetLabels()

	// Get pods running on this node
	pods, err := steveClient.ListResources(ctx, cluster, "pod", "", &steve.ListOptions{
		FieldSelector: "spec.nodeName=" + name,
	})
	if err != nil {
		return "", fmt.Errorf("failed to list pods on node: %w", err)
	}

	// Process each pod
	for _, pod := range pods.Items {
		podInfo := NodePodInfo{
			Namespace: pod.GetNamespace(),
			Name:      pod.GetName(),
		}

		// Get pod phase
		if phase, found, _ := unstructured.NestedString(pod.Object, "status", "phase"); found {
			podInfo.Phase = phase
		}

		// Extract container resource requests and limits
		containers, found, _ := unstructured.NestedSlice(pod.Object, "spec", "containers")
		if found {
			var totalCPURequest, totalMemoryRequest, totalCPULimit, totalMemoryLimit int64

			for _, c := range containers {
				container, ok := c.(map[string]interface{})
				if !ok {
					continue
				}

				resources, found, _ := unstructured.NestedMap(container, "resources")
				if !found {
					continue
				}

				// Parse requests
				if requests, found, _ := unstructured.NestedMap(resources, "requests"); found {
					if cpu, ok := requests["cpu"].(string); ok {
						totalCPURequest += parseResourceQuantity(cpu)
					}
					if memory, ok := requests["memory"].(string); ok {
						totalMemoryRequest += parseResourceQuantity(memory)
					}
				}

				// Parse limits
				if limits, found, _ := unstructured.NestedMap(resources, "limits"); found {
					if cpu, ok := limits["cpu"].(string); ok {
						totalCPULimit += parseResourceQuantity(cpu)
					}
					if memory, ok := limits["memory"].(string); ok {
						totalMemoryLimit += parseResourceQuantity(memory)
					}
				}
			}

			if totalCPURequest > 0 {
				podInfo.CPURequest = formatResourceQuantity(totalCPURequest, "cpu")
			}
			if totalMemoryRequest > 0 {
				podInfo.MemoryRequest = formatResourceQuantity(totalMemoryRequest, "memory")
			}
			if totalCPULimit > 0 {
				podInfo.CPULimit = formatResourceQuantity(totalCPULimit, "cpu")
			}
			if totalMemoryLimit > 0 {
				podInfo.MemoryLimit = formatResourceQuantity(totalMemoryLimit, "memory")
			}
		}

		result.Pods = append(result.Pods, podInfo)
	}

	// Format output
	switch format {
	case handler.FormatYAML:
		data, err := yaml.Marshal(result)
		if err != nil {
			return "", fmt.Errorf("failed to format as YAML: %w", err)
		}
		return string(data), nil
	default: // json
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to format as JSON: %w", err)
		}
		return string(data), nil
	}
}

// parseResourceQuantity parses a Kubernetes resource quantity string to a numeric value.
// Supports millicores (m) for CPU and binary (Ki, Mi, Gi) and decimal (K, M, G) for memory.
func parseResourceQuantity(q string) int64 {
	q = strings.TrimSpace(q)
	if q == "" {
		return 0
	}

	// Handle millicores (e.g., "100m", "500m")
	if strings.HasSuffix(q, "m") {
		val, err := parseNumeric(q[:len(q)-1])
		if err != nil {
			return 0
		}
		return val // Keep as millicores
	}

	// Handle binary memory units
	if strings.HasSuffix(q, "Ki") {
		val, err := parseNumeric(q[:len(q)-2])
		if err != nil {
			return 0
		}
		return val * 1024
	}
	if strings.HasSuffix(q, "Mi") {
		val, err := parseNumeric(q[:len(q)-2])
		if err != nil {
			return 0
		}
		return val * 1024 * 1024
	}
	if strings.HasSuffix(q, "Gi") {
		val, err := parseNumeric(q[:len(q)-2])
		if err != nil {
			return 0
		}
		return val * 1024 * 1024 * 1024
	}
	if strings.HasSuffix(q, "Ti") {
		val, err := parseNumeric(q[:len(q)-2])
		if err != nil {
			return 0
		}
		return val * 1024 * 1024 * 1024 * 1024
	}

	// Handle decimal memory units
	if strings.HasSuffix(q, "k") || strings.HasSuffix(q, "K") {
		val, err := parseNumeric(q[:len(q)-1])
		if err != nil {
			return 0
		}
		return val * 1000
	}
	if strings.HasSuffix(q, "M") {
		val, err := parseNumeric(q[:len(q)-1])
		if err != nil {
			return 0
		}
		return val * 1000 * 1000
	}
	if strings.HasSuffix(q, "G") {
		val, err := parseNumeric(q[:len(q)-1])
		if err != nil {
			return 0
		}
		return val * 1000 * 1000 * 1000
	}

	// Plain number
	val, err := parseNumeric(q)
	if err != nil {
		return 0
	}
	return val
}

// parseNumeric parses a numeric string to int64.
func parseNumeric(s string) (int64, error) {
	var result int64
	_, err := fmt.Sscanf(s, "%d", &result)
	return result, err
}

// formatResourceQuantity formats a numeric resource quantity back to a human-readable string.
func formatResourceQuantity(val int64, resourceType string) string {
	if resourceType == "cpu" {
		if val >= 1000 {
			return fmt.Sprintf("%dm (%dc)", val, val/1000)
		}
		return fmt.Sprintf("%dm", val)
	}

	// Memory
	if val >= 1024*1024*1024 {
		return fmt.Sprintf("%dGi (%d bytes)", val/(1024*1024*1024), val)
	}
	if val >= 1024*1024 {
		return fmt.Sprintf("%dMi (%d bytes)", val/(1024*1024), val)
	}
	if val >= 1024 {
		return fmt.Sprintf("%dKi (%d bytes)", val/1024, val)
	}
	return fmt.Sprintf("%d bytes", val)
}
