package kubernetes

import (
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestEventTime(t *testing.T) {
	now := time.Now()

	t.Run("prefers LastTimestamp", func(t *testing.T) {
		e := corev1.Event{
			LastTimestamp: metav1.NewTime(now),
			EventTime:     metav1.NewMicroTime(now.Add(-1 * time.Hour)),
		}
		got := eventTime(e)
		if !got.Equal(now) {
			t.Errorf("expected LastTimestamp (%v), got %v", now, got)
		}
	})

	t.Run("falls back to EventTime", func(t *testing.T) {
		e := corev1.Event{
			EventTime: metav1.NewMicroTime(now),
		}
		got := eventTime(e)
		if !got.Equal(now) {
			t.Errorf("expected EventTime (%v), got %v", now, got)
		}
	})

	t.Run("zero when both empty", func(t *testing.T) {
		e := corev1.Event{}
		got := eventTime(e)
		if !got.IsZero() {
			t.Errorf("expected zero time, got %v", got)
		}
	})
}

func TestFormatEventsAsTable(t *testing.T) {
	t.Run("empty events", func(t *testing.T) {
		out := formatEventsAsTable(nil)
		if !strings.Contains(out, "TYPE") {
			t.Error("expected table header even for empty events")
		}
	})

	t.Run("single event", func(t *testing.T) {
		now := time.Now()
		events := []corev1.Event{
			{
				Type:           "Warning",
				Reason:         "FailedScheduling",
				Message:        "0/3 nodes are available",
				Count:          5,
				LastTimestamp:  metav1.NewTime(now),
				InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "my-pod"},
			},
		}
		out := formatEventsAsTable(events)
		if !strings.Contains(out, "Warning") {
			t.Error("expected Warning type in table")
		}
		if !strings.Contains(out, "FailedScheduling") {
			t.Error("expected FailedScheduling reason in table")
		}
		if !strings.Contains(out, "Pod/my-pod") {
			t.Error("expected Pod/my-pod object in table")
		}
		if !strings.Contains(out, "5") {
			t.Error("expected count 5 in table")
		}
	})

	t.Run("truncates long message", func(t *testing.T) {
		longMsg := strings.Repeat("x", 200)
		events := []corev1.Event{
			{
				Type:           "Normal",
				Reason:         "Started",
				Message:        longMsg,
				Count:          1,
				InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "test"},
			},
		}
		out := formatEventsAsTable(events)
		if strings.Contains(out, longMsg) {
			t.Error("expected long message to be truncated")
		}
		if !strings.Contains(out, "...") {
			t.Error("expected ... for truncated message")
		}
	})
}

func TestSortEventsByTime(t *testing.T) {
	now := time.Now()
	events := []corev1.Event{
		{LastTimestamp: metav1.NewTime(now.Add(-2 * time.Hour))}, // oldest
		{LastTimestamp: metav1.NewTime(now)},                     // newest
		{LastTimestamp: metav1.NewTime(now.Add(-1 * time.Hour))}, // middle
	}

	sortEventsByTime(events)

	// Should be sorted most-recent-first
	if !eventTime(events[0]).Equal(now) {
		t.Errorf("expected newest first, got %v", eventTime(events[0]))
	}
	if !eventTime(events[2]).Equal(now.Add(-2 * time.Hour)) {
		t.Errorf("expected oldest last, got %v", eventTime(events[2]))
	}
}
