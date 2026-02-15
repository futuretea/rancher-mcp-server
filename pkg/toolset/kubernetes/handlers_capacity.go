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
}

// CapacityPodInfo holds resource information for a pod
type CapacityPodInfo struct {
	Namespace    string                  `json:"namespace"`
	Name         string                  `json:"name"`
	CPU          CapacityResource        `json:"cpu"`
	Memory       CapacityResource        `json:"memory"`
	ContainerCnt int                     `json:"containerCount"`
	Containers   []CapacityContainerInfo `json:"containers,omitempty"`
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

// capacityHandler handles the kubernetes_capacity tool
func capacityHandler(client interface{}, params map[string]interface{}) (string, error) {
	steveClient, err := toolset.ValidateSteveClient(client)
	if err != nil {
		return "", err
	}

	cluster, err := handler.ExtractRequiredString(params, handler.ParamCluster)
	if err != nil {
		return "", err
	}
	showPods := handler.ExtractBool(params, "pods", false)
	showContainers := handler.ExtractBool(params, "containers", false)
	showUtil := handler.ExtractBool(params, "util", false)
	showAvailable := handler.ExtractBool(params, "available", false)
	showPodCount := handler.ExtractBool(params, "podCount", false)
	showLabels := handler.ExtractBool(params, "showLabels", false)
	hideRequests := handler.ExtractBool(params, "hideRequests", false)
	hideLimits := handler.ExtractBool(params, "hideLimits", false)
	namespace := handler.ExtractOptionalString(params, handler.ParamNamespace)
	labelSelector := handler.ExtractOptionalString(params, handler.ParamLabelSelector)
	nodeLabelSelector := handler.ExtractOptionalString(params, "nodeLabelSelector")
	namespaceLabelSelector := handler.ExtractOptionalString(params, "namespaceLabelSelector")
	nodeTaints := handler.ExtractOptionalString(params, "nodeTaints")
	noTaint := handler.ExtractBool(params, "noTaint", false)
	sortBy := handler.ExtractOptionalString(params, "sortBy")
	format := handler.ExtractOptionalStringWithDefault(params, handler.ParamFormat, handler.FormatTable)

	// containers implies pods
	if showContainers {
		showPods = true
	}

	ctx := context.Background()

	// Get all nodes
	nodes, err := steveClient.ListResources(ctx, cluster, "node", "", nil)
	if err != nil {
		return "", fmt.Errorf("failed to list nodes: %w", err)
	}

	// Parse node label selector if provided
	nodeSelectorMap := parseLabelSelector(nodeLabelSelector)

	// Build node info map
	nodeInfoMap := make(map[string]*CapacityNodeInfo)
	for _, node := range nodes.Items {
		// Filter by node label selector if provided
		if len(nodeSelectorMap) > 0 {
			labels := node.GetLabels()
			if !matchLabels(labels, nodeSelectorMap) {
				continue
			}
		}

		info := &CapacityNodeInfo{
			Name:   node.GetName(),
			CPU:    CapacityResource{},
			Memory: CapacityResource{},
			Labels: node.GetLabels(),
		}

		// Extract capacity
		if capacity, found, _ := unstructured.NestedMap(node.Object, "status", "capacity"); found {
			if cpu, ok := capacity["cpu"].(string); ok {
				info.CPU.Capacity = parseResourceQuantity(cpu)
			}
			if mem, ok := capacity["memory"].(string); ok {
				info.Memory.Capacity = parseResourceQuantity(mem)
			}
			if pods, ok := capacity["pods"].(string); ok {
				info.PodCount.Capacity, _ = strconv.ParseInt(pods, 10, 64)
			}
		}

		// Extract allocatable
		if allocatable, found, _ := unstructured.NestedMap(node.Object, "status", "allocatable"); found {
			if cpu, ok := allocatable["cpu"].(string); ok {
				info.CPU.Allocatable = parseResourceQuantity(cpu)
			}
			if mem, ok := allocatable["memory"].(string); ok {
				info.Memory.Allocatable = parseResourceQuantity(mem)
			}
			if pods, ok := allocatable["pods"].(string); ok {
				info.PodCount.Allocatable, _ = strconv.ParseInt(pods, 10, 64)
			}
		}

		// Extract taints
		if taints, found, _ := unstructured.NestedSlice(node.Object, "spec", "taints"); found {
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
				info.Taints = append(info.Taints, taint)
			}
		}

		// Filter by noTaint - skip nodes with any taints
		if noTaint && len(info.Taints) > 0 {
			continue
		}

		// Filter by taint selector if provided
		if nodeTaints != "" && !matchTaints(info.Taints, nodeTaints) {
			continue
		}

		nodeInfoMap[info.Name] = info
	}

	// Build namespace filter map if namespaceLabelSelector is provided
	namespaceFilter := make(map[string]bool)
	if namespaceLabelSelector != "" {
		nsSelector := parseLabelSelector(namespaceLabelSelector)
		if len(nsSelector) > 0 {
			nsList, err := steveClient.ListResources(ctx, cluster, "namespace", "", nil)
			if err == nil {
				for _, ns := range nsList.Items {
					if matchLabels(ns.GetLabels(), nsSelector) {
						namespaceFilter[ns.GetName()] = true
					}
				}
			}
		}
	}

	// Get pods
	podOpts := &steve.ListOptions{}
	if labelSelector != "" {
		podOpts.LabelSelector = labelSelector
	}
	pods, err := steveClient.ListResources(ctx, cluster, "pod", namespace, podOpts)
	if err != nil {
		return "", fmt.Errorf("failed to list pods: %w", err)
	}

	// Process pods and aggregate by node
	for _, pod := range pods.Items {
		// Filter by namespace labels if provided
		if len(namespaceFilter) > 0 {
			if !namespaceFilter[pod.GetNamespace()] {
				continue
			}
		}

		nodeName := ""
		if n, found, _ := unstructured.NestedString(pod.Object, "spec", "nodeName"); found {
			nodeName = n
		}

		// Skip pods not assigned to a node
		if nodeName == "" {
			continue
		}

		nodeInfo, ok := nodeInfoMap[nodeName]
		if !ok {
			continue
		}

		// Count this pod
		nodeInfo.PodCount.Requested++

		// Extract container resources
		containers, found, _ := unstructured.NestedSlice(pod.Object, "spec", "containers")
		if !found {
			continue
		}

		podInfo := CapacityPodInfo{
			Namespace:    pod.GetNamespace(),
			Name:         pod.GetName(),
			ContainerCnt: len(containers),
		}

		for _, c := range containers {
			container, ok := c.(map[string]interface{})
			if !ok {
				continue
			}

			containerName := ""
			if name, ok := container["name"].(string); ok {
				containerName = name
			}

			containerInfo := CapacityContainerInfo{
				Name: containerName,
			}

			resources, found, _ := unstructured.NestedMap(container, "resources")
			if !found {
				continue
			}

			// Parse requests
			if requests, found, _ := unstructured.NestedMap(resources, "requests"); found {
				if cpu, ok := requests["cpu"].(string); ok {
					containerInfo.CPU.Requested = parseResourceQuantity(cpu)
					podInfo.CPU.Requested += containerInfo.CPU.Requested
				}
				if memory, ok := requests["memory"].(string); ok {
					containerInfo.Memory.Requested = parseResourceQuantity(memory)
					podInfo.Memory.Requested += containerInfo.Memory.Requested
				}
			}

			// Parse limits
			if limits, found, _ := unstructured.NestedMap(resources, "limits"); found {
				if cpu, ok := limits["cpu"].(string); ok {
					containerInfo.CPU.Limited = parseResourceQuantity(cpu)
					podInfo.CPU.Limited += containerInfo.CPU.Limited
				}
				if memory, ok := limits["memory"].(string); ok {
					containerInfo.Memory.Limited = parseResourceQuantity(memory)
					podInfo.Memory.Limited += containerInfo.Memory.Limited
				}
			}

			if showContainers {
				podInfo.Containers = append(podInfo.Containers, containerInfo)
			}
		}

		// Aggregate to node
		nodeInfo.CPU.Requested += podInfo.CPU.Requested
		nodeInfo.Memory.Requested += podInfo.Memory.Requested
		nodeInfo.CPU.Limited += podInfo.CPU.Limited
		nodeInfo.Memory.Limited += podInfo.Memory.Limited

		if showPods {
			nodeInfo.Pods = append(nodeInfo.Pods, podInfo)
		}
	}

	// Get utilization metrics if requested
	if showUtil {
		getNodeMetrics(ctx, steveClient, cluster, nodeInfoMap)
	}

	// Build result
	result := CapacityResult{
		Nodes:          make([]CapacityNodeInfo, 0, len(nodeInfoMap)),
		ShowPods:       showPods,
		ShowContainers: showContainers,
		ShowUtil:       showUtil,
		ShowAvailable:  showAvailable,
		ShowPodCount:   showPodCount,
		ShowLabels:     showLabels,
		HideRequests:   hideRequests,
		HideLimits:     hideLimits,
	}

	// Calculate cluster totals
	clusterInfo := CapacityNodeInfo{
		Name:     "*",
		CPU:      CapacityResource{},
		Memory:   CapacityResource{},
		PodCount: PodCountInfo{},
	}

	for _, info := range nodeInfoMap {
		result.Nodes = append(result.Nodes, *info)
		clusterInfo.CPU.Capacity += info.CPU.Capacity
		clusterInfo.CPU.Allocatable += info.CPU.Allocatable
		clusterInfo.CPU.Requested += info.CPU.Requested
		clusterInfo.CPU.Limited += info.CPU.Limited
		clusterInfo.CPU.Utilized += info.CPU.Utilized
		clusterInfo.Memory.Capacity += info.Memory.Capacity
		clusterInfo.Memory.Allocatable += info.Memory.Allocatable
		clusterInfo.Memory.Requested += info.Memory.Requested
		clusterInfo.Memory.Limited += info.Memory.Limited
		clusterInfo.Memory.Utilized += info.Memory.Utilized
		clusterInfo.PodCount.Capacity += info.PodCount.Capacity
		clusterInfo.PodCount.Allocatable += info.PodCount.Allocatable
		clusterInfo.PodCount.Requested += info.PodCount.Requested
	}

	// Sort nodes if requested
	if sortBy != "" {
		sortCapacityNodes(result.Nodes, sortBy)
	}

	result.Cluster = clusterInfo

	// Format output
	switch format {
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
	default: // table
		return formatCapacityAsTable(result, showAvailable), nil
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
				nodeInfo.CPU.Utilized = parseResourceQuantity(cpu)
			}
			if memory, ok := usage["memory"].(string); ok {
				nodeInfo.Memory.Utilized = parseResourceQuantity(memory)
			}
		}
	}
}

// sortCapacityNodes sorts nodes by the specified field
func sortCapacityNodes(nodes []CapacityNodeInfo, sortBy string) {
	less := sortLessFunc(sortBy)
	if less != nil {
		sort.Slice(nodes, less(nodes))
	}
}

// sortLessFunc returns a comparison function for the given sort field
func sortLessFunc(sortBy string) func([]CapacityNodeInfo) func(i, j int) bool {
	switch sortBy {
	case "cpu.util":
		return func(nodes []CapacityNodeInfo) func(i, j int) bool {
			return func(i, j int) bool { return nodes[i].CPU.Utilized > nodes[j].CPU.Utilized }
		}
	case "mem.util", "memory.util":
		return func(nodes []CapacityNodeInfo) func(i, j int) bool {
			return func(i, j int) bool { return nodes[i].Memory.Utilized > nodes[j].Memory.Utilized }
		}
	case "cpu.request":
		return func(nodes []CapacityNodeInfo) func(i, j int) bool {
			return func(i, j int) bool { return nodes[i].CPU.Requested > nodes[j].CPU.Requested }
		}
	case "mem.request", "memory.request":
		return func(nodes []CapacityNodeInfo) func(i, j int) bool {
			return func(i, j int) bool { return nodes[i].Memory.Requested > nodes[j].Memory.Requested }
		}
	case "cpu.limit":
		return func(nodes []CapacityNodeInfo) func(i, j int) bool {
			return func(i, j int) bool { return nodes[i].CPU.Limited > nodes[j].CPU.Limited }
		}
	case "mem.limit", "memory.limit":
		return func(nodes []CapacityNodeInfo) func(i, j int) bool {
			return func(i, j int) bool { return nodes[i].Memory.Limited > nodes[j].Memory.Limited }
		}
	case "cpu.util.percentage":
		return func(nodes []CapacityNodeInfo) func(i, j int) bool {
			return func(i, j int) bool {
				return calcPercentage(nodes[i].CPU.Utilized, nodes[i].CPU.Allocatable) >
					calcPercentage(nodes[j].CPU.Utilized, nodes[j].CPU.Allocatable)
			}
		}
	case "mem.util.percentage", "memory.util.percentage":
		return func(nodes []CapacityNodeInfo) func(i, j int) bool {
			return func(i, j int) bool {
				return calcPercentage(nodes[i].Memory.Utilized, nodes[i].Memory.Allocatable) >
					calcPercentage(nodes[j].Memory.Utilized, nodes[j].Memory.Allocatable)
			}
		}
	case "cpu.request.percentage":
		return func(nodes []CapacityNodeInfo) func(i, j int) bool {
			return func(i, j int) bool {
				return calcPercentage(nodes[i].CPU.Requested, nodes[i].CPU.Allocatable) >
					calcPercentage(nodes[j].CPU.Requested, nodes[j].CPU.Allocatable)
			}
		}
	case "mem.request.percentage", "memory.request.percentage":
		return func(nodes []CapacityNodeInfo) func(i, j int) bool {
			return func(i, j int) bool {
				return calcPercentage(nodes[i].Memory.Requested, nodes[i].Memory.Allocatable) >
					calcPercentage(nodes[j].Memory.Requested, nodes[j].Memory.Allocatable)
			}
		}
	case "cpu.limit.percentage":
		return func(nodes []CapacityNodeInfo) func(i, j int) bool {
			return func(i, j int) bool {
				return calcPercentage(nodes[i].CPU.Limited, nodes[i].CPU.Allocatable) >
					calcPercentage(nodes[j].CPU.Limited, nodes[j].CPU.Allocatable)
			}
		}
	case "mem.limit.percentage", "memory.limit.percentage":
		return func(nodes []CapacityNodeInfo) func(i, j int) bool {
			return func(i, j int) bool {
				return calcPercentage(nodes[i].Memory.Limited, nodes[i].Memory.Allocatable) >
					calcPercentage(nodes[j].Memory.Limited, nodes[j].Memory.Allocatable)
			}
		}
	case "name":
		return func(nodes []CapacityNodeInfo) func(i, j int) bool {
			return func(i, j int) bool { return nodes[i].Name < nodes[j].Name }
		}
	}
	return nil
}

// formatCapacityAsTable formats capacity result as a human-readable table
func formatCapacityAsTable(result CapacityResult, showAvailable bool) string {
	var b strings.Builder

	// Print node and cluster summary
	writeNodeSummary(&b, result, showAvailable)

	// Print utilization section if requested
	if result.ShowUtil {
		writeUtilizationSection(&b, result.Nodes)
	}

	// Print pods section if requested
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

	// Print nodes section
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

	// Print cluster totals
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

// writeUtilizationSection writes the utilization section
func writeUtilizationSection(b *strings.Builder, nodes []CapacityNodeInfo) {
	fmt.Fprintf(b, "\nNODE UTILIZATION\n")
	fmt.Fprintf(b, "%-25s %-12s %-12s %-12s %-12s\n", "NAME", "CPU CAP", "CPU UTIL%", "MEM CAP", "MEM UTIL%")
	fmt.Fprintf(b, "%-25s %-12s %-12s %-12s %-12s\n", "----", "-------", "---------", "-------", "---------")

	for _, node := range nodes {
		cpuUtilPct := calcPercentage(node.CPU.Utilized, node.CPU.Allocatable)
		memUtilPct := calcPercentage(node.Memory.Utilized, node.Memory.Allocatable)
		fmt.Fprintf(b, "%-25s %-12s %-11.1f%% %-12s %-11.1f%%\n",
			truncate(node.Name, 25),
			formatCPU(node.CPU.Allocatable, true),
			cpuUtilPct,
			formatMemory(node.Memory.Allocatable, true),
			memUtilPct,
		)
	}
}

// calcPercentage calculates percentage with zero check
func calcPercentage(value, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return float64(value) / float64(total) * 100
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
		row := []interface{}{truncate(c.Name, 38)}
		if !result.HideRequests {
			row = append(row, formatCPU(c.CPU.Requested, showAvailable), formatMemory(c.Memory.Requested, showAvailable))
		}
		if !result.HideLimits {
			row = append(row, formatCPU(c.CPU.Limited, showAvailable), formatMemory(c.Memory.Limited, showAvailable))
		}
		tb.writeRow(b, row)
	}
}

// toAnySlice converts a string slice to any slice for fmt.Fprintf
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
	var parts []string
	for k, v := range labels {
		if v == "" {
			parts = append(parts, k)
		} else {
			parts = append(parts, fmt.Sprintf("%s=%s", k, v))
		}
	}
	return truncate(strings.Join(parts, ","), 60)
}

// formatCPU formats CPU value (millicores)
func formatCPU(val int64, showRaw bool) string {
	if showRaw {
		if val >= 1000 {
			return fmt.Sprintf("%dm", val)
		}
		return fmt.Sprintf("%dm", val)
	}
	// Show as cores
	return fmt.Sprintf("%.2f", float64(val)/1000)
}

// formatMemory formats memory value (bytes)
func formatMemory(val int64, showRaw bool) string {
	if showRaw {
		if val >= 1024*1024*1024 {
			return fmt.Sprintf("%dGi", val/(1024*1024*1024))
		}
		if val >= 1024*1024 {
			return fmt.Sprintf("%dMi", val/(1024*1024))
		}
		if val >= 1024 {
			return fmt.Sprintf("%dKi", val/1024)
		}
		return fmt.Sprintf("%d", val)
	}
	// Show as Gi
	return fmt.Sprintf("%.2fGi", float64(val)/(1024*1024*1024))
}

// parseLabelSelector parses a label selector string into a map.
// Supports format: "key1=value1,key2=value2" or "key1=value1 key2=value2"
// Also supports "key1,key2" format (existence check only)
func parseLabelSelector(selector string) map[string]string {
	result := make(map[string]string)
	if selector == "" {
		return result
	}

	// Split by comma or space
	parts := strings.FieldsFunc(selector, func(r rune) bool {
		return r == ',' || r == ' '
	})

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Check for key=value or key==value format
		if idx := strings.Index(part, "=="); idx != -1 {
			key := strings.TrimSpace(part[:idx])
			value := strings.TrimSpace(part[idx+2:])
			result[key] = value
		} else if idx := strings.Index(part, "="); idx != -1 {
			key := strings.TrimSpace(part[:idx])
			value := strings.TrimSpace(part[idx+1:])
			result[key] = value
		}
		// Note: existence check only (e.g., "key") is not supported in this simple parser
	}

	return result
}

// matchTaints checks if node taints match the taint selector expression.
// Format: "key=value:effect" to include, "key=value:effect-" to exclude
// Multiple taints can be separated by comma
func matchTaints(taints []corev1.Taint, selector string) bool {
	if selector == "" {
		return true
	}

	// Parse taint selector
	parts := strings.Split(selector, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Check for exclusion (ends with -)
		exclude := false
		if strings.HasSuffix(part, "-") {
			exclude = true
			part = part[:len(part)-1]
		}

		// Parse taint: key=value:effect
		var key, value, effect string
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
			// key:effect format (no value)
			key = part[:idx]
			effect = part[idx+1:]
		} else {
			// Just key
			key = part
		}

		// Check if taint exists on node
		found := false
		for _, t := range taints {
			if t.Key == key {
				if value != "" && t.Value != value {
					continue
				}
				if effect != "" && string(t.Effect) != effect {
					continue
				}
				found = true
				break
			}
		}

		// For inclusion (no suffix), taint must be found
		// For exclusion (- suffix), taint must NOT be found
		if exclude && found {
			return false
		}
		if !exclude && !found {
			return false
		}
	}

	return true
}

// matchLabels checks if the given labels match the selector map.
// All selector key-value pairs must match the labels.
func matchLabels(labels, selector map[string]string) bool {
	for key, value := range selector {
		if labels[key] != value {
			return false
		}
	}
	return true
}
