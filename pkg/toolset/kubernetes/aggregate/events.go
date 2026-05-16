// Package aggregate provides event pattern analysis, grouping Kubernetes events
// by reason, kind, and frequency to identify recurring issues.
package aggregate

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/futuretea/rancher-mcp-server/pkg/client/steve"
	corev1 "k8s.io/api/core/v1"
)

// EventAnalyzer performs event summary analysis
type EventAnalyzer struct {
	client steve.ResourceReader
}

// NewEventAnalyzer creates a new event analyzer
func NewEventAnalyzer(client steve.ResourceReader) *EventAnalyzer {
	return &EventAnalyzer{client: client}
}

// Analyze performs event summary analysis
func (a *EventAnalyzer) Analyze(ctx context.Context, p EventParams) (*EventResult, error) {
	// Parse since duration
	var sinceThreshold time.Time
	if p.Since != "" {
		d, err := time.ParseDuration(p.Since)
		if err != nil {
			return nil, fmt.Errorf("invalid since duration: %w", err)
		}
		sinceThreshold = time.Now().Add(-d)
	}

	// Get events; kind filter is applied to involvedObject.kind
	events, err := a.client.GetEvents(ctx, p.Cluster, p.Namespace, "", p.Kind)
	if err != nil {
		return nil, fmt.Errorf("failed to get events: %w", err)
	}

	return summarizeEvents(events, p, sinceThreshold), nil
}

func summarizeEvents(events []corev1.Event, p EventParams, sinceThreshold time.Time) *EventResult {
	// Group by (reason, kind, namespace)
	type groupKey struct{ reason, kind, ns string }
	groups := make(map[groupKey]*EventItem)
	for _, event := range events {
		// Filter by type
		if p.Type != "" && event.Type != p.Type {
			continue
		}

		// Filter by since
		if !sinceThreshold.IsZero() {
			lastTime := event.LastTimestamp.Time
			if lastTime.IsZero() {
				lastTime = event.EventTime.Time
			}
			if !lastTime.IsZero() && lastTime.Before(sinceThreshold) {
				continue
			}
		}

		key := groupKey{
			reason: event.Reason,
			kind:   event.InvolvedObject.Kind,
			ns:     event.InvolvedObject.Namespace,
		}
		if key.ns == "" {
			key.ns = event.Namespace
		}

		if _, ok := groups[key]; !ok {
			groups[key] = &EventItem{
				Reason:    key.reason,
				Kind:      key.kind,
				Namespace: key.ns,
			}
		}

		item := groups[key]
		item.Count++

		// Track latest timestamp
		lastTime := event.LastTimestamp.Time
		if lastTime.IsZero() {
			lastTime = event.EventTime.Time
		}
		if !lastTime.IsZero() && lastTime.After(item.LastSeen) {
			item.LastSeen = lastTime
		}
	}

	// Convert map to slice
	items := make([]EventItem, 0, len(groups))
	for _, item := range groups {
		items = append(items, *item)
	}

	total := len(items)

	// Sort
	if p.SortBy != "" {
		sortEventItems(items, p.SortBy)
	}

	// Truncate to limit (capped at MaxItems)
	limit := ClampLimit(p.Limit)
	truncated := len(items) > limit
	if truncated {
		items = items[:limit]
	}

	return &EventResult{
		Items:     items,
		Truncated: truncated,
		Total:     total,
	}
}

// sortEventItems sorts event items by the specified field
func sortEventItems(items []EventItem, sortBy string) {
	sort.Slice(items, func(i, j int) bool {
		a, b := items[i], items[j]
		switch sortBy {
		case "count":
			return a.Count > b.Count
		case "lastSeen":
			return a.LastSeen.After(b.LastSeen)
		case "name":
			return strings.ToLower(a.Reason) < strings.ToLower(b.Reason)
		default:
			return strings.ToLower(a.Reason) < strings.ToLower(b.Reason)
		}
	})
}
