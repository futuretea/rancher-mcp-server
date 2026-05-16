package kubernetes

import "testing"

func TestExtractStringParam(t *testing.T) {
	params := map[string]interface{}{
		"kind":  "pod",
		"count": 42,
	}

	t.Run("present string value", func(t *testing.T) {
		got := extractStringParam(params, "kind", "default")
		if got != "pod" {
			t.Fatalf("expected 'pod', got %q", got)
		}
	})

	t.Run("missing key returns default", func(t *testing.T) {
		got := extractStringParam(params, "missing", "fallback")
		if got != "fallback" {
			t.Fatalf("expected 'fallback', got %q", got)
		}
	})

	t.Run("non-string value returns default", func(t *testing.T) {
		got := extractStringParam(params, "count", "fallback")
		if got != "fallback" {
			t.Fatalf("expected 'fallback' for non-string, got %q", got)
		}
	})

	t.Run("empty default", func(t *testing.T) {
		got := extractStringParam(params, "missing", "")
		if got != "" {
			t.Fatalf("expected empty, got %q", got)
		}
	})
}

func TestExtractIntParam(t *testing.T) {
	params := map[string]interface{}{
		"asFloat": float64(50),
		"asInt":   100,
		"asInt64": int64(200),
		"asStr":   "not-a-number",
	}

	t.Run("float64 value", func(t *testing.T) {
		got := extractIntParam(params, "asFloat", 10)
		if got != 50 {
			t.Fatalf("expected 50, got %d", got)
		}
	})

	t.Run("int value", func(t *testing.T) {
		got := extractIntParam(params, "asInt", 10)
		if got != 100 {
			t.Fatalf("expected 100, got %d", got)
		}
	})

	t.Run("int64 value", func(t *testing.T) {
		got := extractIntParam(params, "asInt64", 10)
		if got != 200 {
			t.Fatalf("expected 200, got %d", got)
		}
	})

	t.Run("missing key returns default", func(t *testing.T) {
		got := extractIntParam(params, "missing", 42)
		if got != 42 {
			t.Fatalf("expected 42, got %d", got)
		}
	})

	t.Run("string value returns default", func(t *testing.T) {
		got := extractIntParam(params, "asStr", 10)
		if got != 10 {
			t.Fatalf("expected 10 for string, got %d", got)
		}
	})

	t.Run("zero default", func(t *testing.T) {
		got := extractIntParam(params, "missing", 0)
		if got != 0 {
			t.Fatalf("expected 0, got %d", got)
		}
	})
}
