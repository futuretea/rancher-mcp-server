package kubernetes

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/futuretea/rancher-mcp-server/pkg/client/steve"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

func TestAllowlistURLReferences(t *testing.T) {
	t.Cleanup(resetNamespaceAllowlist)
	SetNamespaceAllowlist(map[string][]string{"c-abc12": {"app"}})
	for _, reference := range []string{"c-abc12", "c-abc12/", "c-%61bc12", "c-abc12?x=y", "c-abc12#fragment", "c-abc12//"} {
		t.Run(reference, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"apiVersion":"v1","kind":"PodList","items":[]}`))
			}))
			defer server.Close()
			client := steve.NewClient(server.URL, "", "", "", false)
			_, err := listHandler(context.Background(), client, map[string]interface{}{
				"cluster": reference, "kind": "pod", "namespace": "kube-system", "format": "json",
			})
			if err == nil || calls != 0 {
				t.Fatalf("restricted query: error=%v backend calls=%d; want rejection before HTTP", err, calls)
			}
		})
	}
}

type shortPageReader struct{ *recordingReader }

func (r *shortPageReader) ListResources(ctx context.Context, cluster, kind, namespace string, opts *steve.ListOptions) (*unstructured.UnstructuredList, error) {
	list, err := r.recordingReader.ListResources(ctx, cluster, kind, namespace, opts)
	if err == nil && namespace == "app" {
		list.SetContinue("next-page")
	}
	return list, err
}

func TestAllowlistWatchRejectsShortPage(t *testing.T) {
	t.Cleanup(resetNamespaceAllowlist)
	SetNamespaceAllowlist(map[string][]string{"c-abc12": {"app", "other"}})
	reader := &shortPageReader{newRecordingReader()}
	reader.inner.AddResource(podObject("app", "web"))
	_, err := watchDiffWithReader(context.Background(), reader, &watchRequest{
		cluster: "c-abc12", kind: "pod", iterations: 1, maxItems: 10, maxOutputBytes: 10000,
	})
	if err == nil || !strings.Contains(err.Error(), "limit") {
		t.Fatalf("watch error=%v; want incomplete snapshot rejection", err)
	}
}

func TestAllowlistDependencyTotalBudget(t *testing.T) {
	t.Cleanup(resetNamespaceAllowlist)
	SetNamespaceAllowlist(map[string][]string{"c-abc12": {"app", "other"}})
	for _, budget := range []int{2, 3, 0} {
		t.Run(strconv.Itoa(budget), func(t *testing.T) {
			reader := newRecordingReader()
			node := nodeObject("worker-1")
			node.SetUID("node-1")
			reader.inner.AddResource(node)
			for _, ns := range []string{"app", "other"} {
				pod := podObject(ns, "web")
				pod.SetUID(types.UID("pod-" + ns))
				pod.Object["spec"] = map[string]interface{}{"nodeName": "worker-1"}
				reader.inner.AddResource(pod)
			}
			out, err := depHandler(context.Background(), reader, map[string]interface{}{
				"cluster": "c-abc12", "kind": "node", "name": "worker-1", "maxScannedObjects": budget, "format": "json",
			})
			if budget == 2 {
				if err == nil || !strings.Contains(err.Error(), "budget exceeded") {
					t.Fatalf("error=%v; want total budget rejection", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out, "app") || !strings.Contains(out, "other") {
				t.Fatalf("missing dependents: %s", out)
			}
			nodeLists := 0
			for _, call := range reader.calls {
				if call.op == "list" && call.kind == "node" {
					nodeLists++
				}
			}
			if nodeLists != 1 {
				t.Fatalf("node lists=%d; want one cluster scan", nodeLists)
			}
			reader.assertNoEmptyNamespacedList(t)
		})
	}
}

type limitRecordingReader struct {
	*recordingReader
	limits []int64
}

func (r *limitRecordingReader) ListResources(ctx context.Context, cluster, kind, namespace string, opts *steve.ListOptions) (*unstructured.UnstructuredList, error) {
	r.limits = append(r.limits, opts.Limit)
	return r.recordingReader.ListResources(ctx, cluster, kind, namespace, opts)
}

func TestAllowlistListSharesLimit(t *testing.T) {
	t.Cleanup(resetNamespaceAllowlist)
	SetNamespaceAllowlist(map[string][]string{"c-abc12": {"app", "other"}})
	for _, limit := range []int64{1, 2, 3} {
		reader := &limitRecordingReader{recordingReader: newRecordingReader()}
		reader.inner.AddResource(podObject("app", "one"))
		reader.inner.AddResource(podObject("other", "two"))
		opts := &steve.ListOptions{Limit: limit}
		list, err := listResourcesAllowed(context.Background(), reader, "c-abc12", "pod", "", opts)
		if err != nil {
			t.Fatal(err)
		}
		if opts.Limit != limit {
			t.Fatal("caller options mutated")
		}
		if limit == 1 {
			if len(list.Items) != 1 || list.GetContinue() == "" || len(reader.limits) != 1 {
				t.Fatalf("limit 1: list=%v limits=%v", list, reader.limits)
			}
		} else if len(list.Items) != 2 || list.GetContinue() != "" || len(reader.limits) != 2 || reader.limits[1] != limit-1 {
			t.Fatalf("limit %d: list=%v limits=%v", limit, list, reader.limits)
		}
	}
}

type namespaceErrorReader struct {
	*recordingReader
	failedNamespace string
}

func (r *namespaceErrorReader) ListResources(ctx context.Context, cluster, kind, namespace string, opts *steve.ListOptions) (*unstructured.UnstructuredList, error) {
	if kind == "pod" && namespace == r.failedNamespace {
		return nil, fmt.Errorf("namespace list forbidden")
	}
	return r.recordingReader.ListResources(ctx, cluster, kind, namespace, opts)
}

func TestAllowlistDependencyNamespaceError(t *testing.T) {
	t.Cleanup(resetNamespaceAllowlist)
	SetNamespaceAllowlist(map[string][]string{"c-abc12": {"app", "other"}})
	for _, failed := range []string{"app", "other"} {
		for _, budget := range []int{0, 2, 3} {
			t.Run(failed+"/"+strconv.Itoa(budget), func(t *testing.T) {
				visible := "app"
				if failed == visible {
					visible = "other"
				}
				reader := &namespaceErrorReader{newRecordingReader(), failed}
				node := nodeObject("worker-1")
				node.SetUID("node-1")
				pod := podObject(visible, "visible-pod")
				pod.SetUID("pod-1")
				pod.Object["spec"] = map[string]interface{}{"nodeName": "worker-1"}
				cm := podObject(visible, "cm")
				cm.SetKind("ConfigMap")
				cm.SetUID("cm-1")
				for _, obj := range []*unstructured.Unstructured{node, pod, cm} {
					reader.inner.AddResource(obj)
				}
				out, err := depHandler(context.Background(), reader, map[string]interface{}{
					"cluster": "c-abc12", "kind": "node", "name": "worker-1", "maxScannedObjects": budget, "format": "json",
				})
				if budget == 2 {
					if err == nil || !strings.Contains(err.Error(), "budget exceeded") {
						t.Fatalf("error=%v; want total budget rejection", err)
					}
				} else if err != nil || !strings.Contains(out, "visible-pod") {
					t.Fatalf("error=%v output=%s; want successful namespace Pod", err, out)
				}
				if _, err := listResourcesAllowed(context.Background(), reader, "c-abc12", "pod", "", nil); err == nil {
					t.Fatal("ordinary list must propagate namespace error")
				}
			})
		}
	}
}
