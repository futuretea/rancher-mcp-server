package mcp

import (
	"fmt"
	"sort"
	"strings"

	"github.com/futuretea/rancher-mcp-server/pkg/toolset"
)

// CapabilityStatus describes whether a runtime capability is configured and available.
type CapabilityStatus struct {
	Configured bool   `json:"configured"`
	Available  bool   `json:"available"`
	Reason     string `json:"reason,omitempty"`
}

// HealthStatus is the health payload exposed by /healthz.
type HealthStatus struct {
	Status       string                      `json:"status"`
	Capabilities map[string]CapabilityStatus `json:"capabilities"`
}

func (s *Server) capabilityStatuses() map[string]CapabilityStatus {
	hasConfig := s.configuration != nil && s.configuration.HasRancherConfig()
	rancherAvailable := hasConfig && s.normanClient != nil && s.normanClient.IsUsable()
	kubernetesAvailable := hasConfig && s.steveClient != nil

	rancherStatus := CapabilityStatus{
		Configured: hasConfig,
		Available:  rancherAvailable,
	}
	if !hasConfig {
		rancherStatus.Reason = "rancher configuration missing"
	} else if !rancherAvailable {
		rancherStatus.Reason = "rancher client unavailable"
	}

	kubernetesStatus := CapabilityStatus{
		Configured: hasConfig,
		Available:  kubernetesAvailable,
	}
	if !hasConfig {
		kubernetesStatus.Reason = "rancher configuration missing"
	} else if !kubernetesAvailable {
		kubernetesStatus.Reason = "kubernetes client unavailable"
	}

	return map[string]CapabilityStatus{
		"rancher":    rancherStatus,
		"kubernetes": kubernetesStatus,
	}
}

// GetHealthStatus returns process health plus current capability availability.
func (s *Server) GetHealthStatus() HealthStatus {
	status := "ok"
	if !s.IsHealthy() {
		status = "unhealthy"
	}

	return HealthStatus{
		Status:       status,
		Capabilities: s.capabilityStatuses(),
	}
}

func (s *Server) capabilitySummary() string {
	statuses := s.capabilityStatuses()
	names := make([]string, 0, len(statuses))
	for name := range statuses {
		names = append(names, name)
	}
	sort.Strings(names)

	summary := make([]string, 0, len(names))
	for _, name := range names {
		status := statuses[name]
		state := "available"
		if !status.Available {
			state = fmt.Sprintf("unavailable(%s)", status.Reason)
		}
		summary = append(summary, fmt.Sprintf("%s=%s", name, state))
	}

	return strings.Join(summary, ", ")
}

func applyDefaultAnnotations(toolsetName string, serverTool toolset.ServerTool) toolset.ServerTool {
	switch toolsetName {
	case "rancher":
		if serverTool.Annotations.RequiresRancher == nil {
			serverTool.Annotations.RequiresRancher = boolPtr(true)
		}
	case "kubernetes":
		if serverTool.Annotations.RequiresKubernetes == nil {
			serverTool.Annotations.RequiresKubernetes = boolPtr(true)
		}
	}

	return serverTool
}

func (s *Server) capabilityAllowsTool(serverTool toolset.ServerTool) (bool, string) {
	statuses := s.capabilityStatuses()

	if serverTool.Annotations.RequiresRancher != nil && *serverTool.Annotations.RequiresRancher && !statuses["rancher"].Available {
		return false, statuses["rancher"].Reason
	}
	if serverTool.Annotations.RequiresKubernetes != nil && *serverTool.Annotations.RequiresKubernetes && !statuses["kubernetes"].Available {
		return false, statuses["kubernetes"].Reason
	}

	return true, ""
}

func validateUniqueToolNames(enabledToolsets []toolset.Toolset, client interface{}) error {
	seen := make(map[string]string)

	for _, ts := range enabledToolsets {
		for _, serverTool := range ts.GetTools(client) {
			if previous, exists := seen[serverTool.Tool.Name]; exists {
				return fmt.Errorf("duplicate tool name %q defined in toolsets %q and %q", serverTool.Tool.Name, previous, ts.GetName())
			}
			seen[serverTool.Tool.Name] = ts.GetName()
		}
	}

	return nil
}

func boolPtr(value bool) *bool {
	return &value
}
