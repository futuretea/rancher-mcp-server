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
func rolloutHistoryHandler(client interface{}, params map[string]interface{}) (string, error) {
	steveClient, err := toolset.ValidateSteveClient(client)
	if err != nil {
		return "", err
	}

	cluster, err := handler.ExtractRequiredString(params, handler.ParamCluster)
	if err != nil {
		return "", err
	}
	namespace, err := handler.ExtractRequiredString(params, handler.ParamNamespace)
	if err != nil {
		return "", err
	}
	name, err := handler.ExtractRequiredString(params, handler.ParamName)
	if err != nil {
		return "", err
	}
	format := handler.ExtractOptionalStringWithDefault(params, handler.ParamFormat, handler.FormatTable)

	ctx := context.Background()

	// Get the Deployment
	deployment, err := steveClient.GetResource(ctx, cluster, "deployment", namespace, name)
	if err != nil {
		return "", fmt.Errorf("failed to get deployment: %w", err)
	}

	// Get ReplicaSets owned by this Deployment
	// Use label selector to find ReplicaSets with the deployment's selector
	selector, found, err := unstructured.NestedStringMap(deployment.Object, "spec", "selector", "matchLabels")
	if err != nil || !found || len(selector) == 0 {
		// Try to get the selector from the deployment's spec.selector
		selector = make(map[string]string)
	}

	// Build label selector string
	var labelSelectors []string
	for k, v := range selector {
		labelSelectors = append(labelSelectors, fmt.Sprintf("%s=%s", k, v))
	}

	var rsList *unstructured.UnstructuredList
	if len(labelSelectors) > 0 {
		rsList, err = steveClient.ListResources(ctx, cluster, "replicaset", namespace, &steve.ListOptions{
			LabelSelector: strings.Join(labelSelectors, ","),
		})
		if err != nil {
			return "", fmt.Errorf("failed to list replicasets: %w", err)
		}
	} else {
		// If no selector, list all replicasets in namespace and filter by owner
		rsList, err = steveClient.ListResources(ctx, cluster, "replicaset", namespace, nil)
		if err != nil {
			return "", fmt.Errorf("failed to list replicasets: %w", err)
		}
	}

	// Extract revision history from ReplicaSets
	var history []RevisionInfo
	for _, rs := range rsList.Items {
		// Check if this RS is owned by the deployment
		ownerRefs, found, _ := unstructured.NestedSlice(rs.Object, "metadata", "ownerReferences")
		if !found {
			continue
		}

		isOwned := false
		for _, ref := range ownerRefs {
			ownerRef, ok := ref.(map[string]interface{})
			if !ok {
				continue
			}
			kind, _ := ownerRef["kind"].(string)
			ownerName, _ := ownerRef["name"].(string)
			if kind == "Deployment" && ownerName == name {
				isOwned = true
				break
			}
		}

		if !isOwned {
			continue
		}

		// Extract revision number and change cause
		revision, _, _ := unstructured.NestedString(rs.Object, "metadata", "annotations", "deployment.kubernetes.io/revision")
		changeCause, _, _ := unstructured.NestedString(rs.Object, "metadata", "annotations", "deployment.kubernetes.io/change-cause")

		// Get creation timestamp
		created := rs.GetCreationTimestamp()

		history = append(history, RevisionInfo{
			Revision:    revision,
			ChangeCause: changeCause,
			Created:     created.Format("2006-01-02T15:04:05Z"),
			Name:        rs.GetName(),
		})
	}

	// Sort by revision number (descending)
	sort.Slice(history, func(i, j int) bool {
		revI, _ := strconv.Atoi(history[i].Revision)
		revJ, _ := strconv.Atoi(history[j].Revision)
		return revI > revJ
	})

	// Format output
	switch format {
	case handler.FormatTable:
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
