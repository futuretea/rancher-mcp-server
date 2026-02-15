package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/futuretea/rancher-mcp-server/pkg/toolset"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset/handler"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
)

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
