package kubernetes

import (
	"testing"
	"time"
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
		// Invalid
		{"abc", 0, true},
		{"", 0, true},
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
