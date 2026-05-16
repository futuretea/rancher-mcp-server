package steve

import (
	"reflect"
	"strings"
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

func TestGVRMatchesAPIVersion(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}

	if !gvrMatchesAPIVersion(gvr, "apps/v1") {
		t.Fatal("gvrMatchesAPIVersion() should match same group/version")
	}
	if gvrMatchesAPIVersion(gvr, "apps/v2") {
		t.Fatal("gvrMatchesAPIVersion() should not match different version")
	}
	if gvrMatchesAPIVersion(gvr, "batch/v1") {
		t.Fatal("gvrMatchesAPIVersion() should not match different group")
	}
	if gvrMatchesAPIVersion(gvr, "invalid") {
		t.Fatal("gvrMatchesAPIVersion() should not match invalid apiVersion")
	}
}

func TestFindPreferredGroupVersion(t *testing.T) {
	groups := &metav1.APIGroupList{
		Groups: []metav1.APIGroup{
			{Name: "apps", PreferredVersion: metav1.GroupVersionForDiscovery{GroupVersion: "apps/v1"}},
			{Name: "batch", PreferredVersion: metav1.GroupVersionForDiscovery{GroupVersion: "batch/v1"}},
		},
	}

	gv, ok := findPreferredGroupVersion(groups, "apps")
	if !ok {
		t.Fatal("findPreferredGroupVersion() ok = false for existing group")
	}
	if gv != "apps/v1" {
		t.Errorf("expected apps/v1, got %s", gv)
	}

	_, ok = findPreferredGroupVersion(groups, "nonexistent")
	if ok {
		t.Fatal("findPreferredGroupVersion() ok = true for missing group")
	}
}

func TestAppendUniqueGVR(t *testing.T) {
	gvr1 := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	gvr2 := schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}

	matches := appendUniqueGVR(nil, gvr1)
	if len(matches) != 1 || matches[0] != gvr1 {
		t.Fatalf("expected [gvr1], got %v", matches)
	}

	matches = appendUniqueGVR(matches, gvr1)
	if len(matches) != 1 {
		t.Fatalf("duplicate should not be appended, got %d", len(matches))
	}

	matches = appendUniqueGVR(matches, gvr2)
	if len(matches) != 2 {
		t.Fatalf("expected 2 unique GVRs, got %d", len(matches))
	}
}

func TestDescribeGVRMatches(t *testing.T) {
	matches := []schema.GroupVersionResource{
		{Group: "apps", Version: "v1", Resource: "deployments"},
		{Group: "", Version: "v1", Resource: "pods"},
		{Group: "batch", Version: "v1beta1", Resource: "cronjobs"},
	}

	got := describeGVRMatches(matches)
	if got == "" {
		t.Fatal("expected non-empty description")
	}
	if !strings.Contains(got, "deployments") {
		t.Fatal("expected deployments in output")
	}
	if !strings.Contains(got, "v1 pods") {
		t.Fatal("expected 'v1 pods' for core group")
	}
	if !strings.Contains(got, "batch/v1beta1") {
		t.Fatal("expected batch/v1beta1 in output")
	}
}

func TestBuildEventFieldSelector(t *testing.T) {
	got := buildEventFieldSelector("my-pod", "default", "Pod")
	if got != "involvedObject.name=my-pod,involvedObject.namespace=default,involvedObject.kind=Pod" {
		t.Errorf("unexpected selector: %s", got)
	}

	got = buildEventFieldSelector("", "", "")
	if got != "" {
		t.Errorf("expected empty selector, got %s", got)
	}

	got = buildEventFieldSelector("test", "", "")
	if got != "involvedObject.name=test" {
		t.Errorf("expected name-only selector, got %s", got)
	}
}

func TestHasVerb(t *testing.T) {
	verbs := []string{"get", "list", "watch"}

	if !hasVerb(verbs, "list") {
		t.Fatal("hasVerb() should find 'list'")
	}
	if hasVerb(verbs, "delete") {
		t.Fatal("hasVerb() should not find 'delete'")
	}
	if hasVerb(nil, "get") {
		t.Fatal("hasVerb() should return false for nil slice")
	}
}

func TestAPIResourceInfo_GVR(t *testing.T) {
	info := &APIResourceInfo{
		Name:    "deployments",
		Group:   "apps",
		Version: "v1",
	}
	gvr := info.GVR()
	if gvr.Group != "apps" || gvr.Version != "v1" || gvr.Resource != "deployments" {
		t.Errorf("unexpected GVR: %#v", gvr)
	}
}

func TestToJSON(t *testing.T) {
	r := &InspectPodResult{}
	jsonStr, err := r.ToJSON()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(jsonStr, "pod") {
		t.Fatal("expected JSON to contain pod field")
	}

	d := &DescribeResult{}
	jsonStr, err = d.ToJSON()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(jsonStr, "resource") {
		t.Fatal("expected JSON to contain resource field")
	}
}
