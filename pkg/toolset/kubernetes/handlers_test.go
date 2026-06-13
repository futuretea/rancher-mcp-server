package kubernetes

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		maxLen int
		want   string
	}{
		{"shorter", "hello", 10, "hello"},
		{"equal", "hello", 5, "hello"},
		{"longer", "hello world", 8, "hello..."},
		{"maxLen less than 3", "hello", 2, "he"},
		{"maxLen 3 exactly", "hello world", 3, "hel"},
		{"maxLen zero returns full", "hello", 0, "hello"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.s, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.s, tt.maxLen, got, tt.want)
			}
		})
	}
}

func makeUnstructuredItem(name, namespace, kind string) unstructured.Unstructured {
	u := unstructured.Unstructured{}
	u.SetName(name)
	u.SetNamespace(namespace)
	u.SetGroupVersionKind(schema.GroupVersionKind{Group: "", Version: "v1", Kind: kind})
	return u
}

func TestFilterResourcesByName(t *testing.T) {
	list := &unstructured.UnstructuredList{
		Items: []unstructured.Unstructured{
			makeUnstructuredItem("nginx-deployment", "default", "Deployment"),
			makeUnstructuredItem("redis-deployment", "cache", "Deployment"),
			makeUnstructuredItem("nginx-service", "default", "Service"),
		},
	}

	t.Run("partial case-insensitive match", func(t *testing.T) {
		result := filterResourcesByName(list, "nginx")
		if len(result.Items) != 2 {
			t.Fatalf("expected 2 matches, got %d", len(result.Items))
		}
	})

	t.Run("exact match", func(t *testing.T) {
		result := filterResourcesByName(list, "redis-deployment")
		if len(result.Items) != 1 {
			t.Fatalf("expected 1 match, got %d", len(result.Items))
		}
	})

	t.Run("no match", func(t *testing.T) {
		result := filterResourcesByName(list, "nonexistent")
		if len(result.Items) != 0 {
			t.Fatalf("expected 0 matches, got %d", len(result.Items))
		}
	})
}

func TestPaginateResourceList(t *testing.T) {
	items := []unstructured.Unstructured{
		makeUnstructuredItem("a", "ns", "Pod"),
		makeUnstructuredItem("b", "ns", "Pod"),
		makeUnstructuredItem("c", "ns", "Pod"),
		makeUnstructuredItem("d", "ns", "Pod"),
		makeUnstructuredItem("e", "ns", "Pod"),
	}
	list := &unstructured.UnstructuredList{Items: items}

	t.Run("first page", func(t *testing.T) {
		result := paginateResourceList(list, 2, 1)
		if len(result.Items) != 2 || result.Items[0].GetName() != "a" {
			t.Fatalf("expected [a b], got %d items", len(result.Items))
		}
	})

	t.Run("last page partial", func(t *testing.T) {
		result := paginateResourceList(list, 2, 3)
		if len(result.Items) != 1 || result.Items[0].GetName() != "e" {
			t.Fatalf("expected [e], got %d items", len(result.Items))
		}
	})

	t.Run("page beyond range", func(t *testing.T) {
		result := paginateResourceList(list, 3, 10)
		if len(result.Items) != 0 {
			t.Fatalf("expected empty, got %d items", len(result.Items))
		}
	})

	t.Run("zero limit returns all", func(t *testing.T) {
		result := paginateResourceList(list, 0, 1)
		if len(result.Items) != 5 {
			t.Fatalf("expected all 5, got %d", len(result.Items))
		}
	})

	t.Run("zero page defaults to 1", func(t *testing.T) {
		result := paginateResourceList(list, 2, 0)
		if len(result.Items) != 2 {
			t.Fatalf("expected first page, got %d items", len(result.Items))
		}
	})
}

func TestFormatAsTable(t *testing.T) {
	t.Run("empty list", func(t *testing.T) {
		result := formatAsTable(&unstructured.UnstructuredList{})
		if result != "No resources found" {
			t.Fatalf("expected 'No resources found', got %q", result)
		}
	})

	t.Run("with items", func(t *testing.T) {
		list := &unstructured.UnstructuredList{
			Items: []unstructured.Unstructured{
				makeUnstructuredItem("nginx", "default", "Deployment"),
			},
		}
		result := formatAsTable(list)
		if result == "" || result == "No resources found" {
			t.Fatal("expected table output")
		}
		if !strings.Contains(result, "nginx") || !strings.Contains(result, "NAME") {
			t.Errorf("expected table with headers and data, got: %s", result)
		}
	})
}

func TestParseMaxFileSize(t *testing.T) {
	t.Run("default value", func(t *testing.T) {
		val, err := parseMaxFileSize(map[string]interface{}{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if val != 10*1024*1024 { // 10Mi
			t.Errorf("expected 10Mi (10485760), got %d", val)
		}
	})

	t.Run("custom valid size", func(t *testing.T) {
		val, err := parseMaxFileSize(map[string]interface{}{
			"maxFileSize": "1Mi",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if val != 1024*1024 {
			t.Errorf("expected 1Mi (1048576), got %d", val)
		}
	})

	t.Run("invalid size", func(t *testing.T) {
		_, err := parseMaxFileSize(map[string]interface{}{
			"maxFileSize": "not-a-size",
		})
		if err == nil {
			t.Fatal("expected error for invalid size")
		}
	})
}

