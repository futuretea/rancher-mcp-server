package aggregate

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/futuretea/rancher-mcp-server/pkg/client/steve"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// WorkloadAnalyzer performs workload health analysis
type WorkloadAnalyzer struct {
	client *steve.Client
}

// NewWorkloadAnalyzer creates a new workload analyzer
func NewWorkloadAnalyzer(client *steve.Client) *WorkloadAnalyzer {
	return &WorkloadAnalyzer{client: client}
}

// Analyze performs workload health analysis
func (a *WorkloadAnalyzer) Analyze(ctx context.Context, p WorkloadParams) (*WorkloadResult, error) {
	var allItems []WorkloadItem

	kind := strings.ToLower(p.Kind)
	if kind == "" || kind == "all" {
		// List all workload kinds
		for _, k := range []string{"deployment", "statefulset", "daemonset"} {
			items, err := a.listWorkload(ctx, p.Cluster, p.Namespace, k, p.LabelSelector)
			if err != nil {
				// Log and continue - some kinds might not be available
				continue
			}
			allItems = append(allItems, items...)
		}
	} else {
		items, err := a.listWorkload(ctx, p.Cluster, p.Namespace, kind, p.LabelSelector)
		if err != nil {
			return nil, err
		}
		allItems = items
	}

	total := len(allItems)
	truncated := false

	// Sort
	if p.SortBy != "" {
		sortWorkloadItems(allItems, p.SortBy)
	}

	// Truncate if exceeds max
	if len(allItems) > MaxItems {
		allItems = allItems[:MaxItems]
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
	if len(allItems) > limit {
		allItems = allItems[:limit]
	}

	return &WorkloadResult{
		Items:     allItems,
		Truncated: truncated,
		Total:     total,
	}, nil
}

// listWorkload lists a specific workload kind and extracts health info
func (a *WorkloadAnalyzer) listWorkload(ctx context.Context, cluster, namespace, kind, labelSelector string) ([]WorkloadItem, error) {
	opts := &steve.ListOptions{}
	if labelSelector != "" {
		opts.LabelSelector = labelSelector
	}

	list, err := a.client.ListResources(ctx, cluster, kind, namespace, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list %s: %w", kind, err)
	}

	items := make([]WorkloadItem, 0, len(list.Items))
	for _, obj := range list.Items {
		item := extractWorkloadItem(obj, kind)
		items = append(items, item)
	}

	return items, nil
}

// extractWorkloadItem extracts a WorkloadItem from an unstructured object
func extractWorkloadItem(obj unstructured.Unstructured, kind string) WorkloadItem {
	item := WorkloadItem{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		Kind:      capitalize(kind),
	}

	// Extract status fields
	replicas, _, _ := unstructured.NestedInt64(obj.Object, "status", "replicas")
	readyReplicas, _, _ := unstructured.NestedInt64(obj.Object, "status", "readyReplicas")
	updatedReplicas, _, _ := unstructured.NestedInt64(obj.Object, "status", "updatedReplicas")
	unavailableReplicas, _, _ := unstructured.NestedInt64(obj.Object, "status", "unavailableReplicas")

	item.Desired = int32(replicas)
	item.Ready = int32(readyReplicas)
	item.Updated = int32(updatedReplicas)
	item.Unavailable = int32(unavailableReplicas)

	// Derive status
	item.Status = deriveWorkloadStatus(item)

	// Extract age from creation timestamp
	creationTime := obj.GetCreationTimestamp()
	if !creationTime.IsZero() {
		item.Age = formatAge(creationTime.Time)
	}

	return item
}

// deriveWorkloadStatus derives the workload status based on replica counts
func deriveWorkloadStatus(item WorkloadItem) string {
	if item.Unavailable > 0 {
		return "Progressing"
	}
	if item.Ready == item.Desired && item.Desired > 0 {
		return "Healthy"
	}
	return "Degraded"
}

// sortWorkloadItems sorts workload items by the specified field
func sortWorkloadItems(items []WorkloadItem, sortBy string) {
	sort.Slice(items, func(i, j int) bool {
		a, b := items[i], items[j]
		switch sortBy {
		case "unready.count":
			return (a.Desired - a.Ready) > (b.Desired - b.Ready)
		case "ready.ratio":
			ra := calcRatio(a.Ready, a.Desired)
			rb := calcRatio(b.Ready, b.Desired)
			return ra < rb // Lower ready ratio first (worst first)
		case "age":
			// We only have formatted age string, can't sort by it precisely
			// Fall through to name
			return strings.ToLower(a.Name) < strings.ToLower(b.Name)
		case "name":
			return strings.ToLower(a.Name) < strings.ToLower(b.Name)
		default:
			return strings.ToLower(a.Name) < strings.ToLower(b.Name)
		}
	})
}

// calcRatio calculates a ratio with zero check
func calcRatio(part, total int32) float64 {
	if total <= 0 {
		return 0
	}
	return float64(part) / float64(total)
}

// capitalize capitalizes the first letter of a string
func capitalize(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
