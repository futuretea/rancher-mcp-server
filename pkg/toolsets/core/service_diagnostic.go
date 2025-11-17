package core

import (
	"context"

	projectClient "github.com/rancher/rancher/pkg/client/generated/project/v3"
	"github.com/futuretea/rancher-mcp-server/pkg/rancher"
)

// ServiceDiagnosticStatus represents the diagnostic status of a service with degraded support
type ServiceDiagnosticStatus struct {
	Ready                   bool                    `json:"ready"`
	Degraded                bool                    `json:"degraded"`
	EndpointReady           bool                    `json:"endpointReady"`
	PodDetails              *ServicePodDetails      `json:"podDetails,omitempty"`
	Errors                  ServiceDiagnosticErrors `json:"errors"`
	PublicEndpointAddresses []string                `json:"publicEndpointAddresses,omitempty"`
}

// ServiceDiagnosticErrors represents error flags for service diagnostic with degraded support
type ServiceDiagnosticErrors struct {
	NoSelector       bool `json:"noSelector"`
	NoMatchingPods   bool `json:"noMatchingPods"`
	HasNotReadyPods  bool `json:"hasNotReadyPods"`
	EndpointNotFound bool `json:"endpointNotFound"`
}

// ServicePodDetails represents pod details for a service
type ServicePodDetails struct {
	TotalPods      int `json:"totalPods"`
	ReadyPods      int `json:"readyPods"`
	NotReadyPods   int `json:"notReadyPods"`
}

// diagnoseService performs diagnostic checks for a service with degraded support
// It checks the service's selector and finds matching pods to determine readiness
// The podList is passed in to avoid redundant API calls when diagnosing multiple services
func diagnoseService(ctx context.Context,
	rancherClient *rancher.Client,
	clusterID, projectID string,
	service rancher.Service,
	podList []rancher.Pod) ServiceDiagnosticStatus {

	status := ServiceDiagnosticStatus{
		Ready:  false,
		Errors: ServiceDiagnosticErrors{},
	}

	// Extract PublicEndpoint addresses for informational purposes only
	status.PublicEndpointAddresses = extractEndpointAddresses(service.PublicEndpoints)

	// Check if service has selector
	if service.Selector == nil || len(service.Selector) == 0 {
		status.Errors.NoSelector = true
		status.Degraded = true
		status.PodDetails = &ServicePodDetails{
			TotalPods:    0,
			ReadyPods:    0,
			NotReadyPods: 0,
		}
		return status
	}

	// Filter pods by service selector from the provided list
	matchingPods := []rancher.Pod{}
	for _, pod := range podList {
		if pod.NamespaceId != service.NamespaceId {
			continue
		}

		// Check if pod matches service selector
		matches := true
		for selectorKey, selectorValue := range service.Selector {
			if podLabelValue, exists := pod.Labels[selectorKey]; !exists || podLabelValue != selectorValue {
				matches = false
				break
			}
		}

		if matches {
			matchingPods = append(matchingPods, pod)
		}
	}

	// Calculate pod status
	readyCount := 0
	notReadyCount := 0
	for _, pod := range matchingPods {
		if isPodReady(pod) {
			readyCount++
		} else {
			notReadyCount++
		}
	}

	status.PodDetails = &ServicePodDetails{
		TotalPods:    len(matchingPods),
		ReadyPods:    readyCount,
		NotReadyPods: notReadyCount,
	}

	// Determine Ready, Degraded, and EndpointReady based on Pod health
	// Ready: at least 1 pod is ready (can accept traffic)
	status.Ready = (len(matchingPods) > 0 && readyCount > 0)

	// Degraded: no pods OR some pods are not ready (capacity issue)
	status.Degraded = (readyCount == 0 || readyCount < len(matchingPods))

	// EndpointReady: same as Ready (endpoint exists if there are pods)
	status.EndpointReady = status.Ready

	// Set error flags
	if len(matchingPods) == 0 {
		status.Errors.NoMatchingPods = true
	}
	if readyCount < len(matchingPods) {
		status.Errors.HasNotReadyPods = true
	}

	return status
}

// isPodReady checks if a pod is in ready state
func isPodReady(pod rancher.Pod) bool {
	return pod.State == "running" || pod.State == "active"
}

// extractEndpointAddresses extracts addresses from PublicEndpoints for informational purposes
func extractEndpointAddresses(endpoints []projectClient.PublicEndpoint) []string {
	addresses := []string{}
	if len(endpoints) == 0 {
		return addresses
	}

	for _, ep := range endpoints {
		if len(ep.Addresses) > 0 {
			addresses = append(addresses, ep.Addresses...)
		}
	}
	return addresses
}
