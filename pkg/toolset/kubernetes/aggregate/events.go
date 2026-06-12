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

// eventGroupKey identifies a group of events by reason, kind, and namespace.
type eventGroupKey struct{ reason, kind, ns string }

func summarizeEvents(events []corev1.Event, p EventParams, sinceThreshold time.Time) *EventResult {
	groups := make(map[eventGroupKey]*EventItem)
	for _, event := range events {
		if shouldSkipEvent(event, p, sinceThreshold) {
			continue
		}
		key := makeEventGroupKey(event)
		item := groups[key]
		if item == nil {
			item = &EventItem{
				Reason:    key.reason,
				Kind:      key.kind,
				Namespace: key.ns,
			}
			groups[key] = item
		}
		updateEventItem(item, event)
	}
	return buildEventResult(groups, p.SortBy, p.Limit)
}

func shouldSkipEvent(event corev1.Event, p EventParams, sinceThreshold time.Time) bool {
	if p.Type != "" && event.Type != p.Type {
		return true
	}
	if !sinceThreshold.IsZero() {
		lastTime := eventLastTimestamp(event)
		if !lastTime.IsZero() && lastTime.Before(sinceThreshold) {
			return true
		}
	}
	return false
}

func eventLastTimestamp(event corev1.Event) time.Time {
	if !event.LastTimestamp.Time.IsZero() {
		return event.LastTimestamp.Time
	}
	return event.EventTime.Time
}

func makeEventGroupKey(event corev1.Event) eventGroupKey {
	key := eventGroupKey{
		reason: event.Reason,
		kind:   event.InvolvedObject.Kind,
		ns:     event.InvolvedObject.Namespace,
	}
	if key.ns == "" {
		key.ns = event.Namespace
	}
	return key
}

func updateEventItem(item *EventItem, event corev1.Event) {
	item.Count++
	lastTime := eventLastTimestamp(event)
	if !lastTime.IsZero() && lastTime.After(item.LastSeen) {
		item.LastSeen = lastTime
	}
}

func buildEventResult(groups map[eventGroupKey]*EventItem, sortBy string, limit int) *EventResult {
	items := make([]EventItem, 0, len(groups))
	for _, item := range groups {
		items = append(items, *item)
	}

	total := len(items)

	if sortBy != "" {
		sortEventItems(items, sortBy)
	}

	limit = ClampLimit(limit)
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
