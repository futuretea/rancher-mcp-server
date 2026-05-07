package steve

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
