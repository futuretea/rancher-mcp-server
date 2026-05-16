package kubernetes

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestTrimMetadataForDiff(t *testing.T) {
	t.Run("keeps essential fields only", func(t *testing.T) {
		u := &unstructured.Unstructured{}
		u.SetUnstructuredContent(map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":              "nginx",
				"namespace":         "default",
				"labels":            map[string]interface{}{"app": "nginx"},
				"annotations":       map[string]interface{}{"kubernetes.io/change-cause": "scale up"},
				"uid":               "abc-123",
				"resourceVersion":   "456",
				"creationTimestamp": "2024-01-01T00:00:00Z",
				"managedFields":     []interface{}{},
			},
			"spec": map[string]interface{}{
				"replicas": int64(3),
			},
		})

		trimMetadataForDiff(u)

		meta := u.Object["metadata"].(map[string]interface{})
		if meta["name"] != "nginx" {
			t.Error("name should be preserved")
		}
		if meta["namespace"] != "default" {
			t.Error("namespace should be preserved")
		}
		if meta["labels"] == nil {
			t.Error("labels should be preserved")
		}
		if meta["annotations"] == nil {
			t.Error("annotations should be preserved")
		}
		// Non-essential fields should be removed
		if _, ok := meta["uid"]; ok {
			t.Error("uid should be removed")
		}
		if _, ok := meta["resourceVersion"]; ok {
			t.Error("resourceVersion should be removed")
		}
		if _, ok := meta["creationTimestamp"]; ok {
			t.Error("creationTimestamp should be removed")
		}
		if _, ok := meta["managedFields"]; ok {
			t.Error("managedFields should be removed")
		}
		// spec should be untouched
		if u.Object["spec"] == nil {
			t.Error("spec should be preserved")
		}
	})

	t.Run("no metadata field", func(_ *testing.T) {
		u := &unstructured.Unstructured{}
		u.SetUnstructuredContent(map[string]interface{}{
			"kind": "Pod",
		})
		// Should not panic
		trimMetadataForDiff(u)
	})

	t.Run("non-map metadata", func(_ *testing.T) {
		u := &unstructured.Unstructured{}
		u.SetUnstructuredContent(map[string]interface{}{
			"metadata": "not-a-map",
		})
		// Should not panic
		trimMetadataForDiff(u)
	})
}
