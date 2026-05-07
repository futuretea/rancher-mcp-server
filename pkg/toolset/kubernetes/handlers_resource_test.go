package kubernetes

import "testing"

func TestExtractResourceKindWithAPIVersion(t *testing.T) {
	params := map[string]interface{}{
		"apiVersion": "catalog.cattle.io/v1",
		"kind":       "App",
	}

	got, err := extractResourceKind(params)
	if err != nil {
		t.Fatalf("extractResourceKind() unexpected error: %v", err)
	}
	if got != "catalog.cattle.io/v1/App" {
		t.Fatalf("extractResourceKind() = %q, want catalog.cattle.io/v1/App", got)
	}
}

func TestExtractResourceKindWithoutAPIVersion(t *testing.T) {
	params := map[string]interface{}{
		"kind": "deployment",
	}

	got, err := extractResourceKind(params)
	if err != nil {
		t.Fatalf("extractResourceKind() unexpected error: %v", err)
	}
	if got != "deployment" {
		t.Fatalf("extractResourceKind() = %q, want deployment", got)
	}
}
