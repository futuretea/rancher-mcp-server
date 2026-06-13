package mcp

import (
	"testing"
	"time"
)

func resetMetrics(m *expvarMetrics) {
	m.resolveDuration.Set(0)
	m.resolveMemoryBytes.Set(0)
	m.activeClientCount.Set(0)
	m.rancherRequestErrors.Set(0)
	m.resolveCount.Set(0)
	m.resolveTotalMs.Set(0)
}

func TestExpvarMetrics_RecordDuration(t *testing.T) {
	m := NewExpvarMetrics().(*expvarMetrics)
	resetMetrics(m)

	m.RecordClientResolveDuration(42 * time.Millisecond)
	if m.resolveDuration.Value() != 42 {
		t.Fatalf("expected duration 42, got %v", m.resolveDuration.Value())
	}
	if m.resolveCount.Value() != 1 {
		t.Fatalf("expected count 1, got %v", m.resolveCount.Value())
	}

	m.RecordClientResolveDuration(58 * time.Millisecond)
	if m.resolveDuration.Value() != 50 {
		t.Fatalf("expected average duration 50, got %v", m.resolveDuration.Value())
	}
	if m.resolveCount.Value() != 2 {
		t.Fatalf("expected count 2, got %v", m.resolveCount.Value())
	}
}

func TestExpvarMetrics_RecordMemory(t *testing.T) {
	m := NewExpvarMetrics().(*expvarMetrics)
	resetMetrics(m)
	m.RecordClientResolveMemoryBytes(1024)
	if m.resolveMemoryBytes.Value() != 1024 {
		t.Fatalf("expected memory 1024, got %v", m.resolveMemoryBytes.Value())
	}
}

func TestExpvarMetrics_ActiveClientCount(t *testing.T) {
	m := NewExpvarMetrics().(*expvarMetrics)
	resetMetrics(m)
	m.IncrementActiveClientCount()
	m.IncrementActiveClientCount()
	m.DecrementActiveClientCount()
	if m.activeClientCount.Value() != 1 {
		t.Fatalf("expected active count 1, got %v", m.activeClientCount.Value())
	}
}

func TestExpvarMetrics_RancherRequestErrors(t *testing.T) {
	m := NewExpvarMetrics().(*expvarMetrics)
	resetMetrics(m)
	m.IncrementRancherRequestErrors()
	m.IncrementRancherRequestErrors()
	if m.rancherRequestErrors.Value() != 2 {
		t.Fatalf("expected error count 2, got %v", m.rancherRequestErrors.Value())
	}
}

func TestExpvarMetrics_NilSafe(t *testing.T) {
	var m Metrics = (*expvarMetrics)(nil)
	m.RecordClientResolveDuration(time.Millisecond)
	m.RecordClientResolveMemoryBytes(1)
	m.IncrementActiveClientCount()
	m.DecrementActiveClientCount()
	m.IncrementRancherRequestErrors()
}
