package dep

import (
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func TestTruncateStr(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		maxLen int
		want   string
	}{
		{"shorter than max", "hello", 10, "hello"},
		{"equal to max", "hello", 5, "hello"},
		{"longer than max", "hello world", 8, "hello..."},
		{"very short max", "hello world", 3, "hel"},
		{"max zero", "hello", 0, "hello"},
		{"max one", "hello", 1, "h"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateStr(tt.s, tt.maxLen)
			if got != tt.want {
				t.Fatalf("truncateStr(%q, %d) = %q, want %q", tt.s, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestGetNodeAge(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name string
		age  time.Duration
		want string
	}{
		{"seconds", 30 * time.Second, "30s"},
		{"minutes", 5 * time.Minute, "5m"},
		{"hours", 3 * time.Hour, "3h"},
		{"days", 48 * time.Hour, "2d"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &unstructured.Unstructured{}
			u.SetCreationTimestamp(metav1.NewTime(now.Add(-tt.age)))
			node := &Node{Unstructured: u}
			got := getNodeAge(node)
			if got != tt.want {
				t.Fatalf("getNodeAge() = %q, want %q", got, tt.want)
			}
		})
	}

	t.Run("zero timestamp", func(t *testing.T) {
		u := &unstructured.Unstructured{}
		node := &Node{Unstructured: u}
		if got := getNodeAge(node); got != "-" {
			t.Fatalf("expected '-', got %q", got)
		}
	})
}

func TestGetNodeReady(t *testing.T) {
	t.Run("pod with containerStatuses", func(t *testing.T) {
		content := map[string]interface{}{
			"status": map[string]interface{}{
				"containerStatuses": []interface{}{
					map[string]interface{}{"ready": true},
					map[string]interface{}{"ready": true},
					map[string]interface{}{"ready": false},
				},
			},
		}
		u := &unstructured.Unstructured{}
		u.SetUnstructuredContent(content)
		node := &Node{Unstructured: u, Kind: "Pod"}
		if got := getNodeReady(node); got != "2/3" {
			t.Fatalf("expected '2/3', got %q", got)
		}
	})

	t.Run("deployment with replicas", func(t *testing.T) {
		content := map[string]interface{}{
			"status": map[string]interface{}{
				"replicas":      int64(3),
				"readyReplicas": int64(3),
			},
		}
		u := &unstructured.Unstructured{}
		u.SetUnstructuredContent(content)
		node := &Node{Unstructured: u, Kind: "Deployment"}
		if got := getNodeReady(node); got != "3/3" {
			t.Fatalf("expected '3/3', got %q", got)
		}
	})

	t.Run("ready condition fallback", func(t *testing.T) {
		content := map[string]interface{}{
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Ready",
						"status": "True",
					},
				},
			},
		}
		u := &unstructured.Unstructured{}
		u.SetUnstructuredContent(content)
		node := &Node{Unstructured: u, Kind: "Unknown"}
		if got := getNodeReady(node); got != "True" {
			t.Fatalf("expected 'True', got %q", got)
		}
	})

	t.Run("no status", func(t *testing.T) {
		u := &unstructured.Unstructured{}
		node := &Node{Unstructured: u, Kind: "Pod"}
		if got := getNodeReady(node); got != "-" {
			t.Fatalf("expected '-', got %q", got)
		}
	})
}

func TestGetNodeStatus(t *testing.T) {
	t.Run("pod phase", func(t *testing.T) {
		content := map[string]interface{}{
			"status": map[string]interface{}{
				"phase": "Running",
			},
		}
		u := &unstructured.Unstructured{}
		u.SetUnstructuredContent(content)
		node := &Node{Unstructured: u, Kind: "Pod"}
		if got := getNodeStatus(node); got != "Running" {
			t.Fatalf("expected 'Running', got %q", got)
		}
	})

	t.Run("node ready condition", func(t *testing.T) {
		content := map[string]interface{}{
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Ready",
						"reason": "KubeletReady",
					},
				},
			},
		}
		u := &unstructured.Unstructured{}
		u.SetUnstructuredContent(content)
		node := &Node{Unstructured: u, Kind: "Node"}
		if got := getNodeStatus(node); got != "KubeletReady" {
			t.Fatalf("expected 'KubeletReady', got %q", got)
		}
	})

	t.Run("unknown kind", func(t *testing.T) {
		u := &unstructured.Unstructured{}
		node := &Node{Unstructured: u, Kind: "ConfigMap"}
		if got := getNodeStatus(node); got != "" {
			t.Fatalf("expected empty, got %q", got)
		}
	})
}

func TestGetNestedInt64(t *testing.T) {
	t.Run("int64", func(t *testing.T) {
		obj := map[string]interface{}{"status": map[string]interface{}{"replicas": int64(5)}}
		if got := getNestedInt64(obj, "status", "replicas"); got != 5 {
			t.Fatalf("expected 5, got %d", got)
		}
	})

	t.Run("float64", func(t *testing.T) {
		obj := map[string]interface{}{"count": float64(10)}
		if got := getNestedInt64(obj, "count"); got != 10 {
			t.Fatalf("expected 10, got %d", got)
		}
	})

	t.Run("missing", func(t *testing.T) {
		obj := map[string]interface{}{}
		if got := getNestedInt64(obj, "missing"); got != 0 {
			t.Fatalf("expected 0, got %d", got)
		}
	})

	t.Run("nested path incomplete", func(t *testing.T) {
		obj := map[string]interface{}{"status": map[string]interface{}{}}
		if got := getNestedInt64(obj, "status", "replicas"); got != 0 {
			t.Fatalf("expected 0, got %d", got)
		}
	})
}

func TestSortedChildren(t *testing.T) {
	nodeMap := NodeMap{
		"uid-a":    {UID: "uid-a", Namespace: "ns-2", Kind: "Pod", Name: "pod-a"},
		"uid-b":    {UID: "uid-b", Namespace: "ns-1", Kind: "Pod", Name: "pod-b"},
		"uid-self": {UID: "uid-self", Namespace: "ns-1", Kind: "Pod", Name: "self"},
	}

	deps := map[types.UID]RelationshipSet{
		"uid-a":    {RelationshipPodNode: {}},
		"uid-b":    {RelationshipService: {}},
		"uid-self": {RelationshipControllerRef: {}},
	}

	children := sortedChildren(nodeMap, deps, "uid-self")
	if len(children) != 2 {
		t.Fatalf("expected 2 children (self excluded), got %d", len(children))
	}
	// Sorted by ns/kind/name: ns-1/pod-b, ns-2/pod-a
	if children[0].Name != "pod-b" {
		t.Errorf("expected pod-b first, got %s", children[0].Name)
	}
	if children[1].Name != "pod-a" {
		t.Errorf("expected pod-a second, got %s", children[1].Name)
	}
}

func TestFormatTree(t *testing.T) {
	t.Run("nil result", func(t *testing.T) {
		got := FormatTree(nil, true)
		if got != "No dependency data found" {
			t.Fatalf("expected 'No dependency data found', got %q", got)
		}
	})

	t.Run("empty node map", func(t *testing.T) {
		result := &Result{NodeMap: NodeMap{}}
		got := FormatTree(result, true)
		if got != "No dependency data found" {
			t.Fatalf("expected 'No dependency data found', got %q", got)
		}
	})

	t.Run("missing root", func(t *testing.T) {
		result := &Result{NodeMap: NodeMap{"uid-1": {}}, RootUID: "missing"}
		got := FormatTree(result, true)
		if got != "Root node not found" {
			t.Fatalf("expected 'Root node not found', got %q", got)
		}
	})

	t.Run("single node tree", func(t *testing.T) {
		u := &unstructured.Unstructured{}
		u.SetName("nginx")
		u.SetNamespace("default")
		u.SetGroupVersionKind(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})

		root := &Node{
			Unstructured: u,
			UID:          "root-uid",
			Kind:         "Deployment",
			Namespace:    "default",
			Name:         "nginx",
			Dependencies: map[types.UID]RelationshipSet{},
			Dependents:   map[types.UID]RelationshipSet{},
		}

		result := &Result{
			NodeMap: NodeMap{"root-uid": root},
			RootUID: "root-uid",
		}

		got := FormatTree(result, true)
		if got == "" || got == "No dependency data found" || got == "Root node not found" {
			t.Fatalf("expected tree output, got %q", got)
		}
		// Check header and node line present
		if !strings.Contains(got, "NAMESPACE") || !strings.Contains(got, "nginx") {
			t.Errorf("expected tree to contain 'NAMESPACE' and 'nginx', got: %s", got)
		}
	})
}

func TestFormatJSON(t *testing.T) {
	t.Run("nil result", func(t *testing.T) {
		got, err := FormatJSON(nil, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "[]" {
			t.Fatalf("expected '[]', got %q", got)
		}
	})

	t.Run("empty node map", func(t *testing.T) {
		result := &Result{NodeMap: NodeMap{}}
		got, err := FormatJSON(result, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "[]" {
			t.Fatalf("expected '[]', got %q", got)
		}
	})

	t.Run("single node JSON", func(t *testing.T) {
		u := &unstructured.Unstructured{}
		u.SetName("nginx")
		u.SetNamespace("default")
		u.SetGroupVersionKind(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})

		root := &Node{
			Unstructured: u,
			UID:          "root-uid",
			Kind:         "Deployment",
			Namespace:    "default",
			Name:         "nginx",
			Dependencies: map[types.UID]RelationshipSet{},
			Dependents:   map[types.UID]RelationshipSet{},
		}

		result := &Result{
			NodeMap: NodeMap{"root-uid": root},
			RootUID: "root-uid",
		}

		got, err := FormatJSON(result, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(got, `"kind"`) || !strings.Contains(got, `"nginx"`) {
			t.Errorf("expected JSON to contain kind and name, got: %s", got)
		}
	})
}
