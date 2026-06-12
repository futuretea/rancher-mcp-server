package dep

import (
	"context"
	"fmt"
	"sync"

	"github.com/futuretea/rancher-mcp-server/pkg/client/steve"

	unstructuredv1 "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

// resourceKindsToList defines the resource kinds to list for graph building.
var resourceKindsToList = []resourceKindSpec{
	// Core (namespaced)
	{kind: "pod", clusterScoped: false},
	{kind: "service", clusterScoped: false},
	{kind: "configmap", clusterScoped: false},
	{kind: "secret", clusterScoped: false},
	{kind: "serviceaccount", clusterScoped: false},
	{kind: "persistentvolumeclaim", clusterScoped: false},
	{kind: "event", clusterScoped: false},
	// Core (cluster-scoped)
	{kind: "node", clusterScoped: true},
	{kind: "persistentvolume", clusterScoped: true},
	// Apps (namespaced)
	{kind: "deployment", clusterScoped: false},
	{kind: "statefulset", clusterScoped: false},
	{kind: "daemonset", clusterScoped: false},
	{kind: "replicaset", clusterScoped: false},
	// Batch (namespaced)
	{kind: "job", clusterScoped: false},
	{kind: "cronjob", clusterScoped: false},
	// Networking
	{kind: "ingress", clusterScoped: false},
	{kind: "ingressclass", clusterScoped: true},
	// Policy (namespaced)
	{kind: "poddisruptionbudget", clusterScoped: false},
	// RBAC
	{kind: "role", clusterScoped: false},
	{kind: "rolebinding", clusterScoped: false},
	{kind: "clusterrole", clusterScoped: true},
	{kind: "clusterrolebinding", clusterScoped: true},
	// Storage (cluster-scoped)
	{kind: "storageclass", clusterScoped: true},
}

type resourceKindSpec struct {
	kind          string
	clusterScoped bool
}

// Result holds the output of a dependency resolution.
type Result struct {
	NodeMap NodeMap
	RootUID types.UID
}

// ResolveOptions controls the scan scope and traversal budget.
type ResolveOptions struct {
	Direction         string
	MaxDepth          int
	ScanNamespace     string
	MaxScannedObjects int
}

// Resolve resolves the dependency/dependent graph for a Kubernetes resource.
// Direction should be "dependents" (default) or "dependencies".
func Resolve(ctx context.Context, client steve.ResourceReader, clusterID, rootKind, rootNS, rootName string, options ResolveOptions) (*Result, error) {
	switch options.Direction {
	case "", "dependents":
		options.Direction = "dependents"
	case "dependencies":
		// valid
	default:
		return nil, fmt.Errorf("invalid direction %q, must be %q or %q", options.Direction, "dependents", "dependencies")
	}

	scanNamespace, err := normalizeScanNamespace(rootNS, options.ScanNamespace)
	if err != nil {
		return nil, err
	}

	root, err := client.GetResource(ctx, clusterID, rootKind, rootNS, rootName)
	if err != nil {
		return nil, fmt.Errorf("failed to get root resource %s/%s: %w", rootKind, rootName, err)
	}

	allObjects, err := listAllResources(ctx, client, clusterID, scanNamespace, options.MaxScannedObjects)
	if err != nil {
		return nil, fmt.Errorf("failed to list resources: %w", err)
	}
	// Include root object to handle edge cases.
	allObjects = append(allObjects, *root)

	globalMapByUID, globalMapByKey := buildNodeMaps(allObjects)

	populateOwnerReferences(globalMapByUID)
	populateSemanticRelationships(globalMapByUID, globalMapByKey)

	nodeMap, err := traverseGraph(root.GetUID(), options.Direction, options.MaxDepth, globalMapByUID)
	if err != nil {
		return nil, err
	}

	return &Result{
		NodeMap: nodeMap,
		RootUID: root.GetUID(),
	}, nil
}

// buildNodeMaps creates UID and object-reference keyed maps from a list of objects.
func buildNodeMaps(objects []unstructuredv1.Unstructured) (map[types.UID]*Node, map[ObjectReferenceKey]*Node) {
	byUID := make(map[types.UID]*Node, len(objects))
	byKey := make(map[ObjectReferenceKey]*Node, len(objects))

	for i := range objects {
		node := newNode(&objects[i])
		if node == nil {
			continue
		}
		if _, exists := byUID[node.UID]; exists {
			continue
		}
		byUID[node.UID] = node
		byKey[node.GetObjectReferenceKey()] = node
	}

	return byUID, byKey
}

// newNode creates a graph Node from an unstructured object.
// It returns nil when the object has no UID.
func newNode(o *unstructuredv1.Unstructured) *Node {
	uid := o.GetUID()
	if uid == "" {
		return nil
	}
	gvk := o.GroupVersionKind()
	return &Node{
		Unstructured: o,
		UID:          uid,
		Kind:         gvk.Kind,
		Namespace:    o.GetNamespace(),
		Name:         o.GetName(),
		Dependencies: map[types.UID]RelationshipSet{},
		Dependents:   map[types.UID]RelationshipSet{},
	}
}

// populateOwnerReferences adds ControllerRef and OwnerRef relationships between nodes.
func populateOwnerReferences(byUID map[types.UID]*Node) {
	for _, node := range byUID {
		for _, ref := range node.GetOwnerReferences() {
			owner, ok := byUID[ref.UID]
			if !ok {
				continue
			}
			if ref.Controller != nil && *ref.Controller {
				node.AddDependency(owner.UID, RelationshipControllerRef)
				owner.AddDependent(node.UID, RelationshipControllerRef)
			}
			node.AddDependency(owner.UID, RelationshipOwnerRef)
			owner.AddDependent(node.UID, RelationshipOwnerRef)
		}
	}
}

// populateSemanticRelationships runs the kind-specific extractors on every node.
func populateSemanticRelationships(byUID map[types.UID]*Node, byKey map[ObjectReferenceKey]*Node) {
	for _, node := range byUID {
		rmap := extractRelationships(node, byUID)
		if rmap == nil {
			continue
		}
		applyRelationships(node, rmap, byUID, byKey)
	}
}

// traverseGraph performs a breadth-first traversal starting from rootUID and
// returns the visited NodeMap. It honors maxDepth when positive.
func traverseGraph(rootUID types.UID, direction string, maxDepth int, globalMapByUID map[types.UID]*Node) (NodeMap, error) {
	rootNode := globalMapByUID[rootUID]
	if rootNode == nil {
		return nil, fmt.Errorf("root resource not found in graph")
	}

	nodeMap := NodeMap{rootUID: rootNode}
	rootNode.Depth = 0

	depsIsDependencies := direction == "dependencies"
	uidQueue := []types.UID{rootUID, ""} // sentinel marks depth boundaries
	visited := map[types.UID]struct{}{}
	var depth uint

	for len(uidQueue) > 1 {
		uid := uidQueue[0]
		uidQueue = uidQueue[1:]

		if uid == "" {
			depth++
			if maxDepth > 0 && depth >= uint(maxDepth) {
				break
			}
			uidQueue = append(uidQueue, "") // next depth sentinel
			continue
		}

		if _, ok := visited[uid]; ok {
			continue
		}
		visited[uid] = struct{}{}

		node := nodeMap[uid]
		if node == nil {
			continue
		}

		// Allow nodes to keep the smallest depth.
		if node.Depth == 0 || depth < node.Depth {
			node.Depth = depth
		}

		for depUID := range node.GetDeps(depsIsDependencies) {
			depNode := globalMapByUID[depUID]
			if depNode == nil {
				continue
			}
			if _, exists := nodeMap[depUID]; !exists {
				depNode.Depth = depth + 1
				nodeMap[depUID] = depNode
			}
			uidQueue = append(uidQueue, depUID)
		}
	}

	return nodeMap, nil
}

// applyRelationships applies the extracted relationship map to the node and global maps.
func applyRelationships(node *Node, rmap *RelationshipMap, globalMapByUID map[types.UID]*Node, globalMapByKey map[ObjectReferenceKey]*Node) {
	// Dependencies by ref
	for k, rset := range rmap.DependenciesByRef {
		if n, ok := globalMapByKey[k]; ok {
			for r := range rset {
				node.AddDependency(n.UID, r)
				n.AddDependent(node.UID, r)
			}
		}
	}
	// Dependents by ref
	for k, rset := range rmap.DependentsByRef {
		if n, ok := globalMapByKey[k]; ok {
			for r := range rset {
				n.AddDependency(node.UID, r)
				node.AddDependent(n.UID, r)
			}
		}
	}
	// Dependencies by UID
	for uid, rset := range rmap.DependenciesByUID {
		if n, ok := globalMapByUID[uid]; ok {
			for r := range rset {
				node.AddDependency(n.UID, r)
				n.AddDependent(node.UID, r)
			}
		}
	}
	// Dependents by UID (not currently used but included for completeness)
	for uid, rset := range rmap.DependentsByUID {
		if n, ok := globalMapByUID[uid]; ok {
			for r := range rset {
				n.AddDependency(node.UID, r)
				node.AddDependent(n.UID, r)
			}
		}
	}
}

func normalizeScanNamespace(rootNamespace, scanNamespace string) (string, error) {
	if rootNamespace == "" {
		return scanNamespace, nil
	}
	if scanNamespace == "" || scanNamespace == rootNamespace {
		return rootNamespace, nil
	}
	return "", fmt.Errorf("scan namespace %q does not match namespaced root namespace %q", scanNamespace, rootNamespace)
}

// listAllResources lists all relevant resource types, optionally under a budget.
func listAllResources(ctx context.Context, client steve.ResourceReader, clusterID, namespace string, maxScannedObjects int) ([]unstructuredv1.Unstructured, error) {
	if maxScannedObjects > 0 {
		return listAllResourcesWithBudget(ctx, client, clusterID, namespace, maxScannedObjects)
	}
	return listAllResourcesConcurrently(ctx, client, clusterID, namespace)
}

// listAllResourcesConcurrently lists all relevant resource types concurrently.
func listAllResourcesConcurrently(ctx context.Context, client steve.ResourceReader, clusterID, namespace string) ([]unstructuredv1.Unstructured, error) {
	var (
		mu       sync.Mutex
		wg       sync.WaitGroup
		allItems []unstructuredv1.Unstructured
	)

	for _, spec := range resourceKindsToList {
		wg.Add(1)
		go func(s resourceKindSpec) {
			defer wg.Done()

			ns := namespace
			if s.clusterScoped {
				ns = ""
			}

			list, err := client.ListResources(ctx, clusterID, s.kind, ns, nil)
			if err != nil {
				// Non-fatal: some resource types may not exist on the cluster
				return
			}

			mu.Lock()
			allItems = append(allItems, list.Items...)
			mu.Unlock()
		}(spec)
	}

	wg.Wait()
	return allItems, nil
}

// listAllResourcesWithBudget lists resource kinds serially and fails fast when
// the total scanned object count would exceed the configured budget.
func listAllResourcesWithBudget(ctx context.Context, client steve.ResourceReader, clusterID, namespace string, maxScannedObjects int) ([]unstructuredv1.Unstructured, error) {
	allItems := make([]unstructuredv1.Unstructured, 0, maxScannedObjects)
	remaining := maxScannedObjects
	scannedKinds := 0

	for _, spec := range resourceKindsToList {
		ns := namespace
		if spec.clusterScoped {
			ns = ""
		}

		limit := 1
		if remaining > 0 {
			limit = remaining + 1
		}

		list, err := client.ListResources(ctx, clusterID, spec.kind, ns, &steve.ListOptions{Limit: int64(limit)})
		if err != nil {
			// Non-fatal: some resource types may not exist on the cluster
			continue
		}

		scannedKinds++
		if list.GetContinue() != "" || len(list.Items) > remaining {
			return nil, fmt.Errorf(
				"dependency scan budget exceeded: scannedObjects=%d scannedKinds=%d budget=%d scanNamespace=%q blockedKind=%s",
				len(allItems),
				scannedKinds,
				maxScannedObjects,
				namespace,
				spec.kind,
			)
		}

		allItems = append(allItems, list.Items...)
		remaining -= len(list.Items)
	}

	return allItems, nil
}
