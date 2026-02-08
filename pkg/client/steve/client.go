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
func (c *Client) getResourceInterfaceByKind(clusterID, kind, namespace string) (dynamic.ResourceInterface, error) {
	kind = strings.ToLower(kind)
	gvr, ok := GetGVR(kind)
	if !ok {
		return nil, fmt.Errorf("unsupported resource kind: %s", kind)
	}
	return c.getResourceInterface(clusterID, gvr, namespace)
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
func (c *Client) GetAllContainerLogs(ctx context.Context, clusterID, namespace, podName string, tailLines int64) (map[string]string, error) {
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
			Container: name,
			TailLines: &tailLines,
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
	if logs, err := c.GetAllContainerLogs(ctx, clusterID, namespace, podName, 50); err == nil {
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
