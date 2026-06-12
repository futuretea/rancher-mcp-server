package steve

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

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
