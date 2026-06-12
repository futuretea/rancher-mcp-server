package kubernetes

import (
	"strings"
	"testing"
	"time"

	"github.com/futuretea/rancher-mcp-server/pkg/client/steve"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestMatchesLabelSelector(t *testing.T) {
	labels := map[string]string{"app": "nginx", "env": "prod", "tier": "frontend"}

	t.Run("empty selector matches all", func(t *testing.T) {
		if !matchesLabelSelector(labels, "") {
			t.Fatal("empty selector should match")
		}
	})

	t.Run("single equality match", func(t *testing.T) {
		if !matchesLabelSelector(labels, "app=nginx") {
			t.Fatal("expected match on app=nginx")
		}
	})

	t.Run("single equality mismatch", func(t *testing.T) {
		if matchesLabelSelector(labels, "app=redis") {
			t.Fatal("expected no match on app=redis")
		}
	})

	t.Run("multiple equality all match", func(t *testing.T) {
		if !matchesLabelSelector(labels, "app=nginx,env=prod") {
			t.Fatal("expected match on both labels")
		}
	})

	t.Run("multiple equality partial mismatch", func(t *testing.T) {
		if matchesLabelSelector(labels, "app=nginx,env=dev") {
			t.Fatal("expected no match on env=dev")
		}
	})

	t.Run("existence check - key exists", func(t *testing.T) {
		if !matchesLabelSelector(labels, "tier") {
			t.Fatal("expected match when key exists")
		}
	})

	t.Run("existence check - key missing", func(t *testing.T) {
		if matchesLabelSelector(labels, "missing") {
			t.Fatal("expected no match when key missing")
		}
	})

	t.Run("nil labels", func(t *testing.T) {
		if matchesLabelSelector(nil, "app=nginx") {
			t.Fatal("expected no match with nil labels")
		}
	})
}

func TestFormatAllResources(t *testing.T) {
	items := []steve.AllResourceItem{
		{Name: "pod-1", Namespace: "default", Kind: "Pod", APIVersion: "v1"},
		{Name: "node-1", Namespace: "", Kind: "Node", APIVersion: "v1"},
	}

	t.Run("json", func(t *testing.T) {
		out, err := formatAllResources(items, "json")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out, "\"name\"") {
			t.Error("expected JSON output")
		}
	})

	t.Run("yaml", func(t *testing.T) {
		out, err := formatAllResources(items, "yaml")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out, "name:") {
			t.Error("expected YAML output")
		}
	})

	t.Run("table", func(t *testing.T) {
		out, err := formatAllResources(items, "table")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out, "NAME") {
			t.Error("expected table header")
		}
		if !strings.Contains(out, "pod-1") {
			t.Error("expected pod-1 in table")
		}
	})

	t.Run("table with namespace shows dash", func(t *testing.T) {
		out, err := formatAllResources(items, "table")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out, "-") {
			t.Error("expected - for empty namespace")
		}
	})
}

func TestFormatAllResources_Empty(t *testing.T) {
	out, err := formatAllResources(nil, "table")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "No resources found" {
		t.Errorf("expected 'No resources found', got %q", out)
	}
}

func TestFormatAllResources_JSON_WithResource(t *testing.T) {
	items := []steve.AllResourceItem{
		{
			Name: "pod-1", Namespace: "default", Kind: "Pod", APIVersion: "v1",
			Resource: func() *unstructured.Unstructured {
				u := &unstructured.Unstructured{}
				u.SetUnstructuredContent(map[string]interface{}{"status": map[string]interface{}{"phase": "Running"}})
				return u
			}(),
		},
	}

	out, err := formatAllResources(items, "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "\"resource\"") {
		t.Error("expected resource field in JSON")
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		s    string
		want time.Duration
		err  bool
	}{
		// Standard Go durations
		{"30s", 30 * time.Second, false},
		{"5m", 5 * time.Minute, false},
		{"2h", 2 * time.Hour, false},
		{"1h30m", 90 * time.Minute, false},
		// Custom: days and weeks
		{"1d", 24 * time.Hour, false},
		{"2d", 48 * time.Hour, false},
		{"1w", 7 * 24 * time.Hour, false},
		{"2d12h", 60 * time.Hour, false},
		{"1w2d", 9 * 24 * time.Hour, false},
		// Standard edge cases
		{"1.5h", 90 * time.Minute, false},
		{"-30m", -30 * time.Minute, false},
		// Invalid
		{"abc", 0, true},
		{"", 0, true},
		{"1h30", 0, true},
		{"30", 0, true},
		{"1x", 0, true},
		{"h", 0, true},
		{"1h 30m", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.s, func(t *testing.T) {
			got, err := parseDuration(tt.s)
			if tt.err && err == nil {
				t.Errorf("parseDuration(%q) expected error", tt.s)
			}
			if !tt.err && err != nil {
				t.Errorf("parseDuration(%q) unexpected error: %v", tt.s, err)
			}
			if !tt.err && got != tt.want {
				t.Errorf("parseDuration(%q) = %v, want %v", tt.s, got, tt.want)
			}
		})
	}
}
