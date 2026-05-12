package aggregate

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func makeEvent(reason, kind, namespace string, count int32, lastTime time.Time) corev1.Event {
	return corev1.Event{
		Reason: reason,
		Type:   "Warning",
		InvolvedObject: corev1.ObjectReference{
			Kind:      kind,
			Namespace: namespace,
		},
		Count:          count,
		LastTimestamp:  metav1.Time{Time: lastTime},
		EventTime:      metav1.MicroTime{Time: lastTime},
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
}
