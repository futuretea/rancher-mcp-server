package kubernetes

import (
	"encoding/json"
	"fmt"

	"github.com/futuretea/rancher-mcp-server/pkg/toolset/handler"
	"github.com/futuretea/rancher-mcp-server/pkg/watchdiff"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

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
