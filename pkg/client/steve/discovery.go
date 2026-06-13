package steve

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
)

// APIResourceInfo contains information about a Kubernetes API resource type.
type APIResourceInfo struct {
	Name         string
	SingularName string
	Namespaced   bool
	Kind         string
	Group        string
	Version      string
	Verbs        []string
}

// GVR returns the GroupVersionResource for this API resource.
func (r *APIResourceInfo) GVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    r.Group,
		Version:  r.Version,
		Resource: r.Name,
	}
}

// ListAPIResources discovers all API resources available in the cluster.
// It returns a list of resource types that can be used to fetch all resources.
func (c *Client) ListAPIResources(_ context.Context, clusterID string) ([]APIResourceInfo, error) {
	clientset, err := c.getClientset(clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	groups, err := clientset.Discovery().ServerGroups()
	if err != nil {
		return nil, fmt.Errorf("failed to discover API groups: %w", err)
	}

	allResources, err := c.listCoreAPIResources(clientset)
	if err != nil {
		return nil, err
	}

	for _, g := range groups.Groups {
		groupVersion := g.PreferredVersion.GroupVersion
		resources, err := clientset.Discovery().ServerResourcesForGroupVersion(groupVersion)
		if err != nil {
			// Some groups might not be accessible; skip them.
			continue
		}

		gv, _ := schema.ParseGroupVersion(groupVersion)
		allResources = append(allResources, convertAPIResources(resources.APIResources, gv.Group, gv.Version)...)
	}

	return allResources, nil
}

// convertAPIResources transforms discovery API resources into APIResourceInfo values,
// filtering out sub-resources (names containing "/").
func convertAPIResources(resources []metav1.APIResource, group, version string) []APIResourceInfo {
	var result []APIResourceInfo
	for _, r := range resources {
		if strings.Contains(r.Name, "/") {
			continue
		}
		result = append(result, APIResourceInfo{
			Name:         r.Name,
			SingularName: r.SingularName,
			Namespaced:   r.Namespaced,
			Kind:         r.Kind,
			Group:        group,
			Version:      version,
			Verbs:        r.Verbs,
		})
	}
	return result
}

func (c *Client) listCoreAPIResources(clientset kubernetes.Interface) ([]APIResourceInfo, error) {
	coreResources, err := clientset.Discovery().ServerResourcesForGroupVersion("v1")
	if err != nil {
		return nil, fmt.Errorf("failed to discover core API resources: %w", err)
	}

	return convertAPIResources(coreResources.APIResources, "", "v1"), nil
}

// GetAllOptions contains options for GetAllResources.
type GetAllOptions struct {
	Namespace     string
	ExcludeEvents bool
	Scope         string // "namespaced", "cluster", or "" (all)
	Limit         int64
}

// AllResourceItem represents a single resource found by GetAllResources.
type AllResourceItem struct {
	Name       string
	Namespace  string
	Kind       string
	APIVersion string
	Resource   *unstructured.Unstructured
}

// AllResourcesResult contains all resources retrieved by GetAllResources.
type AllResourcesResult struct {
	Items []AllResourceItem
}

// GetAllResources retrieves all resources across all (or specified) resource types.
// Similar to ketall, it fetches resources that "kubectl get all" doesn't show.
func (c *Client) GetAllResources(ctx context.Context, clusterID string, opts *GetAllOptions) (*AllResourcesResult, error) {
	if opts == nil {
		opts = &GetAllOptions{}
	}

	apiResources, err := c.ListAPIResources(ctx, clusterID)
	if err != nil {
		return nil, err
	}

	result := &AllResourcesResult{
		Items: make([]AllResourceItem, 0),
	}

	for _, ar := range apiResources {
		if !c.shouldFetchResource(ar, opts) {
			continue
		}

		namespace := resolveResourceNamespace(ar, opts.Namespace)
		items, err := c.listResourcesForType(ctx, clusterID, ar, namespace, opts.Limit)
		if err != nil {
			// Skip resources that cannot be listed.
			continue
		}
		result.Items = append(result.Items, items...)
	}

	return result, nil
}

func (c *Client) shouldFetchResource(ar APIResourceInfo, opts *GetAllOptions) bool {
	if !hasVerb(ar.Verbs, "list") {
		return false
	}
	if opts.ExcludeEvents && ar.Name == "events" {
		return false
	}
	return matchesScope(ar.Namespaced, opts.Scope)
}

func matchesScope(isNamespaced bool, scope string) bool {
	switch scope {
	case "namespaced":
		return isNamespaced
	case "cluster":
		return !isNamespaced
	default:
		return true
	}
}

func resolveResourceNamespace(ar APIResourceInfo, optsNamespace string) string {
	if ar.Namespaced {
		return optsNamespace
	}
	return ""
}

func (c *Client) listResourcesForType(ctx context.Context, clusterID string, ar APIResourceInfo, namespace string, limit int64) ([]AllResourceItem, error) {
	gvr := ar.GVR()
	ri, err := c.getResourceInterface(clusterID, gvr, namespace)
	if err != nil {
		return nil, err
	}

	list, err := ri.List(ctx, metav1.ListOptions{Limit: limit})
	if err != nil {
		return nil, err
	}

	items := make([]AllResourceItem, 0, len(list.Items))
	for i := range list.Items {
		item := &list.Items[i]
		items = append(items, AllResourceItem{
			Name:       item.GetName(),
			Namespace:  item.GetNamespace(),
			Kind:       ar.Kind,
			APIVersion: item.GetAPIVersion(),
			Resource:   item,
		})
	}
	return items, nil
}

// hasVerb checks if a verb is in the list of verbs.
func hasVerb(verbs []string, verb string) bool {
	for _, v := range verbs {
		if v == verb {
			return true
		}
	}
	return false
}
