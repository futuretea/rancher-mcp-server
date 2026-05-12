package aggregate

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/futuretea/rancher-mcp-server/pkg/client/steve"
	"github.com/futuretea/rancher-mcp-server/pkg/core/logging"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// TopAnalyzer performs top-ranking analysis for pods or nodes
type TopAnalyzer struct {
	client *steve.Client
}

// NewTopAnalyzer creates a new top analyzer
func NewTopAnalyzer(client *steve.Client) *TopAnalyzer {
	return &TopAnalyzer{client: client}
}

// Analyze performs the top analysis
func (a *TopAnalyzer) Analyze(ctx context.Context, p TopParams) (*TopResult, error) {
	// Normalize kind
	kind := strings.ToLower(p.Kind)
	if kind == "" {
		kind = "pod"
	}

	switch kind {
	case "pod":
		return a.analyzePods(ctx, p)
	case "node":
		return a.analyzeNodes(ctx, p)
	default:
		return nil, fmt.Errorf("unsupported kind for top: %s (supported: pod, node)", p.Kind)
	}
}

// analyzePods ranks pods by resource usage
func (a *TopAnalyzer) analyzePods(ctx context.Context, p TopParams) (*TopResult, error) {
	opts := &steve.ListOptions{}
	if p.LabelSelector != "" {
		opts.LabelSelector = p.LabelSelector
	}

	pods, err := a.client.ListResources(ctx, p.Cluster, "pod", p.Namespace, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	// Fetch pod metrics if needed for utilization sorting
	metricsMap := make(map[string]*podMetrics)
	var warning string
	if needsPodMetrics(p.SortBy) {
		m, w := a.fetchPodMetrics(ctx, p.Cluster, p.Namespace)
		metricsMap = m
		warning = w
	}

	items := make([]TopItem, 0, len(pods.Items))
	for _, pod := range pods.Items {
		item := extractPodTopItem(pod, metricsMap)
		items = append(items, item)
	}

	return a.buildResult(items, p, warning)
}

// analyzeNodes ranks nodes by resource usage
func (a *TopAnalyzer) analyzeNodes(ctx context.Context, p TopParams) (*TopResult, error) {
	nodes, err := a.client.ListResources(ctx, p.Cluster, "node", "", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	// Fetch node metrics if needed
	metricsMap := make(map[string]*nodeMetrics)
	var warning string
	if needsNodeMetrics(p.SortBy) {
		m, w := a.fetchNodeMetrics(ctx, p.Cluster)
		metricsMap = m
		warning = w
	}

	items := make([]TopItem, 0, len(nodes.Items))
	for _, node := range nodes.Items {
		item := extractNodeTopItem(node, metricsMap)
		items = append(items, item)
	}

	return a.buildResult(items, p, warning)
}

// podMetrics holds metrics data for a single pod
type podMetrics struct {
	cpuUtil int64
	memUtil int64
}

// nodeMetrics holds metrics data for a single node
type nodeMetrics struct {
	cpuUtil int64
	memUtil int64
}

// fetchPodMetrics retrieves pod metrics from metrics-server
func (a *TopAnalyzer) fetchPodMetrics(ctx context.Context, cluster, namespace string) (map[string]*podMetrics, string) {
	metricsList, err := a.client.ListResources(ctx, cluster, "pod.metrics.k8s.io", namespace, nil)
	if err != nil {
		logging.Debug("Failed to get pod metrics (metrics-server may not be installed): %v", err)
		return nil, "metrics-server unavailable: utilization data omitted"
	}

	result := make(map[string]*podMetrics)
	for _, m := range metricsList.Items {
		name := m.GetName()
		ns := m.GetNamespace()
		key := ns + "/" + name

		pm := &podMetrics{}
		if containers, found, _ := unstructured.NestedSlice(m.Object, "containers"); found {
			for _, c := range containers {
				container, ok := c.(map[string]interface{})
				if !ok {
					continue
				}
				if usage, found, _ := unstructured.NestedMap(container, "usage"); found {
					if cpu, ok := usage["cpu"].(string); ok {
						pm.cpuUtil += resourceQuantityToMilli(cpu)
					}
					if mem, ok := usage["memory"].(string); ok {
						pm.memUtil += resourceQuantityToBytes(mem)
					}
				}
			}
		}
		result[key] = pm
	}
	return result, ""
}

// fetchNodeMetrics retrieves node metrics from metrics-server
func (a *TopAnalyzer) fetchNodeMetrics(ctx context.Context, cluster string) (map[string]*nodeMetrics, string) {
	metricsList, err := a.client.ListResources(ctx, cluster, "node.metrics.k8s.io", "", nil)
	if err != nil {
		logging.Debug("Failed to get node metrics (metrics-server may not be installed): %v", err)
		return nil, "metrics-server unavailable: utilization data omitted"
	}

	result := make(map[string]*nodeMetrics)
	for _, m := range metricsList.Items {
		name := m.GetName()
		nm := &nodeMetrics{}
		if usage, found, _ := unstructured.NestedMap(m.Object, "usage"); found {
			if cpu, ok := usage["cpu"].(string); ok {
				nm.cpuUtil = resourceQuantityToMilli(cpu)
			}
			if mem, ok := usage["memory"].(string); ok {
				nm.memUtil = resourceQuantityToBytes(mem)
			}
		}
		result[name] = nm
	}
	return result, ""
}

// extractPodTopItem extracts a TopItem from a pod unstructured object
func extractPodTopItem(pod unstructured.Unstructured, metricsMap map[string]*podMetrics) TopItem {
	item := TopItem{
		Name:      pod.GetName(),
		Namespace: pod.GetNamespace(),
	}

	// Extract container resources
	containers, found, _ := unstructured.NestedSlice(pod.Object, "spec", "containers")
	if found {
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

	// Extract restart count from container statuses
	statuses, found, _ := unstructured.NestedSlice(pod.Object, "status", "containerStatuses")
	if found {
		for _, s := range statuses {
			status, ok := s.(map[string]interface{})
			if !ok {
				continue
			}
			item.Restarts += extractRestartCount(status)
		}
	}

	// Apply metrics if available
	key := item.Namespace + "/" + item.Name
	if m, ok := metricsMap[key]; ok {
		item.CPUUtil = m.cpuUtil
		item.MemUtil = m.memUtil
	}

	return item
}

// extractNodeTopItem extracts a TopItem from a node unstructured object
func extractNodeTopItem(node unstructured.Unstructured, metricsMap map[string]*nodeMetrics) TopItem {
	item := TopItem{
		Name: node.GetName(),
	}

	// Extract capacity and allocatable
	if capacity, found, _ := unstructured.NestedMap(node.Object, "status", "capacity"); found {
		if cpu, ok := capacity["cpu"].(string); ok {
			item.CPUReq = resourceQuantityToMilli(cpu) // Use CPUReq as capacity for nodes
		}
		if mem, ok := capacity["memory"].(string); ok {
			item.MemReq = resourceQuantityToBytes(mem) // Use MemReq as capacity for nodes
		}
	}

	// Extract allocatable as limit
	if allocatable, found, _ := unstructured.NestedMap(node.Object, "status", "allocatable"); found {
		if cpu, ok := allocatable["cpu"].(string); ok {
			item.CPULimit = resourceQuantityToMilli(cpu)
		}
		if mem, ok := allocatable["memory"].(string); ok {
			item.MemLimit = resourceQuantityToBytes(mem)
		}
	}

	// Apply metrics if available
	name := item.Name
	if m, ok := metricsMap[name]; ok {
		item.CPUUtil = m.cpuUtil
		item.MemUtil = m.memUtil
	}

	return item
}

// buildResult sorts, truncates, and builds the final TopResult
func (a *TopAnalyzer) buildResult(items []TopItem, p TopParams, warning string) (*TopResult, error) {
	total := len(items)
	truncated := false

	// Sort
	if p.SortBy != "" {
		sortTopItems(items, p.Kind, p.SortBy)
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

	return &TopResult{
		Items:     items,
		Truncated: truncated,
		Total:     total,
		Warning:   warning,
	}, nil
}

// needsPodMetrics returns true if the sortBy requires pod metrics data
func needsPodMetrics(sortBy string) bool {
	switch sortBy {
	case "cpu.util", "mem.util", "cpu.util.percentage", "mem.util.percentage":
		return true
	default:
		return false
	}
}

// needsNodeMetrics returns true if the sortBy requires node metrics data
func needsNodeMetrics(sortBy string) bool {
	switch sortBy {
	case "cpu.util", "mem.util", "cpu.util.percentage", "mem.util.percentage":
		return true
	default:
		return false
	}
}

// sortTopItems sorts top items by the specified field
func sortTopItems(items []TopItem, kind, sortBy string) {
	sort.Slice(items, func(i, j int) bool {
		a, b := items[i], items[j]
		switch sortBy {
		case "cpu.util":
			return a.CPUUtil > b.CPUUtil
		case "mem.util", "memory.util":
			return a.MemUtil > b.MemUtil
		case "cpu.request":
			return a.CPUReq > b.CPUReq
		case "mem.request", "memory.request":
			return a.MemReq > b.MemReq
		case "cpu.limit":
			return a.CPULimit > b.CPULimit
		case "mem.limit", "memory.limit":
			return a.MemLimit > b.MemLimit
		case "cpu.util.percentage":
			if strings.ToLower(kind) == "node" {
				return calcPercentage(a.CPUUtil, a.CPUReq) > calcPercentage(b.CPUUtil, b.CPUReq)
			}
			return calcPercentage(a.CPUUtil, a.CPUReq) > calcPercentage(b.CPUUtil, b.CPUReq)
		case "mem.util.percentage", "memory.util.percentage":
			if strings.ToLower(kind) == "node" {
				return calcPercentage(a.MemUtil, a.MemReq) > calcPercentage(b.MemUtil, b.MemReq)
			}
			return calcPercentage(a.MemUtil, a.MemReq) > calcPercentage(b.MemUtil, b.MemReq)
		case "restart.count":
			return a.Restarts > b.Restarts
		case "pod.count":
			// For nodes, pod.count is not directly available in TopItem
			// Fall through to name
			return strings.ToLower(a.Name) < strings.ToLower(b.Name)
		case "name":
			return strings.ToLower(a.Name) < strings.ToLower(b.Name)
		default:
			return strings.ToLower(a.Name) < strings.ToLower(b.Name)
		}
	})
}

// calcPercentage calculates percentage with zero check
func calcPercentage(value, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return float64(value) / float64(total) * 100
}

// resourceQuantityToMilli parses a resource quantity string and returns millivalue
func resourceQuantityToMilli(q string) int64 {
	if q == "" {
		return 0
	}
	qty := resource.MustParse(q)
	return qty.MilliValue()
}

// resourceQuantityToBytes parses a resource quantity string and returns bytes
func resourceQuantityToBytes(q string) int64 {
	if q == "" {
		return 0
	}
	qty := resource.MustParse(q)
	return qty.Value()
}

// extractRestartCount extracts restart count from a map, handling int64/float64 types
func extractRestartCount(status map[string]interface{}) int32 {
	v := status["restartCount"]
	switch n := v.(type) {
	case int64:
		return int32(n)
	case int32:
		return n
	case int:
		return int32(n)
	case float64:
		return int32(n)
	case float32:
		return int32(n)
	default:
		return 0
	}
}
