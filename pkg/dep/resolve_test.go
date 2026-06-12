package dep

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/futuretea/rancher-mcp-server/pkg/client/steve"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

func TestResolve_RejectsBudgetOverflow(t *testing.T) {
	reader := newResolveTestReader(newResolveTestObject("apps/v1", "Deployment", "default", "demo", "root-uid"))
	overflowList := &unstructured.UnstructuredList{}
	overflowList.Items = []unstructured.Unstructured{
		newResolveTestObject("v1", "Pod", "default", "pod-a", "pod-a"),
		newResolveTestObject("v1", "Pod", "default", "pod-b", "pod-b"),
	}
	reader.listResponses["pod"] = overflowList

	_, err := Resolve(context.Background(), reader, "c1", "deployment", "default", "demo", ResolveOptions{
		Direction:         "dependents",
		MaxDepth:          10,
		ScanNamespace:     "default",
		MaxScannedObjects: 1,
	})
	if err == nil || !strings.Contains(err.Error(), "budget exceeded") {
		t.Fatalf("expected budget exceeded error, got %v", err)
	}
}

func TestResolve_UsesScanNamespaceForClusterScopedRoot(t *testing.T) {
	reader := newResolveTestReader(newResolveTestObject("v1", "Node", "", "node-1", "node-uid"))

	result, err := Resolve(context.Background(), reader, "c1", "node", "", "node-1", ResolveOptions{
		Direction:     "dependents",
		MaxDepth:      10,
		ScanNamespace: "team-a",
	})
	if err != nil {
		t.Fatalf("Resolve() returned unexpected error: %v", err)
	}
	if result.RootUID != types.UID("node-uid") {
		t.Fatalf("unexpected root UID %q", result.RootUID)
	}
	if got := reader.requestedNamespaces["pod"]; len(got) == 0 || got[0] != "team-a" {
		t.Fatalf("expected pod scan namespace team-a, got %v", got)
	}
	if got := reader.requestedNamespaces["node"]; len(got) == 0 || got[0] != "" {
		t.Fatalf("expected cluster-scoped node scan namespace to stay empty, got %v", got)
	}
}

func TestResolve_InvalidDirection(t *testing.T) {
	reader := newResolveTestReader(newResolveTestObject("v1", "Pod", "default", "demo", "root-uid"))

	_, err := Resolve(context.Background(), reader, "c1", "pod", "default", "demo", ResolveOptions{
		Direction: "sideways",
	})
	if err == nil || !strings.Contains(err.Error(), "invalid direction") {
		t.Fatalf("expected invalid direction error, got %v", err)
	}
}

func TestResolve_DefaultsToDependents(t *testing.T) {
	reader := newResolveTestReader(newResolveTestObject("v1", "Pod", "default", "demo", "root-uid"))

	result, err := Resolve(context.Background(), reader, "c1", "pod", "default", "demo", ResolveOptions{
		Direction: "",
	})
	if err != nil {
		t.Fatalf("Resolve() returned unexpected error: %v", err)
	}
	if result.RootUID != types.UID("root-uid") {
		t.Fatalf("unexpected root UID %q", result.RootUID)
	}
}

type resolveTestReader struct {
	root                *unstructured.Unstructured
	listResponses       map[string]*unstructured.UnstructuredList
	requestedNamespaces map[string][]string
	mu                  sync.Mutex
}

func newResolveTestReader(root unstructured.Unstructured) *resolveTestReader {
	return &resolveTestReader{
		root:                root.DeepCopy(),
		listResponses:       make(map[string]*unstructured.UnstructuredList),
		requestedNamespaces: make(map[string][]string),
	}
}

func (r *resolveTestReader) GetResource(context.Context, string, string, string, string) (*unstructured.Unstructured, error) {
	return r.root.DeepCopy(), nil
}

func (r *resolveTestReader) ListResources(_ context.Context, _ string, kind, namespace string, _ *steve.ListOptions) (*unstructured.UnstructuredList, error) {
	r.mu.Lock()
	r.requestedNamespaces[kind] = append(r.requestedNamespaces[kind], namespace)
	r.mu.Unlock()
	if list, ok := r.listResponses[kind]; ok {
		return list.DeepCopy(), nil
	}
	return &unstructured.UnstructuredList{}, nil
}

func (r *resolveTestReader) GetEvents(context.Context, string, string, string, string) ([]corev1.Event, error) {
	return nil, nil
}

func newResolveTestObject(apiVersion, kind, namespace, name string, uid types.UID) unstructured.Unstructured {
	return unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": apiVersion,
			"kind":       kind,
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
				"uid":       string(uid),
			},
		},
	}
}
