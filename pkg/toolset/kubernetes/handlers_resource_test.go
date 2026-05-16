package kubernetes

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestExtractResourceKindWithAPIVersion(t *testing.T) {
	params := map[string]interface{}{
		"apiVersion": "catalog.cattle.io/v1",
		"kind":       "App",
	}

	got, err := extractResourceKind(params)
	if err != nil {
		t.Fatalf("extractResourceKind() unexpected error: %v", err)
	}
	if got != "catalog.cattle.io/v1/App" {
		t.Fatalf("extractResourceKind() = %q, want catalog.cattle.io/v1/App", got)
	}
}

func TestExtractResourceKindWithoutAPIVersion(t *testing.T) {
	params := map[string]interface{}{
		"kind": "deployment",
	}

	got, err := extractResourceKind(params)
	if err != nil {
		t.Fatalf("extractResourceKind() unexpected error: %v", err)
	}
	if got != "deployment" {
		t.Fatalf("extractResourceKind() = %q, want deployment", got)
	}
}

func TestFormatResource(t *testing.T) {
	u := &unstructured.Unstructured{}
	u.SetUnstructuredContent(map[string]interface{}{
		"metadata": map[string]interface{}{"name": "test-pod"},
		"kind":     "Pod",
	})

	t.Run("json", func(t *testing.T) {
		out, err := formatResource(u, "json", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out, "\"name\"") {
			t.Error("expected JSON output")
		}
	})

	t.Run("yaml", func(t *testing.T) {
		out, err := formatResource(u, "yaml", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out, "name:") {
			t.Error("expected YAML output")
		}
	})

	t.Run("defaults to json", func(t *testing.T) {
		out, err := formatResource(u, "", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out, "\"name\"") {
			t.Error("expected JSON output for default format")
		}
	})
}

func TestFormatResourceList(t *testing.T) {
	list := &unstructured.UnstructuredList{
		Items: []unstructured.Unstructured{
			newTestUnstructured("pod-1", "default", "Pod"),
			newTestUnstructured("pod-2", "kube-system", "Pod"),
		},
	}

	t.Run("json", func(t *testing.T) {
		out, err := formatResourceList(list, "json", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out, "\"name\"") {
			t.Error("expected JSON output")
		}
	})

	t.Run("table", func(t *testing.T) {
		out, err := formatResourceList(list, "table", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out, "NAME") {
			t.Error("expected table header")
		}
		if !strings.Contains(out, "pod-1") {
			t.Error("expected pod-1 in table")
		}
	})
}

func TestFormatAsTable_Empty(t *testing.T) {
	list := &unstructured.UnstructuredList{}
	out := formatAsTable(list)
	if out != "No resources found" {
		t.Errorf("expected 'No resources found', got %q", out)
	}
}

func newTestUnstructured(name, namespace, kind string) unstructured.Unstructured {
	u := unstructured.Unstructured{}
	u.SetUnstructuredContent(map[string]interface{}{
		"metadata": map[string]interface{}{"name": name, "namespace": namespace},
	})
	u.SetKind(kind)
	return u
}
