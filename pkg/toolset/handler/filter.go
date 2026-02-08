package handler

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ResourceFilter provides configurable filtering for Kubernetes resources.
// It removes specified fields from resources based on user-configured paths.
type ResourceFilter struct {
	// paths contains the list of field paths to remove from resources.
	// Path format: dot-separated keys, e.g. "metadata.managedFields"
	// Supports nested paths like "metadata.annotations.kubectl.kubernetes.io/last-applied-configuration"
	paths []string
}

// NewResourceFilter creates a new ResourceFilter with the specified paths.
func NewResourceFilter(paths []string) *ResourceFilter {
	return &ResourceFilter{
		paths: paths,
	}
}

// NewResourceFilterFromParams creates a ResourceFilter from handler params.
// Returns nil if no filters are configured.
func NewResourceFilterFromParams(params map[string]interface{}) *ResourceFilter {
	filters, ok := params["outputFilters"]
	if !ok {
		return nil
	}

	// Handle []string type
	if paths, ok := filters.([]string); ok {
		if len(paths) == 0 {
			return nil
		}
		return NewResourceFilter(paths)
	}

	// Handle []interface{} type (from JSON unmarshaling)
	if ifaces, ok := filters.([]interface{}); ok {
		paths := make([]string, 0, len(ifaces))
		for _, v := range ifaces {
			if s, ok := v.(string); ok {
				paths = append(paths, s)
			}
		}
		if len(paths) == 0 {
			return nil
		}
		return NewResourceFilter(paths)
	}

	return nil
}

// DefaultFilterPaths returns recommended default filter paths for reducing output verbosity.
// Users can use these as a starting point for their configuration.
func DefaultFilterPaths() []string {
	return []string{
		"metadata.managedFields",
		"metadata.annotations.kubectl.kubernetes.io/last-applied-configuration",
	}
}

// Filter removes the configured fields from a resource and returns a cleaned copy.
// The original resource is not modified.
func (f *ResourceFilter) Filter(obj *unstructured.Unstructured) *unstructured.Unstructured {
	if obj == nil || len(f.paths) == 0 {
		return obj
	}

	// Create a deep copy to avoid modifying the original
	result := obj.DeepCopy()

	for _, path := range f.paths {
		f.removePath(result.Object, path)
	}

	return result
}

// FilterList applies filtering to all resources in a list.
func (f *ResourceFilter) FilterList(list *unstructured.UnstructuredList) *unstructured.UnstructuredList {
	if list == nil || len(f.paths) == 0 {
		return list
	}

	result := &unstructured.UnstructuredList{
		Object: list.Object,
		Items:  make([]unstructured.Unstructured, len(list.Items)),
	}

	for i, item := range list.Items {
		filtered := f.Filter(&item)
		if filtered != nil {
			result.Items[i] = *filtered
		}
	}

	return result
}

// removePath removes a field at the specified path from the object.
// Path format: dot-separated keys, e.g. "metadata.managedFields"
func (f *ResourceFilter) removePath(obj map[string]interface{}, path string) {
	if obj == nil || path == "" {
		return
	}

	parts := parsePath(path)
	if len(parts) == 0 {
		return
	}

	// Navigate to parent and remove the final key
	current := obj
	for i := 0; i < len(parts)-1; i++ {
		next, ok := current[parts[i]]
		if !ok {
			return // Path doesn't exist
		}
		nextMap, ok := next.(map[string]interface{})
		if !ok {
			return // Path doesn't lead to a map
		}
		current = nextMap
	}

	// Remove the final key
	delete(current, parts[len(parts)-1])
}

// parsePath splits a path string into parts.
// Handles both dot notation and slash-containing keys.
// e.g., "metadata.annotations.kubectl.kubernetes.io/last-applied-configuration"
// becomes ["metadata", "annotations", "kubectl.kubernetes.io/last-applied-configuration"]
func parsePath(path string) []string {
	if path == "" {
		return nil
	}

	// Special handling for well-known annotation keys containing dots
	wellKnownAnnotations := []string{
		"kubectl.kubernetes.io/last-applied-configuration",
		"kubernetes.io/",
		"k8s.io/",
	}

	var parts []string
	remaining := path

	for remaining != "" {
		// Find the next dot
		dotIdx := strings.Index(remaining, ".")
		if dotIdx == -1 {
			parts = append(parts, remaining)
			break
		}

		// Check if this might be part of a well-known annotation key
		isAnnotationKey := false
		for _, annotation := range wellKnownAnnotations {
			if strings.Contains(remaining[dotIdx:], annotation) {
				// Check if we're at the annotations level
				if len(parts) >= 2 && parts[len(parts)-1] == "annotations" {
					// Rest of the path is the annotation key
					parts = append(parts, remaining)
					remaining = ""
					isAnnotationKey = true
					break
				}
			}
		}

		if isAnnotationKey {
			break
		}

		parts = append(parts, remaining[:dotIdx])
		remaining = remaining[dotIdx+1:]
	}

	return parts
}
