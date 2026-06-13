package kubernetes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/futuretea/rancher-mcp-server/pkg/client/steve"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestExtractRolloutParams(t *testing.T) {
	t.Run("all required params", func(t *testing.T) {
		params := map[string]interface{}{
			"cluster":   "c1",
			"namespace": "default",
			"name":      "nginx",
		}
		cluster, namespace, name, err := extractRolloutParams(params)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cluster != "c1" || namespace != "default" || name != "nginx" {
			t.Errorf("unexpected values: %q, %q, %q", cluster, namespace, name)
		}
	})

	t.Run("missing cluster", func(t *testing.T) {
		_, _, _, err := extractRolloutParams(map[string]interface{}{
			"namespace": "default",
			"name":      "nginx",
		})
		if err == nil {
			t.Fatal("expected error for missing cluster")
		}
	})
}

func TestBuildDeploymentSelector(t *testing.T) {
	t.Run("with selector", func(t *testing.T) {
		deployment := &unstructured.Unstructured{}
		deployment.SetUnstructuredContent(map[string]interface{}{
			"spec": map[string]interface{}{
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"app": "nginx",
					},
				},
			},
		})

		got := buildDeploymentSelector(deployment)
		if got["app"] != "nginx" {
			t.Errorf("expected app=nginx selector, got %v", got)
		}
	})

	t.Run("without selector", func(t *testing.T) {
		deployment := &unstructured.Unstructured{}
		deployment.SetUnstructuredContent(map[string]interface{}{})

		got := buildDeploymentSelector(deployment)
		if len(got) != 0 {
			t.Errorf("expected empty selector, got %v", got)
		}
	})
}

func TestBuildLabelSelector(t *testing.T) {
	got := buildLabelSelector(map[string]string{
		"app":     "nginx",
		"version": "v1",
	})
	if !strings.Contains(got, "app=nginx") || !strings.Contains(got, "version=v1") {
		t.Errorf("expected selector to contain both pairs, got %q", got)
	}
}

func TestIsOwnedByDeployment(t *testing.T) {
	tests := []struct {
		name       string
		ownerRefs  []interface{}
		deployment string
		want       bool
	}{
		{
			name:       "owned by target deployment",
			deployment: "nginx",
			ownerRefs: []interface{}{
				map[string]interface{}{"kind": "Deployment", "name": "nginx"},
			},
			want: true,
		},
		{
			name:       "owned by different deployment",
			deployment: "nginx",
			ownerRefs: []interface{}{
				map[string]interface{}{"kind": "Deployment", "name": "other"},
			},
			want: false,
		},
		{
			name:       "non-deployment owner",
			deployment: "nginx",
			ownerRefs: []interface{}{
				map[string]interface{}{"kind": "ReplicaSet", "name": "nginx-1"},
			},
			want: false,
		},
		{
			name:       "invalid owner reference",
			deployment: "nginx",
			ownerRefs:  []interface{}{"not-a-map"},
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isOwnedByDeployment(tt.ownerRefs, tt.deployment); got != tt.want {
				t.Errorf("isOwnedByDeployment() = %v, want %v", got, tt.want)
			}
		})
	}
}

func newReplicaSet(name, deploymentName, revision, changeCause string, created time.Time) unstructured.Unstructured {
	rs := unstructured.Unstructured{}
	rs.SetUnstructuredContent(map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": "default",
			"annotations": map[string]interface{}{
				"deployment.kubernetes.io/revision":     revision,
				"deployment.kubernetes.io/change-cause": changeCause,
			},
			"ownerReferences": []interface{}{
				map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"name":       deploymentName,
				},
			},
		},
	})
	rs.SetCreationTimestamp(metav1.NewTime(created))
	return rs
}

func TestExtractRolloutHistory(t *testing.T) {
	base := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	owned := newReplicaSet("nginx-1", "nginx", "2", "scale up", base)
	unowned := newReplicaSet("nginx-0", "other", "1", "", base.Add(-24*time.Hour))

	rsList := &unstructured.UnstructuredList{
		Items: []unstructured.Unstructured{owned, unowned},
	}

	history := extractRolloutHistory(rsList, "nginx")
	if len(history) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(history))
	}

	got := history[0]
	if got.Name != "nginx-1" || got.Revision != "2" || got.ChangeCause != "scale up" {
		t.Errorf("unexpected history entry: %+v", got)
	}
	if !strings.HasPrefix(got.Created, "2024-01-15") {
		t.Errorf("unexpected created timestamp: %q", got.Created)
	}
}

func TestSortRolloutHistory(t *testing.T) {
	history := []RevisionInfo{
		{Revision: "1", Name: "first"},
		{Revision: "10", Name: "tenth"},
		{Revision: "2", Name: "second"},
	}
	sortRolloutHistory(history)

	want := []string{"10", "2", "1"}
	for i, rev := range want {
		if history[i].Revision != rev {
			t.Errorf("position %d: got revision %q, want %q", i, history[i].Revision, rev)
		}
	}
}

func TestFormatRolloutHistory(t *testing.T) {
	history := []RevisionInfo{
		{Revision: "3", ChangeCause: "scale up", Created: "2024-01-15T10:30:00Z", Name: "nginx-abc123"},
	}

	t.Run("table", func(t *testing.T) {
		got, err := formatRolloutHistory(history, "table")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(got, "REVISION") || !strings.Contains(got, "nginx-abc123") {
			t.Errorf("expected table output, got %q", got)
		}
	})

	t.Run("json", func(t *testing.T) {
		got, err := formatRolloutHistory(history, "json")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var parsed []RevisionInfo
		if err := json.Unmarshal([]byte(got), &parsed); err != nil {
			t.Fatalf("failed to parse JSON output: %v", err)
		}
		if len(parsed) != 1 || parsed[0].Revision != "3" {
			t.Errorf("unexpected JSON output: %q", got)
		}
	})

	t.Run("defaults to json", func(t *testing.T) {
		got, err := formatRolloutHistory(history, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(got, "revision") {
			t.Errorf("expected JSON output for default format, got %q", got)
		}
	})
}

func TestFormatRolloutHistoryAsTable(t *testing.T) {
	t.Run("empty history", func(t *testing.T) {
		got := formatRolloutHistoryAsTable(nil)
		if got != "No rollout history found" {
			t.Fatalf("expected 'No rollout history found', got %q", got)
		}
	})

	t.Run("with revisions", func(t *testing.T) {
		history := []RevisionInfo{
			{Revision: "3", ChangeCause: "scale up", Created: "2024-01-15T10:30:00Z", Name: "nginx-abc123"},
			{Revision: "2", ChangeCause: "", Created: "2024-01-14T08:00:00Z", Name: "nginx-def456"},
		}
		got := formatRolloutHistoryAsTable(history)
		if got == "" || got == "No rollout history found" {
			t.Fatal("expected table output")
		}
		// Should contain revision numbers
		if !strings.Contains(got, "3") || !strings.Contains(got, "2") {
			t.Errorf("expected revision numbers in output: %s", got)
		}
		// Empty change cause should show "-"
		if !strings.Contains(got, "-") {
			t.Errorf("expected '-' for empty change cause: %s", got)
		}
		// Should have a header
		if !strings.Contains(got, "REVISION") {
			t.Errorf("expected REVISION header: %s", got)
		}
	})
}

func TestRolloutHistoryHandler_Integration(t *testing.T) {
	server := newRolloutHistoryTestServer(t)
	defer server.Close()

	client := steve.NewClient(server.URL, "token", "", "", true)

	t.Run("table output", func(t *testing.T) {
		testRolloutHistoryTableOutput(t, client)
	})

	t.Run("json output", func(t *testing.T) {
		testRolloutHistoryJSONOutput(t, client)
	})

	t.Run("combined client accepted", func(t *testing.T) {
		testRolloutHistoryCombinedClient(t, client)
	})
}

func newRolloutHistoryTestServer(t *testing.T) *httptest.Server {
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/k8s/clusters/c1/apis/apps/v1/namespaces/default/deployments/nginx"):
			writeRolloutHistoryDeployment(w)

		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/k8s/clusters/c1/apis/apps/v1/namespaces/default/replicasets"):
			writeRolloutHistoryReplicaSets(w)

		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
}

func writeRolloutHistoryDeployment(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]interface{}{
			"name":      "nginx",
			"namespace": "default",
		},
		"spec": map[string]interface{}{
			"selector": map[string]interface{}{
				"matchLabels": map[string]interface{}{
					"app": "nginx",
				},
			},
		},
	})
}

func writeRolloutHistoryReplicaSets(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "ReplicaSetList",
		"metadata":   map[string]interface{}{},
		"items": []interface{}{
			map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "ReplicaSet",
				"metadata": map[string]interface{}{
					"name":              "nginx-2",
					"namespace":         "default",
					"creationTimestamp": "2024-01-15T10:30:00Z",
					"annotations": map[string]interface{}{
						"deployment.kubernetes.io/revision":     "2",
						"deployment.kubernetes.io/change-cause": "scale up",
					},
					"ownerReferences": []interface{}{
						map[string]interface{}{"apiVersion": "apps/v1", "kind": "Deployment", "name": "nginx"},
					},
				},
			},
			map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "ReplicaSet",
				"metadata": map[string]interface{}{
					"name":              "nginx-1",
					"namespace":         "default",
					"creationTimestamp": "2024-01-14T08:00:00Z",
					"annotations": map[string]interface{}{
						"deployment.kubernetes.io/revision": "1",
					},
					"ownerReferences": []interface{}{
						map[string]interface{}{"apiVersion": "apps/v1", "kind": "Deployment", "name": "other"},
					},
				},
			},
		},
	})
}

func testRolloutHistoryTableOutput(t *testing.T, client *steve.Client) {
	params := map[string]interface{}{
		"cluster":   "c1",
		"namespace": "default",
		"name":      "nginx",
		"format":    "table",
	}
	out, err := rolloutHistoryHandler(context.Background(), client, params)
	if err != nil {
		t.Fatalf("rolloutHistoryHandler() error = %v", err)
	}
	if !strings.Contains(out, "nginx-2") || !strings.Contains(out, "scale up") {
		t.Errorf("expected owned replicaset in output, got %q", out)
	}
	if strings.Contains(out, "nginx-1") {
		t.Errorf("unowned replicaset should not appear, got %q", out)
	}
}

func testRolloutHistoryJSONOutput(t *testing.T, client *steve.Client) {
	params := map[string]interface{}{
		"cluster":   "c1",
		"namespace": "default",
		"name":      "nginx",
		"format":    "json",
	}
	out, err := rolloutHistoryHandler(context.Background(), client, params)
	if err != nil {
		t.Fatalf("rolloutHistoryHandler() error = %v", err)
	}
	var history []RevisionInfo
	if err := json.Unmarshal([]byte(out), &history); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if len(history) != 1 || history[0].Name != "nginx-2" {
		t.Errorf("unexpected JSON history: %q", out)
	}
}

func testRolloutHistoryCombinedClient(t *testing.T, client *steve.Client) {
	combined := &toolset.CombinedClient{
		Norman: nil,
		Steve:  client,
	}
	params := map[string]interface{}{
		"cluster":   "c1",
		"namespace": "default",
		"name":      "nginx",
		"format":    "table",
	}
	out, err := rolloutHistoryHandler(context.Background(), combined, params)
	if err != nil {
		t.Fatalf("rolloutHistoryHandler() error = %v", err)
	}
	if !strings.Contains(out, "nginx-2") {
		t.Errorf("expected output from combined client, got %q", out)
	}
}
