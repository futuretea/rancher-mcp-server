package kubernetes

import (
	"context"
	"testing"

	"github.com/futuretea/rancher-mcp-server/pkg/client/steve"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset/paramutil"
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

func TestDiffHandler_DoesNotRequireClient(t *testing.T) {
	params := map[string]interface{}{
		"resource1": `{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"demo","namespace":"default"},"data":{"key":"a"}}`,
		"resource2": `{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"demo","namespace":"default"},"data":{"key":"b"}}`,
	}

	if _, err := diffHandler(context.Background(), nil, params); err != nil {
		t.Fatalf("diffHandler() returned unexpected error without client: %v", err)
	}
}

func TestResourceDiffHandler_CombinedClientNilSteve(t *testing.T) {
	params := map[string]interface{}{
		"kind": "deployment",
		"left": map[string]interface{}{
			"cluster": "c1",
			"name":    "left",
		},
		"right": map[string]interface{}{
			"cluster": "c1",
			"name":    "right",
		},
	}

	_, err := resourceDiffHandler(context.Background(), &toolset.CombinedClient{}, params)
	if err != paramutil.ErrSteveNotConfigured {
		t.Fatalf("resourceDiffHandler() error = %v, want %v", err, paramutil.ErrSteveNotConfigured)
	}
}

func TestResourceDiffHandler_AcceptsCombinedClientWithSteve(t *testing.T) {
	params := map[string]interface{}{
		"kind": "deployment",
		"left": map[string]interface{}{
			"cluster": "c1",
			"name":    "left",
		},
		"right": map[string]interface{}{
			"cluster": "c1",
			"name":    "right",
		},
	}

	combinedClient := &toolset.CombinedClient{
		Steve: steve.NewClient("https://example.com", "token", "", "", false),
	}

	_, err := resourceDiffHandler(context.Background(), combinedClient, params)
	if err == paramutil.ErrSteveNotConfigured {
		t.Fatal("resourceDiffHandler() should accept a CombinedClient with a Steve client")
	}
}

func TestExtractDiffTarget_UsesOptionalNamespaceSemantics(t *testing.T) {
	target, err := extractDiffTarget(map[string]interface{}{
		"left": map[string]interface{}{
			"cluster": "c1",
			"name":    "node-1",
		},
	}, "left")
	if err != nil {
		t.Fatalf("extractDiffTarget() returned unexpected error: %v", err)
	}
	if target.Namespace != "" {
		t.Fatalf("expected empty namespace for cluster-scoped lookups, got %q", target.Namespace)
	}
}
