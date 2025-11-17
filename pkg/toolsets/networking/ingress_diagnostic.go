package networking

import (
	"context"
	"strings"

	projectClient "github.com/rancher/rancher/pkg/client/generated/project/v3"
	"github.com/futuretea/rancher-mcp-server/pkg/rancher"
)

// IngressDiagnosticStatus represents the diagnostic status of an ingress with degraded support
type IngressDiagnosticStatus struct {
	Ready        bool                                `json:"ready"`
	Degraded     bool                                `json:"degraded"`
	PathStatus   map[string]IngressPathDiagnosticStatus `json:"pathStatus,omitempty"`
	LoadBalancer map[string]interface{}              `json:"loadBalancer,omitempty"`
}

// IngressPathDiagnosticStatus represents the diagnostic status of a specific path with degraded support
type IngressPathDiagnosticStatus struct {
	ServiceName             string                 `json:"serviceName"`
	Ready                   bool                   `json:"ready"`
	Degraded                bool                   `json:"degraded"`
	Errors                  IngressDiagnosticErrors `json:"errors"`
	ServiceDetails          *ServiceDetails        `json:"serviceDetails,omitempty"`
	PublicEndpointAddresses []string               `json:"publicEndpointAddresses,omitempty"`
}

// IngressDiagnosticErrors represents error flags for diagnostic
type IngressDiagnosticErrors struct {
	NoService        bool `json:"noService"`
	NoPods           bool `json:"noPods"`
	HasNotReadyPods  bool `json:"hasNotReadyPods"`
	ServiceNotFound  bool `json:"serviceNotFound"`
	EndpointNotFound bool `json:"endpointNotFound"`
}

// ExtractServiceName extracts service name from either ServiceId or Service object
// It tries ServiceId first (actual data is usually here), falls back to Service.Name
type ExtractServiceName struct{}

func (e ExtractServiceName) FromPath(path projectClient.HTTPIngressPath) string {
	if path.ServiceId != "" {
		return extractServiceNameFromID(path.ServiceId)
	}
	if path.Service != nil && path.Service.Name != "" {
		return path.Service.Name
	}
	return ""
}

func (e ExtractServiceName) FromBackend(backend *projectClient.IngressBackend) string {
	if backend == nil {
		return ""
	}
	if backend.ServiceId != "" {
		return extractServiceNameFromID(backend.ServiceId)
	}
	if backend.Service != nil && backend.Service.Name != "" {
		return backend.Service.Name
	}
	return ""
}

// ServiceDetails represents service and pod details
type ServiceDetails struct {
	PodCount     int `json:"podCount"`
	ReadyPods    int `json:"readyPods"`
	NotReadyPods int `json:"notReadyPods"`
}

// diagnoseIngressService performs diagnostic checks for a service backend used by ingress
func diagnoseIngressService(ctx context.Context,
	rancherClient *rancher.Client,
	namespace, serviceName string,
	serviceList []rancher.Service,
	podList []rancher.Pod) IngressPathDiagnosticStatus {

	status := IngressPathDiagnosticStatus{
		Ready:        true,
		Errors:       IngressDiagnosticErrors{},
		ServiceName:  serviceName,
	}

	// Find the target service from the provided list
	var targetService *rancher.Service
	for _, svc := range serviceList {
		if svc.NamespaceId == namespace && svc.Name == serviceName {
			targetService = &svc
			break
		}
	}

	if targetService == nil {
		status.Errors.ServiceNotFound = true
		status.Degraded = true
		status.Ready = false
		return status
	}

	// Extract PublicEndpoint addresses for informational purposes only
	status.PublicEndpointAddresses = extractEndpointAddresses(targetService.PublicEndpoints)

	// Filter pods by service selector from the provided list
	if targetService.Selector != nil && len(targetService.Selector) > 0 {
		matchingPods := []rancher.Pod{}
		for _, pod := range podList {
			if pod.NamespaceId != namespace {
				continue
			}

			// Check if pod matches service selector
			matches := true
			for selectorKey, selectorValue := range targetService.Selector {
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

		status.ServiceDetails = &ServiceDetails{
			PodCount:     len(matchingPods),
			ReadyPods:    readyCount,
			NotReadyPods: notReadyCount,
		}

		// Determine Ready and Degraded status based on Pod health
		// Ready: at least 1 pod is ready (can accept traffic)
		status.Ready = (len(matchingPods) > 0 && readyCount > 0)

		// Degraded: no pods OR some pods are not ready (capacity issue)
		status.Degraded = (readyCount == 0 || readyCount < len(matchingPods))

		// Set error flags
		if len(matchingPods) == 0 {
			status.Errors.NoPods = true
		} else if readyCount < len(matchingPods) {
			status.Errors.HasNotReadyPods = true
		}
	} else {
		// No selector means no pods will match
		status.Ready = false
		status.Degraded = true
		status.Errors.NoPods = true
		status.ServiceDetails = &ServiceDetails{
			PodCount:     0,
			ReadyPods:    0,
			NotReadyPods: 0,
		}
	}

	// logging.Info("Ingress service diagnostic status: %+v", status)

	return status
}

// diagnoseIngressPath performs diagnostic checks for an ingress path with degraded support
func diagnoseIngressPath(ctx context.Context,
	rancherClient *rancher.Client,
	namespace string,
	path projectClient.HTTPIngressPath,
	serviceList []rancher.Service,
	podList []rancher.Pod) IngressPathDiagnosticStatus {

	// Extract service name from path
	extractor := ExtractServiceName{}
	serviceName := extractor.FromPath(path)

	if serviceName == "" {
		// No service reference found
		return IngressPathDiagnosticStatus{
			Ready:    false,
			Errors:   IngressDiagnosticErrors{NoService: true},
			Degraded: true,
		}
	}

	// Use common service diagnostic logic
	status := diagnoseIngressService(ctx, rancherClient, namespace, serviceName, serviceList, podList)
	return status
}

// diagnoseIngressBackend performs diagnostic checks for an ingress backend (DefaultBackend)
func diagnoseIngressBackend(ctx context.Context,
	rancherClient *rancher.Client,
	namespace string,
	backend *projectClient.IngressBackend,
	serviceList []rancher.Service,
	podList []rancher.Pod) IngressPathDiagnosticStatus {

	// Extract service name from backend
	extractor := ExtractServiceName{}
	serviceName := extractor.FromBackend(backend)

	if serviceName == "" {
		// No service reference found
		return IngressPathDiagnosticStatus{
			Ready:    false,
			Errors:   IngressDiagnosticErrors{NoService: true},
			Degraded: true,
		}
	}

	// Use common service diagnostic logic
	status := diagnoseIngressService(ctx, rancherClient, namespace, serviceName, serviceList, podList)
	return status
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

// extractServiceNameFromID extracts service name from a service ID string
// ServiceId format can be: "namespace:service-name" or "cluster:project:namespace:service-name"
func extractServiceNameFromID(serviceID string) string {
	if serviceID == "" {
		return ""
	}
	parts := strings.Split(serviceID, ":")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

// isPodReady checks if a pod is in ready state
func isPodReady(pod rancher.Pod) bool {
	return pod.State == "running" || pod.State == "active"
}
