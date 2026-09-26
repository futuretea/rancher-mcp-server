package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/futuretea/rancher-mcp-server/pkg/client/steve"
	"github.com/futuretea/rancher-mcp-server/pkg/dep"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset/paramutil"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

// depHandler handles the kubernetes_dep tool
func depHandler(ctx context.Context, client interface{}, params map[string]interface{}) (string, error) {
	request, err := buildDepRequest(params)
	if err != nil {
		return "", err
	}
	if err := allowNamedAccess(client, request.Cluster, request.Kind, request.Namespace, request.Name); err != nil {
		return "", err
	}
	scans, err := depScanNamespaces(client, request.Cluster, request.ResolveOptions.ScanNamespace)
	if err != nil {
		return "", err
	}
	reader, err := kubernetesReader(client)
	if err != nil {
		return "", err
	}
	return resolveDepScans(ctx, reader, request, scans)
}

func depScanNamespaces(client interface{}, cluster, scanNamespace string) ([]string, error) {
	query, err := planNamespaceQuery(client, cluster, "pod", scanNamespace)
	if err != nil {
		return nil, err
	}
	if query.passthrough {
		return []string{scanNamespace}, nil
	}
	return query.names, nil
}

func resolveDepScans(ctx context.Context, reader steve.ResourceReader, request *depRequest, scans []string) (string, error) {
	var merged *dep.Result
	for _, scanNamespace := range scans {
		options := request.ResolveOptions
		options.ScanNamespace = scanNamespace
		result, err := dep.Resolve(ctx, reader, request.Cluster, request.Kind, request.Namespace, request.Name, options)
		if err != nil {
			return "", fmt.Errorf("failed to resolve dependencies: %w", err)
		}
		if merged == nil {
			merged = result
			continue
		}
		if err := mergeDepResult(merged, result); err != nil {
			return "", err
		}
	}
	return formatDepResult(merged, request.ResolveOptions.Direction, request.Format)
}

func mergeDepResult(target, source *dep.Result) error {
	if target.RootUID != source.RootUID {
		return fmt.Errorf("dependency scans resolved different root resources")
	}
	for uid, sourceNode := range source.NodeMap {
		targetNode, exists := target.NodeMap[uid]
		if !exists {
			target.NodeMap[uid] = sourceNode
			continue
		}
		mergeRelationshipMap(targetNode.Dependencies, sourceNode.Dependencies)
		mergeRelationshipMap(targetNode.Dependents, sourceNode.Dependents)
	}
	return nil
}

func mergeRelationshipMap(target, source map[types.UID]dep.RelationshipSet) {
	for uid, sourceRelationships := range source {
		targetRelationships, exists := target[uid]
		if !exists {
			targetRelationships = dep.RelationshipSet{}
			target[uid] = targetRelationships
		}
		for relationship := range sourceRelationships {
			targetRelationships[relationship] = struct{}{}
		}
	}
}

func formatDepResult(result *dep.Result, direction, format string) (string, error) {
	depsIsDependencies := direction == "dependencies"
	if format == "json" {
		return dep.FormatJSON(result, depsIsDependencies)
	}
	return dep.FormatTree(result, depsIsDependencies), nil
}

type depRequest struct {
	Cluster        string
	Kind           string
	Name           string
	Namespace      string
	Format         string
	ResolveOptions dep.ResolveOptions
}

func buildDepRequest(params map[string]interface{}) (*depRequest, error) {
	cluster, err := paramutil.ExtractRequiredString(params, paramutil.ParamCluster)
	if err != nil {
		return nil, err
	}
	kind, err := extractResourceKind(params)
	if err != nil {
		return nil, err
	}
	name, err := paramutil.ExtractRequiredString(params, paramutil.ParamName)
	if err != nil {
		return nil, err
	}

	namespace := paramutil.ExtractOptionalString(params, paramutil.ParamNamespace)
	scanNamespace := paramutil.ExtractOptionalString(params, paramutil.ParamScanNamespace)
	direction := paramutil.ExtractOptionalStringWithDefault(params, paramutil.ParamDirection, "dependents")
	maxDepth := int(paramutil.ExtractInt64(params, paramutil.ParamDepth, DefaultMaxDepth))
	maxScannedObjects := int(paramutil.ExtractInt64(params, paramutil.ParamMaxScannedObjects, 0))
	format := paramutil.ExtractOptionalStringWithDefault(params, paramutil.ParamFormat, "tree")

	if direction != "dependents" && direction != "dependencies" {
		return nil, fmt.Errorf("%w: direction must be 'dependents' or 'dependencies'", paramutil.ErrMissingParameter)
	}
	if maxDepth < MinDepth || maxDepth > MaxDepth {
		maxDepth = DefaultMaxDepth
	}
	if maxScannedObjects < 0 {
		return nil, fmt.Errorf("%w: maxScannedObjects must be >= 0", paramutil.ErrMissingParameter)
	}
	if namespace != "" {
		if scanNamespace == "" {
			scanNamespace = namespace
		} else if scanNamespace != namespace {
			return nil, fmt.Errorf("scan namespace %q does not match namespaced root namespace %q", scanNamespace, namespace)
		}
	}

	return &depRequest{
		Cluster:   cluster,
		Kind:      kind,
		Name:      name,
		Namespace: namespace,
		Format:    format,
		ResolveOptions: dep.ResolveOptions{
			Direction:         direction,
			MaxDepth:          maxDepth,
			ScanNamespace:     scanNamespace,
			MaxScannedObjects: maxScannedObjects,
		},
	}, nil
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
	cluster, err := paramutil.ExtractRequiredString(params, paramutil.ParamCluster)
	if err != nil {
		return "", err
	}
	name, err := paramutil.ExtractRequiredString(params, paramutil.ParamName)
	if err != nil {
		return "", err
	}
	format := paramutil.ExtractFormat(params)
	reader, err := kubernetesReader(client)
	if err != nil {
		return "", err
	}

	node, err := reader.GetResource(ctx, cluster, "node", "", name)
	if err != nil {
		return "", fmt.Errorf("failed to get node: %w", err)
	}

	result, err := buildNodeAnalysisResult(ctx, reader, cluster, node, name)
	if err != nil {
		return "", err
	}

	return formatNodeAnalysisResult(result, format)
}

// buildNodeAnalysisResult aggregates node metadata and the pods scheduled on it.
func buildNodeAnalysisResult(ctx context.Context, client steve.ResourceReader, cluster string, node *unstructured.Unstructured, name string) (*NodeAnalysisResult, error) {
	result := &NodeAnalysisResult{
		Node:      node,
		Capacity:  extractStringMap(node.Object, "status", "capacity"),
		Allocated: extractStringMap(node.Object, "status", "allocatable"),
		Taints:    extractNodeTaints(node.Object),
		Labels:    node.GetLabels(),
		Pods:      []NodePodInfo{},
	}

	pods, err := listResourcesAllowed(ctx, client, cluster, "pod", "", &steve.ListOptions{
		FieldSelector: "spec.nodeName=" + name,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods on node: %w", err)
	}

	result.Pods = extractNodePods(pods)
	return result, nil
}

// extractStringMap reads a nested string map from an unstructured object.
func extractStringMap(obj map[string]interface{}, fields ...string) map[string]string {
	result := make(map[string]string)
	if m, found, _ := unstructured.NestedMap(obj, fields...); found {
		for k, v := range m {
			if s, ok := v.(string); ok {
				result[k] = s
			}
		}
	}
	return result
}

// extractNodeTaints parses the node spec taints into typed Taint values.
func extractNodeTaints(obj map[string]interface{}) []corev1.Taint {
	taints, found, _ := unstructured.NestedSlice(obj, "spec", "taints")
	if !found {
		return []corev1.Taint{}
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

// extractNodePods summarizes the pods running on a node.
func extractNodePods(pods *unstructured.UnstructuredList) []NodePodInfo {
	result := make([]NodePodInfo, 0, len(pods.Items))
	for _, pod := range pods.Items {
		podInfo := NodePodInfo{
			Namespace: pod.GetNamespace(),
			Name:      pod.GetName(),
		}

		if phase, found, _ := unstructured.NestedString(pod.Object, "status", "phase"); found {
			podInfo.Phase = phase
		}

		podInfo.CPURequest, podInfo.MemoryRequest, podInfo.CPULimit, podInfo.MemoryLimit = extractPodResourceUsage(pod.Object)
		result = append(result, podInfo)
	}
	return result
}

// extractPodResourceUsage returns aggregated CPU/memory requests and limits for a pod.
func extractPodResourceUsage(obj map[string]interface{}) (cpuRequest, memoryRequest, cpuLimit, memoryLimit string) {
	containers, found, _ := unstructured.NestedSlice(obj, "spec", "containers")
	if !found {
		return
	}

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

		totalCPURequest += resourceQuantityFromMap(resources, "requests", "cpu")
		totalMemoryRequest += resourceQuantityFromMap(resources, "requests", "memory")
		totalCPULimit += resourceQuantityFromMap(resources, "limits", "cpu")
		totalMemoryLimit += resourceQuantityFromMap(resources, "limits", "memory")
	}

	if totalCPURequest > 0 {
		cpuRequest = formatResourceQuantity(totalCPURequest, "cpu")
	}
	if totalMemoryRequest > 0 {
		memoryRequest = formatResourceQuantity(totalMemoryRequest, "memory")
	}
	if totalCPULimit > 0 {
		cpuLimit = formatResourceQuantity(totalCPULimit, "cpu")
	}
	if totalMemoryLimit > 0 {
		memoryLimit = formatResourceQuantity(totalMemoryLimit, "memory")
	}
	return
}

// resourceQuantityFromMap extracts a single resource quantity from a resources map.
func resourceQuantityFromMap(resources map[string]interface{}, section, resource string) int64 {
	sectionMap, found, _ := unstructured.NestedMap(resources, section)
	if !found {
		return 0
	}
	q, ok := sectionMap[resource].(string)
	if !ok {
		return 0
	}
	return parseResourceQuantity(q)
}

// formatNodeAnalysisResult renders the analysis result as JSON or YAML.
func formatNodeAnalysisResult(result *NodeAnalysisResult, format string) (string, error) {
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

// resourceUnit maps a Kubernetes quantity suffix to its scaling factor.
type resourceUnit struct {
	suffix string
	factor int64
}

// resourceUnits orders suffixes from longest/most-specific to shortest to avoid
// ambiguous matches (e.g., "Ki" must be checked before "K").
var resourceUnits = []resourceUnit{
	{"Ki", BytesPerKi},
	{"Mi", BytesPerMi},
	{"Gi", BytesPerGi},
	{"Ti", BytesPerTi},
	{"m", 1},
	{"k", DecimalKilo},
	{"K", DecimalKilo},
	{"M", DecimalMega},
	{"G", DecimalGiga},
}

// parseResourceQuantity parses a Kubernetes resource quantity string to a numeric value.
// Supports millicores (m) for CPU and binary (Ki, Mi, Gi, Ti) and decimal (K, M, G) for memory.
// Decimal and exponential notation (e.g. 0.5Gi, 1e9) are also accepted.
func parseResourceQuantity(q string) int64 {
	q = strings.TrimSpace(q)
	if q == "" {
		return 0
	}

	for _, unit := range resourceUnits {
		if strings.HasSuffix(q, unit.suffix) {
			f, err := parseNumericFloat(q[:len(q)-len(unit.suffix)])
			if err != nil {
				return 0
			}
			return int64(math.Round(f * float64(unit.factor)))
		}
	}

	f, err := parseNumericFloat(q)
	if err != nil {
		return 0
	}
	return int64(math.Round(f))
}

// parseNumeric parses a numeric string to int64.
func parseNumeric(s string) (int64, error) {
	f, err := parseNumericFloat(s)
	if err != nil {
		return 0, err
	}
	return int64(math.Round(f)), nil
}

// parseNumericFloat parses a numeric string to float64, accepting decimal and
// exponential notation.
func parseNumericFloat(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty numeric string")
	}
	return strconv.ParseFloat(s, 64)
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
