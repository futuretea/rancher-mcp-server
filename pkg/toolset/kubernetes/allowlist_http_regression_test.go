package kubernetes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/futuretea/rancher-mcp-server/pkg/client/steve"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset/kubernetes/aggregate"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
)

func TestAllowlistGetAllPreservesReadableResources(t *testing.T) {
	for _, test := range []struct {
		name            string
		restricted      bool
		scope           string
		want            []string
		cancelNamespace bool
	}{
		{name: "unrestricted", want: []string{"Node/worker-1", "Pod/web"}},
		{name: "restricted-default", restricted: true, want: []string{"Namespace/readable", "Node/worker-1", "Pod/web"}},
		{name: "restricted-cluster", restricted: true, scope: "cluster", want: []string{"Namespace/readable", "Node/worker-1"}},
		{name: "cancel-namespace-read", restricted: true, scope: "cluster", cancelNamespace: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			resetNamespaceAllowlist()
			t.Cleanup(resetNamespaceAllowlist)
			if test.restricted {
				SetNamespaceAllowlist(map[string][]string{"c-test": {"app", "readable"}})
			}
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			client, requests := newAllowlistHTTPClient(t, func(w http.ResponseWriter, r *http.Request) {
				if test.cancelNamespace && r.URL.Path == "/api/v1/namespaces/app" {
					cancel()
				}
				writeAllowlistHTTPResources(t, w, r)
			})
			out, err := getAllHandler(ctx, client, map[string]interface{}{
				"cluster": "c-test", "scope": test.scope, "format": "json",
			})
			if test.cancelNamespace {
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("error=%v; want context cancellation; requests=%v", err, requests())
				}
				return
			}
			if err != nil {
				t.Fatalf("readable resources lost: %v; requests=%v", err, requests())
			}
			var items []simpleItem
			if err := json.Unmarshal([]byte(out), &items); err != nil {
				t.Fatal(err)
			}
			identities := make(map[string]bool)
			for _, item := range items {
				identities[item.Kind+"/"+item.Name] = true
			}
			var got []string
			for identity := range identities {
				got = append(got, identity)
			}
			sort.Strings(got)
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("resources=%v, want %v; requests=%v", got, test.want, requests())
			}
			if test.restricted {
				for _, request := range requests() {
					if request.path == "/api/v1/namespaces" || request.path == "/api/v1/pods" || strings.Contains(request.path, "kube-system") {
						t.Errorf("allowlist expanded by request: %+v", request)
					}
				}
			}
		})
	}
}

func TestAllowlistEventsPreserveObjectNamespace(t *testing.T) {
	for _, entry := range []struct {
		name    string
		handler func(context.Context, interface{}, map[string]interface{}) (string, error)
	}{
		{name: "events", handler: eventsHandler},
		{name: "describe", handler: describeHandler},
		{name: "summary", handler: eventSummaryHandler},
	} {
		cases := []struct {
			name       string
			restricted bool
			namespace  string
			kind       string
			objectName string
			reason     string
			denied     bool
		}{
			{name: "unrestricted-node", kind: "Node", objectName: "worker-1", reason: "NodeNotReady"},
			{name: "restricted-node", restricted: true, kind: "Node", objectName: "worker-1", reason: "NodeNotReady"},
			{name: "restricted-pod", restricted: true, namespace: "default", kind: "Pod", objectName: "web", reason: "Started"},
			{name: "denied-namespace", restricted: true, namespace: "kube-system", kind: "Pod", objectName: "web", denied: true},
			{name: "node-in-explicit-namespace", restricted: true, namespace: "default", kind: "Node", objectName: "worker-1", reason: "NodeNotReady"},
			{name: "pod-without-namespace", restricted: true, kind: "Pod", objectName: "web", reason: "Started"},
		}
		for _, test := range cases {
			if entry.name == "describe" && (test.name == "node-in-explicit-namespace" || test.name == "pod-without-namespace") {
				// Describe requires the object's namespace, unlike event queries.
				continue
			}
			t.Run(entry.name+"/"+test.name, func(t *testing.T) {
				resetNamespaceAllowlist()
				t.Cleanup(resetNamespaceAllowlist)
				if test.restricted {
					SetNamespaceAllowlist(map[string][]string{"c-test": {"default"}})
				}
				client, requests := newAllowlistHTTPClient(t, func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/api/v1/nodes/worker-1":
						_, _ = w.Write([]byte(`{"apiVersion":"v1","kind":"Node","metadata":{"name":"worker-1"}}`))
					case "/api/v1/namespaces/default/pods/web":
						_, _ = w.Write([]byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"web","namespace":"default"}}`))
					case "/api/v1/events", "/api/v1/namespaces/default/events":
						writeAllowlistHTTPEvents(t, w, r)
					default:
						t.Errorf("unexpected request: %s", r.URL)
						http.NotFound(w, r)
					}
				})
				out, err := entry.handler(context.Background(), client, map[string]interface{}{
					"cluster": "c-test", "namespace": test.namespace, "kind": test.kind,
					"name": test.objectName, "format": "json",
				})
				if test.denied {
					if err == nil || len(requests()) != 0 {
						t.Fatalf("error=%v requests=%v; want denial before HTTP", err, requests())
					}
					return
				}
				if err != nil {
					t.Fatalf("%s: %v; requests=%v", entry.name, err, requests())
				}
				assertAllowlistEventOutput(t, entry.name, out, test.reason, requests())
				for _, request := range requests() {
					if test.restricted && (request.path == "/api/v1/events" || strings.Contains(request.path, "kube-system")) {
						t.Errorf("allowlist expanded by request: %+v", request)
					}
				}
			})
		}
	}
}

type allowlistHTTPRequest struct {
	path     string
	selector string
}

func writeAllowlistHTTPResources(t *testing.T, w http.ResponseWriter, r *http.Request) {
	t.Helper()
	switch r.URL.Path {
	case "/api":
		_, _ = w.Write([]byte(`{"kind":"APIVersions","versions":["v1"]}`))
	case "/apis":
		_, _ = w.Write([]byte(`{"kind":"APIGroupList","groups":[]}`))
	case "/api/v1":
		_, _ = w.Write([]byte(`{"groupVersion":"v1","resources":[{"name":"namespaces","namespaced":false,"kind":"Namespace","verbs":["list","get"]},{"name":"nodes","namespaced":false,"kind":"Node","verbs":["list"]},{"name":"pods","namespaced":true,"kind":"Pod","verbs":["list"]}]}`))
	case "/api/v1/namespaces", "/api/v1/namespaces/app":
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"apiVersion":"v1","kind":"Status","status":"Failure","reason":"Forbidden","code":403,"message":"namespace access denied"}`))
	case "/api/v1/namespaces/readable":
		_, _ = w.Write([]byte(`{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"readable"}}`))
	case "/api/v1/nodes":
		_, _ = w.Write([]byte(`{"apiVersion":"v1","kind":"NodeList","items":[{"apiVersion":"v1","kind":"Node","metadata":{"name":"worker-1"}}]}`))
	case "/api/v1/pods", "/api/v1/namespaces/app/pods":
		_, _ = w.Write([]byte(`{"apiVersion":"v1","kind":"PodList","items":[{"apiVersion":"v1","kind":"Pod","metadata":{"name":"web","namespace":"app"}}]}`))
	case "/api/v1/namespaces/readable/pods":
		_, _ = w.Write([]byte(`{"apiVersion":"v1","kind":"PodList","items":[]}`))
	default:
		t.Errorf("unexpected request: %s", r.URL)
		http.NotFound(w, r)
	}
}

func newAllowlistHTTPClient(t *testing.T, handler http.HandlerFunc) (*steve.Client, func() []allowlistHTTPRequest) {
	t.Helper()
	var mu sync.Mutex
	var requests []allowlistHTTPRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = strings.TrimPrefix(r.URL.Path, "/k8s/clusters/c-test")
		mu.Lock()
		requests = append(requests, allowlistHTTPRequest{r.URL.Path, r.URL.Query().Get("fieldSelector")})
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	return steve.NewClient(server.URL, "", "", "", false), func() []allowlistHTTPRequest {
		mu.Lock()
		defer mu.Unlock()
		return append([]allowlistHTTPRequest(nil), requests...)
	}
}

func writeAllowlistHTTPEvents(t *testing.T, w http.ResponseWriter, r *http.Request) {
	t.Helper()
	selector, err := fields.ParseSelector(r.URL.Query().Get("fieldSelector"))
	if err != nil {
		t.Error(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	list := corev1.EventList{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "EventList"}, Items: []corev1.Event{}}
	for _, event := range []corev1.Event{
		{ObjectMeta: metav1.ObjectMeta{Name: "node-event", Namespace: "default"}, InvolvedObject: corev1.ObjectReference{Kind: "Node", Name: "worker-1"}, Reason: "NodeNotReady"},
		{ObjectMeta: metav1.ObjectMeta{Name: "other-node-event", Namespace: "default"}, InvolvedObject: corev1.ObjectReference{Kind: "Node", Name: "worker-2"}, Reason: "NodeNotReady"},
		{ObjectMeta: metav1.ObjectMeta{Name: "pod-event", Namespace: "default"}, InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "web", Namespace: "default"}, Reason: "Started"},
		{ObjectMeta: metav1.ObjectMeta{Name: "other-pod-event", Namespace: "default"}, InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "api", Namespace: "default"}, Reason: "Started"},
	} {
		if selector.Matches(fields.Set{
			"involvedObject.name": event.InvolvedObject.Name, "involvedObject.kind": event.InvolvedObject.Kind,
			"involvedObject.namespace": event.InvolvedObject.Namespace,
		}) {
			list.Items = append(list.Items, event)
		}
	}
	if err := json.NewEncoder(w).Encode(list); err != nil {
		t.Error(err)
	}
}

func assertAllowlistEventOutput(t *testing.T, entry, out, reason string, requests []allowlistHTTPRequest) {
	t.Helper()
	var reasons []string
	var err error
	switch entry {
	case "summary":
		var result aggregate.EventResult
		err = json.Unmarshal([]byte(out), &result)
		for _, item := range result.Items {
			reasons = append(reasons, fmt.Sprintf("%s/%d", item.Reason, item.Count))
		}
		reason += "/2"
	default:
		var events []corev1.Event
		if entry == "describe" {
			var result steve.DescribeResult
			err = json.Unmarshal([]byte(out), &result)
			events = result.Events
		} else {
			err = json.Unmarshal([]byte(out), &events)
		}
		for _, event := range events {
			reasons = append(reasons, event.Reason)
		}
	}
	if err != nil || !reflect.DeepEqual(reasons, []string{reason}) {
		t.Fatalf("event reasons=%v, want [%s], decode error=%v; output=%s; requests=%v", reasons, reason, err, out, requests)
	}
}
