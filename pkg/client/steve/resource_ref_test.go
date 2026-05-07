package steve

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestKindWithAPIVersion(t *testing.T) {
	tests := []struct {
		name       string
		apiVersion string
		kind       string
		want       string
	}{
		{
			name:       "kind only",
			apiVersion: "",
			kind:       "App",
			want:       "App",
		},
		{
			name:       "api version and kind",
			apiVersion: "catalog.cattle.io/v1",
			kind:       "App",
			want:       "catalog.cattle.io/v1/App",
		},
		{
			name:       "trims whitespace",
			apiVersion: " catalog.cattle.io/v1 ",
			kind:       " App ",
			want:       "catalog.cattle.io/v1/App",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := KindWithAPIVersion(tt.apiVersion, tt.kind); got != tt.want {
				t.Fatalf("KindWithAPIVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseAPIVersionKind(t *testing.T) {
	apiVersion, kind, ok := parseAPIVersionKind("catalog.cattle.io/v1/App")
	if !ok {
		t.Fatal("parseAPIVersionKind() ok = false, want true")
	}
	if apiVersion != "catalog.cattle.io/v1" {
		t.Fatalf("apiVersion = %q, want catalog.cattle.io/v1", apiVersion)
	}
	if kind != "App" {
		t.Fatalf("kind = %q, want App", kind)
	}

	if _, _, ok := parseAPIVersionKind("apps.catalog.cattle.io"); ok {
		t.Fatal("parseAPIVersionKind() ok = true for dotted kind, want false")
	}
}

func TestParseDottedKindCandidates(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []dottedKindCandidate
	}{
		{
			name: "resource group form",
			in:   "apps.catalog.cattle.io",
			want: []dottedKindCandidate{
				{resource: "apps", apiGroup: "catalog.cattle.io"},
				{resource: "io", apiGroup: "apps.catalog.cattle"},
			},
		},
		{
			name: "steve type form",
			in:   "catalog.cattle.io.app",
			want: []dottedKindCandidate{
				{resource: "catalog", apiGroup: "cattle.io.app"},
				{resource: "app", apiGroup: "catalog.cattle.io"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseDottedKindCandidates(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseDottedKindCandidates() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestMatchesResourceName(t *testing.T) {
	resource := metav1.APIResource{
		Name:         "apps",
		SingularName: "app",
		Kind:         "App",
	}

	for _, name := range []string{"app", "apps", "App"} {
		if !matchesResourceName(resource, name) {
			t.Fatalf("matchesResourceName(%q) = false, want true", name)
		}
	}
}

func TestFindAPIResourceGVRByAPIVersionAndKind(t *testing.T) {
	resources := []metav1.APIResource{
		{Name: "apps/status", Kind: "App"},
		{Name: "apps", SingularName: "app", Kind: "App"},
	}

	got, ok := findAPIResourceGVR("catalog.cattle.io/v1", "App", resources)
	if !ok {
		t.Fatal("findAPIResourceGVR() ok = false, want true")
	}

	want := schema.GroupVersionResource{
		Group:    "catalog.cattle.io",
		Version:  "v1",
		Resource: "apps",
	}
	if got != want {
		t.Fatalf("findAPIResourceGVR() = %#v, want %#v", got, want)
	}
}
