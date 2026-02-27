package capacity

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/futuretea/rancher-mcp-server/pkg/client/steve"
	"github.com/futuretea/rancher-mcp-server/pkg/core/logging"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Analyzer performs capacity analysis
type Analyzer struct {
	client *steve.Client
}

// NewAnalyzer creates a new capacity analyzer
func NewAnalyzer(client *steve.Client) *Analyzer {
	return &Analyzer{client: client}
}

// Analyze performs the capacity analysis
func (a *Analyzer) Analyze(ctx context.Context, p Params) (*Result, error) {
	// containers implies pods
	if p.ShowContainers {
		p.ShowPods = true
	}

	// Build node info map
	nodeInfoMap, err := a.buildNodeInfoMap(ctx, p)
	if err != nil {
		return nil, err
	}

	// Process pods
	if err := a.processPods(ctx, nodeInfoMap, p); err != nil {
		return nil, err
	}

	// Get utilization metrics if requested
	if p.ShowUtil {
		a.getNodeMetrics(ctx, p.Cluster, nodeInfoMap)
	}

	// Build result
	result := a.buildResult(nodeInfoMap, p)
	return &result, nil
}

// buildNodeInfoMap builds a map of node name to NodeInfo
func (a *Analyzer) buildNodeInfoMap(ctx context.Context, p Params) (map[string]*NodeInfo, error) {
	nodes, err := a.client.ListResources(ctx, p.Cluster, "node", "", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	nodeSelectorMap := parseLabelSelector(p.NodeLabelSelector)
	nodeInfoMap := make(map[string]*NodeInfo)

	for _, node := range nodes.Items {
		if !matchesNodeSelector(node, nodeSelectorMap) {
			continue
		}

		info := extractNodeInfo(node)

		if p.NoTaint && len(info.Taints) > 0 {
			continue
		}

		if p.NodeTaints != "" && !matchTaints(info.Taints, p.NodeTaints) {
			continue
		}

		nodeInfoMap[info.Name] = info
	}

	return nodeInfoMap, nil
}

// processPods processes all pods and aggregates resources by node
func (a *Analyzer) processPods(ctx context.Context, nodeInfoMap map[string]*NodeInfo, p Params) error {
	namespaceFilter, err := a.buildNamespaceFilter(ctx, p)
	if err != nil {
		return err
	}

	podOpts := &steve.ListOptions{}
	if p.LabelSelector != "" {
		podOpts.LabelSelector = p.LabelSelector
	}

	pods, err := a.client.ListResources(ctx, p.Cluster, "pod", p.Namespace, podOpts)
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	for _, pod := range pods.Items {
		if !shouldProcessPod(pod, nodeInfoMap, namespaceFilter) {
			continue
		}

		nodeName, _, _ := unstructured.NestedString(pod.Object, "spec", "nodeName")
		processSinglePod(pod, nodeInfoMap[nodeName], p.ShowPods, p.ShowContainers)
	}

	return nil
}

// buildNamespaceFilter builds a filter map for namespace label selection
func (a *Analyzer) buildNamespaceFilter(ctx context.Context, p Params) (map[string]bool, error) {
	if p.NamespaceLabelSelector == "" {
		return nil, nil
	}

	nsSelector := parseLabelSelector(p.NamespaceLabelSelector)
	if len(nsSelector) == 0 {
		return nil, nil
	}

	nsList, err := a.client.ListResources(ctx, p.Cluster, "namespace", "", nil)
	if err != nil {
		return nil, err
	}

	filter := make(map[string]bool)
	for _, ns := range nsList.Items {
		if matchLabels(ns.GetLabels(), nsSelector) {
			filter[ns.GetName()] = true
		}
	}

	return filter, nil
}

// getNodeMetrics retrieves node metrics from metrics-server
func (a *Analyzer) getNodeMetrics(ctx context.Context, cluster string, nodeInfoMap map[string]*NodeInfo) {
	metrics, err := a.client.ListResources(ctx, cluster, "node.metrics.k8s.io", "", nil)
	if err != nil {
		// metrics-server not available, log at debug level and skip
		logging.Debug("Failed to get node metrics (metrics-server may not be installed): %v", err)
		return
	}

	for _, metric := range metrics.Items {
		nodeName := metric.GetName()
		nodeInfo, ok := nodeInfoMap[nodeName]
		if !ok {
			continue
		}

		if usage, found, _ := unstructured.NestedMap(metric.Object, "usage"); found {
			if cpu, ok := usage["cpu"].(string); ok {
				nodeInfo.CPU.Utilized = resourceQuantityToMilli(cpu)
			}
			if mem, ok := usage["memory"].(string); ok {
				nodeInfo.Memory.Utilized = resourceQuantityToBytes(mem)
			}
		}
	}
}

// buildResult builds the Result from node info map
func (a *Analyzer) buildResult(nodeInfoMap map[string]*NodeInfo, p Params) Result {
	result := Result{
		Nodes:          make([]NodeInfo, 0, len(nodeInfoMap)),
		ShowPods:       p.ShowPods,
		ShowContainers: p.ShowContainers,
		ShowUtil:       p.ShowUtil,
		ShowAvailable:  p.ShowAvailable,
		ShowPodCount:   p.ShowPodCount,
		ShowLabels:     p.ShowLabels,
		HideRequests:   p.HideRequests,
		HideLimits:     p.HideLimits,
	}

	clusterInfo := NodeInfo{Name: "*"}

	for _, info := range nodeInfoMap {
		result.Nodes = append(result.Nodes, *info)
		aggregateNodeToCluster(&clusterInfo, info)
	}

	// Sort nodes if requested
	if p.SortBy != "" {
		SortNodes(result.Nodes, p.SortBy)
	}

	result.Cluster = clusterInfo
	return result
}

// matchesNodeSelector checks if a node matches the label selector
func matchesNodeSelector(node unstructured.Unstructured, selector map[string]string) bool {
	if len(selector) == 0 {
		return true
	}
	return matchLabels(node.GetLabels(), selector)
}

// extractNodeInfo extracts NodeInfo from an unstructured node object
func extractNodeInfo(node unstructured.Unstructured) *NodeInfo {
	info := &NodeInfo{
		Name:   node.GetName(),
		Labels: node.GetLabels(),
	}

	// Extract capacity
	if capacity, found, _ := unstructured.NestedMap(node.Object, "status", "capacity"); found {
		if cpu, ok := capacity["cpu"].(string); ok {
			info.CPU.Capacity = resourceQuantityToMilli(cpu)
		}
		if mem, ok := capacity["memory"].(string); ok {
			info.Memory.Capacity = resourceQuantityToBytes(mem)
		}
		if pods, ok := capacity["pods"].(string); ok {
			info.PodCount.Capacity, _ = strconv.ParseInt(pods, 10, 64)
		}
	}

	// Extract allocatable
	if allocatable, found, _ := unstructured.NestedMap(node.Object, "status", "allocatable"); found {
		if cpu, ok := allocatable["cpu"].(string); ok {
			info.CPU.Allocatable = resourceQuantityToMilli(cpu)
		}
		if mem, ok := allocatable["memory"].(string); ok {
			info.Memory.Allocatable = resourceQuantityToBytes(mem)
		}
		if pods, ok := allocatable["pods"].(string); ok {
			info.PodCount.Allocatable, _ = strconv.ParseInt(pods, 10, 64)
		}
	}

	// Extract taints
	info.Taints = extractTaints(node)

	return info
}

// extractTaints extracts taints from a node object
func extractTaints(node unstructured.Unstructured) []corev1.Taint {
	taints, found, _ := unstructured.NestedSlice(node.Object, "spec", "taints")
	if !found {
		return nil
	}

	result := make([]corev1.Taint, 0, len(taints))
	for _, t := range taints {
		taintMap, ok := t.(map[string]interface{})
		if !ok {
			continue
		}

		taint := corev1.Taint{}
		if key, ok := taintMap["key"].(string); ok {
			taint.Key = key
		}
		if value, ok := taintMap["value"].(string); ok {
			taint.Value = value
		}
		if effect, ok := taintMap["effect"].(string); ok {
			taint.Effect = corev1.TaintEffect(effect)
		}
		result = append(result, taint)
	}

	return result
}

// shouldProcessPod checks if a pod should be processed
func shouldProcessPod(pod unstructured.Unstructured, nodeInfoMap map[string]*NodeInfo, namespaceFilter map[string]bool) bool {
	// Filter by namespace labels
	if len(namespaceFilter) > 0 && !namespaceFilter[pod.GetNamespace()] {
		return false
	}

	// Filter out completed/failed pods
	phase, _, _ := unstructured.NestedString(pod.Object, "status", "phase")
	if phase == "Succeeded" || phase == "Failed" {
		return false
	}

	// Skip unassigned pods
	nodeName, _, _ := unstructured.NestedString(pod.Object, "spec", "nodeName")
	if nodeName == "" {
		return false
	}

	// Skip pods on filtered-out nodes
	_, ok := nodeInfoMap[nodeName]
	return ok
}

// processSinglePod processes a single pod and adds its resources to the node
func processSinglePod(pod unstructured.Unstructured, nodeInfo *NodeInfo, showPods, showContainers bool) {
	nodeInfo.PodCount.Requested++

	podInfo := PodInfo{
		Namespace: pod.GetNamespace(),
		Name:      pod.GetName(),
	}

	// Process containers and init containers
	processContainers(pod, &podInfo, showContainers)
	processInitContainers(pod, &podInfo, showContainers)

	// Aggregate to node
	nodeInfo.CPU.Requested += podInfo.CPU.Requested
	nodeInfo.Memory.Requested += podInfo.Memory.Requested
	nodeInfo.CPU.Limited += podInfo.CPU.Limited
	nodeInfo.Memory.Limited += podInfo.Memory.Limited

	if showPods {
		nodeInfo.Pods = append(nodeInfo.Pods, podInfo)
	}
}

// processContainers processes regular containers from a pod
func processContainers(pod unstructured.Unstructured, podInfo *PodInfo, showContainers bool) {
	containers, found, _ := unstructured.NestedSlice(pod.Object, "spec", "containers")
	if !found {
		return
	}

	podInfo.ContainerCnt += len(containers)
	for _, c := range containers {
		container, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		processContainerResources(container, podInfo, showContainers, false)
	}
}

// processInitContainers processes init containers from a pod
func processInitContainers(pod unstructured.Unstructured, podInfo *PodInfo, showContainers bool) {
	containers, found, _ := unstructured.NestedSlice(pod.Object, "spec", "initContainers")
	if !found {
		return
	}

	podInfo.ContainerCnt += len(containers)
	for _, c := range containers {
		container, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		processContainerResources(container, podInfo, showContainers, true)
	}
}

// processContainerResources extracts resource information from a container
func processContainerResources(container map[string]interface{}, podInfo *PodInfo, showContainers, isInit bool) {
	name, _ := container["name"].(string)

	containerInfo := ContainerInfo{
		Name: name,
		Init: isInit,
	}

	resources, found, _ := unstructured.NestedMap(container, "resources")
	if !found {
		if showContainers {
			podInfo.Containers = append(podInfo.Containers, containerInfo)
		}
		return
	}

	// Parse requests
	extractResourceRequests(resources, &containerInfo, podInfo)

	// Parse limits
	extractResourceLimits(resources, &containerInfo, podInfo)

	if showContainers {
		podInfo.Containers = append(podInfo.Containers, containerInfo)
	}
}

// extractResourceRequests extracts request resources from container resources
func extractResourceRequests(resources map[string]interface{}, containerInfo *ContainerInfo, podInfo *PodInfo) {
	requests, found, _ := unstructured.NestedMap(resources, "requests")
	if !found {
		return
	}

	if cpu, ok := requests["cpu"].(string); ok {
		containerInfo.CPU.Requested = resourceQuantityToMilli(cpu)
		podInfo.CPU.Requested += containerInfo.CPU.Requested
	}
	if mem, ok := requests["memory"].(string); ok {
		containerInfo.Memory.Requested = resourceQuantityToBytes(mem)
		podInfo.Memory.Requested += containerInfo.Memory.Requested
	}
}

// extractResourceLimits extracts limit resources from container resources
func extractResourceLimits(resources map[string]interface{}, containerInfo *ContainerInfo, podInfo *PodInfo) {
	limits, found, _ := unstructured.NestedMap(resources, "limits")
	if !found {
		return
	}

	if cpu, ok := limits["cpu"].(string); ok {
		containerInfo.CPU.Limited = resourceQuantityToMilli(cpu)
		podInfo.CPU.Limited += containerInfo.CPU.Limited
	}
	if mem, ok := limits["memory"].(string); ok {
		containerInfo.Memory.Limited = resourceQuantityToBytes(mem)
		podInfo.Memory.Limited += containerInfo.Memory.Limited
	}
}

// aggregateNodeToCluster aggregates node resources to cluster totals
func aggregateNodeToCluster(cluster, node *NodeInfo) {
	cluster.CPU.Capacity += node.CPU.Capacity
	cluster.CPU.Allocatable += node.CPU.Allocatable
	cluster.CPU.Requested += node.CPU.Requested
	cluster.CPU.Limited += node.CPU.Limited
	cluster.CPU.Utilized += node.CPU.Utilized

	cluster.Memory.Capacity += node.Memory.Capacity
	cluster.Memory.Allocatable += node.Memory.Allocatable
	cluster.Memory.Requested += node.Memory.Requested
	cluster.Memory.Limited += node.Memory.Limited
	cluster.Memory.Utilized += node.Memory.Utilized

	cluster.PodCount.Capacity += node.PodCount.Capacity
	cluster.PodCount.Allocatable += node.PodCount.Allocatable
	cluster.PodCount.Requested += node.PodCount.Requested
}

// parseLabelSelector parses a label selector string into a map
func parseLabelSelector(selector string) map[string]string {
	result := make(map[string]string)
	if selector == "" {
		return result
	}

	parts := strings.FieldsFunc(selector, func(r rune) bool { return r == ',' || r == ' ' })

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		if idx := strings.Index(part, "=="); idx != -1 {
			result[strings.TrimSpace(part[:idx])] = strings.TrimSpace(part[idx+2:])
		} else if idx := strings.Index(part, "="); idx != -1 {
			result[strings.TrimSpace(part[:idx])] = strings.TrimSpace(part[idx+1:])
		}
	}

	return result
}

// matchTaints checks if node taints match the taint selector expression
func matchTaints(taints []corev1.Taint, selector string) bool {
	if selector == "" {
		return true
	}

	for _, part := range strings.Split(selector, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		exclude := strings.HasSuffix(part, "-")
		if exclude {
			part = part[:len(part)-1]
		}

		key, value, effect := parseTaintPart(part)
		found := taintMatches(taints, key, value, effect)

		if exclude && found {
			return false
		}
		if !exclude && !found {
			return false
		}
	}

	return true
}

// parseTaintPart parses a single taint selector part
func parseTaintPart(part string) (key, value, effect string) {
	if idx := strings.Index(part, "="); idx != -1 {
		key = part[:idx]
		rest := part[idx+1:]
		if idx2 := strings.Index(rest, ":"); idx2 != -1 {
			value = rest[:idx2]
			effect = rest[idx2+1:]
		} else {
			value = rest
		}
	} else if idx := strings.Index(part, ":"); idx != -1 {
		key = part[:idx]
		effect = part[idx+1:]
	} else {
		key = part
	}
	return
}

// taintMatches checks if a taint matching the criteria exists in the list
func taintMatches(taints []corev1.Taint, key, value, effect string) bool {
	for _, t := range taints {
		if t.Key != key {
			continue
		}
		if value != "" && t.Value != value {
			continue
		}
		if effect != "" && string(t.Effect) != effect {
			continue
		}
		return true
	}
	return false
}

// matchLabels checks if the given labels match the selector map
func matchLabels(labels, selector map[string]string) bool {
	for key, value := range selector {
		if labels[key] != value {
			return false
		}
	}
	return true
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

// FormatResult formats the result according to the specified format
func FormatResult(result *Result, format string, showAvailable bool) (string, error) {
	switch format {
	case "yaml":
		data, err := yaml.Marshal(result)
		if err != nil {
			return "", fmt.Errorf("failed to format as YAML: %w", err)
		}
		return string(data), nil
	case "json":
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to format as JSON: %w", err)
		}
		return string(data), nil
	default:
		return FormatAsTable(*result, showAvailable), nil
	}
}
