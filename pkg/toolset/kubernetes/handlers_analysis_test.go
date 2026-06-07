package kubernetes

import "testing"

func TestParseNumeric(t *testing.T) {
	tests := []struct {
		s    string
		want int64
		err  bool
	}{
		{"0", 0, false},
		{"42", 42, false},
		{"-1", -1, false},
		{"999999", 999999, false},
		{"", 0, true},
		{"abc", 0, true},
	}
	for _, tt := range tests {
		got, err := parseNumeric(tt.s)
		if tt.err && err == nil {
			t.Errorf("parseNumeric(%q) expected error", tt.s)
		}
		if !tt.err && err != nil {
			t.Errorf("parseNumeric(%q) unexpected error: %v", tt.s, err)
		}
		if !tt.err && got != tt.want {
			t.Errorf("parseNumeric(%q) = %d, want %d", tt.s, got, tt.want)
		}
	}
}

func TestParseResourceQuantity(t *testing.T) {
	tests := []struct {
		q    string
		want int64
	}{
		{"", 0},
		{"100m", 100},
		{"1500m", 1500},
		{"0m", 0},
		{"128Ki", 128 * 1024},
		{"10Mi", 10 * 1024 * 1024},
		{"2Gi", 2 * 1024 * 1024 * 1024},
		{"1Ti", 1024 * 1024 * 1024 * 1024},
		{"1K", 1000},
		{"1k", 1000},
		{"2M", 2 * 1000 * 1000},
		{"3G", 3 * 1000 * 1000 * 1000},
		{"500", 500},
		{"abcMi", 0},
		{"invalid", 0},
	}
	for _, tt := range tests {
		t.Run(tt.q, func(t *testing.T) {
			got := parseResourceQuantity(tt.q)
			if got != tt.want {
				t.Errorf("parseResourceQuantity(%q) = %d, want %d", tt.q, got, tt.want)
			}
		})
	}
}

func TestFormatResourceQuantity(t *testing.T) {
	t.Run("cpu millicores below 1000", func(t *testing.T) {
		got := formatResourceQuantity(500, "cpu")
		if got != "500m" {
			t.Errorf("expected '500m', got %q", got)
		}
	})

	t.Run("cpu millicores above 1000", func(t *testing.T) {
		got := formatResourceQuantity(1500, "cpu")
		if got != "1500m (1c)" {
			t.Errorf("expected '1500m (1c)', got %q", got)
		}
	})

	t.Run("memory in Gi", func(t *testing.T) {
		got := formatResourceQuantity(2*1024*1024*1024, "memory")
		if got != "2Gi (2147483648 bytes)" {
			t.Errorf("expected '2Gi (2147483648 bytes)', got %q", got)
		}
	})

	t.Run("memory in Mi", func(t *testing.T) {
		got := formatResourceQuantity(128*1024*1024, "memory")
		if got != "128Mi (134217728 bytes)" {
			t.Errorf("expected '128Mi (134217728 bytes)', got %q", got)
		}
	})

	t.Run("memory in bytes", func(t *testing.T) {
		got := formatResourceQuantity(500, "memory")
		if got != "500 bytes" {
			t.Errorf("expected '500 bytes', got %q", got)
		}
	})
}

func TestBuildDepRequest_RejectsMismatchedScanNamespace(t *testing.T) {
	_, err := buildDepRequest(map[string]interface{}{
		"cluster":       "c1",
		"kind":          "deployment",
		"namespace":     "default",
		"name":          "demo",
		"scanNamespace": "kube-system",
	})
	if err == nil {
		t.Fatal("expected mismatched scan namespace error")
	}
}

func TestBuildDepRequest_DefaultsScanNamespaceToRootNamespace(t *testing.T) {
	request, err := buildDepRequest(map[string]interface{}{
		"cluster":   "c1",
		"kind":      "deployment",
		"namespace": "default",
		"name":      "demo",
	})
	if err != nil {
		t.Fatalf("buildDepRequest() returned unexpected error: %v", err)
	}
	if request.ResolveOptions.ScanNamespace != "default" {
		t.Fatalf("expected default scan namespace to match root namespace, got %q", request.ResolveOptions.ScanNamespace)
	}
}
