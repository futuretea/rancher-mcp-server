package watchdiff

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestGetKey(t *testing.T) {
	u := &unstructured.Unstructured{}
	u.SetAPIVersion("apps/v1")
	u.SetKind("Deployment")
	u.SetNamespace("default")
	u.SetName("nginx")

	key := getKey(u)
	want := "apps/v1/Deployment/default/nginx"
	if key != want {
		t.Fatalf("getKey() = %q, want %q", key, want)
	}
}

func TestNewEmptyObject(t *testing.T) {
	u := &unstructured.Unstructured{}
	u.SetAPIVersion("v1")
	u.SetKind("Pod")
	u.SetNamespace("kube-system")
	u.SetName("coredns")

	empty := newEmptyObject(u)
	if empty.GetAPIVersion() != "v1" {
		t.Errorf("expected apiVersion v1, got %q", empty.GetAPIVersion())
	}
	if empty.GetKind() != "Pod" {
		t.Errorf("expected Kind Pod, got %q", empty.GetKind())
	}
	if empty.GetName() != "coredns" {
		t.Errorf("expected name coredns, got %q", empty.GetName())
	}
	if empty.GetNamespace() != "kube-system" {
		t.Errorf("expected namespace kube-system, got %q", empty.GetNamespace())
	}
}

func TestAreObjectsEqual(t *testing.T) {
	t.Run("identical flat maps", func(t *testing.T) {
		a := map[string]interface{}{"name": "nginx", "replicas": int64(3)}
		b := map[string]interface{}{"name": "nginx", "replicas": int64(3)}
		if !areObjectsEqual(a, b) {
			t.Fatal("expected equal")
		}
	})

	t.Run("different values", func(t *testing.T) {
		a := map[string]interface{}{"name": "nginx"}
		b := map[string]interface{}{"name": "redis"}
		if areObjectsEqual(a, b) {
			t.Fatal("expected not equal")
		}
	})

	t.Run("different key count", func(t *testing.T) {
		a := map[string]interface{}{"name": "nginx", "replicas": int64(3)}
		b := map[string]interface{}{"name": "nginx"}
		if areObjectsEqual(a, b) {
			t.Fatal("expected not equal (different lengths)")
		}
	})

	t.Run("nested maps equal", func(t *testing.T) {
		a := map[string]interface{}{"spec": map[string]interface{}{"replicas": int64(3)}}
		b := map[string]interface{}{"spec": map[string]interface{}{"replicas": int64(3)}}
		if !areObjectsEqual(a, b) {
			t.Fatal("expected nested maps equal")
		}
	})

	t.Run("nested maps differ", func(t *testing.T) {
		a := map[string]interface{}{"spec": map[string]interface{}{"replicas": int64(3)}}
		b := map[string]interface{}{"spec": map[string]interface{}{"replicas": int64(5)}}
		if areObjectsEqual(a, b) {
			t.Fatal("expected nested maps not equal")
		}
	})

	t.Run("nested type mismatch", func(t *testing.T) {
		a := map[string]interface{}{"spec": map[string]interface{}{"replicas": int64(3)}}
		b := map[string]interface{}{"spec": "string-instead-of-map"}
		if areObjectsEqual(a, b) {
			t.Fatal("expected not equal (type mismatch)")
		}
	})
}

func TestAreSlicesEqual(t *testing.T) {
	t.Run("identical slices", func(t *testing.T) {
		a := []interface{}{"a", "b", "c"}
		b := []interface{}{"a", "b", "c"}
		if !areSlicesEqual(a, b) {
			t.Fatal("expected equal")
		}
	})

	t.Run("different lengths", func(t *testing.T) {
		a := []interface{}{"a", "b"}
		b := []interface{}{"a"}
		if areSlicesEqual(a, b) {
			t.Fatal("expected not equal")
		}
	})

	t.Run("different values", func(t *testing.T) {
		a := []interface{}{"a", "b"}
		b := []interface{}{"a", "c"}
		if areSlicesEqual(a, b) {
			t.Fatal("expected not equal")
		}
	})

	t.Run("nested maps in slices", func(t *testing.T) {
		a := []interface{}{map[string]interface{}{"name": "nginx"}}
		b := []interface{}{map[string]interface{}{"name": "nginx"}}
		if !areSlicesEqual(a, b) {
			t.Fatal("expected nested slices equal")
		}
	})

	t.Run("nested map type mismatch", func(t *testing.T) {
		a := []interface{}{map[string]interface{}{"name": "nginx"}}
		b := []interface{}{"not-a-map"}
		if areSlicesEqual(a, b) {
			t.Fatal("expected not equal (nested type mismatch)")
		}
	})

	t.Run("nested slices in slices", func(t *testing.T) {
		a := []interface{}{[]interface{}{"a", "b"}}
		b := []interface{}{[]interface{}{"a", "b"}}
		if !areSlicesEqual(a, b) {
			t.Fatal("expected nested slices equal")
		}
	})

	t.Run("nested slice type mismatch", func(t *testing.T) {
		a := []interface{}{[]interface{}{"a"}}
		b := []interface{}{"not-a-slice"}
		if areSlicesEqual(a, b) {
			t.Fatal("expected not equal (nested slice type mismatch)")
		}
	})
}

func TestAreObjectsEqual_AdditionalEdges(t *testing.T) {
	t.Run("nested slices in objects", func(t *testing.T) {
		a := map[string]interface{}{"items": []interface{}{"a", "b"}}
		b := map[string]interface{}{"items": []interface{}{"a", "b"}}
		if !areObjectsEqual(a, b) {
			t.Fatal("expected equal with nested slices")
		}
	})

	t.Run("object with slice type mismatch", func(t *testing.T) {
		a := map[string]interface{}{"items": []interface{}{"a"}}
		b := map[string]interface{}{"items": "not-a-slice"}
		if areObjectsEqual(a, b) {
			t.Fatal("expected not equal (object slice type mismatch)")
		}
	})
}

func TestDiffDeleteClearsCacheAndMarksDeletion(t *testing.T) {
	differ := NewDiffer(false)

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      "demo",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"replicas": int64(1),
			},
		},
	}

	if _, err := differ.Diff(obj); err != nil {
		t.Fatalf("Diff() returned unexpected error: %v", err)
	}

	deleted, err := differ.DiffDelete(obj)
	if err != nil {
		t.Fatalf("DiffDelete() returned unexpected error: %v", err)
	}
	if deleted == "" {
		t.Fatal("expected deletion diff output")
	}
	if !strings.Contains(deleted, "Deleted Resource") {
		t.Fatalf("expected explicit deleted marker, got %q", deleted)
	}

	deletedAgain, err := differ.DiffDelete(obj)
	if err != nil {
		t.Fatalf("second DiffDelete() returned unexpected error: %v", err)
	}
	if deletedAgain != "" {
		t.Fatalf("expected empty output after cache cleanup, got %q", deletedAgain)
	}
}
