package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/futuretea/rancher-mcp-server/pkg/client/steve"
)

func TestGetAllTool_SharesPerResourceBudgetAcrossNamespaces(t *testing.T) {
	for _, test := range []struct {
		name  string
		scope string
		limit int64
		want  []string
		lists map[string]string
	}{
		{name: "exhausted", scope: "namespaced", limit: 1,
			want:  []string{"v1/ConfigMap/alpha/c1", "v1/Pod/alpha/p1"},
			lists: map[string]string{"/api/v1/namespaces/alpha/pods": "1", "/api/v1/namespaces/alpha/configmaps": "1"}},
		{name: "remaining", scope: "namespaced", limit: 2,
			want:  []string{"v1/ConfigMap/alpha/c1", "v1/ConfigMap/alpha/c2", "v1/Pod/alpha/p1", "v1/Pod/beta/p2"},
			lists: map[string]string{"/api/v1/namespaces/alpha/pods": "2", "/api/v1/namespaces/beta/pods": "1", "/api/v1/namespaces/alpha/configmaps": "2"}},
		{name: "unlimited", scope: "namespaced",
			want:  []string{"v1/ConfigMap/alpha/c1", "v1/ConfigMap/alpha/c2", "v1/ConfigMap/alpha/c3", "v1/ConfigMap/beta/c4", "v1/Pod/alpha/p1", "v1/Pod/beta/p2", "v1/Pod/beta/p3", "v1/Pod/gamma/p4"},
			lists: map[string]string{"/api/v1/namespaces/alpha/pods": "", "/api/v1/namespaces/beta/pods": "", "/api/v1/namespaces/gamma/pods": "", "/api/v1/namespaces/alpha/configmaps": "", "/api/v1/namespaces/beta/configmaps": "", "/api/v1/namespaces/gamma/configmaps": ""}},
		{name: "default scope", limit: 2,
			want:  []string{"v1/ConfigMap/alpha/c1", "v1/ConfigMap/alpha/c2", "v1/Pod/alpha/p1", "v1/Pod/beta/p2", "v1/Node//worker-1", "v1/Node//worker-2", "v1/Namespace//alpha", "v1/Namespace//beta"},
			lists: map[string]string{"/api/v1/namespaces/alpha/pods": "2", "/api/v1/namespaces/beta/pods": "1", "/api/v1/namespaces/alpha/configmaps": "2", "/api/v1/nodes": "2", "/api/v1/namespaces/alpha": "", "/api/v1/namespaces/beta": ""}},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newGetAllLimitFixture(t, false)
			got := fixture.call(t, test.scope, "", test.limit, []string{"gamma", "beta", "alpha"})
			assertGetAllLimitIdentities(t, got, test.want)
			fixture.assertFetches(t, test.lists)
		})
	}
}

func TestGetAllTool_LimitScopeControls(t *testing.T) {
	for _, test := range []struct {
		name      string
		scope     string
		namespace string
		allowed   []string
		want      []string
		fetches   map[string]string
	}{
		{name: "explicit namespace", scope: "namespaced", namespace: "beta", allowed: []string{"alpha", "beta", "gamma"},
			want:    []string{"v1/ConfigMap/beta/c4", "v1/Pod/beta/p2", "v1/Pod/beta/p3"},
			fetches: map[string]string{"/api/v1/namespaces/beta/pods": "2", "/api/v1/namespaces/beta/configmaps": "2"}},
		{name: "unrestricted namespaced", scope: "namespaced",
			want:    []string{"v1/ConfigMap/alpha/c1", "v1/ConfigMap/alpha/c2", "v1/Pod/alpha/p1", "v1/Pod/beta/p2"},
			fetches: map[string]string{"/api/v1/pods": "2", "/api/v1/configmaps": "2"}},
		{name: "restricted cluster", scope: "cluster", allowed: []string{"alpha", "beta", "gamma"},
			want:    []string{"v1/Namespace//alpha", "v1/Namespace//beta", "v1/Node//worker-1", "v1/Node//worker-2"},
			fetches: map[string]string{"/api/v1/nodes": "2", "/api/v1/namespaces/alpha": "", "/api/v1/namespaces/beta": ""}},
		{name: "unrestricted cluster", scope: "cluster",
			want:    []string{"v1/Namespace//alpha", "v1/Namespace//beta", "v1/Node//worker-1", "v1/Node//worker-2"},
			fetches: map[string]string{"/api/v1/nodes": "2", "/api/v1/namespaces": "2"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newGetAllLimitFixture(t, false)
			got := fixture.call(t, test.scope, test.namespace, 2, test.allowed)
			assertGetAllLimitIdentities(t, got, test.want)
			fixture.assertFetches(t, test.fetches)
		})
	}
}

func TestGetAllTool_NamespaceBudgetCountsReadableObjectsIndependently(t *testing.T) {
	fixture := newGetAllLimitFixture(t, false)
	fixture.statuses["/api/v1/namespaces/a-missing"] = http.StatusNotFound
	fixture.statuses["/api/v1/namespaces/b-forbidden"] = http.StatusForbidden
	fixture.objects["/api/v1/namespaces/c-readable"] = getAllLimitObject("v1", "Namespace", "", "c-readable")
	fixture.objects["/api/v1/namespaces/d-unread"] = getAllLimitObject("v1", "Namespace", "", "d-unread")
	got := fixture.call(t, "cluster", "", 1, []string{"d-unread", "c-readable", "b-forbidden", "a-missing"})
	assertGetAllLimitIdentities(t, got, []string{"v1/Namespace//c-readable", "v1/Node//worker-1"})
	fixture.assertFetches(t, map[string]string{
		"/api/v1/nodes": "1", "/api/v1/namespaces/a-missing": "", "/api/v1/namespaces/b-forbidden": "", "/api/v1/namespaces/c-readable": "",
	})
}

func TestGetAllTool_SameKindInDifferentAPIGroupsHasSeparateBudget(t *testing.T) {
	fixture := newGetAllLimitFixture(t, true)
	got := fixture.call(t, "namespaced", "", 1, []string{"alpha", "beta", "gamma"})
	assertGetAllLimitIdentities(t, got, []string{
		"v1/ConfigMap/alpha/c1", "v1/Pod/alpha/p1", "first.example/v1/Gadget/alpha/g1", "second.example/v1/Gadget/alpha/g2",
	})
	fixture.assertFetches(t, map[string]string{
		"/api/v1/namespaces/alpha/pods": "1", "/api/v1/namespaces/alpha/configmaps": "1",
		"/apis/first.example/v1/namespaces/alpha/gadgets": "1", "/apis/second.example/v1/namespaces/alpha/gadgets": "1",
	})
}

type getAllLimitItem struct {
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
	Kind       string `json:"kind"`
	APIVersion string `json:"apiVersion"`
}

type getAllLimitFixture struct {
	client   *steve.Client
	groups   bool
	lists    map[string][]map[string]interface{}
	objects  map[string]map[string]interface{}
	statuses map[string]int
	mu       sync.Mutex
	fetches  []string
}

func newGetAllLimitFixture(t *testing.T, groups bool) *getAllLimitFixture {
	t.Helper()
	f := &getAllLimitFixture{groups: groups, lists: make(map[string][]map[string]interface{}), objects: make(map[string]map[string]interface{}), statuses: make(map[string]int)}
	for _, resource := range []struct {
		base, apiVersion, kind, plural string
		names                          [][]string
	}{
		{base: "/api/v1", apiVersion: "v1", kind: "Pod", plural: "pods", names: [][]string{{"p1"}, {"p2", "p3"}, {"p4"}}},
		{base: "/api/v1", apiVersion: "v1", kind: "ConfigMap", plural: "configmaps", names: [][]string{{"c1", "c2", "c3"}, {"c4"}, {}}},
		{base: "/apis/first.example/v1", apiVersion: "first.example/v1", kind: "Gadget", plural: "gadgets", names: [][]string{{"g1"}, {"g1-beta"}, {}}},
		{base: "/apis/second.example/v1", apiVersion: "second.example/v1", kind: "Gadget", plural: "gadgets", names: [][]string{{"g2"}, {"g2-beta"}, {}}},
	} {
		for i, namespace := range []string{"alpha", "beta", "gamma"} {
			path := resource.base + "/namespaces/" + namespace + "/" + resource.plural
			f.lists[path] = []map[string]interface{}{}
			for _, name := range resource.names[i] {
				obj := getAllLimitObject(resource.apiVersion, resource.kind, namespace, name)
				f.lists[path] = append(f.lists[path], obj)
				f.lists[resource.base+"/"+resource.plural] = append(f.lists[resource.base+"/"+resource.plural], obj)
			}
		}
	}
	for _, name := range []string{"alpha", "beta", "gamma"} {
		obj := getAllLimitObject("v1", "Namespace", "", name)
		f.objects["/api/v1/namespaces/"+name] = obj
		f.lists["/api/v1/namespaces"] = append(f.lists["/api/v1/namespaces"], obj)
	}
	for _, name := range []string{"worker-1", "worker-2", "worker-3"} {
		f.lists["/api/v1/nodes"] = append(f.lists["/api/v1/nodes"], getAllLimitObject("v1", "Node", "", name))
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { f.serve(t, w, r) }))
	t.Cleanup(server.Close)
	f.client = steve.NewClient(server.URL, "", "", "", false)
	return f
}

func (f *getAllLimitFixture) serve(t *testing.T, w http.ResponseWriter, r *http.Request) {
	t.Helper()
	path := strings.TrimPrefix(r.URL.Path, "/k8s/clusters/c-limit")
	w.Header().Set("Content-Type", "application/json")
	if discovery := f.discovery(path); discovery != "" {
		_, _ = w.Write([]byte(discovery))
		return
	}
	f.mu.Lock()
	f.fetches = append(f.fetches, path+"?limit="+r.URL.Query().Get("limit"))
	f.mu.Unlock()
	if code, ok := f.statuses[path]; ok {
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"apiVersion": "v1", "kind": "Status", "status": "Failure", "reason": http.StatusText(code), "code": code})
		return
	}
	if obj, ok := f.objects[path]; ok {
		_ = json.NewEncoder(w).Encode(obj)
		return
	}
	if items, ok := f.lists[path]; ok {
		limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
		if r.URL.Query().Get("limit") != "" && err != nil {
			t.Errorf("invalid API limit: %s", r.URL)
		}
		if limit > 0 && limit < len(items) {
			items = items[:limit]
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"apiVersion": "v1", "kind": "List", "items": items})
		return
	}
	t.Errorf("unexpected API request: %s", r.URL)
	http.NotFound(w, r)
}

func (f *getAllLimitFixture) discovery(path string) string {
	switch path {
	case "/api":
		return `{"kind":"APIVersions","versions":["v1"]}`
	case "/apis":
		if f.groups {
			return `{"kind":"APIGroupList","groups":[{"name":"first.example","versions":[{"groupVersion":"first.example/v1","version":"v1"}],"preferredVersion":{"groupVersion":"first.example/v1","version":"v1"}},{"name":"second.example","versions":[{"groupVersion":"second.example/v1","version":"v1"}],"preferredVersion":{"groupVersion":"second.example/v1","version":"v1"}}]}`
		}
		return `{"kind":"APIGroupList","groups":[]}`
	case "/api/v1":
		return `{"groupVersion":"v1","resources":[{"name":"namespaces","namespaced":false,"kind":"Namespace","verbs":["get","list"]},{"name":"nodes","namespaced":false,"kind":"Node","verbs":["list"]},{"name":"pods","namespaced":true,"kind":"Pod","verbs":["list"]},{"name":"configmaps","namespaced":true,"kind":"ConfigMap","verbs":["list"]}]}`
	case "/apis/first.example/v1", "/apis/second.example/v1":
		return fmt.Sprintf(`{"groupVersion":%q,"resources":[{"name":"gadgets","namespaced":true,"kind":"Gadget","verbs":["list"]}]}`, strings.TrimPrefix(path, "/apis/"))
	default:
		return ""
	}
}

func (f *getAllLimitFixture) call(t *testing.T, scope, namespace string, limit int64, allowed []string) []getAllLimitItem {
	t.Helper()
	tool := findKubernetesTool(t, "kubernetes_get_all")
	out, err := tool.Handler(context.Background(), f.client, map[string]interface{}{
		"cluster": "c-limit", "scope": scope, "namespace": namespace, "limit": limit, "format": "json",
		"namespaceAllowlist": map[string][]string{"c-limit": allowed},
	})
	if err != nil {
		t.Fatalf("kubernetes_get_all error = %v", err)
	}
	var items []getAllLimitItem
	if err := json.Unmarshal([]byte(out), &items); err != nil {
		t.Fatalf("decode tool response: %v; output=%s", err, out)
	}
	return items
}

func (f *getAllLimitFixture) assertFetches(t *testing.T, want map[string]string) {
	t.Helper()
	var expected []string
	for path, limit := range want {
		expected = append(expected, path+"?limit="+limit)
	}
	f.mu.Lock()
	got := append([]string(nil), f.fetches...)
	f.mu.Unlock()
	sort.Strings(got)
	sort.Strings(expected)
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("API fetches = %v, want %v", got, expected)
	}
}

func assertGetAllLimitIdentities(t *testing.T, items []getAllLimitItem, want []string) {
	t.Helper()
	var got []string
	for _, item := range items {
		got = append(got, item.APIVersion+"/"+item.Kind+"/"+item.Namespace+"/"+item.Name)
	}
	want = append([]string(nil), want...)
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("resource identities = %v, want %v", got, want)
	}
}

func getAllLimitObject(apiVersion, kind, namespace, name string) map[string]interface{} {
	return map[string]interface{}{"apiVersion": apiVersion, "kind": kind, "metadata": map[string]interface{}{"name": name, "namespace": namespace}}
}
