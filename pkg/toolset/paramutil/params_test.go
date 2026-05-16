package paramutil

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestExtractRequiredString(t *testing.T) {
	t.Run("valid string", func(t *testing.T) {
		v, err := ExtractRequiredString(map[string]interface{}{"cluster": "c-123"}, "cluster")
		if err != nil || v != "c-123" {
			t.Fatalf("expected 'c-123', got %q err=%v", v, err)
		}
	})

	t.Run("missing", func(t *testing.T) {
		_, err := ExtractRequiredString(map[string]interface{}{}, "cluster")
		if err == nil {
			t.Fatal("expected error for missing key")
		}
	})

	t.Run("empty string", func(t *testing.T) {
		_, err := ExtractRequiredString(map[string]interface{}{"cluster": ""}, "cluster")
		if err == nil {
			t.Fatal("expected error for empty value")
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		_, err := ExtractRequiredString(map[string]interface{}{"cluster": 42}, "cluster")
		if err == nil {
			t.Fatal("expected error for non-string")
		}
	})
}

func TestExtractOptionalString(t *testing.T) {
	params := map[string]interface{}{"name": "test", "count": 42}

	if v := ExtractOptionalString(params, "name"); v != "test" {
		t.Errorf("expected 'test', got %q", v)
	}
	if v := ExtractOptionalString(params, "missing"); v != "" {
		t.Errorf("expected empty for missing, got %q", v)
	}
	if v := ExtractOptionalString(params, "count"); v != "" {
		t.Errorf("expected empty for non-string, got %q", v)
	}
}

func TestExtractOptionalStringWithDefault(t *testing.T) {
	params := map[string]interface{}{"name": "test", "empty": ""}

	if v := ExtractOptionalStringWithDefault(params, "name", "fallback"); v != "test" {
		t.Errorf("expected 'test', got %q", v)
	}
	if v := ExtractOptionalStringWithDefault(params, "missing", "fallback"); v != "fallback" {
		t.Errorf("expected 'fallback', got %q", v)
	}
	if v := ExtractOptionalStringWithDefault(params, "empty", "fallback"); v != "fallback" {
		t.Errorf("expected 'fallback' for empty, got %q", v)
	}
}

func TestExtractBool(t *testing.T) {
	params := map[string]interface{}{"enabled": true, "disabled": false, "notBool": "true"}

	if v := ExtractBool(params, "enabled", false); !v {
		t.Error("expected true for 'enabled'")
	}
	if v := ExtractBool(params, "disabled", true); v {
		t.Error("expected false for 'disabled'")
	}
	if v := ExtractBool(params, "missing", true); !v {
		t.Error("expected default true for missing")
	}
	if v := ExtractBool(params, "notBool", true); !v {
		t.Error("expected default true for non-bool")
	}
}

func TestExtractFormat(t *testing.T) {
	if v := ExtractFormat(map[string]interface{}{}); v != FormatJSON {
		t.Errorf("default should be json, got %q", v)
	}
	if v := ExtractFormat(map[string]interface{}{"format": "yaml"}); v != "yaml" {
		t.Errorf("expected 'yaml', got %q", v)
	}
}

func TestValidateFormat(t *testing.T) {
	for _, f := range []string{"json", "yaml", "table"} {
		if err := ValidateFormat(f); err != nil {
			t.Errorf("expected valid format %q, got error: %v", f, err)
		}
	}
	if err := ValidateFormat("xml"); err == nil {
		t.Error("expected error for invalid format 'xml'")
	}
}

func TestGetStringValue(t *testing.T) {
	if v := GetStringValue(nil); v != "-" {
		t.Errorf("expected '-' for nil, got %q", v)
	}
	if v := GetStringValue("hello"); v != "hello" {
		t.Errorf("expected 'hello', got %q", v)
	}
	if v := GetStringValue(42); v != "42" {
		t.Errorf("expected '42', got %q", v)
	}
}

func TestFormatTime(t *testing.T) {
	if v := FormatTime(""); v != "-" {
		t.Errorf("expected '-' for empty, got %q", v)
	}
	if v := FormatTime("2024-01-15T10:30:00Z"); v != "2024-01-15T10:30:00Z" {
		t.Errorf("expected passthrough, got %q", v)
	}
}

func TestBoolPtr(t *testing.T) {
	ptr := BoolPtr(true)
	if ptr == nil || !*ptr {
		t.Error("expected pointer to true")
	}
	ptr2 := BoolPtr(false)
	if ptr2 == nil || *ptr2 {
		t.Error("expected pointer to false")
	}
}

func TestApplyPagination(t *testing.T) {
	items := []string{"a", "b", "c", "d", "e"}

	t.Run("first page", func(t *testing.T) {
		result, total := ApplyPagination(items, 2, 1)
		if total != 5 || len(result) != 2 || result[0] != "a" || result[1] != "b" {
			t.Fatalf("got %v (total=%d), want [a b] (total=5)", result, total)
		}
	})

	t.Run("last page partial", func(t *testing.T) {
		result, total := ApplyPagination(items, 2, 3)
		if total != 5 || len(result) != 1 || result[0] != "e" {
			t.Fatalf("got %v (total=%d), want [e] (total=5)", result, total)
		}
	})

	t.Run("page beyond range", func(t *testing.T) {
		result, total := ApplyPagination(items, 3, 10)
		if total != 5 || len(result) != 0 {
			t.Fatalf("expected empty, got %v (total=%d)", result, total)
		}
	})

	t.Run("zero limit returns all", func(t *testing.T) {
		result, total := ApplyPagination(items, 0, 1)
		if total != 5 || len(result) != 5 {
			t.Fatalf("expected all items, got %v (total=%d)", result, total)
		}
	})

	t.Run("zero page defaults to 1", func(t *testing.T) {
		result, total := ApplyPagination(items, 2, 0)
		if total != 5 || len(result) != 2 || result[0] != "a" {
			t.Fatalf("got %v (total=%d), want [a b]", result, total)
		}
	})

	t.Run("empty items", func(t *testing.T) {
		result, total := ApplyPagination([]string{}, 2, 1)
		if total != 0 || len(result) != 0 {
			t.Fatalf("expected empty, got %v (total=%d)", result, total)
		}
	})
}

func TestExtractInt64(t *testing.T) {
	params := map[string]interface{}{
		"asFloat": float64(100),
		"asInt64": int64(200),
		"asInt":   300,
	}

	if v := ExtractInt64(params, "asFloat", 0); v != 100 {
		t.Errorf("expected 100, got %d", v)
	}
	if v := ExtractInt64(params, "asInt64", 0); v != 200 {
		t.Errorf("expected 200, got %d", v)
	}
	if v := ExtractInt64(params, "asInt", 0); v != 300 {
		t.Errorf("expected 300, got %d", v)
	}
	if v := ExtractInt64(params, "missing", 42); v != 42 {
		t.Errorf("expected default 42, got %d", v)
	}
}

func TestFormatOutput(t *testing.T) {
	data := []map[string]string{
		{"name": "nginx", "namespace": "default"},
	}

	t.Run("json format", func(t *testing.T) {
		got, err := FormatOutput(data, "json", []string{"name", "namespace"}, nil)
		if err != nil || got == "" {
			t.Fatalf("expected JSON output, got err=%v, result=%q", err, got)
		}
	})

	t.Run("table format", func(t *testing.T) {
		got, err := FormatOutput(data, "table", []string{"name", "namespace"}, nil)
		if err != nil || got == "" {
			t.Fatalf("expected table output, got err=%v, result=%q", err, got)
		}
	})

	t.Run("empty data with json", func(t *testing.T) {
		got, err := FormatOutput([]map[string]string{}, "json", nil, nil)
		if err != nil || got == "" {
			t.Fatalf("expected JSON empty array, got err=%v, result=%q", err, got)
		}
	})

	t.Run("invalid format", func(t *testing.T) {
		_, err := FormatOutput(data, "xml", nil, nil)
		if err == nil {
			t.Fatal("expected error for invalid format")
		}
	})
}

func TestFormatSingleResult(t *testing.T) {
	data := map[string]interface{}{"name": "nginx", "namespace": "default"}

	t.Run("json format", func(t *testing.T) {
		got, err := FormatSingleResult(data, "json")
		if err != nil || got == "" {
			t.Fatalf("expected JSON output, got err=%v, result=%q", err, got)
		}
	})

	t.Run("table format requires headers", func(t *testing.T) {
		_, err := FormatSingleResult(data, "table")
		if err == nil {
			t.Fatal("expected error for table without headers")
		}
	})

	t.Run("table format with headers", func(t *testing.T) {
		got, err := FormatSingleResult(data, "table", "name", "namespace")
		if err != nil || got == "" {
			t.Fatalf("expected table output, got err=%v, result=%q", err, got)
		}
	})
}

func TestExtractOptionalInt64(t *testing.T) {
	params := map[string]interface{}{
		"asFloat": float64(42),
		"asInt64": int64(100),
		"asInt":   200,
	}

	if v := ExtractOptionalInt64(params, "asFloat"); v == nil || *v != 42 {
		t.Errorf("expected 42 from float64, got %v", v)
	}
	if v := ExtractOptionalInt64(params, "asInt64"); v == nil || *v != 100 {
		t.Errorf("expected 100 from int64, got %v", v)
	}
	if v := ExtractOptionalInt64(params, "asInt"); v == nil || *v != 200 {
		t.Errorf("expected 200 from int, got %v", v)
	}
	if v := ExtractOptionalInt64(params, "missing"); v != nil {
		t.Errorf("expected nil for missing, got %v", v)
	}
	if v := ExtractOptionalInt64(params, "notANumber"); v != nil {
		t.Errorf("expected nil for non-number, got %v", v)
	}
}

func TestExtractAndValidateFormat(t *testing.T) {
	v, err := ExtractAndValidateFormat(map[string]interface{}{})
	if err != nil || v != FormatJSON {
		t.Fatalf("expected json by default, got %q err=%v", v, err)
	}

	v, err = ExtractAndValidateFormat(map[string]interface{}{"format": "yaml"})
	if err != nil || v != "yaml" {
		t.Fatalf("expected yaml, got %q err=%v", v, err)
	}

	_, err = ExtractAndValidateFormat(map[string]interface{}{"format": "xml"})
	if err == nil {
		t.Fatal("expected error for invalid format")
	}
}

func TestFormatAsYAML(t *testing.T) {
	data := map[string]string{"name": "test", "namespace": "default"}
	out, err := FormatAsYAML(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "name:") || !strings.Contains(out, "test") {
		t.Errorf("expected YAML output, got %q", out)
	}
}

func TestParsePath(t *testing.T) {
	tests := []struct {
		path string
		want []string
	}{
		{"", nil},
		{"metadata.name", []string{"metadata", "name"}},
		{"metadata.annotations.app", []string{"metadata", "annotations", "app"}},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := parsePath(tt.path)
			if len(got) != len(tt.want) {
				t.Fatalf("parsePath(%q) = %v, want %v", tt.path, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("parsePath(%q)[%d] = %q, want %q", tt.path, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestNewResourceFilterFromParams(t *testing.T) {
	t.Run("no filter param returns nil", func(t *testing.T) {
		if f := NewResourceFilterFromParams(map[string]interface{}{}); f != nil {
			t.Fatal("expected nil when no filter param")
		}
	})

	t.Run("empty string slice returns nil", func(t *testing.T) {
		if f := NewResourceFilterFromParams(map[string]interface{}{"outputFilters": []string{}}); f != nil {
			t.Fatal("expected nil for empty string slice")
		}
	})

	t.Run("string slice", func(t *testing.T) {
		f := NewResourceFilterFromParams(map[string]interface{}{"outputFilters": []string{"metadata.managedFields"}})
		if f == nil {
			t.Fatal("expected non-nil filter")
		}
	})

	t.Run("interface slice", func(t *testing.T) {
		f := NewResourceFilterFromParams(map[string]interface{}{"outputFilters": []interface{}{"metadata.managedFields"}})
		if f == nil {
			t.Fatal("expected non-nil filter from []interface{}")
		}
	})

	t.Run("empty interface slice returns nil", func(t *testing.T) {
		if f := NewResourceFilterFromParams(map[string]interface{}{"outputFilters": []interface{}{}}); f != nil {
			t.Fatal("expected nil for empty interface slice")
		}
	})

	t.Run("unsupported type returns nil", func(t *testing.T) {
		if f := NewResourceFilterFromParams(map[string]interface{}{"outputFilters": 42}); f != nil {
			t.Fatal("expected nil for unsupported type")
		}
	})
}

func TestResourceFilter_Filter(t *testing.T) {
	t.Run("nil object returns nil", func(t *testing.T) {
		f := NewResourceFilter([]string{"metadata.managedFields"})
		if out := f.Filter(nil); out != nil {
			t.Fatal("expected nil from nil input")
		}
	})

	t.Run("empty paths returns same object", func(t *testing.T) {
		f := NewResourceFilter(nil)
		u := &unstructured.Unstructured{}
		u.SetUnstructuredContent(map[string]interface{}{"metadata": map[string]interface{}{"name": "test"}})
		out := f.Filter(u)
		if out != u {
			t.Fatal("expected same object when no paths")
		}
	})

	t.Run("removes managed fields", func(t *testing.T) {
		f := NewResourceFilter([]string{"metadata.managedFields"})
		u := &unstructured.Unstructured{}
		u.SetUnstructuredContent(map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":          "test-pod",
				"managedFields": []interface{}{"field-data"},
			},
		})
		out := f.Filter(u)
		_, found, _ := unstructured.NestedSlice(out.Object, "metadata", "managedFields")
		if found {
			t.Fatal("expected managedFields to be removed")
		}
		if name, _, _ := unstructured.NestedString(out.Object, "metadata", "name"); name != "test-pod" {
			t.Errorf("expected name 'test-pod', got %q", name)
		}
	})

	t.Run("removes nested annotation", func(t *testing.T) {
		f := NewResourceFilter([]string{"metadata.annotations.kubectl.kubernetes.io/last-applied-configuration"})
		u := &unstructured.Unstructured{}
		u.SetUnstructuredContent(map[string]interface{}{
			"metadata": map[string]interface{}{
				"name": "test-pod",
				"annotations": map[string]interface{}{
					"other": "value",
					"kubectl.kubernetes.io/last-applied-configuration": "big-json",
				},
			},
		})
		out := f.Filter(u)
		ann, _, _ := unstructured.NestedMap(out.Object, "metadata", "annotations")
		if _, exists := ann["kubectl.kubernetes.io/last-applied-configuration"]; exists {
			t.Fatal("expected annotation to be removed")
		}
		if ann["other"] != "value" {
			t.Error("expected other annotation to remain")
		}
	})
}

func TestResourceFilter_FilterList(t *testing.T) {
	t.Run("nil list returns nil", func(t *testing.T) {
		f := NewResourceFilter([]string{"metadata.managedFields"})
		if out := f.FilterList(nil); out != nil {
			t.Fatal("expected nil from nil input")
		}
	})

	t.Run("filters each item", func(t *testing.T) {
		f := NewResourceFilter([]string{"metadata.managedFields"})
		u1 := unstructured.Unstructured{Object: map[string]interface{}{
			"metadata": map[string]interface{}{"name": "pod-1", "managedFields": "data"},
		}}
		u2 := unstructured.Unstructured{Object: map[string]interface{}{
			"metadata": map[string]interface{}{"name": "pod-2", "managedFields": "data"},
		}}
		list := &unstructured.UnstructuredList{Items: []unstructured.Unstructured{u1, u2}}
		out := f.FilterList(list)
		if len(out.Items) != 2 {
			t.Fatalf("expected 2 items, got %d", len(out.Items))
		}
		for _, item := range out.Items {
			if _, found, _ := unstructured.NestedString(item.Object, "metadata", "managedFields"); found {
				t.Errorf("expected managedFields removed from %s", item.GetName())
			}
		}
	})
}

func TestDefaultFilterPaths(t *testing.T) {
	paths := DefaultFilterPaths()
	if len(paths) != 2 {
		t.Fatalf("expected 2 default paths, got %d", len(paths))
	}
	if paths[0] != "metadata.managedFields" {
		t.Errorf("unexpected first path: %s", paths[0])
	}
}

func TestFilterFields(t *testing.T) {
	data := []map[string]string{
		{"name": "nginx", "namespace": "default", "status": "Running"},
		{"name": "redis", "namespace": "cache", "status": "Running"},
	}

	t.Run("nil fields returns all", func(t *testing.T) {
		result := FilterFields(data, nil)
		if len(result) != 2 || len(result[0]) != 3 {
			t.Fatalf("expected 2 rows with 3 cols, got %d rows", len(result))
		}
	})

	t.Run("subset fields", func(t *testing.T) {
		result := FilterFields(data, []string{"name", "namespace"})
		if len(result[0]) != 2 || result[0]["name"] != "nginx" {
			t.Fatalf("expected 2 fields, got %v", result[0])
		}
	})

	t.Run("missing field gets empty string", func(t *testing.T) {
		result := FilterFields(data, []string{"name", "nonexistent"})
		if result[0]["nonexistent"] != "" {
			t.Fatalf("expected empty string for missing field, got %q", result[0]["nonexistent"])
		}
	})
}
