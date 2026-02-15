package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/futuretea/rancher-mcp-server/pkg/client/steve"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset/handler"
	"github.com/futuretea/rancher-mcp-server/pkg/watchdiff"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// watchDiffHandler handles the kubernetes_watch_diff tool.
// It behaves similarly to the Linux `watch` command: it repeatedly
// evaluates the current state of matching resources at a configurable
// interval and returns the concatenated diffs from all iterations.
func watchDiffHandler(client interface{}, params map[string]interface{}) (string, error) {
	steveClient, err := toolset.ValidateSteveClient(client)
	if err != nil {
		return "", err
	}

	cluster, err := handler.ExtractRequiredString(params, handler.ParamCluster)
	if err != nil {
		return "", err
	}
	kind, err := handler.ExtractRequiredString(params, handler.ParamKind)
	if err != nil {
		return "", err
	}
	namespace := handler.ExtractOptionalString(params, handler.ParamNamespace)
	labelSelector := handler.ExtractOptionalString(params, handler.ParamLabelSelector)
	fieldSelector := handler.ExtractOptionalString(params, handler.ParamFieldSelector)

	ignoreStatus := handler.ExtractBool(params, "ignoreStatus", false)
	ignoreMeta := handler.ExtractBool(params, "ignoreMeta", false)

	intervalSeconds := handler.ExtractInt64(params, handler.ParamIntervalSeconds, 10)
	if intervalSeconds < 1 {
		intervalSeconds = 1
	}
	if intervalSeconds > 600 {
		intervalSeconds = 600
	}

	iterations := handler.ExtractInt64(params, handler.ParamIterations, 6)
	if iterations < 1 {
		iterations = 1
	}
	if iterations > 100 {
		iterations = 100
	}

	ctx := context.Background()

	differ := watchdiff.NewDiffer(true)
	differ.SetIgnoreStatus(ignoreStatus)
	differ.SetIgnoreMeta(ignoreMeta)

	var resultLines []string

	for i := int64(0); i < iterations; i++ {
		// List current resources for this iteration
		listOpts := &steve.ListOptions{
			LabelSelector: labelSelector,
			FieldSelector: fieldSelector,
		}
		list, err := steveClient.ListResources(ctx, cluster, kind, namespace, listOpts)
		if err != nil {
			return "", fmt.Errorf("failed to list resources: %w", err)
		}

		// Sort for deterministic output
		sort.Slice(list.Items, func(i, j int) bool {
			ai := list.Items[i]
			aj := list.Items[j]
			if ai.GetNamespace() != aj.GetNamespace() {
				return ai.GetNamespace() < aj.GetNamespace()
			}
			if ai.GetKind() != aj.GetKind() {
				return ai.GetKind() < aj.GetKind()
			}
			return ai.GetName() < aj.GetName()
		})

		iterationHeader := fmt.Sprintf("# iteration %d\n", i+1)
		iterationLines := []string{iterationHeader}

		for idx := range list.Items {
			obj := &list.Items[idx]
			diffText, err := differ.Diff(obj)
			if err != nil {
				return "", fmt.Errorf("failed to diff resource: %w", err)
			}
			if diffText != "" {
				iterationLines = append(iterationLines, diffText)
			}
		}

		// Only append iteration output if there was any diff beyond the header.
		if len(iterationLines) > 1 {
			resultLines = append(resultLines, strings.Join(iterationLines, "\n"))
		}

		// Sleep between iterations, except after the last one.
		if i+1 < iterations {
			time.Sleep(time.Duration(intervalSeconds) * time.Second)
		}
	}

	if len(resultLines) == 0 {
		return "No changes detected across iterations", nil
	}

	return strings.Join(resultLines, "\n"), nil
}

// diffHandler handles the kubernetes_diff tool.
// It compares two Kubernetes resource versions and shows the differences as a git-style diff.
func diffHandler(client interface{}, params map[string]interface{}) (string, error) {
	// Extract required parameters
	resource1JSON, err := handler.ExtractRequiredString(params, "resource1")
	if err != nil {
		return "", err
	}
	resource2JSON, err := handler.ExtractRequiredString(params, "resource2")
	if err != nil {
		return "", err
	}

	ignoreStatus := handler.ExtractBool(params, "ignoreStatus", false)
	ignoreMeta := handler.ExtractBool(params, "ignoreMeta", false)

	// Parse resource1
	var resource1 unstructured.Unstructured
	if err := json.Unmarshal([]byte(resource1JSON), &resource1.Object); err != nil {
		return "", fmt.Errorf("failed to parse resource1 JSON: %w", err)
	}

	// Parse resource2
	var resource2 unstructured.Unstructured
	if err := json.Unmarshal([]byte(resource2JSON), &resource2.Object); err != nil {
		return "", fmt.Errorf("failed to parse resource2 JSON: %w", err)
	}

	// Create a printer for diff output
	printer := watchdiff.NewPrinter(false)

	// Make copies for potential modifications
	oldCopy := resource1.DeepCopy()
	newCopy := resource2.DeepCopy()

	// Apply ignore options
	if ignoreStatus {
		delete(oldCopy.Object, "status")
		delete(newCopy.Object, "status")
	}

	if ignoreMeta {
		trimMetadataForDiff(oldCopy)
		trimMetadataForDiff(newCopy)
	}

	// Generate the diff
	diffText, err := printer.Diff(oldCopy, newCopy)
	if err != nil {
		return "", fmt.Errorf("failed to compute diff: %w", err)
	}

	if diffText == "" {
		return "No differences found between the two resource versions.", nil
	}

	return diffText, nil
}

// trimMetadataForDiff keeps only essential metadata fields for diff comparison.
func trimMetadataForDiff(obj *unstructured.Unstructured) {
	metaVal, ok := obj.Object["metadata"].(map[string]interface{})
	if !ok {
		return
	}
	// Keep only essential metadata fields
	cleanMeta := map[string]interface{}{
		"name":        metaVal["name"],
		"namespace":   metaVal["namespace"],
		"labels":      metaVal["labels"],
		"annotations": metaVal["annotations"],
	}
	obj.Object["metadata"] = cleanMeta
}
