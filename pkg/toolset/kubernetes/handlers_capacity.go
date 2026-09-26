package kubernetes

import (
	"context"

	"github.com/futuretea/rancher-mcp-server/pkg/toolset/kubernetes/capacity"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset/paramutil"
)

// capacityHandler handles the kubernetes_capacity tool
func capacityHandler(ctx context.Context, client interface{}, params map[string]interface{}) (string, error) {
	p, err := extractCapacityParams(params)
	if err != nil {
		return "", err
	}
	namespace, namespaces, err := listedNamespaces(client, p.Cluster, "pod", p.Namespace)
	if err != nil {
		return "", err
	}
	p.Namespace = namespace
	p.Namespaces = namespaces
	// An explicit namespace still has to intersect namespaceLabelSelector with
	// the allowlist. Listing every namespace would read names outside the list.
	if _, restricted := restrictedNames(p.Cluster); restricted && p.NamespaceLabelSelector != "" && len(p.Namespaces) == 0 && namespace != "" {
		p.Namespaces = []string{namespace}
		p.Namespace = ""
	}
	reader, err := kubernetesReader(client)
	if err != nil {
		return "", err
	}

	analyzer := capacity.NewAnalyzer(reader)
	result, err := analyzer.Analyze(ctx, p)
	if err != nil {
		return "", err
	}

	return capacity.FormatResult(result, p.Format, p.ShowAvailable)
}

// extractCapacityParams extracts parameters from the input map
func extractCapacityParams(params map[string]interface{}) (capacity.Params, error) {
	cluster, err := paramutil.ExtractRequiredString(params, paramutil.ParamCluster)
	if err != nil {
		return capacity.Params{}, err
	}

	return capacity.Params{
		Cluster:                cluster,
		ShowPods:               paramutil.ExtractBool(params, "pods", false),
		ShowContainers:         paramutil.ExtractBool(params, "containers", false),
		ShowUtil:               paramutil.ExtractBool(params, "util", false),
		ShowAvailable:          paramutil.ExtractBool(params, "available", false),
		ShowPodCount:           paramutil.ExtractBool(params, "podCount", false),
		ShowLabels:             paramutil.ExtractBool(params, "showLabels", false),
		HideRequests:           paramutil.ExtractBool(params, "hideRequests", false),
		HideLimits:             paramutil.ExtractBool(params, "hideLimits", false),
		NoTaint:                paramutil.ExtractBool(params, "noTaint", false),
		Namespace:              paramutil.ExtractOptionalString(params, paramutil.ParamNamespace),
		LabelSelector:          paramutil.ExtractOptionalString(params, paramutil.ParamLabelSelector),
		NodeLabelSelector:      paramutil.ExtractOptionalString(params, "nodeLabelSelector"),
		NamespaceLabelSelector: paramutil.ExtractOptionalString(params, "namespaceLabelSelector"),
		NodeTaints:             paramutil.ExtractOptionalString(params, "nodeTaints"),
		SortBy:                 paramutil.ExtractOptionalString(params, "sortBy"),
		Format:                 paramutil.ExtractOptionalStringWithDefault(params, paramutil.ParamFormat, paramutil.FormatTable),
	}, nil
}
