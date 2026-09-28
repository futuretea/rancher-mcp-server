package steve

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestResolveStaticScope(t *testing.T) {
	cases := []struct {
		name    string
		kind    string
		wantGVR schema.GroupVersionResource
		wantNS  bool
		wantOK  bool
	}{
		{name: "singular", kind: "pod", wantGVR: schema.GroupVersionResource{Version: "v1", Resource: "pods"}, wantNS: true, wantOK: true},
		{name: "alias", kind: "no", wantGVR: schema.GroupVersionResource{Version: "v1", Resource: "nodes"}, wantNS: false, wantOK: true},
		{name: "upper kind", kind: "Namespace", wantGVR: schema.GroupVersionResource{Version: "v1", Resource: "namespaces"}, wantNS: false, wantOK: true},
		{name: "plural fallback", kind: "persistentvolumes", wantGVR: schema.GroupVersionResource{Version: "v1", Resource: "persistentvolumes"}, wantNS: false, wantOK: true},
		{name: "apiVersion qualified", kind: "apps/v1/Deployment", wantGVR: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}, wantNS: true, wantOK: true},
		{name: "apiVersion qualified plural", kind: "apps/v1/deployments", wantGVR: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}, wantNS: true, wantOK: true},
		{name: "metrics dotted", kind: "node.metrics.k8s.io", wantGVR: schema.GroupVersionResource{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "nodes"}, wantNS: false, wantOK: true},
		{name: "metrics plural apiVersion", kind: "metrics.k8s.io/v1beta1/nodes", wantGVR: schema.GroupVersionResource{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "nodes"}, wantNS: false, wantOK: true},

		// A group that does not match the built-in table entry must fall
		// through to discovery: inheriting the built-in scope here is what
		// let a conflicting namespaced CRD bypass the namespace allowlist.
		{name: "conflicting CRD group", kind: "widgets.example.com/v1/Node", wantOK: false},
		{name: "conflicting CRD group plural", kind: "widgets.example.com/v1/nodes", wantOK: false},
		{name: "wrong version", kind: "v1beta1/Node", wantOK: false},

		// Static entries outside the built-in set resolve their GVR but not
		// their scope statically.
		{name: "rancher management kind", kind: "cluster", wantOK: false},
		{name: "cert-manager clusterissuer", kind: "clusterissuer", wantOK: false},
		{name: "fleet bundle", kind: "bundle", wantOK: false},

		{name: "unknown kind", kind: "widget", wantOK: false},
		{name: "empty", kind: "", wantOK: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gvr, namespaced, ok := ResolveStaticScope(tc.kind)
			if ok != tc.wantOK {
				t.Fatalf("ResolveStaticScope(%q) ok = %v, want %v", tc.kind, ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if gvr != tc.wantGVR {
				t.Fatalf("ResolveStaticScope(%q) gvr = %#v, want %#v", tc.kind, gvr, tc.wantGVR)
			}
			if namespaced != tc.wantNS {
				t.Fatalf("ResolveStaticScope(%q) namespaced = %v, want %v", tc.kind, namespaced, tc.wantNS)
			}
		})
	}
}

// TestStaticScopeTableCoversBuiltinKinds locks the sync obligation between
// K8sKindsToGVRs and builtinGVRScopes: every built-in Kubernetes entry must
// carry a static scope, and every static scope must belong to a static entry.
func TestStaticScopeTableCoversBuiltinKinds(t *testing.T) {
	builtinGroups := map[string]struct{}{
		"":                          {},
		"apps":                      {},
		"batch":                     {},
		"networking.k8s.io":         {},
		"autoscaling":               {},
		"autoscaling.k8s.io":        {},
		"rbac.authorization.k8s.io": {},
		"storage.k8s.io":            {},
		"apiextensions.k8s.io":      {},
		"discovery.k8s.io":          {},
		"policy":                    {},
		"metrics.k8s.io":            {},
	}

	knownGVRs := make(map[schema.GroupVersionResource]struct{}, len(K8sKindsToGVRs))
	for kind, gvr := range K8sKindsToGVRs {
		knownGVRs[gvr] = struct{}{}
		if _, builtin := builtinGroups[gvr.Group]; !builtin {
			continue
		}
		if _, ok := builtinGVRScopes[gvr]; !ok {
			t.Errorf("K8sKindsToGVRs[%q] = %#v is missing from builtinGVRScopes", kind, gvr)
		}
	}

	for gvr := range builtinGVRScopes {
		if _, ok := knownGVRs[gvr]; !ok {
			t.Errorf("builtinGVRScopes has %#v which no K8sKindsToGVRs entry resolves to", gvr)
		}
	}

	// A plural fallback that randomly picks between two GVRs with the same
	// resource name must never change the scope decision.
	byResource := map[string]bool{}
	for gvr, namespaced := range builtinGVRScopes {
		if existing, ok := byResource[gvr.Resource]; ok && existing != namespaced {
			t.Errorf("resource %q has conflicting scopes in builtinGVRScopes", gvr.Resource)
		}
		byResource[gvr.Resource] = namespaced
	}
}

// TestResolveResourceScopeWithDiscovery exercises ResolveResourceScope against
// a fake Steve endpoint: built-in kinds stay offline, everything else goes
// through the same discovery path the backend resolver uses.
func TestResolveResourceScopeWithDiscovery(t *testing.T) {
	clusterID := "c-test"
	prefix := "/k8s/clusters/" + clusterID

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case prefix + "/api/v1":
			writeJSON(t, w, metav1.APIResourceList{
				GroupVersion: "v1",
				APIResources: []metav1.APIResource{
					{Name: "pods", Kind: "Pod", Namespaced: true},
					{Name: "namespaces", Kind: "Namespace", Namespaced: false},
				},
			})
		case prefix + "/apis":
			writeJSON(t, w, metav1.APIGroupList{Groups: []metav1.APIGroup{
				{
					Name: "widgets.example.com",
					Versions: []metav1.GroupVersionForDiscovery{
						{GroupVersion: "widgets.example.com/v1", Version: "v1"},
					},
					PreferredVersion: metav1.GroupVersionForDiscovery{GroupVersion: "widgets.example.com/v1", Version: "v1"},
				},
			}})
		case prefix + "/apis/widgets.example.com/v1":
			writeJSON(t, w, metav1.APIResourceList{
				GroupVersion: "widgets.example.com/v1",
				APIResources: []metav1.APIResource{
					// Collides with the built-in Node name but is namespaced.
					{Name: "nodes", Kind: "Node", Namespaced: true},
					{Name: "clusterwidgets", Kind: "ClusterWidget", Namespaced: false},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClientWithToken(server.URL, "test-token", true)
	defer client.Close()

	cases := []struct {
		name    string
		kind    string
		wantGVR schema.GroupVersionResource
		wantNS  bool
		wantErr bool
	}{
		{name: "built-in stays static", kind: "pod", wantGVR: schema.GroupVersionResource{Version: "v1", Resource: "pods"}, wantNS: true},
		{name: "conflicting CRD via apiVersion", kind: "widgets.example.com/v1/Node", wantGVR: schema.GroupVersionResource{Group: "widgets.example.com", Version: "v1", Resource: "nodes"}, wantNS: true},
		{name: "conflicting CRD plural via apiVersion", kind: "widgets.example.com/v1/nodes", wantGVR: schema.GroupVersionResource{Group: "widgets.example.com", Version: "v1", Resource: "nodes"}, wantNS: true},
		{name: "cluster-scoped CRD via apiVersion", kind: "widgets.example.com/v1/ClusterWidget", wantGVR: schema.GroupVersionResource{Group: "widgets.example.com", Version: "v1", Resource: "clusterwidgets"}, wantNS: false},
		{name: "cluster-scoped CRD by bare kind", kind: "clusterwidget", wantGVR: schema.GroupVersionResource{Group: "widgets.example.com", Version: "v1", Resource: "clusterwidgets"}, wantNS: false},
		{name: "unknown kind", kind: "nosuchthing", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gvr, namespaced, err := client.ResolveResourceScope(clusterID, tc.kind)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ResolveResourceScope(%q) error = nil, want error", tc.kind)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveResourceScope(%q) error = %v", tc.kind, err)
			}
			if gvr != tc.wantGVR || namespaced != tc.wantNS {
				t.Fatalf("ResolveResourceScope(%q) = %#v, %v; want %#v, %v", tc.kind, gvr, namespaced, tc.wantGVR, tc.wantNS)
			}
		})
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, value interface{}) {
	t.Helper()
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}
