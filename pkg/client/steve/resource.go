package steve

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

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
	kind := KindWithAPIVersion(resource.GetAPIVersion(), resource.GetKind())
	ri, err := c.getResourceInterfaceByKind(clusterID, kind, resource.GetNamespace())
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
