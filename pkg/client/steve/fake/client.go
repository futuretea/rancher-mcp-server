// Package fake provides an in-memory fake Steve client for testing.
package fake

import (
	"context"
	"fmt"
	"strings"

	"github.com/futuretea/rancher-mcp-server/pkg/client/steve"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

var _ steve.ResourceReader = (*Client)(nil)

// Client is a programmable fake for testing analyzers without a live cluster.
type Client struct {
	resources map[string][]*unstructured.Unstructured
	events    []corev1.Event
}

// NewClient creates a new fake client.
func NewClient() *Client {
	return &Client{
		resources: make(map[string][]*unstructured.Unstructured),
	}
}

// AddResource pre-loads a test resource into the fake.
func (c *Client) AddResource(obj *unstructured.Unstructured) {
	kind := strings.ToLower(strings.TrimSpace(obj.GetKind()))
	c.resources[kind] = append(c.resources[kind], obj)
}

// GetResource looks up a resource by kind, namespace, and name.
func (c *Client) GetResource(_ context.Context, _ string, kind, namespace, name string) (*unstructured.Unstructured, error) {
	normalizedKind := strings.ToLower(strings.TrimSpace(kind))
	for _, r := range c.resources[normalizedKind] {
		if r.GetName() == name && (namespace == "" || r.GetNamespace() == namespace) {
			return r, nil
		}
	}
	return nil, fmt.Errorf("resource not found: %s/%s (kind %s)", namespace, name, kind)
}

// ListResources lists resources by kind, filtered by namespace and label selector.
func (c *Client) ListResources(_ context.Context, _ string, kind, namespace string, opts *steve.ListOptions) (*unstructured.UnstructuredList, error) {
	normalizedKind := strings.ToLower(strings.TrimSpace(kind))

	var sel labels.Selector
	if opts != nil && opts.LabelSelector != "" {
		var err error
		sel, err = labels.Parse(opts.LabelSelector)
		if err != nil {
			return nil, fmt.Errorf("invalid label selector: %w", err)
		}
	}

	result := &unstructured.UnstructuredList{}
	for _, r := range c.resources[normalizedKind] {
		if namespace != "" && r.GetNamespace() != namespace {
			continue
		}
		if sel != nil && !sel.Matches(labels.Set(r.GetLabels())) {
			continue
		}
		result.Items = append(result.Items, *r)
	}
	return result, nil
}

// AddEvent pre-loads a test event into the fake.
func (c *Client) AddEvent(event corev1.Event) {
	c.events = append(c.events, event)
}

// GetEvents returns events matching the filters (nil name/kind means no filter).
func (c *Client) GetEvents(_ context.Context, _ string, namespace, name, kind string) ([]corev1.Event, error) {
	var result []corev1.Event
	for _, e := range c.events {
		if namespace != "" && e.Namespace != namespace {
			continue
		}
		if name != "" && e.InvolvedObject.Name != name {
			continue
		}
		if kind != "" && e.InvolvedObject.Kind != kind {
			continue
		}
		result = append(result, e)
	}
	return result, nil
}
