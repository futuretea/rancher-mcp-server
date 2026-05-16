package kubernetes

import (
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
