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
	if len(line) < RFC3339NanoLen {
		return time.Time{}, line
	}
	// Try to parse ISO 8601 timestamp at the beginning of the line
	timestampStr := line[:RFC3339NanoLen]
	if t, err := time.Parse(time.RFC3339Nano, timestampStr); err == nil {
		return t, line[RFC3339NanoLen+1:]
	}
	// Try without nanoseconds
	if len(line) >= RFC3339Len {
		timestampStr = line[:RFC3339Len]
		if t, err := time.Parse(time.RFC3339, timestampStr); err == nil {
			return t, line[RFC3339Len+1:]
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
