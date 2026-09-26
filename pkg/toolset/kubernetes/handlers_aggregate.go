package kubernetes

import (
	"context"
	"fmt"
	"strings"

	"github.com/futuretea/rancher-mcp-server/pkg/toolset/kubernetes/aggregate"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset/paramutil"
)

// topHandler handles the kubernetes_top tool
func topHandler(ctx context.Context, client interface{}, params map[string]interface{}) (string, error) {
	cluster, err := paramutil.ExtractRequiredString(params, paramutil.ParamCluster)
	if err != nil {
		return "", err
	}

	kind := extractStringParam(params, "kind", "pod")
	namespace := paramutil.ExtractOptionalString(params, paramutil.ParamNamespace)
	var namespaces []string
	if !strings.EqualFold(kind, "node") {
		namespace, namespaces, err = listedNamespaces(client, cluster, "pod", namespace)
		if err != nil {
			return "", err
		}
	}
	labelSelector := paramutil.ExtractOptionalString(params, paramutil.ParamLabelSelector)
	sortBy := extractStringParam(params, "sortBy", "")
	limit := aggregate.ClampLimit(extractIntParam(params, paramutil.ParamLimit, aggregate.DefaultLimit))
	format := paramutil.ExtractOptionalStringWithDefault(params, paramutil.ParamFormat, paramutil.FormatTable)
	reader, err := kubernetesReader(client)
	if err != nil {
		return "", err
	}

	analyzer := aggregate.NewTopAnalyzer(reader)
	result, err := analyzer.Analyze(ctx, aggregate.TopParams{
		Cluster:       cluster,
		Kind:          kind,
		Namespace:     namespace,
		Namespaces:    namespaces,
		LabelSelector: labelSelector,
		SortBy:        sortBy,
		Limit:         limit,
		Format:        format,
	})
	if err != nil {
		return "", fmt.Errorf("top analysis failed: %w", err)
	}

	return aggregate.FormatResult(result, format)
}

// workloadHealthHandler handles the kubernetes_workload_health tool
func workloadHealthHandler(ctx context.Context, client interface{}, params map[string]interface{}) (string, error) {
	cluster, err := paramutil.ExtractRequiredString(params, paramutil.ParamCluster)
	if err != nil {
		return "", err
	}

	kind := extractStringParam(params, "kind", "all")
	namespace := paramutil.ExtractOptionalString(params, paramutil.ParamNamespace)
	namespace, namespaces, err := listedNamespaces(client, cluster, "deployment", namespace)
	if err != nil {
		return "", err
	}
	labelSelector := paramutil.ExtractOptionalString(params, paramutil.ParamLabelSelector)
	sortBy := extractStringParam(params, "sortBy", "")
	limit := aggregate.ClampLimit(extractIntParam(params, paramutil.ParamLimit, aggregate.DefaultLimit))
	format := paramutil.ExtractFormat(params)
	reader, err := kubernetesReader(client)
	if err != nil {
		return "", err
	}

	analyzer := aggregate.NewWorkloadAnalyzer(reader)
	result, err := analyzer.Analyze(ctx, aggregate.WorkloadParams{
		Cluster:       cluster,
		Kind:          kind,
		Namespace:     namespace,
		Namespaces:    namespaces,
		LabelSelector: labelSelector,
		SortBy:        sortBy,
		Limit:         limit,
		Format:        format,
	})
	if err != nil {
		return "", fmt.Errorf("workload health analysis failed: %w", err)
	}

	return aggregate.FormatResult(result, format)
}

// resourceSummaryHandler handles the kubernetes_resource_summary tool
func resourceSummaryHandler(ctx context.Context, client interface{}, params map[string]interface{}) (string, error) {
	cluster, err := paramutil.ExtractRequiredString(params, paramutil.ParamCluster)
	if err != nil {
		return "", err
	}

	namespace := paramutil.ExtractOptionalString(params, paramutil.ParamNamespace)
	namespace, namespaces, err := listedNamespaces(client, cluster, "pod", namespace)
	if err != nil {
		return "", err
	}
	labelSelector := paramutil.ExtractOptionalString(params, paramutil.ParamLabelSelector)
	groupBy := extractStringParam(params, "groupBy", "namespace")
	groupByKey := extractStringParam(params, "groupByKey", "")
	sortBy := extractStringParam(params, "sortBy", "")
	limit := aggregate.ClampLimit(extractIntParam(params, paramutil.ParamLimit, aggregate.DefaultLimit))
	format := paramutil.ExtractFormat(params)

	reader, err := kubernetesReader(client)
	if err != nil {
		return "", err
	}

	analyzer := aggregate.NewSummaryAnalyzer(reader)
	result, err := analyzer.Analyze(ctx, aggregate.SummaryParams{
		Cluster:       cluster,
		Namespace:     namespace,
		Namespaces:    namespaces,
		LabelSelector: labelSelector,
		GroupBy:       groupBy,
		GroupByKey:    groupByKey,
		SortBy:        sortBy,
		Limit:         limit,
		Format:        format,
	})
	if err != nil {
		return "", fmt.Errorf("resource summary analysis failed: %w", err)
	}

	return aggregate.FormatResult(result, format)
}

// eventSummaryHandler handles the kubernetes_event_summary tool
func eventSummaryHandler(ctx context.Context, client interface{}, params map[string]interface{}) (string, error) {
	cluster, err := paramutil.ExtractRequiredString(params, paramutil.ParamCluster)
	if err != nil {
		return "", err
	}

	namespace := paramutil.ExtractOptionalString(params, paramutil.ParamNamespace)
	namespace, namespaces, err := listedNamespaces(client, cluster, "event", namespace)
	if err != nil {
		return "", err
	}
	kind := extractStringParam(params, "kind", "")
	eventType := extractStringParam(params, "type", "")
	since := extractStringParam(params, "since", "")
	sortBy := extractStringParam(params, "sortBy", "")
	limit := aggregate.ClampLimit(extractIntParam(params, paramutil.ParamLimit, aggregate.DefaultLimit))
	format := paramutil.ExtractFormat(params)

	reader, err := kubernetesReader(client)
	if err != nil {
		return "", err
	}

	analyzer := aggregate.NewEventAnalyzer(reader)
	result, err := analyzer.Analyze(ctx, aggregate.EventParams{
		Cluster:    cluster,
		Namespace:  namespace,
		Namespaces: namespaces,
		Kind:       kind,
		Type:       eventType,
		Since:      since,
		SortBy:     sortBy,
		Limit:      limit,
		Format:     format,
	})
	if err != nil {
		return "", fmt.Errorf("event summary analysis failed: %w", err)
	}

	return aggregate.FormatResult(result, format)
}

// extractStringParam extracts a string parameter with a default value
func extractStringParam(params map[string]interface{}, key, defaultValue string) string {
	if val, ok := params[key].(string); ok {
		return val
	}
	return defaultValue
}

// extractIntParam extracts an int parameter with a default value
func extractIntParam(params map[string]interface{}, key string, defaultValue int) int {
	// Try float64 first (JSON numbers come as float64)
	if val, ok := params[key].(float64); ok {
		return int(val)
	}
	// Try int
	if val, ok := params[key].(int); ok {
		return val
	}
	// Try int64
	if val, ok := params[key].(int64); ok {
		return int(val)
	}
	return defaultValue
}
