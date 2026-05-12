package kubernetes

import (
	"context"
	"fmt"

	"github.com/futuretea/rancher-mcp-server/pkg/toolset"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset/kubernetes/aggregate"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset/paramutil"
)

// topHandler handles the kubernetes_top tool
func topHandler(ctx context.Context, client interface{}, params map[string]interface{}) (string, error) {
	steveClient, err := toolset.ValidateSteveClient(client)
	if err != nil {
		return "", err
	}

	cluster, err := paramutil.ExtractRequiredString(params, paramutil.ParamCluster)
	if err != nil {
		return "", err
	}

	kind := extractStringParam(params, "kind", "pod")
	namespace := paramutil.ExtractOptionalString(params, paramutil.ParamNamespace)
	labelSelector := paramutil.ExtractOptionalString(params, paramutil.ParamLabelSelector)
	sortBy := extractStringParam(params, "sortBy", "")
	limit := extractIntParam(params, paramutil.ParamLimit, aggregate.DefaultLimit)
	format := paramutil.ExtractOptionalStringWithDefault(params, paramutil.ParamFormat, paramutil.FormatTable)

	// Validate limit
	if limit > aggregate.MaxItems {
		limit = aggregate.MaxItems
	}

	analyzer := aggregate.NewTopAnalyzer(steveClient)
	result, err := analyzer.Analyze(ctx, aggregate.TopParams{
		Cluster:       cluster,
		Kind:          kind,
		Namespace:     namespace,
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
	steveClient, err := toolset.ValidateSteveClient(client)
	if err != nil {
		return "", err
	}

	cluster, err := paramutil.ExtractRequiredString(params, paramutil.ParamCluster)
	if err != nil {
		return "", err
	}

	kind := extractStringParam(params, "kind", "all")
	namespace := paramutil.ExtractOptionalString(params, paramutil.ParamNamespace)
	labelSelector := paramutil.ExtractOptionalString(params, paramutil.ParamLabelSelector)
	sortBy := extractStringParam(params, "sortBy", "")
	limit := extractIntParam(params, paramutil.ParamLimit, aggregate.DefaultLimit)
	format := paramutil.ExtractFormat(params)

	if limit > aggregate.MaxItems {
		limit = aggregate.MaxItems
	}

	analyzer := aggregate.NewWorkloadAnalyzer(steveClient)
	result, err := analyzer.Analyze(ctx, aggregate.WorkloadParams{
		Cluster:       cluster,
		Kind:          kind,
		Namespace:     namespace,
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
	steveClient, err := toolset.ValidateSteveClient(client)
	if err != nil {
		return "", err
	}

	cluster, err := paramutil.ExtractRequiredString(params, paramutil.ParamCluster)
	if err != nil {
		return "", err
	}

	namespace := paramutil.ExtractOptionalString(params, paramutil.ParamNamespace)
	labelSelector := paramutil.ExtractOptionalString(params, paramutil.ParamLabelSelector)
	groupBy := extractStringParam(params, "groupBy", "namespace")
	groupByKey := extractStringParam(params, "groupByKey", "")
	sortBy := extractStringParam(params, "sortBy", "")
	limit := extractIntParam(params, paramutil.ParamLimit, aggregate.DefaultLimit)
	format := paramutil.ExtractFormat(params)

	if limit > aggregate.MaxItems {
		limit = aggregate.MaxItems
	}

	analyzer := aggregate.NewSummaryAnalyzer(steveClient)
	result, err := analyzer.Analyze(ctx, aggregate.SummaryParams{
		Cluster:       cluster,
		Namespace:     namespace,
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
	steveClient, err := toolset.ValidateSteveClient(client)
	if err != nil {
		return "", err
	}

	cluster, err := paramutil.ExtractRequiredString(params, paramutil.ParamCluster)
	if err != nil {
		return "", err
	}

	namespace := paramutil.ExtractOptionalString(params, paramutil.ParamNamespace)
	kind := extractStringParam(params, "kind", "")
	eventType := extractStringParam(params, "type", "")
	since := extractStringParam(params, "since", "")
	sortBy := extractStringParam(params, "sortBy", "")
	limit := extractIntParam(params, paramutil.ParamLimit, aggregate.DefaultLimit)
	format := paramutil.ExtractFormat(params)

	if limit > aggregate.MaxItems {
		limit = aggregate.MaxItems
	}

	analyzer := aggregate.NewEventAnalyzer(steveClient)
	result, err := analyzer.Analyze(ctx, aggregate.EventParams{
		Cluster:   cluster,
		Namespace: namespace,
		Kind:      kind,
		Type:      eventType,
		Since:     since,
		SortBy:    sortBy,
		Limit:     limit,
		Format:    format,
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
