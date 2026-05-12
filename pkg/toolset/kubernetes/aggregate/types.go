// Package aggregate provides aggregate and ranking analysis tools for Kubernetes resources.
package aggregate

import (
	"time"
)

// Constants for limits and defaults
const (
	MaxItems     = 500
	DefaultLimit = 50
)

// ClampLimit clamps a user-provided limit to valid bounds.
func ClampLimit(limit int) int {
	if limit <= 0 {
		return DefaultLimit
	}
	if limit > MaxItems {
		return MaxItems
	}
	return limit
}

// --- Top (kubernetes_top) ---

// TopParams holds parameters for top analysis
type TopParams struct {
	Cluster       string
	Namespace     string
	LabelSelector string
	Kind          string // "pod" or "node"
	SortBy        string
	Limit         int
	Format        string
}

// TopResult holds the result of top analysis
type TopResult struct {
	Items     []TopItem `json:"items"`
	Truncated bool      `json:"truncated"`
	Total     int       `json:"total"`
	Warning   string    `json:"warning,omitempty"`
}

// TopItem holds a single top entry
type TopItem struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	CPUReq    int64  `json:"cpuRequest"`
	CPULimit  int64  `json:"cpuLimit"`
	CPUUtil   int64  `json:"cpuUtilization"`
	MemReq    int64  `json:"memoryRequest"`
	MemLimit  int64  `json:"memoryLimit"`
	MemUtil   int64  `json:"memoryUtilization"`
	Restarts  int32  `json:"restarts,omitempty"`
}

// --- Workload Health (kubernetes_workload_health) ---

// WorkloadParams holds parameters for workload health analysis
type WorkloadParams struct {
	Cluster       string
	Namespace     string
	Kind          string // "deployment", "statefulset", "daemonset", "all"
	LabelSelector string
	SortBy        string
	Limit         int
	Format        string
}

// WorkloadResult holds the result of workload health analysis
type WorkloadResult struct {
	Items     []WorkloadItem `json:"items"`
	Truncated bool           `json:"truncated"`
	Total     int            `json:"total"`
}

// WorkloadItem holds a single workload entry
type WorkloadItem struct {
	Name        string    `json:"name"`
	Namespace   string    `json:"namespace"`
	Kind        string    `json:"kind"`
	Ready       int32     `json:"ready"`
	Desired     int32     `json:"desired"`
	Unavailable int32     `json:"unavailable"`
	Updated     int32     `json:"updated"`
	Age         string    `json:"age"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"-"` // for accurate age sorting
}

// --- Resource Summary (kubernetes_resource_summary) ---

// SummaryParams holds parameters for resource summary analysis
type SummaryParams struct {
	Cluster       string
	Namespace     string
	LabelSelector string
	GroupBy       string // "namespace" or "label"
	GroupByKey    string
	SortBy        string
	Limit         int
	Format        string
}

// SummaryResult holds the result of resource summary analysis
type SummaryResult struct {
	Items     []SummaryItem `json:"items"`
	Truncated bool          `json:"truncated"`
	Total     int           `json:"total"`
}

// SummaryItem holds a single summary entry
type SummaryItem struct {
	Group    string `json:"group"`
	PodCount int    `json:"podCount"`
	CPUReq   int64  `json:"cpuRequest"`
	CPULimit int64  `json:"cpuLimit"`
	MemReq   int64  `json:"memoryRequest"`
	MemLimit int64  `json:"memoryLimit"`
}

// --- Event Summary (kubernetes_event_summary) ---

// EventParams holds parameters for event summary analysis
type EventParams struct {
	Cluster   string
	Namespace string
	Kind      string
	Type      string // "Warning" or "Normal"
	Since     string
	SortBy    string
	Limit     int
	Format    string
}

// EventResult holds the result of event summary analysis
type EventResult struct {
	Items     []EventItem `json:"items"`
	Truncated bool        `json:"truncated"`
	Total     int         `json:"total"`
}

// EventItem holds a single event summary entry
type EventItem struct {
	Reason    string    `json:"reason"`
	Kind      string    `json:"kind"`
	Namespace string    `json:"namespace"`
	Count     int32     `json:"count"`
	LastSeen  time.Time `json:"lastSeen"`
}
