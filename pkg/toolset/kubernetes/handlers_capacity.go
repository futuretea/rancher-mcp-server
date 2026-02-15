package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/futuretea/rancher-mcp-server/pkg/client/steve"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset/handler"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// CapacityNodeInfo holds resource information for a node
type CapacityNodeInfo struct {
	Name     string            `json:"name"`
	CPU      CapacityResource  `json:"cpu"`
	Memory   CapacityResource  `json:"memory"`
	PodCount PodCountInfo      `json:"podCount"`
	Taints   []corev1.Taint    `json:"taints,omitempty"`
	Labels   map[string]string `json:"labels,omitempty"`
	Pods     []CapacityPodInfo `json:"pods,omitempty"`
}

// CapacityResource holds resource metrics for a node
type CapacityResource struct {
	Capacity    int64 `json:"capacity"`
	Allocatable int64 `json:"allocatable"`
	Requested   int64 `json:"requested"`
	Limited     int64 `json:"limited"`
	Utilized    int64 `json:"utilized,omitempty"`
}

// PodCountInfo holds pod count information
type PodCountInfo struct {
	Capacity    int64 `json:"capacity"`
	Allocatable int64 `json:"allocatable"`
	Requested   int64 `json:"requested"`
}

// CapacityContainerInfo holds resource information for a container
type CapacityContainerInfo struct {
	Name   string           `json:"name"`
	CPU    CapacityResource `json:"cpu"`
	Memory CapacityResource `json:"memory"`
	Init   bool             `json:"init,omitempty"`
}

// CapacityPodInfo holds resource information for a pod
type CapacityPodInfo struct {
	Namespace     string                  `json:"namespace"`
	Name          string                  `json:"name"`
	CPU           CapacityResource        `json:"cpu"`
	Memory        CapacityResource        `json:"memory"`
	ContainerCnt  int                     `json:"containerCount"`
	Containers    []CapacityContainerInfo `json:"containers,omitempty"`
}

// CapacityResult holds the complete capacity analysis
type CapacityResult struct {
	Nodes          []CapacityNodeInfo `json:"nodes"`
	Cluster        CapacityNodeInfo   `json:"cluster"`
	ShowPods       bool               `json:"showPods"`
	ShowContainers bool               `json:"showContainers"`
	ShowUtil       bool               `json:"showUtil"`
	ShowAvailable  bool               `json:"showAvailable"`
	ShowPodCount   bool               `json:"showPodCount"`
	ShowLabels     bool               `json:"showLabels"`
	HideRequests   bool               `json:"hideRequests"`
	HideLimits     bool               `json:"hideLimits"`
}

// capacityParams holds all parameters for capacityHandler
type capacityParams struct {
	cluster                string
	namespace              string
	labelSelector          string
	nodeLabelSelector      string
	namespaceLabelSelector string
	nodeTaints             string
	sortBy                 string
	format                 string
	showPods               bool
	showContainers         bool
	showUtil               bool
	showAvailable          bool
	showPodCount           bool
	showLabels             bool
	hideRequests           bool
	hideLimits             bool
	noTaint                bool
}

// capacityHandler handles the kubernetes_capacity tool
func capacityHandler(client interface{}, params map[string]interface{}) (string, error) {
	steveClient, err := toolset.ValidateSteveClient(client)
	if err != nil {
		return "", err
	}

	p, err := extractCapacityParams(params)
	if err != nil {
		return "", err
	}

	// containers implies pods
	if p.showContainers {
		p.showPods = true
	}

	ctx := context.Background()

	// Build node info map
	nodeInfoMap, err := buildNodeInfoMap(ctx, steveClient, p)
	if err != nil {
		return "", err
	}

	// Process pods
	if err := processPods(ctx, steveClient, nodeInfoMap, p); err != nil {
		return "", err
	}

	// Get utilization metrics if requested
	if p.showUtil {
		getNodeMetrics(ctx, steveClient, p.cluster, nodeInfoMap)
	}

	// Build and format result
	result := buildResult(nodeInfoMap, p)
	return formatResult(result, p)
}

// extractCapacityParams extracts parameters from the input map
func extractCapacityParams(params map[string]interface{}) (capacityParams, error) {
	cluster, err := handler.ExtractRequiredString(params, handler.ParamCluster)
	if err != nil {
		return capacityParams{}, err
	}

	return capacityParams{
		cluster:                cluster,
		showPods:               handler.ExtractBool(params, "pods", false),
		showContainers:         handler.ExtractBool(params, "containers", false),
		showUtil:               handler.ExtractBool(params, "util", false),
		showAvailable:          handler.ExtractBool(params, "available", false),
		showPodCount:           handler.ExtractBool(params, "podCount", false),
		showLabels:             handler.ExtractBool(params, "showLabels", false),
		hideRequests:           handler.ExtractBool(params, "hideRequests", false),
		hideLimits:             handler.ExtractBool(params, "hideLimits", false),
		noTaint:                handler.ExtractBool(params, "noTaint", false),
		namespace:              handler.ExtractOptionalString(params, handler.ParamNamespace),
		labelSelector:          handler.ExtractOptionalString(params, handler.ParamLabelSelector),
		nodeLabelSelector:      handler.ExtractOptionalString(params, "nodeLabelSelector"),
		namespaceLabelSelector: handler.ExtractOptionalString(params, "namespaceLabelSelector"),
		nodeTaints:             handler.ExtractOptionalString(params, "nodeTaints"),
		sortBy:                 handler.ExtractOptionalString(params, "sortBy"),
		format:                 handler.ExtractOptionalStringWithDefault(params, handler.ParamFormat, handler.FormatTable),
	}, nil
}

// buildNodeInfoMap builds a map of node name to CapacityNodeInfo
func buildNodeInfoMap(ctx context.Context, steveClient *steve.Client, p capacityParams) (map[string]*CapacityNodeInfo, error) {
	nodes, err := steveClient.ListResources(ctx, p.cluster, "node", "", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	nodeSelectorMap := parseLabelSelector(p.nodeLabelSelector)
	nodeInfoMap := make(map[string]*CapacityNodeInfo)

	for _, node := range nodes.Items {
		if !matchesNodeSelector(node, nodeSelectorMap) {
			continue
		}

		info := extractNodeInfo(node)

		if p.noTaint && len(info.Taints) > 0 {
			continue
		}

		if p.nodeTaints != "" && !matchTaints(info.Taints, p.nodeTaints) {
			continue
		}

		nodeInfoMap[info.Name] = info
	}

	return nodeInfoMap, nil
}

// matchesNodeSelector checks if a node matches the label selector
func matchesNodeSelector(node unstructured.Unstructured, selector map[string]string) bool {
	if len(selector) == 0 {
		return true
	}
	return matchLabels(node.GetLabels(), selector)
}

// extractNodeInfo extracts CapacityNodeInfo from an unstructured node object
func extractNodeInfo(node unstructured.Unstructured) *CapacityNodeInfo {
	info := &CapacityNodeInfo{
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

// processPods processes all pods and aggregates resources by node
func processPods(ctx context.Context, steveClient *steve.Client, nodeInfoMap map[string]*CapacityNodeInfo, p capacityParams) error {
	namespaceFilter, err := buildNamespaceFilter(ctx, steveClient, p)
	if err != nil {
		return err
	}

	podOpts := &steve.ListOptions{}
	if p.labelSelector != "" {
		podOpts.LabelSelector = p.labelSelector
	}

	pods, err := steveClient.ListResources(ctx, p.cluster, "pod", p.namespace, podOpts)
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	for _, pod := range pods.Items {
		if !shouldProcessPod(pod, nodeInfoMap, namespaceFilter) {
			continue
		}

		nodeName, _, _ := unstructured.NestedString(pod.Object, "spec", "nodeName")
		processSinglePod(pod, nodeInfoMap[nodeName], p.showPods, p.showContainers)
	}

	return nil
}

// buildNamespaceFilter builds a filter map for namespace label selection
func buildNamespaceFilter(ctx context.Context, steveClient *steve.Client, p capacityParams) (map[string]bool, error) {
	if p.namespaceLabelSelector == "" {
		return nil, nil
	}

	nsSelector := parseLabelSelector(p.namespaceLabelSelector)
	if len(nsSelector) == 0 {
		return nil, nil
	}

	nsList, err := steveClient.ListResources(ctx, p.cluster, "namespace", "", nil)
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

// shouldProcessPod checks if a pod should be processed
func shouldProcessPod(pod unstructured.Unstructured, nodeInfoMap map[string]*CapacityNodeInfo, namespaceFilter map[string]bool) bool {
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
func processSinglePod(pod unstructured.Unstructured, nodeInfo *CapacityNodeInfo, showPods, showContainers bool) {
	nodeInfo.PodCount.Requested++

	podInfo := CapacityPodInfo{
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
func processContainers(pod unstructured.Unstructured, podInfo *CapacityPodInfo, showContainers bool) {
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
func processInitContainers(pod unstructured.Unstructured, podInfo *CapacityPodInfo, showContainers bool) {
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
func processContainerResources(container map[string]interface{}, podInfo *CapacityPodInfo, showContainers, isInit bool) {
	name, _ := container["name"].(string)

	containerInfo := CapacityContainerInfo{
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
func extractResourceRequests(resources map[string]interface{}, containerInfo *CapacityContainerInfo, podInfo *CapacityPodInfo) {
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
func extractResourceLimits(resources map[string]interface{}, containerInfo *CapacityContainerInfo, podInfo *CapacityPodInfo) {
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

// getNodeMetrics retrieves node metrics from metrics-server
func getNodeMetrics(ctx context.Context, steveClient *steve.Client, cluster string, nodeInfoMap map[string]*CapacityNodeInfo) {
	metrics, err := steveClient.ListResources(ctx, cluster, "node.metrics.k8s.io", "", nil)
	if err != nil {
		// metrics-server not available, silently skip
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

// buildResult builds the CapacityResult from node info map
func buildResult(nodeInfoMap map[string]*CapacityNodeInfo, p capacityParams) CapacityResult {
	result := CapacityResult{
		Nodes:          make([]CapacityNodeInfo, 0, len(nodeInfoMap)),
		ShowPods:       p.showPods,
		ShowContainers: p.showContainers,
		ShowUtil:       p.showUtil,
		ShowAvailable:  p.showAvailable,
		ShowPodCount:   p.showPodCount,
		ShowLabels:     p.showLabels,
		HideRequests:   p.hideRequests,
		HideLimits:     p.hideLimits,
	}

	clusterInfo := CapacityNodeInfo{Name: "*"}

	for _, info := range nodeInfoMap {
		result.Nodes = append(result.Nodes, *info)
		aggregateNodeToCluster(&clusterInfo, info)
	}

	// Sort nodes if requested
	if p.sortBy != "" {
		sortNodes(result.Nodes, p.sortBy)
	}

	result.Cluster = clusterInfo
	return result
}

// aggregateNodeToCluster aggregates node resources to cluster totals
func aggregateNodeToCluster(cluster, node *CapacityNodeInfo) {
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

// formatResult formats the result according to the specified format
func formatResult(result CapacityResult, p capacityParams) (string, error) {
	switch p.format {
	case handler.FormatYAML:
		data, err := yaml.Marshal(result)
		if err != nil {
			return "", fmt.Errorf("failed to format as YAML: %w", err)
		}
		return string(data), nil
	case handler.FormatJSON:
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to format as JSON: %w", err)
		}
		return string(data), nil
	default:
		return formatCapacityAsTable(result, p.showAvailable), nil
	}
}

// sortNodes sorts nodes by the specified field
func sortNodes(nodes []CapacityNodeInfo, sortBy string) {
	sort.Slice(nodes, func(i, j int) bool {
		a, b := nodes[i], nodes[j]

		switch sortBy {
		case "cpu.util":
			return a.CPU.Utilized > b.CPU.Utilized
		case "mem.util", "memory.util":
			return a.Memory.Utilized > b.Memory.Utilized
		case "cpu.request":
			return a.CPU.Requested > b.CPU.Requested
		case "mem.request", "memory.request":
			return a.Memory.Requested > b.Memory.Requested
		case "cpu.limit":
			return a.CPU.Limited > b.CPU.Limited
		case "mem.limit", "memory.limit":
			return a.Memory.Limited > b.Memory.Limited
		case "cpu.util.percentage":
			return calcPercentage(a.CPU.Utilized, a.CPU.Allocatable) > calcPercentage(b.CPU.Utilized, b.CPU.Allocatable)
		case "mem.util.percentage", "memory.util.percentage":
			return calcPercentage(a.Memory.Utilized, a.Memory.Allocatable) > calcPercentage(b.Memory.Utilized, b.Memory.Allocatable)
		case "cpu.request.percentage":
			return calcPercentage(a.CPU.Requested, a.CPU.Allocatable) > calcPercentage(b.CPU.Requested, b.CPU.Allocatable)
		case "mem.request.percentage", "memory.request.percentage":
			return calcPercentage(a.Memory.Requested, a.Memory.Allocatable) > calcPercentage(b.Memory.Requested, b.Memory.Allocatable)
		case "cpu.limit.percentage":
			return calcPercentage(a.CPU.Limited, a.CPU.Allocatable) > calcPercentage(b.CPU.Limited, b.CPU.Allocatable)
		case "mem.limit.percentage", "memory.limit.percentage":
			return calcPercentage(a.Memory.Limited, a.Memory.Allocatable) > calcPercentage(b.Memory.Limited, b.Memory.Allocatable)
		case "pod.count":
			return a.PodCount.Requested > b.PodCount.Requested
		case "name":
			return a.Name < b.Name
		default:
			return a.Name < b.Name
		}
	})
}

// formatCapacityAsTable formats capacity result as a human-readable table
func formatCapacityAsTable(result CapacityResult, showAvailable bool) string {
	var b strings.Builder

	writeNodeSummary(&b, result, showAvailable)

	if result.ShowUtil {
		writeUtilizationSection(&b, result.Nodes)
	}

	if result.ShowPods {
		writePodsSection(&b, result, showAvailable)
	}

	return b.String()
}

// writeNodeSummary writes the node and cluster summary table
func writeNodeSummary(b *strings.Builder, result CapacityResult, showAvailable bool) {
	tb := newTableBuilder("%-25s", "NAME")

	if !result.HideRequests {
		tb.addColumn("%-12s", "CPU REQUEST", "MEM REQUEST")
	}
	if !result.HideLimits {
		tb.addColumn("%-12s", "CPU LIMIT", "MEM LIMIT")
	}
	if result.ShowPodCount {
		tb.addColumn("%-6s", "PODS")
	}
	if result.ShowLabels {
		tb.addColumn("%-s", "LABELS")
	}

	fmt.Fprintf(b, "NODE\n")
	tb.writeHeader(b)
	tb.writeSeparator(b)

	for _, node := range result.Nodes {
		row := []interface{}{truncate(node.Name, 25)}
		if !result.HideRequests {
			row = append(row, formatCPU(node.CPU.Requested, showAvailable), formatMemory(node.Memory.Requested, showAvailable))
		}
		if !result.HideLimits {
			row = append(row, formatCPU(node.CPU.Limited, showAvailable), formatMemory(node.Memory.Limited, showAvailable))
		}
		if result.ShowPodCount {
			row = append(row, fmt.Sprintf("%d/%d", node.PodCount.Requested, node.PodCount.Allocatable))
		}
		if result.ShowLabels {
			row = append(row, formatLabels(node.Labels))
		}
		tb.writeRow(b, row)
	}

	fmt.Fprintf(b, "\nCLUSTER\n")
	tb.writeHeader(b)
	tb.writeSeparator(b)

	row := []interface{}{result.Cluster.Name}
	if !result.HideRequests {
		row = append(row, formatCPU(result.Cluster.CPU.Requested, showAvailable), formatMemory(result.Cluster.Memory.Requested, showAvailable))
	}
	if !result.HideLimits {
		row = append(row, formatCPU(result.Cluster.CPU.Limited, showAvailable), formatMemory(result.Cluster.Memory.Limited, showAvailable))
	}
	if result.ShowPodCount {
		row = append(row, fmt.Sprintf("%d/%d", result.Cluster.PodCount.Requested, result.Cluster.PodCount.Allocatable))
	}
	if result.ShowLabels {
		row = append(row, "")
	}
	tb.writeRow(b, row)
}

// writeUtilizationSection writes the utilization section
func writeUtilizationSection(b *strings.Builder, nodes []CapacityNodeInfo) {
	fmt.Fprintf(b, "\nNODE UTILIZATION\n")
	fmt.Fprintf(b, "%-25s %-12s %-12s %-12s %-12s\n", "NAME", "CPU CAP", "CPU UTIL%", "MEM CAP", "MEM UTIL%")
	fmt.Fprintf(b, "%-25s %-12s %-12s %-12s %-12s\n", "----", "-------", "---------", "-------", "---------")

	for _, node := range nodes {
		fmt.Fprintf(b, "%-25s %-12s %-11.1f%% %-12s %-11.1f%%\n",
			truncate(node.Name, 25),
			formatCPU(node.CPU.Allocatable, true),
			calcPercentage(node.CPU.Utilized, node.CPU.Allocatable),
			formatMemory(node.Memory.Allocatable, true),
			calcPercentage(node.Memory.Utilized, node.Memory.Allocatable),
		)
	}
}

// writePodsSection writes the pods section with optional container details
func writePodsSection(b *strings.Builder, result CapacityResult, showAvailable bool) {
	fmt.Fprintf(b, "\nPODS\n")

	for _, node := range result.Nodes {
		if len(node.Pods) == 0 {
			continue
		}

		fmt.Fprintf(b, "\n%s (%d pods)\n", node.Name, len(node.Pods))

		tb := newTableBuilder("  %-40s", "POD")
		tb.addColumn("%-10s", "NAMESPACE")
		if !result.HideRequests {
			tb.addColumn("%-12s", "CPU REQUEST", "MEM REQUEST")
		}
		if !result.HideLimits {
			tb.addColumn("%-12s", "CPU LIMIT", "MEM LIMIT")
		}

		tb.writeHeader(b)
		tb.writeSeparator(b)

		for _, pod := range node.Pods {
			row := []interface{}{truncate(pod.Name, 40), truncate(pod.Namespace, 10)}
			if !result.HideRequests {
				row = append(row, formatCPU(pod.CPU.Requested, showAvailable), formatMemory(pod.Memory.Requested, showAvailable))
			}
			if !result.HideLimits {
				row = append(row, formatCPU(pod.CPU.Limited, showAvailable), formatMemory(pod.Memory.Limited, showAvailable))
			}
			tb.writeRow(b, row)

			if result.ShowContainers {
				writeContainers(b, pod.Containers, result, showAvailable)
			}
		}
	}
}

// writeContainers writes container details for a pod
func writeContainers(b *strings.Builder, containers []CapacityContainerInfo, result CapacityResult, showAvailable bool) {
	if len(containers) == 0 {
		return
	}

	tb := newTableBuilder("  %-38s", "[C]")
	if !result.HideRequests {
		tb.addColumn("%-12s", "CPU REQUEST", "MEM REQUEST")
	}
	if !result.HideLimits {
		tb.addColumn("%-12s", "CPU LIMIT", "MEM LIMIT")
	}

	for _, c := range containers {
		prefix := "[C]"
		if c.Init {
			prefix = "[I]"
		}
		row := []interface{}{prefix + " " + truncate(c.Name, 35)}
		if !result.HideRequests {
			row = append(row, formatCPU(c.CPU.Requested, showAvailable), formatMemory(c.Memory.Requested, showAvailable))
		}
		if !result.HideLimits {
			row = append(row, formatCPU(c.CPU.Limited, showAvailable), formatMemory(c.Memory.Limited, showAvailable))
		}
		tb.writeRow(b, row)
	}
}

// tableBuilder helps build formatted tables
type tableBuilder struct {
	formats []string
	headers []string
}

func newTableBuilder(format, header string) *tableBuilder {
	return &tableBuilder{
		formats: []string{format},
		headers: []string{header},
	}
}

func (tb *tableBuilder) addColumn(format string, headers ...string) {
	for range headers {
		tb.formats = append(tb.formats, format)
	}
	tb.headers = append(tb.headers, headers...)
}

func (tb *tableBuilder) writeHeader(b *strings.Builder) {
	fmt.Fprintf(b, strings.Join(tb.formats, " ")+"\n", toAnySlice(tb.headers)...)
}

func (tb *tableBuilder) writeSeparator(b *strings.Builder) {
	separators := make([]string, len(tb.headers))
	for i, h := range tb.headers {
		separators[i] = strings.Repeat("-", len(h))
	}
	fmt.Fprintf(b, strings.Join(tb.formats, " ")+"\n", toAnySlice(separators)...)
}

func (tb *tableBuilder) writeRow(b *strings.Builder, values []interface{}) {
	fmt.Fprintf(b, strings.Join(tb.formats, " ")+"\n", values...)
}

// toAnySlice converts a string slice to any slice
func toAnySlice(ss []string) []any {
	result := make([]any, len(ss))
	for i, s := range ss {
		result[i] = s
	}
	return result
}

// formatLabels formats node labels as a string
func formatLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}

	parts := make([]string, 0, len(labels))
	for k, v := range labels {
		if v == "" {
			parts = append(parts, k)
		} else {
			parts = append(parts, k+"="+v)
		}
	}
	return truncate(strings.Join(parts, ","), 60)
}

// formatCPU formats CPU value (millicores) to string
func formatCPU(val int64, showRaw bool) string {
	cores := float64(val) / 1000
	if showRaw && val < 1000 {
		return fmt.Sprintf("%dm", val)
	}
	return fmt.Sprintf("%.2fc", cores)
}

// formatMemory formats memory value (bytes) to string
func formatMemory(val int64, showRaw bool) string {
	if showRaw {
		switch {
		case val >= 1024*1024*1024:
			return fmt.Sprintf("%dGi", val/(1024*1024*1024))
		case val >= 1024*1024:
			return fmt.Sprintf("%dMi", val/(1024*1024))
		case val >= 1024:
			return fmt.Sprintf("%dKi", val/1024)
		default:
			return fmt.Sprintf("%d", val)
		}
	}
	return fmt.Sprintf("%.2fGi", float64(val)/(1024*1024*1024))
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
