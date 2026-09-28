package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/futuretea/rancher-mcp-server/pkg/client/steve"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestAllowlistNamespaceLimit(t *testing.T) {
	t.Cleanup(resetNamespaceAllowlist)
	names := namespaceLimitNames(1000)
	SetNamespaceAllowlist(map[string][]string{"c-test": names})
	for _, test := range []struct {
		name       string
		opts       *steve.ListOptions
		want       int
		incomplete bool
	}{
		{name: "watch bound", opts: &steve.ListOptions{Limit: 201}, want: 201, incomplete: true},
		{name: "nil unlimited", want: 1000},
		{name: "zero unlimited", opts: &steve.ListOptions{}, want: 1000},
		{name: "exact limit complete", opts: &steve.ListOptions{Limit: 1000}, want: 1000},
	} {
		t.Run(test.name, func(t *testing.T) {
			reader := newRecordingReader()
			for _, name := range names {
				reader.inner.AddResource(namespaceObject(name, nil))
			}
			list, err := listResourcesAllowed(context.Background(), reader, "c-test", "Namespace", "", test.opts)
			if err != nil {
				t.Fatal(err)
			}
			if len(list.Items) != test.want || len(reader.calls) != test.want {
				t.Errorf("items=%d requests=%d, want %d of each", len(list.Items), len(reader.calls), test.want)
			}
			if got := list.GetContinue() != ""; got != test.incomplete {
				t.Errorf("Continue=%q, want incomplete=%t", list.GetContinue(), test.incomplete)
			}
			for _, call := range reader.calls {
				if call.op != "get" || call.kind != "namespace" || call.namespace != "" {
					t.Errorf("backend call=%+v, want Namespace Get", call)
				}
			}
		})
	}
}

func TestAllowlistNamespaceLimitCountsMatches(t *testing.T) {
	t.Cleanup(resetNamespaceAllowlist)
	SetNamespaceAllowlist(map[string][]string{"c-test": {"a-missing", "b-label-mismatch", "c-field-mismatch", "d-match", "e-match", "f-unread"}})
	reader := newRecordingReader()
	for _, name := range []string{"b-label-mismatch", "c-field-mismatch", "d-match", "e-match", "f-unread"} {
		item := namespaceObject(name, map[string]string{"env": "prod"})
		item.Object["status"] = map[string]interface{}{"phase": "Active"}
		if name == "b-label-mismatch" {
			item.SetLabels(map[string]string{"env": "dev"})
		}
		if name == "c-field-mismatch" {
			item.Object["status"] = map[string]interface{}{"phase": "Terminating"}
		}
		reader.inner.AddResource(item)
	}
	opts := &steve.ListOptions{Limit: 2, LabelSelector: "env=prod", FieldSelector: "status.phase=Active"}
	before := *opts
	list, err := listResourcesAllowed(context.Background(), reader, "c-test", "Namespace", "", opts)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, item := range list.Items {
		got = append(got, item.GetName())
	}
	if !reflect.DeepEqual(got, []string{"d-match", "e-match"}) || len(reader.calls) != 5 || list.GetContinue() == "" {
		t.Errorf("names=%v requests=%d Continue=%q, want two matches, five requests, incomplete", got, len(reader.calls), list.GetContinue())
	}
	if *opts != before {
		t.Errorf("caller options mutated: got %+v, want %+v", *opts, before)
	}
}

type namespaceGetFailureReader struct {
	*recordingReader
	err error
}

func (r *namespaceGetFailureReader) GetResource(context.Context, string, string, string, string) (*unstructured.Unstructured, error) {
	return nil, r.err
}

func TestAllowlistNamespaceLimitPropagatesErrors(t *testing.T) {
	t.Cleanup(resetNamespaceAllowlist)
	SetNamespaceAllowlist(map[string][]string{"c-test": {"app"}})
	want := errors.New("namespace read failed")
	reader := &namespaceGetFailureReader{recordingReader: newRecordingReader(), err: want}
	_, err := listResourcesAllowed(context.Background(), reader, "c-test", "Namespace", "", &steve.ListOptions{Limit: 1})
	if !errors.Is(err, want) {
		t.Fatalf("error=%v, want %v", err, want)
	}
}

func TestAllowlistNamespaceWatchHTTPStopsAtLimit(t *testing.T) {
	t.Cleanup(resetNamespaceAllowlist)
	SetNamespaceAllowlist(map[string][]string{"c-test": namespaceLimitNames(4)})
	client, requests := newAllowlistHTTPClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || !strings.HasPrefix(r.URL.Path, "/api/v1/namespaces/ns-") {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/api/v1/namespaces/")
		_, _ = fmt.Fprintf(w, `{"apiVersion":"v1","kind":"Namespace","metadata":{"name":%q}}`, name)
	})
	// A small watch budget exercises real HTTP reads without client-side throttling.
	_, err := watchDiffWithReader(context.Background(), client, &watchRequest{
		cluster: "c-test", kind: "Namespace", iterations: 1, maxItems: 2,
	})
	if err == nil || !strings.Contains(err.Error(), "limit") {
		t.Errorf("watch error=%v, want incomplete snapshot rejection", err)
	}
	if got := len(requests()); got != 3 {
		t.Errorf("HTTP requests=%d, want 3 to detect the two-object watch limit", got)
	}
}

func namespaceLimitNames(count int) []string {
	names := make([]string, count)
	for i := range names {
		names[i] = fmt.Sprintf("ns-%04d", i)
	}
	return names
}
