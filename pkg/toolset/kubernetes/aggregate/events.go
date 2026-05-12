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
	client *steve.Client
}

// NewEventAnalyzer creates a new event analyzer
func NewEventAnalyzer(client *steve.Client) *EventAnalyzer {
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

	// Get events
	// kind filter is applied to involvedObject.kind
	var kindFilter string
	if p.Kind != "" {
		kindFilter = p.Kind
	}

	events, err := a.client.GetEvents(ctx, p.Cluster, p.Namespace, "", kindFilter)
	if err != nil {
		return nil, fmt.Errorf("failed to get events: %w", err)
	}

	// Group by (reason, kind, namespace)
	groups := make(map[string]*EventItem)
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
			if lastTime.Before(sinceThreshold) {
				continue
			}
		}

		reason := event.Reason
		kind := event.InvolvedObject.Kind
		ns := event.InvolvedObject.Namespace
		if ns == "" {
			ns = event.Namespace
		}

		key := reason + "|" + kind + "|" + ns
		if _, ok := groups[key]; !ok {
			groups[key] = &EventItem{
				Reason:    reason,
				Kind:      kind,
				Namespace: ns,
			}
		}

		item := groups[key]
		item.Count++

		// Track latest timestamp
		lastTime := event.LastTimestamp.Time
		if lastTime.IsZero() {
			lastTime = event.EventTime.Time
		}
		if lastTime.After(item.LastSeen) {
			item.LastSeen = lastTime
		}
	}

	// Convert map to slice
	items := make([]EventItem, 0, len(groups))
	for _, item := range groups {
		items = append(items, *item)
	}

	total := len(items)
	truncated := false

	// Sort
	if p.SortBy != "" {
		sortEventItems(items, p.SortBy)
	}

	// Truncate if exceeds max
	if len(items) > MaxItems {
		items = items[:MaxItems]
		truncated = true
	}

	// Apply limit
	limit := p.Limit
	if limit <= 0 {
		limit = DefaultLimit
	}
	if limit > MaxItems {
		limit = MaxItems
	}
	if len(items) > limit {
		items = items[:limit]
	}

	return &EventResult{
		Items:     items,
		Truncated: truncated,
		Total:     total,
	}, nil
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

// Ensure corev1 import is used (GetEvents returns []corev1.Event)
var _ corev1.Event
