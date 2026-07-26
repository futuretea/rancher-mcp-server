package kubernetes

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/futuretea/rancher-mcp-server/pkg/client/steve"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset"
)

func TestKubernetesListTool_ClusterReferenceRoutingMatrix(t *testing.T) {
	fixture := newClusterRoutingFixture(t)
	listTool := findKubernetesTool(t, "kubernetes_list")

	for _, test := range routingMatrix(t, fixture) {
		t.Run(test.name, func(t *testing.T) {
			before := fixture.calls()
			output, err := listTool.Handler(context.Background(), test.client, map[string]interface{}{
				"cluster":   test.reference,
				"kind":      "pod",
				"namespace": "default",
				"format":    "json",
			})
			assertRoutingResult(t, fixture, before, test.target, err)
			if test.target != "" && !strings.Contains(output, test.target+"-pod") {
				t.Fatalf("kubernetes_list output = %q, want %s source pod", output, test.target)
			}
		})
	}
}

func TestResourceDiffTool_ClusterReferenceRoutingMatrix(t *testing.T) {
	fixture := newClusterRoutingFixture(t)
	diffTool := findKubernetesTool(t, "kubernetes_resource_diff")

	for _, test := range routingMatrix(t, fixture) {
		t.Run(test.name, func(t *testing.T) {
			before := fixture.calls()
			_, err := diffTool.Handler(context.Background(), test.client, map[string]interface{}{
				"kind": "pod",
				"left": map[string]interface{}{
					"cluster":   test.reference,
					"namespace": "default",
					"name":      "left",
				},
				"right": map[string]interface{}{
					"cluster":   test.reference,
					"namespace": "default",
					"name":      "right",
				},
			})
			assertRoutingResult(t, fixture, before, test.target, err)
		})
	}
}

type routingCase struct {
	name      string
	client    *toolset.CombinedClient
	reference string
	target    string
}

func routingMatrix(t *testing.T, fixture *clusterRoutingFixture) []routingCase {
	t.Helper()
	kubeconfigPath := writeDiffKubeconfigForServer(t, fixture.direct.URL)

	newClient := func(rancher, kubeconfig bool) *toolset.CombinedClient {
		serverURL := ""
		if rancher {
			serverURL = fixture.rancher.URL
		}
		var paths []string
		if kubeconfig {
			paths = []string{kubeconfigPath}
		}
		client, err := steve.NewClientWithKubeconfigPaths(serverURL, "fixture-token", "", "", true, paths)
		if err != nil {
			t.Fatalf("NewClientWithKubeconfigPaths() error = %v", err)
		}
		return toolset.NewCombinedClient(nil, client, false)
	}

	return []routingCase{
		{name: "prefixed configured", client: newClient(true, true), reference: "kubeconfig:direct", target: "direct"},
		{name: "bare Rancher", client: newClient(true, true), reference: "c-rancher", target: "rancher"},
		{name: "prefixed unconfigured", client: newClient(true, false), reference: "kubeconfig:direct"},
		{name: "bare kubeconfig-only", client: newClient(false, true), reference: "c-rancher"},
		{name: "malformed prefix", client: newClient(true, true), reference: "unknown:cluster"},
		{name: "traversal", client: newClient(true, true), reference: "kubeconfig:../escape"},
	}
}

func assertRoutingResult(t *testing.T, fixture *clusterRoutingFixture, before routingCalls, target string, err error) {
	t.Helper()
	after := fixture.calls()
	if target == "" {
		var referenceErr *steve.ClusterReferenceError
		if !errors.As(err, &referenceErr) {
			t.Fatalf("handler error = %v, want ClusterReferenceError", err)
		}
		if after != before {
			t.Fatalf("invalid reference made network calls: before=%+v after=%+v", before, after)
		}
		return
	}
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	if target == "direct" && (after.direct <= before.direct || after.rancher != before.rancher) {
		t.Fatalf("direct routing calls: before=%+v after=%+v", before, after)
	}
	if target == "rancher" && (after.rancher <= before.rancher || after.direct != before.direct) {
		t.Fatalf("Rancher routing calls: before=%+v after=%+v", before, after)
	}
}

type clusterRoutingFixture struct {
	rancher      *httptest.Server
	direct       *httptest.Server
	rancherCalls atomic.Int32
	directCalls  atomic.Int32
}

type routingCalls struct {
	rancher int32
	direct  int32
}

func newClusterRoutingFixture(t *testing.T) *clusterRoutingFixture {
	t.Helper()
	fixture := &clusterRoutingFixture{}
	fixture.rancher = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fixture.rancherCalls.Add(1)
		fixture.writeResponse(w, r, "/k8s/clusters/c-rancher", "rancher")
	}))
	fixture.direct = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fixture.directCalls.Add(1)
		fixture.writeResponse(w, r, "", "direct")
	}))
	t.Cleanup(fixture.rancher.Close)
	t.Cleanup(fixture.direct.Close)
	return fixture
}

func (f *clusterRoutingFixture) calls() routingCalls {
	return routingCalls{rancher: f.rancherCalls.Load(), direct: f.directCalls.Load()}
}

func (f *clusterRoutingFixture) writeResponse(w http.ResponseWriter, r *http.Request, prefix, source string) {
	path := strings.TrimPrefix(r.URL.Path, prefix)
	if r.Method != http.MethodGet || !strings.HasPrefix(path, "/api/v1/namespaces/default/pods") {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if path == "/api/v1/namespaces/default/pods" {
		_, _ = w.Write([]byte(`{"apiVersion":"v1","kind":"PodList","items":[{"apiVersion":"v1","kind":"Pod","metadata":{"name":"` + source + `-pod","namespace":"default"}}]}`))
		return
	}
	_, _ = w.Write([]byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"` + source + `-pod","namespace":"default"}}`))
}
