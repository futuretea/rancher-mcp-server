package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/futuretea/rancher-mcp-server/pkg/client/steve"
	"github.com/futuretea/rancher-mcp-server/pkg/dep"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset/paramutil"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// depHandler handles the kubernetes_dep tool
func depHandler(ctx context.Context, client interface{}, params map[string]interface{}) (string, error) {
	steveClient, err := toolset.ValidateSteveClient(client)
	if err != nil {
		return "", err
	}

	cluster, err := paramutil.ExtractRequiredString(params, paramutil.ParamCluster)
	if err != nil {
		return "", err
	}
	kind, err := paramutil.ExtractRequiredString(params, paramutil.ParamKind)
	if err != nil {
		return "", err
	}
	name, err := paramutil.ExtractRequiredString(params, paramutil.ParamName)
	if err != nil {
		return "", err
	}
	namespace := paramutil.ExtractOptionalString(params, paramutil.ParamNamespace)
	direction := paramutil.ExtractOptionalStringWithDefault(params, paramutil.ParamDirection, "dependents")
	maxDepth := int(paramutil.ExtractInt64(params, paramutil.ParamDepth, DefaultMaxDepth))
	format := paramutil.ExtractOptionalStringWithDefault(params, paramutil.ParamFormat, "tree")

	if direction != "dependents" && direction != "dependencies" {
		return "", fmt.Errorf("%w: direction must be 'dependents' or 'dependencies'", paramutil.ErrMissingParameter)
	}
	if maxDepth < MinDepth || maxDepth > MaxDepth {
		maxDepth = DefaultMaxDepth
	}

	result, err := dep.Resolve(ctx, steveClient, cluster, kind, namespace, name, direction, maxDepth)
	if err != nil {
		return "", fmt.Errorf("failed to resolve dependencies: %w", err)
	}

	depsIsDependencies := direction == "dependencies"

	switch format {
	case "json":
		return dep.FormatJSON(result, depsIsDependencies)
	default: // tree
		return dep.FormatTree(result, depsIsDependencies), nil
	}
}

// NodeAnalysisResult contains the comprehensive analysis of a node.
type NodeAnalysisResult struct {
	Node      *unstructured.Unstructured `json:"node"`
	Capacity  map[string]string          `json:"capacity"`
	Allocated map[string]string          `json:"allocated"`
	Taints    []corev1.Taint             `json:"taints"`
	Labels    map[string]string          `json:"labels"`
	Pods      []NodePodInfo              `json:"pods"`
}

// NodePodInfo contains summary information about a pod running on the node.
type NodePodInfo struct {
	Namespace     string `json:"namespace"`
	Name          string `json:"name"`
	Phase         string `json:"phase"`
	CPURequest    string `json:"cpuRequest,omitempty"`
	MemoryRequest string `json:"memoryRequest,omitempty"`
	CPULimit      string `json:"cpuLimit,omitempty"`
	MemoryLimit   string `json:"memoryLimit,omitempty"`
}

// nodeAnalysisHandler handles the kubernetes_node_analysis tool
func nodeAnalysisHandler(ctx context.Context, client interface{}, params map[string]interface{}) (string, error) {
	steveClient, err := toolset.ValidateSteveClient(client)
	if err != nil {
		return "", err
	}

	cluster, err := paramutil.ExtractRequiredString(params, paramutil.ParamCluster)
	if err != nil {
		return "", err
	}
	name, err := paramutil.ExtractRequiredString(params, paramutil.ParamName)
	if err != nil {
		return "", err
	}
	format := paramutil.ExtractFormat(params)

	// Get node details
	node, err := steveClient.GetResource(ctx, cluster, "node", "", name)
	if err != nil {
		return "", fmt.Errorf("failed to get node: %w", err)
	}

	result := &NodeAnalysisResult{
		Node:      node,
		Capacity:  make(map[string]string),
		Allocated: make(map[string]string),
		Taints:    []corev1.Taint{},
		Labels:    make(map[string]string),
		Pods:      []NodePodInfo{},
	}

	// Extract capacity
	if capacity, found, _ := unstructured.NestedMap(node.Object, "status", "capacity"); found {
		for k, v := range capacity {
			if s, ok := v.(string); ok {
				result.Capacity[k] = s
			}
		}
	}

	// Extract allocatable (as allocated capacity)
	if allocatable, found, _ := unstructured.NestedMap(node.Object, "status", "allocatable"); found {
		for k, v := range allocatable {
			if s, ok := v.(string); ok {
				result.Allocated[k] = s
			}
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
			result.Taints = append(result.Taints, taint)
		}
	}

	// Extract labels
	result.Labels = node.GetLabels()

	// Get pods running on this node
	pods, err := steveClient.ListResources(ctx, cluster, "pod", "", &steve.ListOptions{
		FieldSelector: "spec.nodeName=" + name,
	})
	if err != nil {
		return "", fmt.Errorf("failed to list pods on node: %w", err)
	}

	// Process each pod
	for _, pod := range pods.Items {
		podInfo := NodePodInfo{
			Namespace: pod.GetNamespace(),
			Name:      pod.GetName(),
		}

		// Get pod phase
		if phase, found, _ := unstructured.NestedString(pod.Object, "status", "phase"); found {
			podInfo.Phase = phase
		}

		// Extract container resource requests and limits
		containers, found, _ := unstructured.NestedSlice(pod.Object, "spec", "containers")
		if found {
			var totalCPURequest, totalMemoryRequest, totalCPULimit, totalMemoryLimit int64

			for _, c := range containers {
				container, ok := c.(map[string]interface{})
				if !ok {
					continue
				}

				resources, found, _ := unstructured.NestedMap(container, "resources")
				if !found {
					continue
				}

				// Parse requests
				if requests, found, _ := unstructured.NestedMap(resources, "requests"); found {
					if cpu, ok := requests["cpu"].(string); ok {
						totalCPURequest += parseResourceQuantity(cpu)
					}
					if memory, ok := requests["memory"].(string); ok {
						totalMemoryRequest += parseResourceQuantity(memory)
					}
				}

				// Parse limits
				if limits, found, _ := unstructured.NestedMap(resources, "limits"); found {
					if cpu, ok := limits["cpu"].(string); ok {
						totalCPULimit += parseResourceQuantity(cpu)
					}
					if memory, ok := limits["memory"].(string); ok {
						totalMemoryLimit += parseResourceQuantity(memory)
					}
				}
			}

			if totalCPURequest > 0 {
				podInfo.CPURequest = formatResourceQuantity(totalCPURequest, "cpu")
			}
			if totalMemoryRequest > 0 {
				podInfo.MemoryRequest = formatResourceQuantity(totalMemoryRequest, "memory")
			}
			if totalCPULimit > 0 {
				podInfo.CPULimit = formatResourceQuantity(totalCPULimit, "cpu")
			}
			if totalMemoryLimit > 0 {
				podInfo.MemoryLimit = formatResourceQuantity(totalMemoryLimit, "memory")
			}
		}

		result.Pods = append(result.Pods, podInfo)
	}

	// Format output
	switch format {
	case paramutil.FormatYAML:
		data, err := yaml.Marshal(result)
		if err != nil {
			return "", fmt.Errorf("failed to format as YAML: %w", err)
		}
		return string(data), nil
	default: // json
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to format as JSON: %w", err)
		}
		return string(data), nil
	}
}

// parseResourceQuantity parses a Kubernetes resource quantity string to a numeric value.
// Supports millicores (m) for CPU and binary (Ki, Mi, Gi) and decimal (K, M, G) for memory.
func parseResourceQuantity(q string) int64 {
	q = strings.TrimSpace(q)
	if q == "" {
		return 0
	}

	// Handle millicores (e.g., "100m", "500m")
	if strings.HasSuffix(q, "m") {
		val, err := parseNumeric(q[:len(q)-1])
		if err != nil {
			return 0
		}
		return val // Keep as millicores
	}

	// Handle binary memory units
	if strings.HasSuffix(q, "Ki") {
		val, err := parseNumeric(q[:len(q)-2])
		if err != nil {
			return 0
		}
		return val * BytesPerKi
	}
	if strings.HasSuffix(q, "Mi") {
		val, err := parseNumeric(q[:len(q)-2])
		if err != nil {
			return 0
		}
		return val * BytesPerMi
	}
	if strings.HasSuffix(q, "Gi") {
		val, err := parseNumeric(q[:len(q)-2])
		if err != nil {
			return 0
		}
		return val * BytesPerGi
	}
	if strings.HasSuffix(q, "Ti") {
		val, err := parseNumeric(q[:len(q)-2])
		if err != nil {
			return 0
		}
		return val * BytesPerTi
	}

	// Handle decimal memory units
	if strings.HasSuffix(q, "k") || strings.HasSuffix(q, "K") {
		val, err := parseNumeric(q[:len(q)-1])
		if err != nil {
			return 0
		}
		return val * DecimalKilo
	}
	if strings.HasSuffix(q, "M") {
		val, err := parseNumeric(q[:len(q)-1])
		if err != nil {
			return 0
		}
		return val * DecimalMega
	}
	if strings.HasSuffix(q, "G") {
		val, err := parseNumeric(q[:len(q)-1])
		if err != nil {
			return 0
		}
		return val * DecimalGiga
	}

	// Plain number
	val, err := parseNumeric(q)
	if err != nil {
		return 0
	}
	return val
}

// parseNumeric parses a numeric string to int64.
func parseNumeric(s string) (int64, error) {
	var result int64
	_, err := fmt.Sscanf(s, "%d", &result)
	return result, err
}

// formatResourceQuantity formats a numeric resource quantity back to a human-readable string.
func formatResourceQuantity(val int64, resourceType string) string {
	if resourceType == "cpu" {
		if val >= MilliCPUBase {
			return fmt.Sprintf("%dm (%dc)", val, val/MilliCPUBase)
		}
		return fmt.Sprintf("%dm", val)
	}

	// Memory
	if val >= BytesPerGi {
		return fmt.Sprintf("%dGi (%d bytes)", val/BytesPerGi, val)
	}
	if val >= BytesPerMi {
		return fmt.Sprintf("%dMi (%d bytes)", val/BytesPerMi, val)
	}
	if val >= BytesPerKi {
		return fmt.Sprintf("%dKi (%d bytes)", val/BytesPerKi, val)
	}
	return fmt.Sprintf("%d bytes", val)
}
