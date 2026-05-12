package aggregate

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/futuretea/rancher-mcp-server/pkg/client/steve"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// SummaryAnalyzer performs resource summary analysis
type SummaryAnalyzer struct {
	client *steve.Client
}

// NewSummaryAnalyzer creates a new summary analyzer
func NewSummaryAnalyzer(client *steve.Client) *SummaryAnalyzer {
	return &SummaryAnalyzer{client: client}
}

// Analyze performs the resource summary analysis
func (a *SummaryAnalyzer) Analyze(ctx context.Context, p SummaryParams) (*SummaryResult, error) {
	opts := &steve.ListOptions{}
	if p.LabelSelector != "" {
		opts.LabelSelector = p.LabelSelector
	}

	pods, err := a.client.ListResources(ctx, p.Cluster, "pod", p.Namespace, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	// Group aggregation
	groups := make(map[string]*SummaryItem)
	groupBy := strings.ToLower(p.GroupBy)
	if groupBy == "" {
		groupBy = "namespace"
	}

	for _, pod := range pods.Items {
		var groupKey string
		switch groupBy {
		case "label":
			labels := pod.GetLabels()
			if p.GroupByKey == "" {
				return nil, fmt.Errorf("groupByKey is required when groupBy=label")
			}
			groupKey = labels[p.GroupByKey]
			if groupKey == "" {
				groupKey = "<none>"
			}
		default:
			groupKey = pod.GetNamespace()
		}

		if _, ok := groups[groupKey]; !ok {
			groups[groupKey] = &SummaryItem{Group: groupKey}
		}

		aggregatePodResources(pod, groups[groupKey])
	}

	// Convert map to slice
	items := make([]SummaryItem, 0, len(groups))
	for _, item := range groups {
		items = append(items, *item)
	}

	total := len(items)

	// Sort
	if p.SortBy != "" {
		sortSummaryItems(items, p.SortBy)
	}

	// Truncate to limit (capped at MaxItems)
	limit := ClampLimit(p.Limit)
	truncated := len(items) > limit
	if truncated {
		items = items[:limit]
	}

	return &SummaryResult{
		Items:     items,
		Truncated: truncated,
		Total:     total,
	}, nil
}

// aggregatePodResources aggregates pod container resources into a summary item
func aggregatePodResources(pod unstructured.Unstructured, item *SummaryItem) {
	item.PodCount++

	containers, found, _ := unstructured.NestedSlice(pod.Object, "spec", "containers")
	if !found {
		return
	}

	for _, c := range containers {
		container, ok := c.(map[string]interface{})
		if !ok {
			continue
		}

		resources, found, _ := unstructured.NestedMap(container, "resources")
		if !found {
			continue
		}

		// Requests
		if requests, found, _ := unstructured.NestedMap(resources, "requests"); found {
			if cpu, ok := requests["cpu"].(string); ok {
				item.CPUReq += resourceQuantityToMilli(cpu)
			}
			if mem, ok := requests["memory"].(string); ok {
				item.MemReq += resourceQuantityToBytes(mem)
			}
		}

		// Limits
		if limits, found, _ := unstructured.NestedMap(resources, "limits"); found {
			if cpu, ok := limits["cpu"].(string); ok {
				item.CPULimit += resourceQuantityToMilli(cpu)
			}
			if mem, ok := limits["memory"].(string); ok {
				item.MemLimit += resourceQuantityToBytes(mem)
			}
		}
	}
}

// sortSummaryItems sorts summary items by the specified field
func sortSummaryItems(items []SummaryItem, sortBy string) {
	sort.Slice(items, func(i, j int) bool {
		a, b := items[i], items[j]
		switch sortBy {
		case "cpu.request":
			return a.CPUReq > b.CPUReq
		case "mem.request", "memory.request":
			return a.MemReq > b.MemReq
		case "cpu.limit":
			return a.CPULimit > b.CPULimit
		case "mem.limit", "memory.limit":
			return a.MemLimit > b.MemLimit
		case "pod.count":
			return a.PodCount > b.PodCount
		case "name":
			return strings.ToLower(a.Group) < strings.ToLower(b.Group)
		default:
			return strings.ToLower(a.Group) < strings.ToLower(b.Group)
		}
	})
}
