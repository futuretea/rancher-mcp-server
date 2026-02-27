package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/futuretea/rancher-mcp-server/pkg/toolset"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset/paramutil"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
)

// describeHandler handles the kubernetes_describe tool
func describeHandler(ctx context.Context, client interface{}, params map[string]interface{}) (string, error) {
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

	result, err := steveClient.DescribeResource(ctx, cluster, kind, namespace, name)
	if err != nil {
		return "", fmt.Errorf("failed to describe resource: %w", err)
	}

	// Mask sensitive data (e.g., Secret data) unless showSensitiveData is true
	if sensitiveFilter := paramutil.NewSensitiveDataFilterFromParams(params); sensitiveFilter != nil {
		result.Resource = sensitiveFilter.Filter(result.Resource)
	}

	switch format {
	case paramutil.FormatYAML:
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
func eventsHandler(ctx context.Context, client interface{}, params map[string]interface{}) (string, error) {
	steveClient, err := toolset.ValidateSteveClient(client)
	if err != nil {
		return "", err
	}

	cluster, err := paramutil.ExtractRequiredString(params, paramutil.ParamCluster)
	if err != nil {
		return "", err
	}
	namespace := paramutil.ExtractOptionalString(params, paramutil.ParamNamespace)
	nameFilter := paramutil.ExtractOptionalString(params, paramutil.ParamName)
	kindFilter := paramutil.ExtractOptionalString(params, paramutil.ParamKind)
	limit := paramutil.ExtractInt64(params, paramutil.ParamLimit, 50)
	page := paramutil.ExtractInt64(params, paramutil.ParamPage, 1)
	format := paramutil.ExtractOptionalStringWithDefault(params, paramutil.ParamFormat, paramutil.FormatTable)

	events, err := steveClient.GetEvents(ctx, cluster, namespace, nameFilter, kindFilter)
	if err != nil {
		return "", fmt.Errorf("failed to get events: %w", err)
	}

	sortEventsByTime(events)

	events, _ = paramutil.ApplyPagination(events, limit, page)

	if len(events) == 0 {
		return "No events found", nil
	}

	switch format {
	case paramutil.FormatYAML:
		data, err := yaml.Marshal(events)
		if err != nil {
			return "", fmt.Errorf("failed to format as YAML: %w", err)
		}
		return string(data), nil
	case paramutil.FormatTable:
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
