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

// HasRancherCapability reports whether the server is configured to access Rancher.
// It is true in static credential mode or in per-request token mode when a Rancher
// server URL is configured. HasRancherConfig semantics remain unchanged.
func (c *Configuration) HasRancherCapability() bool {
	if c == nil || c.StaticConfig == nil {
		return false
	}
	if c.HasRancherConfig() {
		return true
	}
	return c.usesRequestScopedRancherToken() && c.RancherServerURL != ""
}

func (c *Configuration) usesRequestScopedRancherToken() bool {
	return c.RancherRequestTokenAuth || c.RancherOAuthTokenAuth
}

func buildCapabilityStatus(configured, available bool, missingReason, unavailableReason string) CapabilityStatus {
	status := CapabilityStatus{
		Configured: configured,
		Available:  available,
	}
	if !configured {
		status.Reason = missingReason
	} else if !available {
		status.Reason = unavailableReason
	}
	return status
}

func (s *Server) capabilityStatuses() map[string]CapabilityStatus {
	hasRancherCapability := s.configuration != nil && s.configuration.HasRancherCapability()
	hasRancherConfig := s.configuration != nil && s.configuration.HasRancherConfig()
	hasKubeconfigConfig := s.configuration != nil && s.configuration.HasKubeconfigConfig()

	requestScoped := s.configuration != nil && s.configuration.usesRequestScopedRancherToken()
	rancherAvailable := hasRancherCapability && (requestScoped || (s.normanClient != nil && s.normanClient.IsUsable()))
	kubernetesAvailable := (hasRancherCapability && (requestScoped || s.steveClient != nil)) || (hasKubeconfigConfig && s.steveClient != nil)

	rancherStatus := buildCapabilityStatus(hasRancherCapability, rancherAvailable, "rancher configuration missing", "rancher client unavailable")
	kubernetesStatus := buildCapabilityStatus(hasRancherCapability || hasKubeconfigConfig, kubernetesAvailable, "kubernetes configuration missing", "kubernetes client unavailable")

	// Suppress misleading "unavailable" reasons in dynamic mode where availability
	// reflects configuration readiness rather than pre-created client usability.
	if hasRancherCapability && !hasRancherConfig {
		rancherStatus.Reason = ""
		kubernetesStatus.Reason = ""
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
		if serverTool.Annotations.ClusterSources == nil {
			serverTool.Annotations.ClusterSources = []string{"rancher"}
		}
	case "kubernetes":
		if serverTool.Annotations.RequiresKubernetes == nil {
			serverTool.Annotations.RequiresKubernetes = boolPtr(true)
		}
		if *serverTool.Annotations.RequiresKubernetes && serverTool.Annotations.ClusterSources == nil {
			serverTool.Annotations.ClusterSources = []string{"rancher", "kubeconfig"}
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
	if len(serverTool.Annotations.ClusterSources) == 0 {
		return true, ""
	}

	var reasons []string
	requiresKubernetes := serverTool.Annotations.RequiresKubernetes != nil && *serverTool.Annotations.RequiresKubernetes
	for _, source := range serverTool.Annotations.ClusterSources {
		available, reason := s.clusterSourceAvailable(source, requiresKubernetes)
		if available {
			return true, ""
		}
		if reason != "" {
			reasons = append(reasons, reason)
		}
	}
	return false, strings.Join(reasons, "; ")
}

func (s *Server) clusterSourceAvailable(source string, requiresKubernetes bool) (bool, string) {
	if s.configuration == nil || s.configuration.StaticConfig == nil {
		return false, "cluster source configuration missing"
	}

	switch source {
	case "rancher":
		requestScoped := s.configuration.usesRequestScopedRancherToken()
		if !s.configuration.HasRancherCapability() {
			return false, "rancher configuration missing"
		}
		if requiresKubernetes && (requestScoped || s.steveClient != nil) {
			return true, ""
		}
		if !requiresKubernetes && (requestScoped || (s.normanClient != nil && s.normanClient.IsUsable())) {
			return true, ""
		}
		if requiresKubernetes {
			return false, "rancher Kubernetes client unavailable"
		}
		return false, "rancher client unavailable"
	case "kubeconfig":
		if !s.configuration.HasKubeconfigConfig() {
			return false, "kubeconfig configuration missing"
		}
		if s.steveClient != nil {
			return true, ""
		}
		return false, "kubeconfig client unavailable"
	default:
		return false, fmt.Sprintf("unknown cluster source %q", source)
	}
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
