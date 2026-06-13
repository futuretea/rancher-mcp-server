package mcp

import (
	"expvar"
	"runtime"
	"sync/atomic"
	"time"
)

// Metrics exposes observability for the per-request token resolution path.
type Metrics interface {
	RecordClientResolveDuration(duration time.Duration)
	RecordClientResolveMemoryBytes(bytes int64)
	IncrementActiveClientCount()
	DecrementActiveClientCount()
	IncrementRancherRequestErrors()
}

// expvarMetrics is an expvar-backed Metrics implementation.
// All methods are safe for concurrent use.
type expvarMetrics struct {
	resolveDuration      *expvar.Float
	resolveMemoryBytes   *expvar.Int
	activeClientCount    *expvar.Int
	rancherRequestErrors *expvar.Int
	resolveCount         *expvar.Int
	resolveTotalMs       *expvar.Int
	resolveCountAtomic   int64
	resolveTotalMsAtomic int64
}

// NewExpvarMetrics creates a Metrics implementation backed by expvar.
// Existing published variables are reused so the function is safe to call multiple
// times in the same process (e.g., during tests).
func NewExpvarMetrics() Metrics {
	return &expvarMetrics{
		resolveDuration:      getOrCreateFloat("client_resolve_duration"),
		resolveMemoryBytes:   getOrCreateInt("client_resolve_memory_bytes"),
		activeClientCount:    getOrCreateInt("active_client_count"),
		rancherRequestErrors: getOrCreateInt("rancher_request_errors"),
		resolveCount:         getOrCreateInt("client_resolve_duration_count"),
		resolveTotalMs:       getOrCreateInt("client_resolve_duration_total_ms"),
	}
}

func getOrCreateInt(name string) *expvar.Int {
	if v := expvar.Get(name); v != nil {
		if iv, ok := v.(*expvar.Int); ok {
			return iv
		}
	}
	return expvar.NewInt(name)
}

func getOrCreateFloat(name string) *expvar.Float {
	if v := expvar.Get(name); v != nil {
		if fv, ok := v.(*expvar.Float); ok {
			return fv
		}
	}
	return expvar.NewFloat(name)
}

func (m *expvarMetrics) RecordClientResolveDuration(duration time.Duration) {
	if m == nil {
		return
	}
	ms := duration.Milliseconds()
	count := atomic.AddInt64(&m.resolveCountAtomic, 1)
	total := atomic.AddInt64(&m.resolveTotalMsAtomic, ms)
	m.resolveCount.Set(count)
	m.resolveTotalMs.Set(total)
	m.resolveDuration.Set(float64(total) / float64(count))
}

func (m *expvarMetrics) RecordClientResolveMemoryBytes(bytes int64) {
	if m == nil {
		return
	}
	m.resolveMemoryBytes.Set(bytes)
}

func (m *expvarMetrics) IncrementActiveClientCount() {
	if m == nil {
		return
	}
	m.activeClientCount.Add(1)
}

func (m *expvarMetrics) DecrementActiveClientCount() {
	if m == nil {
		return
	}
	m.activeClientCount.Add(-1)
}

func (m *expvarMetrics) IncrementRancherRequestErrors() {
	if m == nil {
		return
	}
	m.rancherRequestErrors.Add(1)
}

// readMemoryBytes returns an estimate of allocated heap bytes for observability.
func readMemoryBytes() int64 {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	return int64(stats.Alloc)
}
