package steve

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type dottedKindCandidate struct {
	resource string
	apiGroup string
}

// KindWithAPIVersion builds the internal resource reference used by Steve client
// methods when callers provide Kubernetes manifest-style apiVersion + kind.
func KindWithAPIVersion(apiVersion, kind string) string {
	apiVersion = strings.TrimSpace(apiVersion)
	kind = strings.TrimSpace(kind)
	if apiVersion == "" {
		return kind
	}
	if kind == "" {
		return apiVersion
	}
	return apiVersion + "/" + kind
}

func parseAPIVersionKind(ref string) (apiVersion, kind string, ok bool) {
	ref = strings.TrimSpace(ref)
	idx := strings.LastIndex(ref, "/")
	if idx <= 0 || idx == len(ref)-1 {
		return "", "", false
	}
	apiVersion = strings.TrimSpace(ref[:idx])
	kind = strings.TrimSpace(ref[idx+1:])
	if apiVersion == "" || kind == "" {
		return "", "", false
	}
	if _, err := schema.ParseGroupVersion(apiVersion); err != nil {
		return "", "", false
	}
	return apiVersion, kind, true
}

func gvrMatchesAPIVersion(gvr schema.GroupVersionResource, apiVersion string) bool {
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return false
	}
	return gvr.Group == gv.Group && gvr.Version == gv.Version
}

func parseDottedKindCandidates(dottedKind string) []dottedKindCandidate {
	dottedKind = strings.Trim(strings.ToLower(strings.TrimSpace(dottedKind)), ".")
	if dottedKind == "" {
		return nil
	}

	var candidates []dottedKindCandidate
	if firstDot := strings.Index(dottedKind, "."); firstDot > 0 && firstDot < len(dottedKind)-1 {
		candidates = append(candidates, dottedKindCandidate{
			resource: dottedKind[:firstDot],
			apiGroup: dottedKind[firstDot+1:],
		})
	}
	if lastDot := strings.LastIndex(dottedKind, "."); lastDot > 0 && lastDot < len(dottedKind)-1 {
		lastCandidate := dottedKindCandidate{
			resource: dottedKind[lastDot+1:],
			apiGroup: dottedKind[:lastDot],
		}
		if len(candidates) == 0 || candidates[0] != lastCandidate {
			candidates = append(candidates, lastCandidate)
		}
	}

	return candidates
}

func findPreferredGroupVersion(groups *metav1.APIGroupList, apiGroup string) (string, bool) {
	for _, g := range groups.Groups {
		if g.Name == apiGroup {
			return g.PreferredVersion.GroupVersion, true
		}
	}
	return "", false
}

func findAPIResourceGVR(groupVersion, resourceName string, resources []metav1.APIResource) (schema.GroupVersionResource, bool) {
	gv, err := schema.ParseGroupVersion(groupVersion)
	if err != nil {
		return schema.GroupVersionResource{}, false
	}
	for _, r := range resources {
		if strings.Contains(r.Name, "/") {
			continue
		}
		if matchesResourceName(r, resourceName) {
			return schema.GroupVersionResource{
				Group:    gv.Group,
				Version:  gv.Version,
				Resource: r.Name,
			}, true
		}
	}
	return schema.GroupVersionResource{}, false
}

func appendUniqueGVR(matches []schema.GroupVersionResource, gvr schema.GroupVersionResource) []schema.GroupVersionResource {
	for _, match := range matches {
		if match == gvr {
			return matches
		}
	}
	return append(matches, gvr)
}

func describeGVRMatches(matches []schema.GroupVersionResource) string {
	parts := make([]string, 0, len(matches))
	for _, match := range matches {
		apiVersion := match.Version
		if match.Group != "" {
			apiVersion = match.Group + "/" + match.Version
		}
		parts = append(parts, fmt.Sprintf("%s %s", apiVersion, match.Resource))
	}
	return strings.Join(parts, ", ")
}

// getResourceInterfaceByKind resolves the kind to GVR and returns a dynamic resource interface.
// It accepts built-in kind aliases, Kubernetes apiVersion/kind references, plain discovered
// kinds, and legacy dotted resource forms.
func (c *Client) getResourceInterfaceByKind(clusterID, kind, namespace string) (dynamic.ResourceInterface, error) {
	gvr, err := c.resolveGVR(clusterID, kind)
	if err != nil {
		return nil, err
	}
	return c.getResourceInterface(clusterID, gvr, namespace)
}

func (c *Client) resolveGVR(clusterID, kind string) (schema.GroupVersionResource, error) {
	original := strings.TrimSpace(kind)
	if original == "" {
		return schema.GroupVersionResource{}, fmt.Errorf("unsupported resource kind: %s", kind)
	}

	if apiVersion, apiKind, ok := parseAPIVersionKind(original); ok {
		normalizedKind := strings.ToLower(apiKind)
		if gvr, ok := GetGVR(normalizedKind); ok && gvrMatchesAPIVersion(gvr, apiVersion) {
			return gvr, nil
		}
		gvr, err := c.discoverGVRForAPIVersionKind(clusterID, apiVersion, normalizedKind)
		if err != nil {
			return schema.GroupVersionResource{}, fmt.Errorf("unsupported resource kind: %s (%w)", original, err)
		}
		return gvr, nil
	}

	normalized := strings.ToLower(original)
	if gvr, ok := GetGVR(normalized); ok {
		return gvr, nil
	}

	if strings.Contains(normalized, ".") {
		gvr, err := c.discoverDottedGVR(clusterID, normalized)
		if err == nil {
			return gvr, nil
		}
	}

	gvr, err := c.discoverGVRByKind(clusterID, normalized)
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("unsupported resource kind: %s (%w)", original, err)
	}
	return gvr, nil
}

func (c *Client) discoverGVRForAPIVersionKind(clusterID, apiVersion, kind string) (schema.GroupVersionResource, error) {
	clientset, err := c.getClientset(clusterID)
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("failed to create clientset: %w", err)
	}

	resourceList, err := clientset.Discovery().ServerResourcesForGroupVersion(apiVersion)
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("failed to discover resources for %s: %w", apiVersion, err)
	}

	if gvr, ok := findAPIResourceGVR(apiVersion, kind, resourceList.APIResources); ok {
		return gvr, nil
	}

	return schema.GroupVersionResource{}, fmt.Errorf("resource kind %s not found in %s", kind, apiVersion)
}

func (c *Client) discoverGVRByKind(clusterID, kind string) (schema.GroupVersionResource, error) {
	clientset, err := c.getClientset(clusterID)
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("failed to create clientset: %w", err)
	}

	var matches []schema.GroupVersionResource
	if resourceList, err := clientset.Discovery().ServerResourcesForGroupVersion("v1"); err == nil {
		if gvr, ok := findAPIResourceGVR("v1", kind, resourceList.APIResources); ok {
			matches = appendUniqueGVR(matches, gvr)
		}
	}

	groups, err := clientset.Discovery().ServerGroups()
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("failed to discover API groups: %w", err)
	}

	for _, group := range groups.Groups {
		groupVersion := group.PreferredVersion.GroupVersion
		resourceList, err := clientset.Discovery().ServerResourcesForGroupVersion(groupVersion)
		if err != nil {
			continue
		}
		if gvr, ok := findAPIResourceGVR(groupVersion, kind, resourceList.APIResources); ok {
			matches = appendUniqueGVR(matches, gvr)
		}
	}

	switch len(matches) {
	case 0:
		return schema.GroupVersionResource{}, fmt.Errorf("resource kind %s not found", kind)
	case 1:
		return matches[0], nil
	default:
		return schema.GroupVersionResource{}, fmt.Errorf("resource kind %s is ambiguous; specify apiVersion. Matches: %s", kind, describeGVRMatches(matches))
	}
}

// discoverDottedGVR resolves dotted resource kinds to a GroupVersionResource using
// Kubernetes API discovery. It supports both historical <resource>.<apiGroup>
// input and Steve-style <apiGroup>.<resource-or-kind> input.
func (c *Client) discoverDottedGVR(clusterID, dottedKind string) (schema.GroupVersionResource, error) {
	candidates := parseDottedKindCandidates(dottedKind)
	if len(candidates) == 0 {
		return schema.GroupVersionResource{}, fmt.Errorf("invalid dotted kind format: %s", dottedKind)
	}

	clientset, err := c.getClientset(clusterID)
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("failed to create clientset: %w", err)
	}

	groups, err := clientset.Discovery().ServerGroups()
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("failed to discover API groups: %w", err)
	}

	for _, candidate := range candidates {
		groupVersion, ok := findPreferredGroupVersion(groups, candidate.apiGroup)
		if !ok {
			continue
		}
		resourceList, err := clientset.Discovery().ServerResourcesForGroupVersion(groupVersion)
		if err != nil {
			continue
		}
		if gvr, ok := findAPIResourceGVR(groupVersion, candidate.resource, resourceList.APIResources); ok {
			return gvr, nil
		}
	}

	return schema.GroupVersionResource{}, fmt.Errorf("resource %s not found", dottedKind)
}

// normalizedResourceNames returns the lowercased, trimmed singular, plural and kind
// names for a discovered API resource. An empty SingularName falls back to Kind.
func normalizedResourceNames(r metav1.APIResource) (singular, resource, kind string) {
	singular = strings.ToLower(strings.TrimSpace(r.SingularName))
	resource = strings.ToLower(strings.TrimSpace(r.Name))
	kind = strings.ToLower(strings.TrimSpace(r.Kind))
	if singular == "" {
		singular = kind
	}
	return
}

// matchesResourceName checks if an API resource matches the given name
// by comparing against its singular name, plural name, or lowercased Kind.
func matchesResourceName(r metav1.APIResource, name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	singularName, resourceName, kindName := normalizedResourceNames(r)
	return singularName == name || resourceName == name || kindName == name
}
