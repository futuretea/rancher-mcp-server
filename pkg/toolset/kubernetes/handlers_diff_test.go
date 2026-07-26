package kubernetes

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

func TestDiffHandler_MetadataDifferencesHonorIgnoreMeta(t *testing.T) {
	params := map[string]interface{}{
		"resource1": `{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"demo","namespace":"default","labels":{"version":"one"}},"spec":{"replicas":1}}`,
		"resource2": `{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"demo","namespace":"default","labels":{"version":"two"}},"spec":{"replicas":1}}`,
	}

	output, err := diffHandler(context.Background(), nil, params)
	if err != nil {
		t.Fatalf("diffHandler() returned unexpected error: %v", err)
	}
	if !strings.Contains(output, "labels") {
		t.Fatalf("expected metadata differences, got %q", output)
	}

	params["ignoreMeta"] = true
	output, err = diffHandler(context.Background(), nil, params)
	if err != nil {
		t.Fatalf("diffHandler() with ignoreMeta returned unexpected error: %v", err)
	}
	if !strings.Contains(output, "labels") {
		t.Fatalf("expected labels to remain visible with ignoreMeta, got %q", output)
	}
}

func TestDiffResources_MetadataDifferencesHonorIgnoreMeta(t *testing.T) {
	first := metadataDiffResource("one")
	second := metadataDiffResource("two")

	output, err := diffResources(first, second, false, false)
	if err != nil {
		t.Fatalf("diffResources() returned unexpected error: %v", err)
	}
	if !strings.Contains(output, "labels") {
		t.Fatalf("expected metadata differences, got %q", output)
	}

	output, err = diffResources(first, second, false, true)
	if err != nil {
		t.Fatalf("diffResources() with ignoreMeta returned unexpected error: %v", err)
	}
	if !strings.Contains(output, "labels") {
		t.Fatalf("expected labels to remain visible with ignoreMeta, got %q", output)
	}
}

func TestDiffResources_IgnoreMetaHidesTransientMetadata(t *testing.T) {
	first := metadataDiffResource("one")
	second := metadataDiffResource("one")
	first.SetResourceVersion("1")
	second.SetResourceVersion("2")

	output, err := diffResources(first, second, false, false)
	if err != nil {
		t.Fatalf("diffResources() returned unexpected error: %v", err)
	}
	if !strings.Contains(output, "resourceVersion") {
		t.Fatalf("expected transient metadata difference, got %q", output)
	}

	output, err = diffResources(first, second, false, true)
	if err != nil {
		t.Fatalf("diffResources() with ignoreMeta returned unexpected error: %v", err)
	}
	if output != "No differences found between the two resource versions." {
		t.Fatalf("expected ignored transient metadata difference to be empty, got %q", output)
	}
}

func metadataDiffResource(version string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]interface{}{
			"name":      "demo",
			"namespace": "default",
			"labels":    map[string]interface{}{"version": version},
		},
		"spec": map[string]interface{}{"replicas": int64(1)},
	}}
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

func TestResourceDiffHandler_RejectsInvalidKubeconfigClusterReference(t *testing.T) {
	client, err := steve.NewClientWithKubeconfigPaths("", "", "", "", false, []string{writeDiffKubeconfig(t)})
	if err != nil {
		t.Fatalf("NewClientWithKubeconfigPaths() error = %v", err)
	}

	_, err = resourceDiffHandler(context.Background(), &toolset.CombinedClient{Steve: client}, map[string]interface{}{
		"kind": "deployment",
		"left": map[string]interface{}{
			"cluster": "kubeconfig:../escape",
			"name":    "left",
		},
		"right": map[string]interface{}{
			"cluster": "kubeconfig:direct",
			"name":    "right",
		},
	})

	var referenceErr *steve.ClusterReferenceError
	if !errors.As(err, &referenceErr) {
		t.Fatalf("resourceDiffHandler() error = %v, want ClusterReferenceError", err)
	}
}

func TestKubernetesListTool_UsesKubeconfigClusterThroughCombinedClient(t *testing.T) {
	var requestPath string
	apiServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/namespaces/default/pods" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"apiVersion":"v1","kind":"PodList","items":[{"apiVersion":"v1","kind":"Pod","metadata":{"name":"direct-pod","namespace":"default"}}]}`))
	}))
	defer apiServer.Close()

	client, err := steve.NewClientWithKubeconfigPaths("", "", "", "", false, []string{writeDiffKubeconfigForServer(t, apiServer.URL)})
	if err != nil {
		t.Fatalf("NewClientWithKubeconfigPaths() error = %v", err)
	}
	combinedClient := toolset.NewCombinedClient(nil, client, false)
	listTool := findKubernetesTool(t, "kubernetes_list")
	params := map[string]interface{}{
		"cluster":   "kubeconfig:direct",
		"kind":      "pod",
		"namespace": "default",
		"format":    "json",
	}

	output, err := listTool.Handler(context.Background(), combinedClient, params)
	if err != nil {
		t.Fatalf("kubernetes_list error = %v", err)
	}
	if !strings.Contains(output, "direct-pod") {
		t.Fatalf("kubernetes_list output = %q, want direct kubeconfig pod", output)
	}
	if requestPath != "/api/v1/namespaces/default/pods" {
		t.Fatalf("Kubernetes API path = %q, want direct kubeconfig API path", requestPath)
	}
	if strings.Contains(requestPath, "/k8s/clusters/") {
		t.Fatalf("Kubernetes API path = %q must not use Rancher Steve routing", requestPath)
	}

	params["cluster"] = "kubeconfig:../escape"
	_, err = listTool.Handler(context.Background(), combinedClient, params)
	var referenceErr *steve.ClusterReferenceError
	if !errors.As(err, &referenceErr) {
		t.Fatalf("kubernetes_list invalid reference error = %v, want ClusterReferenceError", err)
	}
}

func findKubernetesTool(t *testing.T, name string) toolset.ServerTool {
	t.Helper()
	for _, registered := range (&Toolset{}).GetTools(nil) {
		if registered.Tool.Name == name {
			return registered
		}
	}
	t.Fatalf("Kubernetes tool %q is not registered", name)
	return toolset.ServerTool{}
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

func writeDiffKubeconfig(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "kubeconfig.yaml")
	content := `apiVersion: v1
kind: Config
clusters:
- name: direct
  cluster:
    server: https://direct.example.test
contexts:
- name: direct
  context:
    cluster: direct
    user: direct
current-context: direct
users:
- name: direct
  user: {}
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write kubeconfig fixture: %v", err)
	}
	return path
}

func writeDiffKubeconfigForServer(t *testing.T, server string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "kubeconfig.yaml")
	content := `apiVersion: v1
kind: Config
clusters:
- name: direct
  cluster:
    server: ` + server + `
    insecure-skip-tls-verify: true
contexts:
- name: direct
  context:
    cluster: direct
    user: direct
current-context: direct
users:
- name: direct
  user: {}
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write kubeconfig fixture: %v", err)
	}
	return path
}
