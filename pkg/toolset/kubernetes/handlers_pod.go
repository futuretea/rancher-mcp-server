package kubernetes

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/futuretea/rancher-mcp-server/pkg/client/steve"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset/paramutil"
)

// LogEntry represents a single log line with its timestamp
type LogEntry struct {
	Timestamp time.Time
	Content   string
	Pod       string
	Container string
}

// multiPodLogClient is the subset of *steve.Client used by getMultiPodLogs.
type multiPodLogClient interface {
	GetMultiPodLogs(ctx context.Context, clusterID, namespace, labelSelector string, opts *steve.PodLogOptions) ([]steve.MultiPodLogResult, error)
}

// allContainerLogClient is the subset of *steve.Client used by getAllContainerLogs.
type allContainerLogClient interface {
	GetAllContainerLogs(ctx context.Context, clusterID, namespace, podName string, opts *steve.PodLogOptions) (map[string]string, error)
}

// logsHandler handles the kubernetes_logs tool
func logsHandler(ctx context.Context, client interface{}, params map[string]interface{}) (string, error) {
	steveClient, err := toolset.ValidateSteveClient(client)
	if err != nil {
		return "", err
	}

	cluster, err := paramutil.ExtractRequiredString(params, paramutil.ParamCluster)
	if err != nil {
		return "", err
	}
	namespace, err := paramutil.ExtractRequiredString(params, paramutil.ParamNamespace)
	if err != nil {
		return "", err
	}
	name := paramutil.ExtractOptionalString(params, paramutil.ParamName)
	labelSelector := paramutil.ExtractOptionalString(params, paramutil.ParamLabelSelector)
	container := paramutil.ExtractOptionalString(params, paramutil.ParamContainer)
	tailLines := paramutil.ExtractInt64(params, paramutil.ParamTailLines, 100)
	sinceSeconds := paramutil.ExtractOptionalInt64(params, paramutil.ParamSinceSeconds)
	timestamps := paramutil.ExtractBool(params, paramutil.ParamTimestamps, false)
	previous := paramutil.ExtractBool(params, paramutil.ParamPrevious, false)
	keyword := paramutil.ExtractOptionalString(params, paramutil.ParamKeyword)

	// If labelSelector is provided, get logs from multiple pods
	if labelSelector != "" {
		return getMultiPodLogs(ctx, steveClient, cluster, namespace, labelSelector, container, tailLines, sinceSeconds, previous, keyword, timestamps)
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

	return getAllContainerLogs(ctx, steveClient, cluster, namespace, name, &steve.PodLogOptions{
		TailLines:    &tailLines,
		SinceSeconds: sinceSeconds,
		Timestamps:   timestamps,
		Previous:     previous,
	}, keyword)
}

// getMultiPodLogs retrieves and merges logs from multiple pods matching the label selector
// Logs are sorted by timestamp when timestamps is true.
func getMultiPodLogs(ctx context.Context, client multiPodLogClient, cluster, namespace, labelSelector, container string, tailLines int64, sinceSeconds *int64, previous bool, keyword string, timestamps bool) (string, error) {
	opts := &steve.PodLogOptions{
		TailLines:    &tailLines,
		SinceSeconds: sinceSeconds,
		Timestamps:   timestamps,
		Previous:     previous,
	}

	results, err := client.GetMultiPodLogs(ctx, cluster, namespace, labelSelector, opts)
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
				appendLogEntries(&allEntries, containerLogs, result.Pod, container, keyword)
			}
			continue
		}

		// Process all containers in this pod
		for containerName, containerLogs := range result.Logs {
			appendLogEntries(&allEntries, containerLogs, result.Pod, containerName, keyword)
		}
	}

	if len(allEntries) == 0 {
		return "No log entries found", nil
	}

	return formatLogEntries(allEntries, timestamps, func(entry LogEntry) string {
		if timestamps && !entry.Timestamp.IsZero() {
			return fmt.Sprintf("[%s/%s] %s", entry.Pod, entry.Container, formatTimestampedContent(entry.Timestamp, entry.Content))
		}
		return fmt.Sprintf("[%s/%s] %s", entry.Pod, entry.Container, entry.Content)
	}), nil
}

// appendLogEntries parses log lines, optionally filters by keyword, and appends them to entries.
func appendLogEntries(entries *[]LogEntry, logs, pod, container, keyword string) {
	filteredLogs := filterLogsByKeyword(logs, keyword)
	lines := strings.Split(filteredLogs, "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		ts, content := parseLogTimestamp(line)
		*entries = append(*entries, LogEntry{
			Timestamp: ts,
			Content:   content,
			Pod:       pod,
			Container: container,
		})
	}
}

// getAllContainerLogs retrieves and formats logs from all containers in a pod.
func getAllContainerLogs(ctx context.Context, client allContainerLogClient, cluster, namespace, name string, opts *steve.PodLogOptions, keyword string) (string, error) {
	logs, err := client.GetAllContainerLogs(ctx, cluster, namespace, name, opts)
	if err != nil {
		return "", fmt.Errorf("failed to get pod logs: %w", err)
	}

	timestamps := false
	if opts != nil {
		timestamps = opts.Timestamps
	}

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

	return formatLogEntries(allEntries, timestamps, func(entry LogEntry) string {
		if timestamps && !entry.Timestamp.IsZero() {
			return fmt.Sprintf("[%s] %s", entry.Container, formatTimestampedContent(entry.Timestamp, entry.Content))
		}
		return fmt.Sprintf("[%s] %s", entry.Container, entry.Content)
	}), nil
}

// formatLogEntries sorts log entries by timestamp and formats them.
func formatLogEntries(entries []LogEntry, timestamps bool, formatEntry func(LogEntry) string) string {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Timestamp.IsZero() || entries[j].Timestamp.IsZero() {
			if entries[i].Pod != entries[j].Pod {
				return entries[i].Pod < entries[j].Pod
			}
			return entries[i].Container < entries[j].Container
		}
		return entries[i].Timestamp.Before(entries[j].Timestamp)
	})

	var resultLines []string
	for _, entry := range entries {
		resultLines = append(resultLines, formatEntry(entry))
	}
	return strings.Join(resultLines, "\n")
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

// parseLogTimestamp extracts timestamp from a log line.
// Kubernetes log format: 2024-01-15T10:30:00.123456789Z log message
// Returns the timestamp and the remaining log content.
func parseLogTimestamp(line string) (time.Time, string) {
	timestampStr, content, ok := splitLogTimestamp(line)
	if !ok {
		return time.Time{}, line
	}

	if t, err := time.Parse(time.RFC3339Nano, timestampStr); err == nil {
		return t, content
	}
	if t, err := time.Parse(time.RFC3339, timestampStr); err == nil {
		return t, content
	}
	return time.Time{}, line
}

func splitLogTimestamp(line string) (timestamp, content string, ok bool) {
	if idx := strings.IndexByte(line, ' '); idx >= 0 {
		return line[:idx], line[idx+1:], true
	}
	return line, "", line != ""
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
			result = append(result, formatTimestampedContent(entry.Timestamp, entry.Content))
		} else {
			result = append(result, entry.Content)
		}
	}
	return strings.Join(result, "\n")
}

func formatTimestampedContent(timestamp time.Time, content string) string {
	line := timestamp.Format(time.RFC3339Nano)
	if content != "" {
		line += " " + content
	}
	return line
}

// inspectPodHandler handles the kubernetes_inspect_pod tool
func inspectPodHandler(ctx context.Context, client interface{}, params map[string]interface{}) (string, error) {
	steveClient, err := toolset.ValidateSteveClient(client)
	if err != nil {
		return "", err
	}

	cluster, err := paramutil.ExtractRequiredString(params, paramutil.ParamCluster)
	if err != nil {
		return "", err
	}
	namespace, err := paramutil.ExtractRequiredString(params, paramutil.ParamNamespace)
	if err != nil {
		return "", err
	}
	name, err := paramutil.ExtractRequiredString(params, paramutil.ParamName)
	if err != nil {
		return "", err
	}

	result, err := steveClient.InspectPod(ctx, cluster, namespace, name)
	if err != nil {
		return "", fmt.Errorf("failed to inspect pod: %w", err)
	}

	return result.ToJSON()
}
