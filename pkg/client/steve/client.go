// Package steve provides a client for the Rancher Steve API, which exposes
// Kubernetes resources through a simplified REST interface.
package steve

import (
	"context"
	"fmt"
	"sync"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// ResourceReader is the read-only interface for querying Kubernetes resources.
// *Client satisfies this interface implicitly.
type ResourceReader interface {
	GetResource(ctx context.Context, clusterID, kind, namespace, name string) (*unstructured.Unstructured, error)
	ListResources(ctx context.Context, clusterID, kind, namespace string, opts *ListOptions) (*unstructured.UnstructuredList, error)
	GetEvents(ctx context.Context, clusterID, namespace, name, kind string) ([]corev1.Event, error)
}

// Client provides methods for interacting with Kubernetes clusters via Rancher's Steve API.
type Client struct {
	serverURL  string
	token      string
	accessKey  string
	secretKey  string
	insecure   bool
	rancher    clusterSource
	kubeconfig *kubeconfigSource

	cacheMu        sync.Mutex
	dynamicClients map[string]dynamic.Interface
	clientsets     map[string]kubernetes.Interface
}

// NewClient creates a new Steve API client.
func NewClient(serverURL, token, accessKey, secretKey string, insecure bool) *Client {
	client, _ := NewClientWithKubeconfigPaths(serverURL, token, accessKey, secretKey, insecure, nil)
	return client
}

// NewClientWithToken creates a new Steve API client bound to a single request token.
func NewClientWithToken(serverURL, token string, insecure bool) *Client {
	return NewClient(serverURL, token, "", "", insecure)
}

// NewClientWithKubeconfigPaths creates a client for configured Rancher and kubeconfig cluster sources.
func NewClientWithKubeconfigPaths(serverURL, token, accessKey, secretKey string, insecure bool, kubeconfigPaths []string) (*Client, error) {
	client := &Client{
		serverURL:      serverURL,
		token:          token,
		accessKey:      accessKey,
		secretKey:      secretKey,
		insecure:       insecure,
		dynamicClients: make(map[string]dynamic.Interface),
		clientsets:     make(map[string]kubernetes.Interface),
	}
	if serverURL != "" {
		client.rancher = &rancherSource{client: client}
	}
	if len(kubeconfigPaths) > 0 {
		source, err := newKubeconfigSource(kubeconfigPaths)
		if err != nil {
			return nil, err
		}
		client.kubeconfig = source
	}
	return client, nil
}

// Close releases resources held by the client. After Close the client must not be used.
func (c *Client) Close() {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	c.dynamicClients = nil
	c.clientsets = nil
}

// ListOptions contains options for listing resources.
type ListOptions struct {
	LabelSelector string
	FieldSelector string
	Limit         int64
}

// WatchOptions contains options for watching resources.
type WatchOptions struct {
	LabelSelector string
	FieldSelector string
}

// createRestConfig creates a Kubernetes REST config for the given cluster.
func (c *Client) createRestConfig(clusterID string) (*rest.Config, error) {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	c.ensureCachesLocked()

	return c.createRestConfigLocked(clusterID)
}

// createRestConfigLocked creates a Kubernetes REST config while cacheMu is held.
func (c *Client) createRestConfigLocked(clusterID string) (*rest.Config, error) {
	rancher := c.rancher
	if rancher == nil && c.serverURL != "" {
		rancher = &rancherSource{client: c}
	}

	source, sourceReference, err := parseClusterReference(clusterID, rancher, c.kubeconfig)
	if err != nil {
		return nil, err
	}
	return source.restConfig(sourceReference)
}

// getDynamicClient creates a dynamic Kubernetes client for the given cluster.
func (c *Client) getDynamicClient(clusterID string) (dynamic.Interface, error) {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	c.ensureCachesLocked()

	if client, ok := c.dynamicClients[clusterID]; ok {
		return client, nil
	}

	restConfig, err := c.createRestConfigLocked(clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to create REST config: %w", err)
	}

	client, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	c.dynamicClients[clusterID] = client
	return client, nil
}

// getClientset creates a typed Kubernetes clientset for the given cluster.
func (c *Client) getClientset(clusterID string) (kubernetes.Interface, error) {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	c.ensureCachesLocked()

	if clientset, ok := c.clientsets[clusterID]; ok {
		return clientset, nil
	}

	restConfig, err := c.createRestConfigLocked(clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to create REST config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	c.clientsets[clusterID] = clientset
	return clientset, nil
}

func (c *Client) ensureCachesLocked() {
	if c.dynamicClients == nil {
		c.dynamicClients = make(map[string]dynamic.Interface)
	}
	if c.clientsets == nil {
		c.clientsets = make(map[string]kubernetes.Interface)
	}
}

// KubeconfigContexts returns configured kubeconfig context names.
func (c *Client) KubeconfigContexts() []string {
	if c == nil || c.kubeconfig == nil {
		return nil
	}
	return c.kubeconfig.contextNames()
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

// WatchResources watches Kubernetes resources matching the provided
// parameters and returns a watch.Interface for consuming events.
func (c *Client) WatchResources(ctx context.Context, clusterID, kind, namespace string, opts *WatchOptions) (watch.Interface, error) {
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
	}

	return ri.Watch(ctx, listOpts)
}
