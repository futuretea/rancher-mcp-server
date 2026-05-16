package aggregate

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSummarizeEvents_GroupsByReasonKindAndNamespace(t *testing.T) {
	now := time.Now()
	events := []corev1.Event{
		eventSummaryTestEvent("FailedScheduling", "Pod", "prod", "Warning", now.Add(-2*time.Minute)),
		eventSummaryTestEvent("FailedScheduling", "Pod", "prod", "Warning", now.Add(-1*time.Minute)),
		eventSummaryTestEvent("OOMKilling", "Pod", "prod", "Warning", now.Add(-5*time.Minute)),
		eventSummaryTestEvent("Scheduled", "Pod", "prod", "Normal", now.Add(-3*time.Minute)),
	}

	result := summarizeEvents(events, EventParams{Type: "Warning", SortBy: "count", Limit: 50}, time.Time{})
	if result.Total != 2 {
		t.Fatalf("Total = %d, want 2", result.Total)
	}
	if result.Items[0].Reason != "FailedScheduling" || result.Items[0].Count != 2 {
		t.Fatalf("first item = %+v, want FailedScheduling count=2", result.Items[0])
	}
	if result.Items[1].Reason != "OOMKilling" || result.Items[1].Count != 1 {
		t.Fatalf("second item = %+v, want OOMKilling count=1", result.Items[1])
	}
}

func TestSummarizeEvents_SinceFilter(t *testing.T) {
	now := time.Now()
	events := []corev1.Event{
		eventSummaryTestEvent("Old", "Pod", "prod", "Warning", now.Add(-2*time.Hour)),
		eventSummaryTestEvent("Recent", "Pod", "prod", "Warning", now.Add(-5*time.Minute)),
	}

	result := summarizeEvents(events, EventParams{SortBy: "name", Limit: 50}, now.Add(-1*time.Hour))
	if result.Total != 1 {
		t.Fatalf("Total = %d, want 1", result.Total)
	}
	if result.Items[0].Reason != "Recent" {
		t.Fatalf("Reason = %q, want Recent", result.Items[0].Reason)
	}
}

func eventSummaryTestEvent(reason, kind, namespace, eventType string, lastTime time.Time) corev1.Event {
	return corev1.Event{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace},
		Reason:     reason,
		Type:       eventType,
		InvolvedObject: corev1.ObjectReference{
			Kind:      kind,
			Namespace: namespace,
		},
		LastTimestamp: metav1.Time{Time: lastTime},
		EventTime:     metav1.MicroTime{Time: lastTime},
	}
}

func TestSortEventItems_ByCount(t *testing.T) {
	now := time.Now()
	items := []EventItem{
		{Reason: "Event-C", Count: 5, LastSeen: now},
		{Reason: "Event-A", Count: 20, LastSeen: now},
		{Reason: "Event-B", Count: 10, LastSeen: now},
	}
	sortEventItems(items, "count")
	if items[0].Reason != "Event-A" {
		t.Errorf("expected first item to be Event-A, got %s", items[0].Reason)
	}
	if items[1].Reason != "Event-B" {
		t.Errorf("expected second item to be Event-B, got %s", items[1].Reason)
	}
	if items[2].Reason != "Event-C" {
		t.Errorf("expected third item to be Event-C, got %s", items[2].Reason)
	}
}

func TestSortEventItems_ByLastSeen(t *testing.T) {
	now := time.Now()
	items := []EventItem{
		{Reason: "Event-C", Count: 5, LastSeen: now.Add(-10 * time.Minute)},
		{Reason: "Event-A", Count: 5, LastSeen: now.Add(-1 * time.Minute)},
		{Reason: "Event-B", Count: 5, LastSeen: now.Add(-5 * time.Minute)},
	}
	sortEventItems(items, "lastSeen")
	if items[0].Reason != "Event-A" {
		t.Errorf("expected first item to be Event-A, got %s", items[0].Reason)
	}
	if items[1].Reason != "Event-B" {
		t.Errorf("expected second item to be Event-B, got %s", items[1].Reason)
	}
	if items[2].Reason != "Event-C" {
		t.Errorf("expected third item to be Event-C, got %s", items[2].Reason)
	}
}

func TestSortEventItems_ByName(t *testing.T) {
	now := time.Now()
	items := []EventItem{
		{Reason: "Event-C", LastSeen: now},
		{Reason: "Event-A", LastSeen: now},
		{Reason: "Event-B", LastSeen: now},
	}
	sortEventItems(items, "name")
	if items[0].Reason != "Event-A" {
		t.Errorf("expected first item to be Event-A, got %s", items[0].Reason)
	}
	if items[1].Reason != "Event-B" {
		t.Errorf("expected second item to be Event-B, got %s", items[1].Reason)
	}
	if items[2].Reason != "Event-C" {
		t.Errorf("expected third item to be Event-C, got %s", items[2].Reason)
	}
}
