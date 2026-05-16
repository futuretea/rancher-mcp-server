package kubernetes

import "testing"

func TestFormatRolloutHistoryAsTable(t *testing.T) {
	t.Run("empty history", func(t *testing.T) {
		got := formatRolloutHistoryAsTable(nil)
		if got != "No rollout history found" {
			t.Fatalf("expected 'No rollout history found', got %q", got)
		}
	})

	t.Run("with revisions", func(t *testing.T) {
		history := []RevisionInfo{
			{Revision: "3", ChangeCause: "scale up", Created: "2024-01-15T10:30:00Z", Name: "nginx-abc123"},
			{Revision: "2", ChangeCause: "", Created: "2024-01-14T08:00:00Z", Name: "nginx-def456"},
		}
		got := formatRolloutHistoryAsTable(history)
		if got == "" || got == "No rollout history found" {
			t.Fatal("expected table output")
		}
		// Should contain revision numbers
		if !containsStr(got, "3") || !containsStr(got, "2") {
			t.Errorf("expected revision numbers in output: %s", got)
		}
		// Empty change cause should show "-"
		if !containsStr(got, "-") {
			t.Errorf("expected '-' for empty change cause: %s", got)
		}
		// Should have a header
		if !containsStr(got, "REVISION") {
			t.Errorf("expected REVISION header: %s", got)
		}
	})
}
