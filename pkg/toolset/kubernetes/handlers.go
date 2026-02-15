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
	"github.com/futuretea/rancher-mcp-server/pkg/watchdiff"
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

// watchDiffHandler handles the kubernetes_watch_diff tool.
// It behaves similarly to the Linux `watch` command: it repeatedly
// evaluates the current state of matching resources at a configurable
// interval and returns the concatenated diffs from all iterations.
func watchDiffHandler(client interface{}, params map[string]interface{}) (string, error) {
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
	labelSelector := handler.ExtractOptionalString(params, handler.ParamLabelSelector)
	fieldSelector := handler.ExtractOptionalString(params, handler.ParamFieldSelector)

	ignoreStatus := handler.ExtractBool(params, "ignoreStatus", false)
	ignoreMeta := handler.ExtractBool(params, "ignoreMeta", false)

	intervalSeconds := handler.ExtractInt64(params, handler.ParamIntervalSeconds, 10)
	if intervalSeconds < 1 {
		intervalSeconds = 1
	}
	if intervalSeconds > 600 {
		intervalSeconds = 600
	}

	iterations := handler.ExtractInt64(params, handler.ParamIterations, 6)
	if iterations < 1 {
		iterations = 1
	}
	if iterations > 100 {
		iterations = 100
	}

	ctx := context.Background()

	differ := watchdiff.NewDiffer(true)
	differ.SetIgnoreStatus(ignoreStatus)
	differ.SetIgnoreMeta(ignoreMeta)

	var resultLines []string

	for i := int64(0); i < iterations; i++ {
		// List current resources for this iteration
		listOpts := &steve.ListOptions{
			LabelSelector: labelSelector,
			FieldSelector: fieldSelector,
		}
		list, err := steveClient.ListResources(ctx, cluster, kind, namespace, listOpts)
		if err != nil {
			return "", fmt.Errorf("failed to list resources: %w", err)
		}

		// Sort for deterministic output
		sort.Slice(list.Items, func(i, j int) bool {
			ai := list.Items[i]
			aj := list.Items[j]
			if ai.GetNamespace() != aj.GetNamespace() {
				return ai.GetNamespace() < aj.GetNamespace()
			}
			if ai.GetKind() != aj.GetKind() {
				return ai.GetKind() < aj.GetKind()
			}
			return ai.GetName() < aj.GetName()
		})

		iterationHeader := fmt.Sprintf("# iteration %d\n", i+1)
		iterationLines := []string{iterationHeader}

		for idx := range list.Items {
			obj := &list.Items[idx]
			diffText, err := differ.Diff(obj)
			if err != nil {
				return "", fmt.Errorf("failed to diff resource: %w", err)
			}
			if diffText != "" {
				iterationLines = append(iterationLines, diffText)
			}
		}

		// Only append iteration output if there was any diff beyond the header.
		if len(iterationLines) > 1 {
			resultLines = append(resultLines, strings.Join(iterationLines, "\n"))
		}

		// Sleep between iterations, except after the last one.
		if i+1 < iterations {
			time.Sleep(time.Duration(intervalSeconds) * time.Second)
		}
	}

	if len(resultLines) == 0 {
		return "No changes detected across iterations", nil
	}

	return strings.Join(resultLines, "\n"), nil
}

// diffHandler handles the kubernetes_diff tool.
// It compares two Kubernetes resource versions and shows the differences as a git-style diff.
func diffHandler(client interface{}, params map[string]interface{}) (string, error) {
	// Extract required parameters
	resource1JSON, err := handler.ExtractRequiredString(params, "resource1")
	if err != nil {
		return "", err
	}
	resource2JSON, err := handler.ExtractRequiredString(params, "resource2")
	if err != nil {
		return "", err
	}

	ignoreStatus := handler.ExtractBool(params, "ignoreStatus", false)
	ignoreMeta := handler.ExtractBool(params, "ignoreMeta", false)

	// Parse resource1
	var resource1 unstructured.Unstructured
	if err := json.Unmarshal([]byte(resource1JSON), &resource1.Object); err != nil {
		return "", fmt.Errorf("failed to parse resource1 JSON: %w", err)
	}

	// Parse resource2
	var resource2 unstructured.Unstructured
	if err := json.Unmarshal([]byte(resource2JSON), &resource2.Object); err != nil {
		return "", fmt.Errorf("failed to parse resource2 JSON: %w", err)
	}

	// Create a printer for diff output
	printer := watchdiff.NewPrinter(false)

	// Make copies for potential modifications
	oldCopy := resource1.DeepCopy()
	newCopy := resource2.DeepCopy()

	// Apply ignore options
	if ignoreStatus {
		delete(oldCopy.Object, "status")
		delete(newCopy.Object, "status")
	}

	if ignoreMeta {
		trimMetadataForDiff(oldCopy)
		trimMetadataForDiff(newCopy)
	}

	// Generate the diff
	diffText, err := printer.Diff(oldCopy, newCopy)
	if err != nil {
		return "", fmt.Errorf("failed to compute diff: %w", err)
	}

	if diffText == "" {
		return "No differences found between the two resource versions.", nil
	}

	return diffText, nil
}

// trimMetadataForDiff keeps only essential metadata fields for diff comparison.
func trimMetadataForDiff(obj *unstructured.Unstructured) {
	metaVal, ok := obj.Object["metadata"].(map[string]interface{})
	if !ok {
		return
	}
	// Keep only essential metadata fields
	cleanMeta := map[string]interface{}{
		"name":        metaVal["name"],
		"namespace":   metaVal["namespace"],
		"labels":      metaVal["labels"],
		"annotations": metaVal["annotations"],
	}
	obj.Object["metadata"] = cleanMeta
}

// CapacityNodeInfo holds resource information for a node
type CapacityNodeInfo struct {
	Name        string            `json:"name"`
	CPU         CapacityResource  `json:"cpu"`
	Memory      CapacityResource  `json:"memory"`
	PodCount    PodCountInfo      `json:"podCount"`
	Taints      []corev1.Taint    `json:"taints,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Pods        []CapacityPodInfo `json:"pods,omitempty"`
}

// CapacityResource holds resource metrics for a node
type CapacityResource struct {
	Capacity    int64 `json:"capacity"`
	Allocatable int64 `json:"allocatable"`
	Requested   int64 `json:"requested"`
	Limited     int64 `json:"limited"`
	Utilized    int64 `json:"utilized,omitempty"`
}

// PodCountInfo holds pod count information
type PodCountInfo struct {
	Capacity   int64 `json:"capacity"`
	 Allocatable int64 `json:"allocatable"`
	Requested  int64 `json:"requested"`
}

// CapacityContainerInfo holds resource information for a container
type CapacityContainerInfo struct {
	Name   string           `json:"name"`
	CPU    CapacityResource `json:"cpu"`
	Memory CapacityResource `json:"memory"`
}

// CapacityPodInfo holds resource information for a pod
type CapacityPodInfo struct {
	Namespace  string                  `json:"namespace"`
	Name       string                  `json:"name"`
	CPU        CapacityResource        `json:"cpu"`
	Memory     CapacityResource        `json:"memory"`
	ContainerCnt int                   `json:"containerCount"`
	Containers []CapacityContainerInfo `json:"containers,omitempty"`
}

// CapacityResult holds the complete capacity analysis
type CapacityResult struct {
	Nodes         []CapacityNodeInfo `json:"nodes"`
	Cluster       CapacityNodeInfo   `json:"cluster"`
	ShowPods      bool               `json:"showPods"`
	ShowContainers bool              `json:"showContainers"`
	ShowUtil      bool               `json:"showUtil"`
	ShowAvailable bool               `json:"showAvailable"`
	ShowPodCount  bool               `json:"showPodCount"`
	ShowLabels    bool               `json:"showLabels"`
	HideRequests  bool               `json:"hideRequests"`
	HideLimits    bool               `json:"hideLimits"`
}

// capacityHandler handles the kubernetes_capacity tool
func capacityHandler(client interface{}, params map[string]interface{}) (string, error) {
	steveClient, err := toolset.ValidateSteveClient(client)
	if err != nil {
		return "", err
	}

	cluster, err := handler.ExtractRequiredString(params, handler.ParamCluster)
	if err != nil {
		return "", err
	}
	showPods := handler.ExtractBool(params, "pods", false)
	showContainers := handler.ExtractBool(params, "containers", false)
	showUtil := handler.ExtractBool(params, "util", false)
	showAvailable := handler.ExtractBool(params, "available", false)
	showPodCount := handler.ExtractBool(params, "podCount", false)
	showLabels := handler.ExtractBool(params, "showLabels", false)
	hideRequests := handler.ExtractBool(params, "hideRequests", false)
	hideLimits := handler.ExtractBool(params, "hideLimits", false)
	namespace := handler.ExtractOptionalString(params, handler.ParamNamespace)
	labelSelector := handler.ExtractOptionalString(params, handler.ParamLabelSelector)
	nodeLabelSelector := handler.ExtractOptionalString(params, "nodeLabelSelector")
	namespaceLabelSelector := handler.ExtractOptionalString(params, "namespaceLabelSelector")
	nodeTaints := handler.ExtractOptionalString(params, "nodeTaints")
	noTaint := handler.ExtractBool(params, "noTaint", false)
	sortBy := handler.ExtractOptionalString(params, "sortBy")
	format := handler.ExtractOptionalStringWithDefault(params, handler.ParamFormat, handler.FormatTable)

	// containers implies pods
	if showContainers {
		showPods = true
	}

	ctx := context.Background()

	// Get all nodes
	nodes, err := steveClient.ListResources(ctx, cluster, "node", "", nil)
	if err != nil {
		return "", fmt.Errorf("failed to list nodes: %w", err)
	}

	// Parse node label selector if provided
	nodeSelectorMap := parseLabelSelector(nodeLabelSelector)

	// Build node info map
	nodeInfoMap := make(map[string]*CapacityNodeInfo)
	for _, node := range nodes.Items {
		// Filter by node label selector if provided
		if len(nodeSelectorMap) > 0 {
			labels := node.GetLabels()
			if !matchLabels(labels, nodeSelectorMap) {
				continue
			}
		}

		info := &CapacityNodeInfo{
			Name:   node.GetName(),
			CPU:    CapacityResource{},
			Memory: CapacityResource{},
			Labels: node.GetLabels(),
		}

		// Extract capacity
		if capacity, found, _ := unstructured.NestedMap(node.Object, "status", "capacity"); found {
			if cpu, ok := capacity["cpu"].(string); ok {
				info.CPU.Capacity = parseResourceQuantity(cpu)
			}
			if mem, ok := capacity["memory"].(string); ok {
				info.Memory.Capacity = parseResourceQuantity(mem)
			}
			if pods, ok := capacity["pods"].(string); ok {
				info.PodCount.Capacity, _ = strconv.ParseInt(pods, 10, 64)
			}
		}

		// Extract allocatable
		if allocatable, found, _ := unstructured.NestedMap(node.Object, "status", "allocatable"); found {
			if cpu, ok := allocatable["cpu"].(string); ok {
				info.CPU.Allocatable = parseResourceQuantity(cpu)
			}
			if mem, ok := allocatable["memory"].(string); ok {
				info.Memory.Allocatable = parseResourceQuantity(mem)
			}
			if pods, ok := allocatable["pods"].(string); ok {
				info.PodCount.Allocatable, _ = strconv.ParseInt(pods, 10, 64)
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
				info.Taints = append(info.Taints, taint)
			}
		}

		// Filter by noTaint - skip nodes with any taints
		if noTaint && len(info.Taints) > 0 {
			continue
		}

		// Filter by taint selector if provided
		if nodeTaints != "" && !matchTaints(info.Taints, nodeTaints) {
			continue
		}

		nodeInfoMap[info.Name] = info
	}

	// Build namespace filter map if namespaceLabelSelector is provided
	namespaceFilter := make(map[string]bool)
	if namespaceLabelSelector != "" {
		nsSelector := parseLabelSelector(namespaceLabelSelector)
		if len(nsSelector) > 0 {
			nsList, err := steveClient.ListResources(ctx, cluster, "namespace", "", nil)
			if err == nil {
				for _, ns := range nsList.Items {
					if matchLabels(ns.GetLabels(), nsSelector) {
						namespaceFilter[ns.GetName()] = true
					}
				}
			}
		}
	}

	// Get pods
	podOpts := &steve.ListOptions{}
	if labelSelector != "" {
		podOpts.LabelSelector = labelSelector
	}
	pods, err := steveClient.ListResources(ctx, cluster, "pod", namespace, podOpts)
	if err != nil {
		return "", fmt.Errorf("failed to list pods: %w", err)
	}

	// Process pods and aggregate by node
	for _, pod := range pods.Items {
		// Filter by namespace labels if provided
		if len(namespaceFilter) > 0 {
			if !namespaceFilter[pod.GetNamespace()] {
				continue
			}
		}

		nodeName := ""
		if n, found, _ := unstructured.NestedString(pod.Object, "spec", "nodeName"); found {
			nodeName = n
		}

		// Skip pods not assigned to a node
		if nodeName == "" {
			continue
		}

		nodeInfo, ok := nodeInfoMap[nodeName]
		if !ok {
			continue
		}

		// Count this pod
		nodeInfo.PodCount.Requested++

		// Extract container resources
		containers, found, _ := unstructured.NestedSlice(pod.Object, "spec", "containers")
		if !found {
			continue
		}

		podInfo := CapacityPodInfo{
			Namespace:    pod.GetNamespace(),
			Name:         pod.GetName(),
			ContainerCnt: len(containers),
		}

		for _, c := range containers {
			container, ok := c.(map[string]interface{})
			if !ok {
				continue
			}

			containerName := ""
			if name, ok := container["name"].(string); ok {
				containerName = name
			}

			containerInfo := CapacityContainerInfo{
				Name: containerName,
			}

			resources, found, _ := unstructured.NestedMap(container, "resources")
			if !found {
				continue
			}

			// Parse requests
			if requests, found, _ := unstructured.NestedMap(resources, "requests"); found {
				if cpu, ok := requests["cpu"].(string); ok {
					containerInfo.CPU.Requested = parseResourceQuantity(cpu)
					podInfo.CPU.Requested += containerInfo.CPU.Requested
				}
				if memory, ok := requests["memory"].(string); ok {
					containerInfo.Memory.Requested = parseResourceQuantity(memory)
					podInfo.Memory.Requested += containerInfo.Memory.Requested
				}
			}

			// Parse limits
			if limits, found, _ := unstructured.NestedMap(resources, "limits"); found {
				if cpu, ok := limits["cpu"].(string); ok {
					containerInfo.CPU.Limited = parseResourceQuantity(cpu)
					podInfo.CPU.Limited += containerInfo.CPU.Limited
				}
				if memory, ok := limits["memory"].(string); ok {
					containerInfo.Memory.Limited = parseResourceQuantity(memory)
					podInfo.Memory.Limited += containerInfo.Memory.Limited
				}
			}

			if showContainers {
				podInfo.Containers = append(podInfo.Containers, containerInfo)
			}
		}

		// Aggregate to node
		nodeInfo.CPU.Requested += podInfo.CPU.Requested
		nodeInfo.Memory.Requested += podInfo.Memory.Requested
		nodeInfo.CPU.Limited += podInfo.CPU.Limited
		nodeInfo.Memory.Limited += podInfo.Memory.Limited

		if showPods {
			nodeInfo.Pods = append(nodeInfo.Pods, podInfo)
		}
	}

	// Get utilization metrics if requested
	if showUtil {
		getNodeMetrics(ctx, steveClient, cluster, nodeInfoMap)
	}

	// Build result
	result := CapacityResult{
		Nodes:          make([]CapacityNodeInfo, 0, len(nodeInfoMap)),
		ShowPods:       showPods,
		ShowContainers: showContainers,
		ShowUtil:       showUtil,
		ShowAvailable:  showAvailable,
		ShowPodCount:   showPodCount,
		ShowLabels:     showLabels,
		HideRequests:   hideRequests,
		HideLimits:     hideLimits,
	}

	// Calculate cluster totals
	clusterInfo := CapacityNodeInfo{
		Name:     "*",
		CPU:      CapacityResource{},
		Memory:   CapacityResource{},
		PodCount: PodCountInfo{},
	}

	for _, info := range nodeInfoMap {
		result.Nodes = append(result.Nodes, *info)
		clusterInfo.CPU.Capacity += info.CPU.Capacity
		clusterInfo.CPU.Allocatable += info.CPU.Allocatable
		clusterInfo.CPU.Requested += info.CPU.Requested
		clusterInfo.CPU.Limited += info.CPU.Limited
		clusterInfo.CPU.Utilized += info.CPU.Utilized
		clusterInfo.Memory.Capacity += info.Memory.Capacity
		clusterInfo.Memory.Allocatable += info.Memory.Allocatable
		clusterInfo.Memory.Requested += info.Memory.Requested
		clusterInfo.Memory.Limited += info.Memory.Limited
		clusterInfo.Memory.Utilized += info.Memory.Utilized
		clusterInfo.PodCount.Capacity += info.PodCount.Capacity
		clusterInfo.PodCount.Allocatable += info.PodCount.Allocatable
		clusterInfo.PodCount.Requested += info.PodCount.Requested
	}

	// Sort nodes if requested
	if sortBy != "" {
		sortCapacityNodes(result.Nodes, sortBy)
	}

	result.Cluster = clusterInfo

	// Format output
	switch format {
	case handler.FormatYAML:
		data, err := yaml.Marshal(result)
		if err != nil {
			return "", fmt.Errorf("failed to format as YAML: %w", err)
		}
		return string(data), nil
	case handler.FormatJSON:
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to format as JSON: %w", err)
		}
		return string(data), nil
	default: // table
		return formatCapacityAsTable(result, showAvailable), nil
	}
}

// getNodeMetrics retrieves node metrics from metrics-server
func getNodeMetrics(ctx context.Context, steveClient *steve.Client, cluster string, nodeInfoMap map[string]*CapacityNodeInfo) {
	metrics, err := steveClient.ListResources(ctx, cluster, "node.metrics.k8s.io", "", nil)
	if err != nil {
		// metrics-server not available, silently skip
		return
	}

	for _, metric := range metrics.Items {
		nodeName := metric.GetName()
		nodeInfo, ok := nodeInfoMap[nodeName]
		if !ok {
			continue
		}

		if usage, found, _ := unstructured.NestedMap(metric.Object, "usage"); found {
			if cpu, ok := usage["cpu"].(string); ok {
				nodeInfo.CPU.Utilized = parseResourceQuantity(cpu)
			}
			if memory, ok := usage["memory"].(string); ok {
				nodeInfo.Memory.Utilized = parseResourceQuantity(memory)
			}
		}
	}
}

// sortCapacityNodes sorts nodes by the specified field
func sortCapacityNodes(nodes []CapacityNodeInfo, sortBy string) {
	less := sortLessFunc(sortBy)
	if less != nil {
		sort.Slice(nodes, less(nodes))
	}
}

// sortLessFunc returns a comparison function for the given sort field
func sortLessFunc(sortBy string) func([]CapacityNodeInfo) func(i, j int) bool {
	switch sortBy {
	case "cpu.util":
		return func(nodes []CapacityNodeInfo) func(i, j int) bool {
			return func(i, j int) bool { return nodes[i].CPU.Utilized > nodes[j].CPU.Utilized }
		}
	case "mem.util", "memory.util":
		return func(nodes []CapacityNodeInfo) func(i, j int) bool {
			return func(i, j int) bool { return nodes[i].Memory.Utilized > nodes[j].Memory.Utilized }
		}
	case "cpu.request":
		return func(nodes []CapacityNodeInfo) func(i, j int) bool {
			return func(i, j int) bool { return nodes[i].CPU.Requested > nodes[j].CPU.Requested }
		}
	case "mem.request", "memory.request":
		return func(nodes []CapacityNodeInfo) func(i, j int) bool {
			return func(i, j int) bool { return nodes[i].Memory.Requested > nodes[j].Memory.Requested }
		}
	case "cpu.limit":
		return func(nodes []CapacityNodeInfo) func(i, j int) bool {
			return func(i, j int) bool { return nodes[i].CPU.Limited > nodes[j].CPU.Limited }
		}
	case "mem.limit", "memory.limit":
		return func(nodes []CapacityNodeInfo) func(i, j int) bool {
			return func(i, j int) bool { return nodes[i].Memory.Limited > nodes[j].Memory.Limited }
		}
	case "cpu.util.percentage":
		return func(nodes []CapacityNodeInfo) func(i, j int) bool {
			return func(i, j int) bool {
				return calcPercentage(nodes[i].CPU.Utilized, nodes[i].CPU.Allocatable) >
					calcPercentage(nodes[j].CPU.Utilized, nodes[j].CPU.Allocatable)
			}
		}
	case "mem.util.percentage", "memory.util.percentage":
		return func(nodes []CapacityNodeInfo) func(i, j int) bool {
			return func(i, j int) bool {
				return calcPercentage(nodes[i].Memory.Utilized, nodes[i].Memory.Allocatable) >
					calcPercentage(nodes[j].Memory.Utilized, nodes[j].Memory.Allocatable)
			}
		}
	case "cpu.request.percentage":
		return func(nodes []CapacityNodeInfo) func(i, j int) bool {
			return func(i, j int) bool {
				return calcPercentage(nodes[i].CPU.Requested, nodes[i].CPU.Allocatable) >
					calcPercentage(nodes[j].CPU.Requested, nodes[j].CPU.Allocatable)
			}
		}
	case "mem.request.percentage", "memory.request.percentage":
		return func(nodes []CapacityNodeInfo) func(i, j int) bool {
			return func(i, j int) bool {
				return calcPercentage(nodes[i].Memory.Requested, nodes[i].Memory.Allocatable) >
					calcPercentage(nodes[j].Memory.Requested, nodes[j].Memory.Allocatable)
			}
		}
	case "cpu.limit.percentage":
		return func(nodes []CapacityNodeInfo) func(i, j int) bool {
			return func(i, j int) bool {
				return calcPercentage(nodes[i].CPU.Limited, nodes[i].CPU.Allocatable) >
					calcPercentage(nodes[j].CPU.Limited, nodes[j].CPU.Allocatable)
			}
		}
	case "mem.limit.percentage", "memory.limit.percentage":
		return func(nodes []CapacityNodeInfo) func(i, j int) bool {
			return func(i, j int) bool {
				return calcPercentage(nodes[i].Memory.Limited, nodes[i].Memory.Allocatable) >
					calcPercentage(nodes[j].Memory.Limited, nodes[j].Memory.Allocatable)
			}
		}
	case "name":
		return func(nodes []CapacityNodeInfo) func(i, j int) bool {
			return func(i, j int) bool { return nodes[i].Name < nodes[j].Name }
		}
	}
	return nil
}

// formatCapacityAsTable formats capacity result as a human-readable table
func formatCapacityAsTable(result CapacityResult, showAvailable bool) string {
	var b strings.Builder

	// Print node and cluster summary
	writeNodeSummary(&b, result, showAvailable)

	// Print utilization section if requested
	if result.ShowUtil {
		writeUtilizationSection(&b, result.Nodes)
	}

	// Print pods section if requested
	if result.ShowPods {
		writePodsSection(&b, result, showAvailable)
	}

	return b.String()
}

// writeNodeSummary writes the node and cluster summary table
func writeNodeSummary(b *strings.Builder, result CapacityResult, showAvailable bool) {
	tb := newTableBuilder("%-25s", "NAME")

	if !result.HideRequests {
		tb.addColumn("%-12s", "CPU REQUEST", "MEM REQUEST")
	}
	if !result.HideLimits {
		tb.addColumn("%-12s", "CPU LIMIT", "MEM LIMIT")
	}
	if result.ShowPodCount {
		tb.addColumn("%-6s", "PODS")
	}
	if result.ShowLabels {
		tb.addColumn("%-s", "LABELS")
	}

	// Print nodes section
	fmt.Fprintf(b, "NODE\n")
	tb.writeHeader(b)
	tb.writeSeparator(b)

	for _, node := range result.Nodes {
		row := []interface{}{truncate(node.Name, 25)}
		if !result.HideRequests {
			row = append(row, formatCPU(node.CPU.Requested, showAvailable), formatMemory(node.Memory.Requested, showAvailable))
		}
		if !result.HideLimits {
			row = append(row, formatCPU(node.CPU.Limited, showAvailable), formatMemory(node.Memory.Limited, showAvailable))
		}
		if result.ShowPodCount {
			row = append(row, fmt.Sprintf("%d/%d", node.PodCount.Requested, node.PodCount.Allocatable))
		}
		if result.ShowLabels {
			row = append(row, formatLabels(node.Labels))
		}
		tb.writeRow(b, row)
	}

	// Print cluster totals
	fmt.Fprintf(b, "\nCLUSTER\n")
	tb.writeHeader(b)
	tb.writeSeparator(b)

	row := []interface{}{result.Cluster.Name}
	if !result.HideRequests {
		row = append(row, formatCPU(result.Cluster.CPU.Requested, showAvailable), formatMemory(result.Cluster.Memory.Requested, showAvailable))
	}
	if !result.HideLimits {
		row = append(row, formatCPU(result.Cluster.CPU.Limited, showAvailable), formatMemory(result.Cluster.Memory.Limited, showAvailable))
	}
	if result.ShowPodCount {
		row = append(row, fmt.Sprintf("%d/%d", result.Cluster.PodCount.Requested, result.Cluster.PodCount.Allocatable))
	}
	if result.ShowLabels {
		row = append(row, "")
	}
	tb.writeRow(b, row)
}

// tableBuilder helps build formatted tables
type tableBuilder struct {
	formats []string
	headers []string
}

func newTableBuilder(format, header string) *tableBuilder {
	return &tableBuilder{
		formats: []string{format},
		headers: []string{header},
	}
}

func (tb *tableBuilder) addColumn(format string, headers ...string) {
	for range headers {
		tb.formats = append(tb.formats, format)
	}
	tb.headers = append(tb.headers, headers...)
}

func (tb *tableBuilder) writeHeader(b *strings.Builder) {
	fmt.Fprintf(b, strings.Join(tb.formats, " ")+"\n", toAnySlice(tb.headers)...)
}

func (tb *tableBuilder) writeSeparator(b *strings.Builder) {
	separators := make([]string, len(tb.headers))
	for i, h := range tb.headers {
		separators[i] = strings.Repeat("-", len(h))
	}
	fmt.Fprintf(b, strings.Join(tb.formats, " ")+"\n", toAnySlice(separators)...)
}

func (tb *tableBuilder) writeRow(b *strings.Builder, values []interface{}) {
	fmt.Fprintf(b, strings.Join(tb.formats, " ")+"\n", values...)
}

// writeUtilizationSection writes the utilization section
func writeUtilizationSection(b *strings.Builder, nodes []CapacityNodeInfo) {
	fmt.Fprintf(b, "\nNODE UTILIZATION\n")
	fmt.Fprintf(b, "%-25s %-12s %-12s %-12s %-12s\n", "NAME", "CPU CAP", "CPU UTIL%", "MEM CAP", "MEM UTIL%")
	fmt.Fprintf(b, "%-25s %-12s %-12s %-12s %-12s\n", "----", "-------", "---------", "-------", "---------")

	for _, node := range nodes {
		cpuUtilPct := calcPercentage(node.CPU.Utilized, node.CPU.Allocatable)
		memUtilPct := calcPercentage(node.Memory.Utilized, node.Memory.Allocatable)
		fmt.Fprintf(b, "%-25s %-12s %-11.1f%% %-12s %-11.1f%%\n",
			truncate(node.Name, 25),
			formatCPU(node.CPU.Allocatable, true),
			cpuUtilPct,
			formatMemory(node.Memory.Allocatable, true),
			memUtilPct,
		)
	}
}

// calcPercentage calculates percentage with zero check
func calcPercentage(value, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return float64(value) / float64(total) * 100
}

// writePodsSection writes the pods section with optional container details
func writePodsSection(b *strings.Builder, result CapacityResult, showAvailable bool) {
	fmt.Fprintf(b, "\nPODS\n")

	for _, node := range result.Nodes {
		if len(node.Pods) == 0 {
			continue
		}
		fmt.Fprintf(b, "\n%s (%d pods)\n", node.Name, len(node.Pods))

		tb := newTableBuilder("  %-40s", "POD")
		tb.addColumn("%-10s", "NAMESPACE")
		if !result.HideRequests {
			tb.addColumn("%-12s", "CPU REQUEST", "MEM REQUEST")
		}
		if !result.HideLimits {
			tb.addColumn("%-12s", "CPU LIMIT", "MEM LIMIT")
		}

		tb.writeHeader(b)
		tb.writeSeparator(b)

		for _, pod := range node.Pods {
			row := []interface{}{truncate(pod.Name, 40), truncate(pod.Namespace, 10)}
			if !result.HideRequests {
				row = append(row, formatCPU(pod.CPU.Requested, showAvailable), formatMemory(pod.Memory.Requested, showAvailable))
			}
			if !result.HideLimits {
				row = append(row, formatCPU(pod.CPU.Limited, showAvailable), formatMemory(pod.Memory.Limited, showAvailable))
			}
			tb.writeRow(b, row)

			if result.ShowContainers {
				writeContainers(b, pod.Containers, result, showAvailable)
			}
		}
	}
}

// writeContainers writes container details for a pod
func writeContainers(b *strings.Builder, containers []CapacityContainerInfo, result CapacityResult, showAvailable bool) {
	if len(containers) == 0 {
		return
	}

	tb := newTableBuilder("  %-38s", "[C]")
	if !result.HideRequests {
		tb.addColumn("%-12s", "CPU REQUEST", "MEM REQUEST")
	}
	if !result.HideLimits {
		tb.addColumn("%-12s", "CPU LIMIT", "MEM LIMIT")
	}

	for _, c := range containers {
		row := []interface{}{truncate(c.Name, 38)}
		if !result.HideRequests {
			row = append(row, formatCPU(c.CPU.Requested, showAvailable), formatMemory(c.Memory.Requested, showAvailable))
		}
		if !result.HideLimits {
			row = append(row, formatCPU(c.CPU.Limited, showAvailable), formatMemory(c.Memory.Limited, showAvailable))
		}
		tb.writeRow(b, row)
	}
}

// toAnySlice converts a string slice to any slice for fmt.Fprintf
func toAnySlice(ss []string) []any {
	result := make([]any, len(ss))
	for i, s := range ss {
		result[i] = s
	}
	return result
}

// formatLabels formats node labels as a string
func formatLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	var parts []string
	for k, v := range labels {
		if v == "" {
			parts = append(parts, k)
		} else {
			parts = append(parts, fmt.Sprintf("%s=%s", k, v))
		}
	}
	return truncate(strings.Join(parts, ","), 60)
}

// formatCPU formats CPU value (millicores)
func formatCPU(val int64, showRaw bool) string {
	if showRaw {
		if val >= 1000 {
			return fmt.Sprintf("%dm", val)
		}
		return fmt.Sprintf("%dm", val)
	}
	// Show as cores
	return fmt.Sprintf("%.2f", float64(val)/1000)
}

// formatMemory formats memory value (bytes)
func formatMemory(val int64, showRaw bool) string {
	if showRaw {
		if val >= 1024*1024*1024 {
			return fmt.Sprintf("%dGi", val/(1024*1024*1024))
		}
		if val >= 1024*1024 {
			return fmt.Sprintf("%dMi", val/(1024*1024))
		}
		if val >= 1024 {
			return fmt.Sprintf("%dKi", val/1024)
		}
		return fmt.Sprintf("%d", val)
	}
	// Show as Gi
	return fmt.Sprintf("%.2fGi", float64(val)/(1024*1024*1024))
}

// parseLabelSelector parses a label selector string into a map.
// Supports format: "key1=value1,key2=value2" or "key1=value1 key2=value2"
// Also supports "key1,key2" format (existence check only)
func parseLabelSelector(selector string) map[string]string {
	result := make(map[string]string)
	if selector == "" {
		return result
	}

	// Split by comma or space
	parts := strings.FieldsFunc(selector, func(r rune) bool {
		return r == ',' || r == ' '
	})

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Check for key=value or key==value format
		if idx := strings.Index(part, "=="); idx != -1 {
			key := strings.TrimSpace(part[:idx])
			value := strings.TrimSpace(part[idx+2:])
			result[key] = value
		} else if idx := strings.Index(part, "="); idx != -1 {
			key := strings.TrimSpace(part[:idx])
			value := strings.TrimSpace(part[idx+1:])
			result[key] = value
		}
		// Note: existence check only (e.g., "key") is not supported in this simple parser
	}

	return result
}

// matchTaints checks if node taints match the taint selector expression.
// Format: "key=value:effect" to include, "key=value:effect-" to exclude
// Multiple taints can be separated by comma
func matchTaints(taints []corev1.Taint, selector string) bool {
	if selector == "" {
		return true
	}

	// Parse taint selector
	parts := strings.Split(selector, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Check for exclusion (ends with -)
		exclude := false
		if strings.HasSuffix(part, "-") {
			exclude = true
			part = part[:len(part)-1]
		}

		// Parse taint: key=value:effect
		var key, value, effect string
		if idx := strings.Index(part, "="); idx != -1 {
			key = part[:idx]
			rest := part[idx+1:]
			if idx2 := strings.Index(rest, ":"); idx2 != -1 {
				value = rest[:idx2]
				effect = rest[idx2+1:]
			} else {
				value = rest
			}
		} else if idx := strings.Index(part, ":"); idx != -1 {
			// key:effect format (no value)
			key = part[:idx]
			effect = part[idx+1:]
		} else {
			// Just key
			key = part
		}

		// Check if taint exists on node
		found := false
		for _, t := range taints {
			if t.Key == key {
				if value != "" && t.Value != value {
					continue
				}
				if effect != "" && string(t.Effect) != effect {
					continue
				}
				found = true
				break
			}
		}

		// For inclusion (no suffix), taint must be found
		// For exclusion (- suffix), taint must NOT be found
		if exclude && found {
			return false
		}
		if !exclude && !found {
			return false
		}
	}

	return true
}

// matchLabels checks if the given labels match the selector map.
// All selector key-value pairs must match the labels.
func matchLabels(labels, selector map[string]string) bool {
	for key, value := range selector {
		if labels[key] != value {
			return false
		}
	}
	return true
}
