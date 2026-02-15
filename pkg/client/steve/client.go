package steve

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/futuretea/rancher-mcp-server/pkg/util/url"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// Client provides methods for interacting with Kubernetes clusters via Rancher's Steve API.
type Client struct {
	serverURL string
	token     string
	accessKey string
	secretKey string
	insecure  bool
}

// NewClient creates a new Steve API client.
func NewClient(serverURL, token, accessKey, secretKey string, insecure bool) *Client {
	return &Client{
		serverURL: serverURL,
		token:     token,
		accessKey: accessKey,
		secretKey: secretKey,
		insecure:  insecure,
	}
}

// ListOptions contains options for listing resources.
type ListOptions struct {
	LabelSelector string
	FieldSelector string
	Limit         int64
}

// createRestConfig creates a Kubernetes REST config for the given cluster.
func (c *Client) createRestConfig(clusterID string) (*rest.Config, error) {
	clusterURL := url.GetSteveURL(c.serverURL, clusterID)

	kubeconfig := clientcmdapi.NewConfig()
	kubeconfig.Clusters["cluster"] = &clientcmdapi.Cluster{
		Server:                clusterURL,
		InsecureSkipTLSVerify: c.insecure,
	}

	authInfo := &clientcmdapi.AuthInfo{}
	if c.token != "" {
		authInfo.Token = c.token
	} else if c.accessKey != "" && c.secretKey != "" {
		authInfo.Username = c.accessKey
		authInfo.Password = c.secretKey
	}
	kubeconfig.AuthInfos["user"] = authInfo

	kubeconfig.Contexts["context"] = &clientcmdapi.Context{
		Cluster:  "cluster",
		AuthInfo: "user",
	}
	kubeconfig.CurrentContext = "context"

	return clientcmd.NewNonInteractiveClientConfig(
		*kubeconfig,
		kubeconfig.CurrentContext,
		&clientcmd.ConfigOverrides{},
		nil,
	).ClientConfig()
}

// getDynamicClient creates a dynamic Kubernetes client for the given cluster.
func (c *Client) getDynamicClient(clusterID string) (dynamic.Interface, error) {
	restConfig, err := c.createRestConfig(clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to create REST config: %w", err)
	}
	return dynamic.NewForConfig(restConfig)
}

// getClientset creates a typed Kubernetes clientset for the given cluster.
func (c *Client) getClientset(clusterID string) (kubernetes.Interface, error) {
	restConfig, err := c.createRestConfig(clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to create REST config: %w", err)
	}
	return kubernetes.NewForConfig(restConfig)
}

// getResourceInterface returns a dynamic resource interface for the given parameters.
func (c *Client) getResourceInterface(clusterID string, gvr schema.GroupVersionResource, namespace string) (dynamic.ResourceInterface, error) {
	dynClient, err := c.getDynamicClient(clusterID)
	if err != nil {
		return nil, err
	}

	if namespace != "" {
		return dynClient.Resource(gvr).Namespace(namespace), nil
	}
	return dynClient.Resource(gvr), nil
}

// getResourceInterfaceByKind resolves the kind to GVR and returns a dynamic resource interface.
// It first checks the static GVR map, then falls back to API discovery for dotted kinds
// (e.g., "cluster.apps.kubeblocks.io").
func (c *Client) getResourceInterfaceByKind(clusterID, kind, namespace string) (dynamic.ResourceInterface, error) {
	kind = strings.ToLower(kind)
	if gvr, ok := GetGVR(kind); ok {
		return c.getResourceInterface(clusterID, gvr, namespace)
	}

	// Fall back to API discovery for dotted resource kinds (<resource>.<apiGroup>)
	if !strings.Contains(kind, ".") {
		return nil, fmt.Errorf("unsupported resource kind: %s", kind)
	}

	gvr, err := c.discoverGVR(clusterID, kind)
	if err != nil {
		return nil, fmt.Errorf("unsupported resource kind: %s (%w)", kind, err)
	}
	return c.getResourceInterface(clusterID, gvr, namespace)
}

// discoverGVR resolves a dotted resource kind (e.g., "cluster.apps.kubeblocks.io") to a
// GroupVersionResource using the Kubernetes API discovery mechanism.
// The format is <singularName>.<apiGroup>.
func (c *Client) discoverGVR(clusterID, dottedKind string) (schema.GroupVersionResource, error) {
	resourceName, apiGroup, ok := parseDottedKind(dottedKind)
	if !ok {
		return schema.GroupVersionResource{}, fmt.Errorf("invalid dotted kind format: %s", dottedKind)
	}

	clientset, err := c.getClientset(clusterID)
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("failed to create clientset: %w", err)
	}

	// Find the API group and its preferred version
	groups, err := clientset.Discovery().ServerGroups()
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("failed to discover API groups: %w", err)
	}

	var groupVersion string
	for _, g := range groups.Groups {
		if g.Name == apiGroup {
			groupVersion = g.PreferredVersion.GroupVersion
			break
		}
	}
	if groupVersion == "" {
		return schema.GroupVersionResource{}, fmt.Errorf("API group %s not found on server", apiGroup)
	}

	// Discover resources for the preferred group version
	resourceList, err := clientset.Discovery().ServerResourcesForGroupVersion(groupVersion)
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("failed to discover resources for %s: %w", groupVersion, err)
	}

	gv, _ := schema.ParseGroupVersion(groupVersion)

	// Match by singular name, resource (plural) name, or lowercased Kind
	for _, r := range resourceList.APIResources {
		// Skip sub-resources (e.g., pods/log)
		if strings.Contains(r.Name, "/") {
			continue
		}
		if matchesResourceName(r, resourceName) {
			return schema.GroupVersionResource{
				Group:    gv.Group,
				Version:  gv.Version,
				Resource: r.Name,
			}, nil
		}
	}

	return schema.GroupVersionResource{}, fmt.Errorf("resource %s not found in group %s", resourceName, apiGroup)
}

// parseDottedKind splits a dotted kind (e.g., "cluster.apps.kubeblocks.io")
// into its singular resource name and API group.
func parseDottedKind(dottedKind string) (resource, apiGroup string, ok bool) {
	idx := strings.Index(dottedKind, ".")
	if idx < 0 {
		return "", "", false
	}
	return dottedKind[:idx], dottedKind[idx+1:], true
}

// matchesResourceName checks if an API resource matches the given name
// by comparing against its singular name, plural name, or lowercased Kind.
func matchesResourceName(r metav1.APIResource, name string) bool {
	singularName := r.SingularName
	if singularName == "" {
		singularName = strings.ToLower(r.Kind)
	}
	return singularName == name || r.Name == name
}

// GetResource retrieves a single Kubernetes resource by name.
func (c *Client) GetResource(ctx context.Context, clusterID, kind, namespace, name string) (*unstructured.Unstructured, error) {
	ri, err := c.getResourceInterfaceByKind(clusterID, kind, namespace)
	if err != nil {
		return nil, err
	}
	return ri.Get(ctx, name, metav1.GetOptions{})
}

// ListResources lists Kubernetes resources matching the provided parameters.
func (c *Client) ListResources(ctx context.Context, clusterID, kind, namespace string, opts *ListOptions) (*unstructured.UnstructuredList, error) {
	ri, err := c.getResourceInterfaceByKind(clusterID, kind, namespace)
	if err != nil {
		return nil, err
	}

	listOpts := metav1.ListOptions{}
	if opts != nil {
		if opts.LabelSelector != "" {
			listOpts.LabelSelector = opts.LabelSelector
		}
		if opts.FieldSelector != "" {
			listOpts.FieldSelector = opts.FieldSelector
		}
		if opts.Limit > 0 {
			listOpts.Limit = opts.Limit
		}
	}
	return ri.List(ctx, listOpts)
}

// CreateResource creates a new Kubernetes resource.
func (c *Client) CreateResource(ctx context.Context, clusterID string, resource *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	ri, err := c.getResourceInterfaceByKind(clusterID, resource.GetKind(), resource.GetNamespace())
	if err != nil {
		return nil, err
	}
	return ri.Create(ctx, resource, metav1.CreateOptions{})
}

// PatchResource patches an existing Kubernetes resource using JSON patch.
func (c *Client) PatchResource(ctx context.Context, clusterID, kind, namespace, name string, patch []byte) (*unstructured.Unstructured, error) {
	ri, err := c.getResourceInterfaceByKind(clusterID, kind, namespace)
	if err != nil {
		return nil, err
	}
	return ri.Patch(ctx, name, types.JSONPatchType, patch, metav1.PatchOptions{})
}

// DeleteResource deletes a Kubernetes resource.
func (c *Client) DeleteResource(ctx context.Context, clusterID, kind, namespace, name string) error {
	ri, err := c.getResourceInterfaceByKind(clusterID, kind, namespace)
	if err != nil {
		return err
	}
	return ri.Delete(ctx, name, metav1.DeleteOptions{})
}

// PodLogOptions contains options for fetching pod logs.
type PodLogOptions struct {
	Container    string
	TailLines    *int64
	SinceSeconds *int64
	Timestamps   bool
	Previous     bool
}

// GetPodLogs retrieves logs from a specific pod and container.
func (c *Client) GetPodLogs(ctx context.Context, clusterID, namespace, podName string, opts *PodLogOptions) (string, error) {
	clientset, err := c.getClientset(clusterID)
	if err != nil {
		return "", fmt.Errorf("failed to create clientset: %w", err)
	}

	podLogOpts := &corev1.PodLogOptions{}
	if opts != nil {
		if opts.Container != "" {
			podLogOpts.Container = opts.Container
		}
		if opts.TailLines != nil {
			podLogOpts.TailLines = opts.TailLines
		}
		if opts.SinceSeconds != nil {
			podLogOpts.SinceSeconds = opts.SinceSeconds
		}
		podLogOpts.Timestamps = opts.Timestamps
		podLogOpts.Previous = opts.Previous
	}

	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, podLogOpts)
	stream, err := req.Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to open log stream: %w", err)
	}
	defer stream.Close()

	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, stream); err != nil {
		return "", fmt.Errorf("failed to read log stream: %w", err)
	}

	return buf.String(), nil
}

// GetAllContainerLogs retrieves logs from all containers in a pod.
func (c *Client) GetAllContainerLogs(ctx context.Context, clusterID, namespace, podName string, opts *PodLogOptions) (map[string]string, error) {
	// First get the pod to find all containers
	pod, err := c.GetResource(ctx, clusterID, "pod", namespace, podName)
	if err != nil {
		return nil, fmt.Errorf("failed to get pod: %w", err)
	}

	// Extract container names from pod spec
	containers, found, err := unstructured.NestedSlice(pod.Object, "spec", "containers")
	if err != nil || !found {
		return nil, fmt.Errorf("failed to get containers from pod spec: %w", err)
	}

	tailLines := int64(100)
	timestamps := false
	if opts != nil {
		if opts.TailLines != nil {
			tailLines = *opts.TailLines
		}
		timestamps = opts.Timestamps
	}

	logs := make(map[string]string)
	for _, ctr := range containers {
		container, ok := ctr.(map[string]interface{})
		if !ok {
			continue
		}
		name, ok := container["name"].(string)
		if !ok {
			continue
		}

		logOpts := &PodLogOptions{
			Container:  name,
			TailLines:  &tailLines,
			Timestamps: timestamps,
		}
		containerLogs, err := c.GetPodLogs(ctx, clusterID, namespace, podName, logOpts)
		if err != nil {
			logs[name] = fmt.Sprintf("Error getting logs: %v", err)
		} else {
			logs[name] = containerLogs
		}
	}

	return logs, nil
}

// MultiPodLogResult contains the log result for a single pod
type MultiPodLogResult struct {
	Pod       string            `json:"pod"`
	Namespace string            `json:"namespace,omitempty"`
	Logs      map[string]string `json:"logs,omitempty"`
	Error     string            `json:"error,omitempty"`
}

// GetMultiPodLogs retrieves logs from multiple pods using label selector and merges them.
// Returns logs organized by pod name, with each pod's logs organized by container name.
func (c *Client) GetMultiPodLogs(ctx context.Context, clusterID, namespace string, labelSelector string, opts *PodLogOptions) ([]MultiPodLogResult, error) {
	// List pods matching the label selector
	listOpts := &ListOptions{
		LabelSelector: labelSelector,
	}
	podList, err := c.ListResources(ctx, clusterID, "pod", namespace, listOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	if len(podList.Items) == 0 {
		return []MultiPodLogResult{}, nil
	}

	results := make([]MultiPodLogResult, 0, len(podList.Items))

	for _, pod := range podList.Items {
		podName := pod.GetName()
		podNamespace := pod.GetNamespace()

		result := MultiPodLogResult{
			Pod:       podName,
			Namespace: podNamespace,
			Logs:      make(map[string]string),
		}

		// Get logs for all containers in this pod
		containerLogs, err := c.GetAllContainerLogs(ctx, clusterID, podNamespace, podName, opts)
		if err != nil {
			result.Error = err.Error()
		} else {
			result.Logs = containerLogs
		}

		results = append(results, result)
	}

	return results, nil
}

// InspectPodResult contains the results of inspecting a pod.
type InspectPodResult struct {
	Pod      *unstructured.Unstructured `json:"pod"`
	Parent   *unstructured.Unstructured `json:"parent,omitempty"`
	Metrics  *unstructured.Unstructured `json:"metrics,omitempty"`
	Logs     map[string]string          `json:"logs"`
}

// findPodParent finds the parent workload (Deployment/StatefulSet/DaemonSet/Job) of a pod.
func (c *Client) findPodParent(ctx context.Context, clusterID, namespace string, pod *unstructured.Unstructured) *unstructured.Unstructured {
	ownerRefs, found, _ := unstructured.NestedSlice(pod.Object, "metadata", "ownerReferences")
	if !found || len(ownerRefs) == 0 {
		return nil
	}

	for _, ref := range ownerRefs {
		ownerRef, ok := ref.(map[string]interface{})
		if !ok {
			continue
		}
		kind, _ := ownerRef["kind"].(string)
		name, _ := ownerRef["name"].(string)

		switch kind {
		case "ReplicaSet":
			// ReplicaSet is usually owned by a Deployment
			if parent := c.findReplicaSetParent(ctx, clusterID, namespace, name); parent != nil {
				return parent
			}
		case "StatefulSet", "DaemonSet", "Job":
			if parent, err := c.GetResource(ctx, clusterID, kind, namespace, name); err == nil {
				return parent
			}
		}
	}
	return nil
}

// findReplicaSetParent finds the parent workload of a ReplicaSet.
func (c *Client) findReplicaSetParent(ctx context.Context, clusterID, namespace, rsName string) *unstructured.Unstructured {
	rs, err := c.GetResource(ctx, clusterID, "replicaset", namespace, rsName)
	if err != nil {
		return nil
	}

	ownerRefs, found, _ := unstructured.NestedSlice(rs.Object, "metadata", "ownerReferences")
	if !found {
		return nil
	}

	for _, ref := range ownerRefs {
		ownerRef, ok := ref.(map[string]interface{})
		if !ok {
			continue
		}
		kind, _ := ownerRef["kind"].(string)
		name, _ := ownerRef["name"].(string)

		if kind == "Deployment" || kind == "StatefulSet" || kind == "DaemonSet" {
			if parent, err := c.GetResource(ctx, clusterID, kind, namespace, name); err == nil {
				return parent
			}
		}
	}
	return nil
}

// InspectPod retrieves comprehensive information about a pod including its parent, metrics, and logs.
func (c *Client) InspectPod(ctx context.Context, clusterID, namespace, podName string) (*InspectPodResult, error) {
	pod, err := c.GetResource(ctx, clusterID, "pod", namespace, podName)
	if err != nil {
		return nil, fmt.Errorf("failed to get pod: %w", err)
	}

	result := &InspectPodResult{
		Pod:    pod,
		Parent: c.findPodParent(ctx, clusterID, namespace, pod),
		Logs:   make(map[string]string),
	}

	// Get pod metrics (ignore error as metrics-server might not be installed)
	result.Metrics, _ = c.GetResource(ctx, clusterID, "pod.metrics.k8s.io", namespace, podName)

	// Get container logs
	tailLines := int64(50)
	if logs, err := c.GetAllContainerLogs(ctx, clusterID, namespace, podName, &PodLogOptions{TailLines: &tailLines}); err == nil {
		result.Logs = logs
	}

	return result, nil
}

// ToJSON converts the InspectPodResult to a JSON string.
func (r *InspectPodResult) ToJSON() (string, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// buildEventFieldSelector constructs a field selector string for filtering events
// by involved object properties.
func buildEventFieldSelector(name, namespace, kind string) string {
	var selectors []string
	if name != "" {
		selectors = append(selectors, "involvedObject.name="+name)
	}
	if namespace != "" {
		selectors = append(selectors, "involvedObject.namespace="+namespace)
	}
	if kind != "" {
		selectors = append(selectors, "involvedObject.kind="+kind)
	}
	return strings.Join(selectors, ",")
}

// GetEvents retrieves Kubernetes events related to a specific resource.
// Filters by involvedObject fields: name, namespace, and optionally kind.
func (c *Client) GetEvents(ctx context.Context, clusterID, namespace, name, kind string) ([]corev1.Event, error) {
	clientset, err := c.getClientset(clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	listOpts := metav1.ListOptions{
		FieldSelector: buildEventFieldSelector(name, namespace, kind),
	}

	eventList, err := clientset.CoreV1().Events(namespace).List(ctx, listOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to list events: %w", err)
	}

	return eventList.Items, nil
}

// DescribeResult contains the results of describing a resource.
type DescribeResult struct {
	Resource *unstructured.Unstructured `json:"resource"`
	Events   []corev1.Event             `json:"events,omitempty"`
}

// ToJSON converts the DescribeResult to a JSON string.
func (r *DescribeResult) ToJSON() (string, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// DescribeResource retrieves a Kubernetes resource along with its related events.
func (c *Client) DescribeResource(ctx context.Context, clusterID, kind, namespace, name string) (*DescribeResult, error) {
	resource, err := c.GetResource(ctx, clusterID, kind, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get resource: %w", err)
	}

	result := &DescribeResult{Resource: resource}

	// Use the resource's actual Kind (proper casing) for event field selector
	if events, err := c.GetEvents(ctx, clusterID, namespace, name, resource.GetKind()); err == nil {
		result.Events = events
	}

	return result, nil
}
