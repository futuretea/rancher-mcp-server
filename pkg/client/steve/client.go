// Package steve provides a client for the Rancher Steve API, which exposes
// Kubernetes resources through a simplified REST interface.
package steve

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"sync"

	"github.com/futuretea/rancher-mcp-server/pkg/util/url"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
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
	serverURL string
	token     string
	accessKey string
	secretKey string
	insecure  bool

	cacheMu        sync.Mutex
	dynamicClients map[string]dynamic.Interface
	clientsets     map[string]kubernetes.Interface
	transports     map[string]*http.Transport
}

// NewClient creates a new Steve API client.
func NewClient(serverURL, token, accessKey, secretKey string, insecure bool) *Client {
	return &Client{
		serverURL:      serverURL,
		token:          token,
		accessKey:      accessKey,
		secretKey:      secretKey,
		insecure:       insecure,
		dynamicClients: make(map[string]dynamic.Interface),
		clientsets:     make(map[string]kubernetes.Interface),
		transports:     make(map[string]*http.Transport),
	}
}

// NewClientWithToken creates a new Steve API client bound to a single request token.
func NewClientWithToken(serverURL, token string, insecure bool) *Client {
	return NewClient(serverURL, token, "", "", insecure)
}

// Close releases resources held by the client. After Close the client must not be used.
func (c *Client) Close() {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	for _, transport := range c.transports {
		if transport != nil {
			transport.CloseIdleConnections()
		}
	}
	c.dynamicClients = nil
	c.clientsets = nil
	c.transports = nil
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

	restConfig, err := clientcmd.NewNonInteractiveClientConfig(
		*kubeconfig,
		kubeconfig.CurrentContext,
		&clientcmd.ConfigOverrides{},
		nil,
	).ClientConfig()
	if err != nil {
		return nil, err
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: c.insecure},
	}
	// A custom transport conflicts with rest.Config TLS flags; clear them and
	// rely on the transport for TLS behavior.
	restConfig.Insecure = false
	restConfig.TLSClientConfig = rest.TLSClientConfig{}
	restConfig.Transport = transport

	// Caller holds cacheMu, so the transports map is guaranteed to be initialized.
	c.transports[clusterID] = transport

	return restConfig, nil
}

// getDynamicClient creates a dynamic Kubernetes client for the given cluster.
func (c *Client) getDynamicClient(clusterID string) (dynamic.Interface, error) {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	c.ensureCachesLocked()

	if client, ok := c.dynamicClients[clusterID]; ok {
		return client, nil
	}

	restConfig, err := c.createRestConfig(clusterID)
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

	restConfig, err := c.createRestConfig(clusterID)
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
	if c.transports == nil {
		c.transports = make(map[string]*http.Transport)
	}
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
