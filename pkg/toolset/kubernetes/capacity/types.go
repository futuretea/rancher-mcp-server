// Package capacity provides resource capacity analysis functionality similar to kube-capacity.
package capacity

import (
	corev1 "k8s.io/api/core/v1"
)

// NodeInfo holds resource information for a node
type NodeInfo struct {
	Name     string            `json:"name"`
	CPU      Resource          `json:"cpu"`
	Memory   Resource          `json:"memory"`
	PodCount PodCountInfo      `json:"podCount"`
	Taints   []corev1.Taint    `json:"taints,omitempty"`
	Labels   map[string]string `json:"labels,omitempty"`
	Pods     []PodInfo         `json:"pods,omitempty"`
}

// Resource holds resource metrics for a node
type Resource struct {
	Capacity    int64 `json:"capacity"`
	Allocatable int64 `json:"allocatable"`
	Requested   int64 `json:"requested"`
	Limited     int64 `json:"limited"`
	Utilized    int64 `json:"utilized,omitempty"`
}

// PodCountInfo holds pod count information
type PodCountInfo struct {
	Capacity    int64 `json:"capacity"`
	Allocatable int64 `json:"allocatable"`
	Requested   int64 `json:"requested"`
}

// ContainerInfo holds resource information for a container
type ContainerInfo struct {
	Name   string   `json:"name"`
	CPU    Resource `json:"cpu"`
	Memory Resource `json:"memory"`
	Init   bool     `json:"init,omitempty"`
}

// PodInfo holds resource information for a pod
type PodInfo struct {
	Namespace    string          `json:"namespace"`
	Name         string          `json:"name"`
	CPU          Resource        `json:"cpu"`
	Memory       Resource        `json:"memory"`
	ContainerCnt int             `json:"containerCount"`
	Containers   []ContainerInfo `json:"containers,omitempty"`
}

// Result holds the complete capacity analysis
type Result struct {
	Nodes          []NodeInfo `json:"nodes"`
	Cluster        NodeInfo   `json:"cluster"`
	ShowPods       bool       `json:"showPods"`
	ShowContainers bool       `json:"showContainers"`
	ShowUtil       bool       `json:"showUtil"`
	ShowAvailable  bool       `json:"showAvailable"`
	ShowPodCount   bool       `json:"showPodCount"`
	ShowLabels     bool       `json:"showLabels"`
	HideRequests   bool       `json:"hideRequests"`
	HideLimits     bool       `json:"hideLimits"`
}

// Params holds all parameters for capacity analysis
type Params struct {
	Cluster                string
	Namespace              string
	LabelSelector          string
	NodeLabelSelector      string
	NamespaceLabelSelector string
	NodeTaints             string
	SortBy                 string
	Format                 string
	ShowPods               bool
	ShowContainers         bool
	ShowUtil               bool
	ShowAvailable          bool
	ShowPodCount           bool
	ShowLabels             bool
	HideRequests           bool
	HideLimits             bool
	NoTaint                bool
}
