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
	"github.com/futuretea/rancher-mcp-server/pkg/toolset/paramutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// RevisionInfo represents a single revision in the rollout history
type RevisionInfo struct {
	Revision    string `json:"revision"`
	ChangeCause string `json:"change_cause"`
	Created     string `json:"created"`
	Name        string `json:"name"`
}

// rolloutHistoryHandler handles the kubernetes_rollout_history tool
func rolloutHistoryHandler(ctx context.Context, client interface{}, params map[string]interface{}) (string, error) {
	steveClient, err := toolset.ValidateSteveClient(client)
	if err != nil {
		return "", err
	}

	cluster, namespace, name, err := extractRolloutParams(params)
	if err != nil {
		return "", err
	}
	format := paramutil.ExtractOptionalStringWithDefault(params, paramutil.ParamFormat, paramutil.FormatTable)

	deployment, err := steveClient.GetResource(ctx, cluster, "deployment", namespace, name)
	if err != nil {
		return "", fmt.Errorf("failed to get deployment: %w", err)
	}

	selector := buildDeploymentSelector(deployment)
	rsList, err := listReplicaSets(ctx, steveClient, cluster, namespace, selector)
	if err != nil {
		return "", fmt.Errorf("failed to list replicasets: %w", err)
	}

	history := extractRolloutHistory(rsList, name)
	sortRolloutHistory(history)

	return formatRolloutHistory(history, format)
}

// extractRolloutParams extracts the common parameters used by rolloutHistoryHandler.
func extractRolloutParams(params map[string]interface{}) (cluster, namespace, name string, err error) {
	cluster, err = paramutil.ExtractRequiredString(params, paramutil.ParamCluster)
	if err != nil {
		return "", "", "", err
	}
	namespace, err = paramutil.ExtractRequiredString(params, paramutil.ParamNamespace)
	if err != nil {
		return "", "", "", err
	}
	name, err = paramutil.ExtractRequiredString(params, paramutil.ParamName)
	if err != nil {
		return "", "", "", err
	}
	return cluster, namespace, name, nil
}

// buildDeploymentSelector extracts the matchLabels selector from a Deployment.
func buildDeploymentSelector(deployment *unstructured.Unstructured) map[string]string {
	selector, found, err := unstructured.NestedStringMap(deployment.Object, "spec", "selector", "matchLabels")
	if err != nil || !found {
		return nil
	}
	return selector
}

// listReplicaSets lists ReplicaSets for a Deployment using its selector.
func listReplicaSets(
	ctx context.Context,
	steveClient *steve.Client,
	cluster, namespace string,
	selector map[string]string,
) (*unstructured.UnstructuredList, error) {
	if len(selector) == 0 {
		return steveClient.ListResources(ctx, cluster, "replicaset", namespace, nil)
	}

	opts := &steve.ListOptions{
		LabelSelector: buildLabelSelector(selector),
	}
	return steveClient.ListResources(ctx, cluster, "replicaset", namespace, opts)
}

// buildLabelSelector converts a label map into a comma-separated selector string.
func buildLabelSelector(labels map[string]string) string {
	pairs := make([]string, 0, len(labels))
	for k, v := range labels {
		pairs = append(pairs, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(pairs, ",")
}

// extractRolloutHistory builds a rollout history from ReplicaSets owned by the Deployment.
func extractRolloutHistory(rsList *unstructured.UnstructuredList, deploymentName string) []RevisionInfo {
	history := make([]RevisionInfo, 0, len(rsList.Items))
	for _, rs := range rsList.Items {
		ownerRefs, found, _ := unstructured.NestedSlice(rs.Object, "metadata", "ownerReferences")
		if !found || !isOwnedByDeployment(ownerRefs, deploymentName) {
			continue
		}
		history = append(history, revisionInfoFromReplicaSet(rs))
	}
	return history
}

// isOwnedByDeployment checks whether the owner references include the target Deployment.
func isOwnedByDeployment(ownerRefs []interface{}, deploymentName string) bool {
	for _, ref := range ownerRefs {
		ownerRef, ok := ref.(map[string]interface{})
		if !ok {
			continue
		}
		kind, _ := ownerRef["kind"].(string)
		ownerName, _ := ownerRef["name"].(string)
		if kind == "Deployment" && ownerName == deploymentName {
			return true
		}
	}
	return false
}

// revisionInfoFromReplicaSet extracts revision metadata from a ReplicaSet.
func revisionInfoFromReplicaSet(rs unstructured.Unstructured) RevisionInfo {
	revision, _, _ := unstructured.NestedString(rs.Object, "metadata", "annotations", "deployment.kubernetes.io/revision")
	changeCause, _, _ := unstructured.NestedString(rs.Object, "metadata", "annotations", "deployment.kubernetes.io/change-cause")

	return RevisionInfo{
		Revision:    revision,
		ChangeCause: changeCause,
		Created:     rs.GetCreationTimestamp().Format("2006-01-02T15:04:05Z"),
		Name:        rs.GetName(),
	}
}

// sortRolloutHistory sorts revisions by revision number in descending order.
func sortRolloutHistory(history []RevisionInfo) {
	sort.Slice(history, func(i, j int) bool {
		revI, _ := strconv.Atoi(history[i].Revision)
		revJ, _ := strconv.Atoi(history[j].Revision)
		return revI > revJ
	})
}

// formatRolloutHistory formats rollout history as a table or JSON.
func formatRolloutHistory(history []RevisionInfo, format string) (string, error) {
	switch format {
	case paramutil.FormatTable:
		return formatRolloutHistoryAsTable(history), nil
	default: // json
		data, err := json.MarshalIndent(history, "", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to format as JSON: %w", err)
		}
		return string(data), nil
	}
}

// formatRolloutHistoryAsTable formats rollout history as a human-readable table
func formatRolloutHistoryAsTable(history []RevisionInfo) string {
	if len(history) == 0 {
		return "No rollout history found"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%-10s %-30s %-25s %s\n", "REVISION", "NAME", "CREATED", "CHANGE_CAUSE")
	fmt.Fprintf(&b, "%-10s %-30s %-25s %s\n", "--------", "----", "-------", "------------")

	for _, rev := range history {
		changeCause := rev.ChangeCause
		if changeCause == "" {
			changeCause = "-"
		}
		fmt.Fprintf(&b, "%-10s %-30s %-25s %s\n",
			rev.Revision,
			truncate(rev.Name, 30),
			truncate(rev.Created, 25),
			changeCause,
		)
	}

	return b.String()
}
