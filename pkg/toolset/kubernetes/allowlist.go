package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/futuretea/rancher-mcp-server/pkg/client/steve"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// namespaceAllowlist is the startup allowlist. An empty map means every
// cluster is unrestricted. cmd.SetNamespaceAllowlist installs it once before
// the server starts; the map is read-only afterwards and must not be mutated
// at runtime.
var namespaceAllowlist = map[string]map[string]struct{}{}

// coreNamespacesGVR is the resolved identity of the Namespace kind.
var coreNamespacesGVR = schema.GroupVersionResource{Version: "v1", Resource: "namespaces"}

// scopeResolver resolves a kind the same way the Steve backend does and
// reports whether the resolved resource is namespaced. *steve.Client
// satisfies this interface.
type scopeResolver interface {
	ResolveResourceScope(clusterID, kind string) (schema.GroupVersionResource, bool, error)
}

// SetNamespaceAllowlist installs the clusters that have at least one
// namespace name. Empty arrays and a nil map leave every cluster unrestricted.
func SetNamespaceAllowlist(raw map[string][]string) {
	next := make(map[string]map[string]struct{}, len(raw))
	for cluster, names := range raw {
		if len(names) == 0 {
			continue
		}
		set := make(map[string]struct{}, len(names))
		for _, name := range names {
			set[name] = struct{}{}
		}
		next[cluster] = set
	}
	namespaceAllowlist = next
}

func resetNamespaceAllowlist() {
	namespaceAllowlist = map[string]map[string]struct{}{}
}

type namespaceQuery struct {
	passthrough bool
	loadByName  bool
	names       []string
}

func restrictedNames(cluster string) (map[string]struct{}, bool) {
	set, ok := namespaceAllowlist[cluster]
	return set, ok && len(set) > 0
}

func denyNamespace(cluster, namespace string) error {
	set, restricted := restrictedNames(cluster)
	if !restricted || namespace == "" {
		return nil
	}
	if _, ok := set[namespace]; ok {
		return nil
	}
	return fmt.Errorf("namespace %q is not allowed on cluster %q", namespace, cluster)
}

// resolveScope resolves kind to its GVR and namespaced bit. Built-in kinds
// resolve statically without a client; every other kind goes through the same
// resolution path the Steve backend uses, so the guard cannot disagree with
// the backend about a resource's scope.
func resolveScope(client interface{}, cluster, kind string) (schema.GroupVersionResource, bool, error) {
	if gvr, namespaced, ok := steve.ResolveStaticScope(kind); ok {
		return gvr, namespaced, nil
	}
	resolver, err := kubernetesScopeResolver(client)
	if err != nil {
		return schema.GroupVersionResource{}, false, err
	}
	gvr, namespaced, err := resolver.ResolveResourceScope(cluster, kind)
	if err != nil {
		return schema.GroupVersionResource{}, false, err
	}
	return gvr, namespaced, nil
}

func allowNamedAccess(client interface{}, cluster, kind, namespace, name string) error {
	if _, restricted := restrictedNames(cluster); !restricted {
		return nil
	}
	gvr, namespaced, err := resolveScope(client, cluster, kind)
	if err != nil {
		return err
	}
	if gvr == coreNamespacesGVR {
		target := name
		if target == "" {
			target = namespace
		}
		if target == "" {
			return fmt.Errorf("namespace is required for %s on restricted cluster %q", kind, cluster)
		}
		return denyNamespace(cluster, target)
	}
	if !namespaced {
		return nil
	}
	if namespace == "" {
		return fmt.Errorf("namespace is required for %s on restricted cluster %q", kind, cluster)
	}
	return denyNamespace(cluster, namespace)
}

func planNamespaceQuery(client interface{}, cluster, kind, namespace string) (namespaceQuery, error) {
	set, restricted := restrictedNames(cluster)
	if !restricted {
		return namespaceQuery{passthrough: true}, nil
	}
	gvr, namespaced, err := resolveScope(client, cluster, kind)
	if err != nil {
		return namespaceQuery{}, err
	}
	if gvr == coreNamespacesGVR {
		if namespace != "" {
			if err := denyNamespace(cluster, namespace); err != nil {
				return namespaceQuery{}, err
			}
			return namespaceQuery{loadByName: true, names: []string{namespace}}, nil
		}
		return namespaceQuery{loadByName: true, names: sortedNames(set)}, nil
	}
	if !namespaced {
		return namespaceQuery{passthrough: true}, nil
	}
	if namespace != "" {
		if err := denyNamespace(cluster, namespace); err != nil {
			return namespaceQuery{}, err
		}
		return namespaceQuery{passthrough: true}, nil
	}
	return namespaceQuery{names: sortedNames(set)}, nil
}

// listedNamespaces splits an omitted namespace on a restricted cluster into
// the allowlist. A passthrough result keeps the caller's namespace, including
// an empty one.
func listedNamespaces(client interface{}, cluster, kind, namespace string) (string, []string, error) {
	query, err := planNamespaceQuery(client, cluster, kind, namespace)
	if err != nil {
		return "", nil, err
	}
	if query.passthrough || query.loadByName {
		return namespace, nil, nil
	}
	return "", query.names, nil
}

func listResourcesAllowed(ctx context.Context, client interface{}, cluster, kind, namespace string, opts *steve.ListOptions) (*unstructured.UnstructuredList, error) {
	query, err := planNamespaceQuery(client, cluster, kind, namespace)
	if err != nil {
		return nil, err
	}
	reader, err := kubernetesReader(client)
	if err != nil {
		return nil, err
	}
	if query.passthrough {
		return reader.ListResources(ctx, cluster, kind, namespace, opts)
	}
	if query.loadByName {
		return getNamedNamespaces(ctx, reader, cluster, query.names, opts)
	}

	merged := &unstructured.UnstructuredList{}
	for _, name := range query.names {
		list, err := reader.ListResources(ctx, cluster, kind, name, opts)
		if err != nil {
			return nil, err
		}
		merged.Items = append(merged.Items, list.Items...)
	}
	return merged, nil
}

// getNamedNamespaces loads the allowlisted namespaces by name. Names that no
// longer exist are skipped; a stale entry must not fail the whole list.
func getNamedNamespaces(ctx context.Context, reader steve.ResourceReader, cluster string, names []string, opts *steve.ListOptions) (*unstructured.UnstructuredList, error) {
	list := &unstructured.UnstructuredList{}
	for _, name := range names {
		item, err := reader.GetResource(ctx, cluster, "namespace", "", name)
		if apierrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		matches, err := matchesNamespaceListOptions(item, opts)
		if err != nil {
			return nil, err
		}
		if !matches {
			continue
		}
		list.Items = append(list.Items, *item)
	}
	return list, nil
}

func matchesNamespaceListOptions(item *unstructured.Unstructured, opts *steve.ListOptions) (bool, error) {
	if opts == nil {
		return true, nil
	}
	if opts.LabelSelector != "" {
		selector, err := labels.Parse(opts.LabelSelector)
		if err != nil {
			return false, err
		}
		if !selector.Matches(labels.Set(item.GetLabels())) {
			return false, nil
		}
	}
	if opts.FieldSelector == "" {
		return true, nil
	}
	selector, err := fields.ParseSelector(opts.FieldSelector)
	if err != nil {
		return false, err
	}
	return selector.Matches(unstructuredFields(item.Object, "")), nil
}

func unstructuredFields(value interface{}, prefix string) fields.Set {
	result := fields.Set{}
	var add func(interface{}, string)
	add = func(current interface{}, path string) {
		switch typed := current.(type) {
		case map[string]interface{}:
			for key, nested := range typed {
				nestedPath := key
				if path != "" {
					nestedPath = path + "." + key
				}
				add(nested, nestedPath)
			}
		case string:
			result[path] = typed
		case bool, float64, int64, int, nil:
			result[path] = fmt.Sprint(typed)
		}
	}
	add(value, prefix)
	return result
}

func kubernetesReader(client interface{}) (steve.ResourceReader, error) {
	if reader, ok := client.(steve.ResourceReader); ok && reader != nil {
		return reader, nil
	}
	steveClient, err := toolset.ValidateSteveClient(client)
	if err != nil {
		return nil, err
	}
	return steveClient, nil
}

func kubernetesScopeResolver(client interface{}) (scopeResolver, error) {
	if resolver, ok := client.(scopeResolver); ok && resolver != nil {
		return resolver, nil
	}
	steveClient, err := toolset.ValidateSteveClient(client)
	if err != nil {
		return nil, err
	}
	return steveClient, nil
}

func namespaceFromPatch(patch string) string {
	var ops []struct {
		Path  string `json:"path"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal([]byte(patch), &ops); err != nil {
		return ""
	}
	for _, op := range ops {
		if op.Path == "/metadata/namespace" && op.Value != "" {
			return op.Value
		}
	}
	return ""
}

func sortedNames(set map[string]struct{}) []string {
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
