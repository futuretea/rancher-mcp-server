package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/futuretea/rancher-mcp-server/pkg/client/steve"
	"github.com/futuretea/rancher-mcp-server/pkg/client/steve/fake"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestAllowlistScenarios(t *testing.T) {
	t.Cleanup(resetNamespaceAllowlist)

	t.Run("S2 omitted namespace lists only default", func(t *testing.T) {
		resetNamespaceAllowlist()
		SetNamespaceAllowlist(map[string][]string{"c-abc12": {"default"}})
		rec := newRecordingReader()
		rec.inner.AddResource(podObject("default", "web"))
		rec.inner.AddResource(podObject("kube-system", "hidden"))

		out, err := listHandler(context.Background(), rec, map[string]interface{}{
			"cluster": "c-abc12",
			"kind":    "pod",
			"format":  "json",
		})
		if err != nil {
			t.Fatalf("listHandler() error = %v", err)
		}
		if !strings.Contains(out, "web") || strings.Contains(out, "hidden") {
			t.Fatalf("listHandler() = %s, want only default/web", out)
		}
		rec.assertNoEmptyNamespacedList(t)
		assertNoNamespace(t, rec.calls, "kube-system")
	})

	t.Run("S2 named kube-system is rejected with no request", func(t *testing.T) {
		resetNamespaceAllowlist()
		SetNamespaceAllowlist(map[string][]string{"c-abc12": {"default"}})
		rec := newRecordingReader()
		_, err := listHandler(context.Background(), rec, map[string]interface{}{
			"cluster":   "c-abc12",
			"kind":      "pod",
			"namespace": "kube-system",
			"format":    "json",
		})
		if err == nil || !strings.Contains(err.Error(), "kube-system") {
			t.Fatalf("listHandler() error = %v, want kube-system rejection", err)
		}
		if len(rec.calls) != 0 {
			t.Fatalf("backend calls = %#v, want none", rec.calls)
		}
	})

	t.Run("S3 namespace get and dep scan reject kube-system", func(t *testing.T) {
		resetNamespaceAllowlist()
		SetNamespaceAllowlist(map[string][]string{"c-abc12": {"default"}})
		_, err := getHandler(context.Background(), nil, map[string]interface{}{
			"cluster": "c-abc12",
			"kind":    "Namespace",
			"name":    "kube-system",
		})
		if err == nil || !strings.Contains(err.Error(), "kube-system") {
			t.Fatalf("getHandler() error = %v, want kube-system rejection", err)
		}
		_, err = depHandler(context.Background(), nil, map[string]interface{}{
			"cluster":       "c-abc12",
			"kind":          "node",
			"name":          "worker-1",
			"scanNamespace": "kube-system",
		})
		if err == nil || !strings.Contains(err.Error(), "kube-system") {
			t.Fatalf("depHandler() error = %v, want kube-system rejection", err)
		}
	})

	t.Run("S4 empty array stays unrestricted", func(t *testing.T) {
		resetNamespaceAllowlist()
		SetNamespaceAllowlist(map[string][]string{"c-abc12": {}})
		rec := newRecordingReader()
		rec.inner.AddResource(podObject("default", "web"))
		rec.inner.AddResource(podObject("kube-system", "dns"))
		out, err := listHandler(context.Background(), rec, map[string]interface{}{
			"cluster": "c-abc12",
			"kind":    "pod",
			"format":  "json",
		})
		if err != nil {
			t.Fatalf("listHandler() error = %v", err)
		}
		if !strings.Contains(out, "web") || !strings.Contains(out, "dns") {
			t.Fatalf("listHandler() = %s, want both namespaces", out)
		}
	})

	t.Run("S5 node pods and namespace label selector stay inside the list", func(t *testing.T) {
		resetNamespaceAllowlist()
		SetNamespaceAllowlist(map[string][]string{"c-abc12": {"default", "app"}})
		rec := newRecordingReader()
		rec.inner.AddResource(nodeObject("worker-1"))
		rec.inner.AddResource(namespaceObject("default", map[string]string{"env": "prod"}))
		rec.inner.AddResource(namespaceObject("app", map[string]string{"env": "dev"}))
		rec.inner.AddResource(podObject("default", "web"))
		rec.inner.AddResource(podObject("app", "api"))
		rec.inner.AddResource(podObject("kube-system", "hidden"))

		out, err := nodeAnalysisHandler(context.Background(), rec, map[string]interface{}{
			"cluster": "c-abc12",
			"name":    "worker-1",
			"format":  "json",
		})
		if err != nil {
			t.Fatalf("nodeAnalysisHandler() error = %v", err)
		}
		if strings.Contains(out, "hidden") {
			t.Fatalf("node analysis included kube-system pod: %s", out)
		}
		rec.assertNoEmptyNamespacedList(t)

		rec.calls = nil
		_, err = capacityHandler(context.Background(), rec, map[string]interface{}{
			"cluster":                "c-abc12",
			"namespaceLabelSelector": "env=prod",
			"format":                 "json",
		})
		if err != nil {
			t.Fatalf("capacityHandler() error = %v", err)
		}
		rec.assertNoEmptyNamespacedList(t)
		if !sawCall(rec.calls, "list", "pod", "default") {
			t.Fatalf("capacity calls = %#v, want pod list in default", rec.calls)
		}
		if sawCall(rec.calls, "list", "pod", "app") {
			t.Fatalf("capacity calls = %#v, want no app pod list", rec.calls)
		}
	})

	t.Run("S6 kubeconfig cluster id uses the same rule", func(t *testing.T) {
		resetNamespaceAllowlist()
		SetNamespaceAllowlist(map[string][]string{"kubeconfig:production": {"app"}})
		rec := newRecordingReader()
		rec.inner.AddResource(podObject("app", "web"))
		rec.inner.AddResource(podObject("default", "other"))
		out, err := listHandler(context.Background(), rec, map[string]interface{}{
			"cluster": "kubeconfig:production",
			"kind":    "pod",
			"format":  "json",
		})
		if err != nil {
			t.Fatalf("listHandler() error = %v", err)
		}
		if !strings.Contains(out, "web") || strings.Contains(out, "other") {
			t.Fatalf("listHandler() = %s, want only app/web", out)
		}
	})

	t.Run("S7 empty dep scan uses only the listed namespace", func(t *testing.T) {
		resetNamespaceAllowlist()
		SetNamespaceAllowlist(map[string][]string{"c-abc12": {"default"}})
		rec := newRecordingReader()
		node := nodeObject("worker-1")
		node.SetUID("uid-worker-1")
		rec.inner.AddResource(node)
		_, err := depHandler(context.Background(), rec, map[string]interface{}{
			"cluster": "c-abc12",
			"kind":    "node",
			"name":    "worker-1",
		})
		if err != nil {
			t.Fatalf("depHandler() error = %v", err)
		}
		rec.assertNoEmptyNamespacedList(t)
		if !sawCall(rec.calls, "list", "pod", "default") {
			t.Fatalf("dep calls = %#v, want pod list in default", rec.calls)
		}
	})

	t.Run("apiVersion-qualified node and namespace kinds use the same rule", func(t *testing.T) {
		resetNamespaceAllowlist()
		SetNamespaceAllowlist(map[string][]string{"c-abc12": {"default"}})
		if err := allowNamedAccess(nil, "c-abc12", "v1/Node", "", "worker-1"); err != nil {
			t.Fatalf("v1/Node error = %v, want allowed", err)
		}
		if err := allowNamedAccess(nil, "c-abc12", "v1/PersistentVolume", "", "pv-1"); err != nil {
			t.Fatalf("v1/PersistentVolume error = %v, want allowed", err)
		}
		if err := allowNamedAccess(nil, "c-abc12", "v1/Namespace", "", "kube-system"); err == nil || !strings.Contains(err.Error(), "kube-system") {
			t.Fatalf("v1/Namespace error = %v, want kube-system rejection", err)
		}
	})

	t.Run("resource aliases preserve cluster and namespace scope", func(t *testing.T) {
		resetNamespaceAllowlist()
		SetNamespaceAllowlist(map[string][]string{"c-abc12": {"default"}})

		rec := newRecordingReader()
		// Kinds absent from the static table resolve their scope through
		// discovery; the fake resolver returns what a real cluster would.
		for kind, gvr := range map[string]schema.GroupVersionResource{
			"clusterissuer":                   {Group: "cert-manager.io", Version: "v1", Resource: "clusterissuers"},
			"clusterissuers":                  {Group: "cert-manager.io", Version: "v1", Resource: "clusterissuers"},
			"priorityclass":                   {Group: "scheduling.k8s.io", Version: "v1", Resource: "priorityclasses"},
			"priorityclasses":                 {Group: "scheduling.k8s.io", Version: "v1", Resource: "priorityclasses"},
			"runtimeclass":                    {Group: "node.k8s.io", Version: "v1", Resource: "runtimeclasses"},
			"runtimeclasses":                  {Group: "node.k8s.io", Version: "v1", Resource: "runtimeclasses"},
			"componentstatus":                 {Version: "v1", Resource: "componentstatuses"},
			"componentstatuses":               {Version: "v1", Resource: "componentstatuses"},
			"mutatingwebhookconfiguration":    {Group: "admissionregistration.k8s.io", Version: "v1", Resource: "mutatingwebhookconfigurations"},
			"mutatingwebhookconfigurations":   {Group: "admissionregistration.k8s.io", Version: "v1", Resource: "mutatingwebhookconfigurations"},
			"validatingwebhookconfiguration":  {Group: "admissionregistration.k8s.io", Version: "v1", Resource: "validatingwebhookconfigurations"},
			"validatingwebhookconfigurations": {Group: "admissionregistration.k8s.io", Version: "v1", Resource: "validatingwebhookconfigurations"},
		} {
			rec.scopes[kind] = fakeScope{gvr: gvr, namespaced: false}
		}

		for _, kind := range []string{
			"no", "nodes",
			"pv", "persistentvolumes",
			"sc", "storageclasses",
			"crd", "customresourcedefinitions",
			"volumeattachment", "volumeattachments",
			"clusterissuer", "clusterissuers",
			"priorityclass", "priorityclasses",
			"runtimeclass", "runtimeclasses",
			"componentstatus", "componentstatuses",
			"mutatingwebhookconfiguration", "mutatingwebhookconfigurations",
			"validatingwebhookconfiguration", "validatingwebhookconfigurations",
		} {
			if err := allowNamedAccess(rec, "c-abc12", kind, "", "resource"); err != nil {
				t.Fatalf("allowNamedAccess(%q) error = %v, want cluster-scoped access", kind, err)
			}
			query, err := planNamespaceQuery(rec, "c-abc12", kind, "")
			if err != nil || !query.passthrough {
				t.Fatalf("planNamespaceQuery(%q) = %#v, %v; want passthrough", kind, query, err)
			}
		}

		if err := allowNamedAccess(nil, "c-abc12", "ns", "", "kube-system"); err == nil || !strings.Contains(err.Error(), "kube-system") {
			t.Fatalf("allowNamedAccess(ns) error = %v, want kube-system rejection", err)
		}
	})

	t.Run("named Namespace reads preserve selectors", func(t *testing.T) {
		resetNamespaceAllowlist()
		SetNamespaceAllowlist(map[string][]string{"c-abc12": {"default", "app"}})
		rec := newRecordingReader()
		rec.inner.AddResource(namespaceObject("default", map[string]string{"env": "prod"}))
		rec.inner.AddResource(namespaceObject("app", map[string]string{"env": "dev"}))

		out, err := listHandler(context.Background(), rec, map[string]interface{}{
			"cluster":       "c-abc12",
			"kind":          "namespace",
			"labelSelector": "env=prod",
			"format":        "json",
		})
		if err != nil {
			t.Fatalf("listHandler() error = %v", err)
		}
		if !strings.Contains(out, "default") || strings.Contains(out, "app") {
			t.Fatalf("listHandler() = %s, want only default", out)
		}

		list, err := getNamedNamespaces(context.Background(), rec, "c-abc12", []string{"default", "app"}, &steve.ListOptions{FieldSelector: "metadata.name=default"})
		if err != nil {
			t.Fatalf("getNamedNamespaces() error = %v", err)
		}
		if len(list.Items) != 1 || list.Items[0].GetName() != "default" {
			t.Fatalf("getNamedNamespaces() = %#v, want only default", list.Items)
		}
	})

	t.Run("describe reads related events only from allowed namespaces", func(t *testing.T) {
		resetNamespaceAllowlist()
		SetNamespaceAllowlist(map[string][]string{"c-abc12": {"default"}})
		rec := newRecordingReader()
		rec.inner.AddResource(nodeObject("worker-1"))
		rec.inner.AddEvent(corev1.Event{
			ObjectMeta:     metav1.ObjectMeta{Namespace: "default"},
			InvolvedObject: corev1.ObjectReference{Kind: "Node", Name: "worker-1"},
			Message:        "visible",
		})
		rec.inner.AddEvent(corev1.Event{
			ObjectMeta:     metav1.ObjectMeta{Namespace: "kube-system"},
			InvolvedObject: corev1.ObjectReference{Kind: "Node", Name: "worker-1"},
			Message:        "hidden",
		})

		out, err := describeHandler(context.Background(), rec, map[string]interface{}{
			"cluster": "c-abc12",
			"kind":    "node",
			"name":    "worker-1",
			"format":  "json",
		})
		if err != nil {
			t.Fatalf("describeHandler() error = %v", err)
		}
		if !json.Valid([]byte(out)) || !strings.Contains(out, "visible") || strings.Contains(out, "hidden") {
			t.Fatalf("describeHandler() = %s, want only allowed events", out)
		}
		if sawCall(rec.calls, "events", "node", "") || sawCall(rec.calls, "events", "node", "kube-system") {
			t.Fatalf("describe event calls = %#v, want only allowed namespaces", rec.calls)
		}
	})

	t.Run("capacity selector with an explicit namespace does not list every namespace", func(t *testing.T) {
		resetNamespaceAllowlist()
		SetNamespaceAllowlist(map[string][]string{"c-abc12": {"default"}})
		rec := newRecordingReader()
		rec.inner.AddResource(namespaceObject("default", map[string]string{"env": "prod"}))
		rec.inner.AddResource(namespaceObject("kube-system", map[string]string{"env": "prod"}))
		rec.inner.AddResource(podObject("default", "web"))
		_, err := capacityHandler(context.Background(), rec, map[string]interface{}{
			"cluster":                "c-abc12",
			"namespace":              "default",
			"namespaceLabelSelector": "env=prod",
			"format":                 "json",
		})
		if err != nil {
			t.Fatalf("capacityHandler() error = %v", err)
		}
		rec.assertNoEmptyNamespacedList(t)
		if sawCall(rec.calls, "list", "namespace", "") {
			t.Fatalf("capacity listed every namespace: %#v", rec.calls)
		}
	})

	t.Run("create patch and logs reject out-of-list namespaces before the backend", func(t *testing.T) {
		resetNamespaceAllowlist()
		SetNamespaceAllowlist(map[string][]string{"c-abc12": {"default"}})
		rec := newRecordingReader()

		_, err := createHandler(context.Background(), rec, map[string]interface{}{
			"cluster":  "c-abc12",
			"resource": `{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"cm","namespace":"kube-system"}}`,
		})
		if err == nil || !strings.Contains(err.Error(), "kube-system") {
			t.Fatalf("createHandler() error = %v, want kube-system rejection", err)
		}

		_, err = patchHandler(context.Background(), rec, map[string]interface{}{
			"cluster":   "c-abc12",
			"kind":      "configmap",
			"name":      "cm",
			"namespace": "default",
			"patch":     `[{"op":"replace","path":"/metadata/namespace","value":"kube-system"}]`,
		})
		if err == nil || !strings.Contains(err.Error(), "kube-system") {
			t.Fatalf("patchHandler() error = %v, want kube-system rejection", err)
		}

		_, err = logsHandler(context.Background(), rec, map[string]interface{}{
			"cluster":   "c-abc12",
			"namespace": "kube-system",
			"name":      "web",
		})
		if err == nil || !strings.Contains(err.Error(), "kube-system") {
			t.Fatalf("logsHandler() error = %v, want kube-system rejection", err)
		}

		if len(rec.calls) != 0 {
			t.Fatalf("backend calls = %#v, want none", rec.calls)
		}
	})

	t.Run("stale allowlist entries are skipped", func(t *testing.T) {
		resetNamespaceAllowlist()
		SetNamespaceAllowlist(map[string][]string{"c-abc12": {"default", "gone"}})
		rec := newRecordingReader()
		rec.inner.AddResource(namespaceObject("default", map[string]string{"env": "prod"}))
		rec.inner.AddResource(podObject("default", "web"))

		out, err := listHandler(context.Background(), rec, map[string]interface{}{
			"cluster": "c-abc12",
			"kind":    "namespace",
			"format":  "json",
		})
		if err != nil {
			t.Fatalf("listHandler() error = %v, want stale entry skipped", err)
		}
		if !strings.Contains(out, "default") || strings.Contains(out, "gone") {
			t.Fatalf("listHandler() = %s, want only default", out)
		}

		_, err = capacityHandler(context.Background(), rec, map[string]interface{}{
			"cluster":                "c-abc12",
			"namespaceLabelSelector": "env=prod",
			"format":                 "json",
		})
		if err != nil {
			t.Fatalf("capacityHandler() error = %v, want stale entry skipped", err)
		}
		rec.assertNoEmptyNamespacedList(t)
	})

	t.Run("exec upload and download reject before the backend", func(t *testing.T) {
		resetNamespaceAllowlist()
		SetNamespaceAllowlist(map[string][]string{"c-abc12": {"default"}})
		params := map[string]interface{}{
			"cluster":   "c-abc12",
			"namespace": "kube-system",
			"name":      "pod",
			"command":   []interface{}{"true"},
			"filePath":  "/tmp/a",
			"content":   "aGVsbG8=",
		}
		for _, call := range []func() (string, error){
			func() (string, error) { return handleExec(context.Background(), nil, params) },
			func() (string, error) { return handleUploadFile(context.Background(), nil, params) },
			func() (string, error) { return handleDownloadFile(context.Background(), nil, params) },
		} {
			_, err := call()
			if err == nil || !strings.Contains(err.Error(), "kube-system") {
				t.Fatalf("error = %v, want kube-system rejection", err)
			}
		}
	})

	t.Run("namespaced CRD conflicting with a built-in name cannot bypass the allowlist", func(t *testing.T) {
		resetNamespaceAllowlist()
		SetNamespaceAllowlist(map[string][]string{"c-abc12": {"default"}})
		rec := newRecordingReader()
		// A namespaced CRD whose kind and plural collide with the built-in
		// Node resource. Discovery reports it as namespaced.
		crdGVR := schema.GroupVersionResource{Group: "widgets.example.com", Version: "v1", Resource: "nodes"}
		rec.scopes["widgets.example.com/v1/node"] = fakeScope{gvr: crdGVR, namespaced: true}
		rec.scopes["widgets.example.com/v1/nodes"] = fakeScope{gvr: crdGVR, namespaced: true}

		_, err := listHandler(context.Background(), rec, map[string]interface{}{
			"cluster":    "c-abc12",
			"apiVersion": "widgets.example.com/v1",
			"kind":       "Node",
			"namespace":  "kube-system",
			"format":     "json",
		})
		if err == nil || !strings.Contains(err.Error(), "kube-system") {
			t.Fatalf("listHandler() error = %v, want kube-system rejection", err)
		}
		if len(rec.calls) != 0 {
			t.Fatalf("backend calls = %#v, want none", rec.calls)
		}

		_, err = listHandler(context.Background(), rec, map[string]interface{}{
			"cluster":    "c-abc12",
			"apiVersion": "widgets.example.com/v1",
			"kind":       "Node",
			"format":     "json",
		})
		if err != nil {
			t.Fatalf("listHandler() error = %v", err)
		}
		rec.assertNoEmptyNamespacedList(t)
		if !sawCall(rec.calls, "list", "widgets.example.com/v1/node", "default") {
			t.Fatalf("calls = %#v, want CRD list in default", rec.calls)
		}

		// A manifest whose bare Kind collides with a built-in name is checked
		// by its full apiVersion/kind reference.
		rec.calls = nil
		manifest := `{"apiVersion":"widgets.example.com/v1","kind":"Node","metadata":{"name":"thing","namespace":"kube-system"}}`
		_, err = createHandler(context.Background(), rec, map[string]interface{}{
			"cluster":  "c-abc12",
			"resource": manifest,
		})
		if err == nil || !strings.Contains(err.Error(), "kube-system") {
			t.Fatalf("createHandler() error = %v, want kube-system rejection", err)
		}
		if len(rec.calls) != 0 {
			t.Fatalf("backend calls = %#v, want none", rec.calls)
		}

		// The built-in node kind still resolves statically as cluster-scoped.
		if err := allowNamedAccess(nil, "c-abc12", "node", "", "worker-1"); err != nil {
			t.Fatalf("allowNamedAccess(node) error = %v, want cluster-scoped access", err)
		}
	})

	t.Run("cluster-scoped CRD stays visible on a restricted cluster", func(t *testing.T) {
		resetNamespaceAllowlist()
		SetNamespaceAllowlist(map[string][]string{"c-abc12": {"default"}})
		rec := newRecordingReader()
		crdGVR := schema.GroupVersionResource{Group: "widgets.example.com", Version: "v1", Resource: "clusterwidgets"}
		rec.scopes["widgets.example.com/v1/clusterwidget"] = fakeScope{gvr: crdGVR, namespaced: false}

		if err := allowNamedAccess(rec, "c-abc12", "widgets.example.com/v1/ClusterWidget", "", "thing"); err != nil {
			t.Fatalf("allowNamedAccess() error = %v, want cluster-scoped access", err)
		}

		_, err := listHandler(context.Background(), rec, map[string]interface{}{
			"cluster":    "c-abc12",
			"apiVersion": "widgets.example.com/v1",
			"kind":       "ClusterWidget",
			"format":     "json",
		})
		if err != nil {
			t.Fatalf("listHandler() error = %v", err)
		}
		if !sawCall(rec.calls, "list", "widgets.example.com/v1/clusterwidget", "") {
			t.Fatalf("calls = %#v, want passthrough list with empty namespace", rec.calls)
		}
	})
}

type recordedCall struct {
	op, kind, namespace string
}

type fakeScope struct {
	gvr        schema.GroupVersionResource
	namespaced bool
}

type recordingReader struct {
	mu     sync.Mutex
	inner  *fake.Client
	scopes map[string]fakeScope
	calls  []recordedCall
}

func newRecordingReader() *recordingReader {
	return &recordingReader{inner: fake.NewClient(), scopes: map[string]fakeScope{}}
}

func (r *recordingReader) record(call recordedCall) {
	r.mu.Lock()
	r.calls = append(r.calls, call)
	r.mu.Unlock()
}

// ResolveResourceScope mirrors steve.Client.ResolveResourceScope: built-in
// kinds resolve statically, everything else consults the fake scope table.
func (r *recordingReader) ResolveResourceScope(clusterID, kind string) (schema.GroupVersionResource, bool, error) {
	if gvr, namespaced, ok := steve.ResolveStaticScope(kind); ok {
		return gvr, namespaced, nil
	}
	if scope, ok := r.scopes[strings.ToLower(kind)]; ok {
		return scope.gvr, scope.namespaced, nil
	}
	return schema.GroupVersionResource{}, false, fmt.Errorf("unsupported resource kind: %s", kind)
}

func (r *recordingReader) GetResource(ctx context.Context, cluster, kind, namespace, name string) (*unstructured.Unstructured, error) {
	r.record(recordedCall{op: "get", kind: strings.ToLower(kind), namespace: namespace})
	return r.inner.GetResource(ctx, cluster, kind, namespace, name)
}

func (r *recordingReader) ListResources(ctx context.Context, cluster, kind, namespace string, opts *steve.ListOptions) (*unstructured.UnstructuredList, error) {
	r.record(recordedCall{op: "list", kind: strings.ToLower(kind), namespace: namespace})
	return r.inner.ListResources(ctx, cluster, kind, namespace, opts)
}

func (r *recordingReader) GetEvents(ctx context.Context, cluster, namespace, name, kind string) ([]corev1.Event, error) {
	r.record(recordedCall{op: "events", kind: strings.ToLower(kind), namespace: namespace})
	return r.inner.GetEvents(ctx, cluster, namespace, name, kind)
}

func podObject(namespace, name string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("Pod")
	obj.SetNamespace(namespace)
	obj.SetName(name)
	return obj
}

func nodeObject(name string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("Node")
	obj.SetName(name)
	return obj
}

func namespaceObject(name string, labels map[string]string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("Namespace")
	obj.SetName(name)
	obj.SetLabels(labels)
	return obj
}

func (r *recordingReader) assertNoEmptyNamespacedList(t *testing.T) {
	t.Helper()
	for _, call := range r.calls {
		if call.op != "list" || call.namespace != "" {
			continue
		}
		_, namespaced, err := r.ResolveResourceScope("c-abc12", call.kind)
		if err != nil {
			t.Fatalf("resolve scope of listed kind %s: %v", call.kind, err)
		}
		if namespaced {
			t.Fatalf("empty-namespace list of %s: %#v", call.kind, r.calls)
		}
	}
}

func assertNoNamespace(t *testing.T, calls []recordedCall, namespace string) {
	t.Helper()
	for _, call := range calls {
		if call.namespace == namespace {
			t.Fatalf("call used namespace %s: %#v", namespace, calls)
		}
	}
}

func sawCall(calls []recordedCall, op, kind, namespace string) bool {
	for _, call := range calls {
		if call.op == op && call.kind == kind && call.namespace == namespace {
			return true
		}
	}
	return false
}
