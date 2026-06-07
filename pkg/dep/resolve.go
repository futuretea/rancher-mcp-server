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
	scanNamespace, err := normalizeScanNamespace(rootNS, options.ScanNamespace)
	if err != nil {
		return nil, err
	}

	// 1. Get the root resource
	root, err := client.GetResource(ctx, clusterID, rootKind, rootNS, rootName)
	if err != nil {
		return nil, fmt.Errorf("failed to get root resource %s/%s: %w", rootKind, rootName, err)
	}

	// 2. List all relevant resources within the requested scope and budget.
	allObjects, err := listAllResources(ctx, client, clusterID, scanNamespace, options.MaxScannedObjects)
	if err != nil {
		return nil, fmt.Errorf("failed to list resources: %w", err)
	}

	// Include root object to handle edge cases
	allObjects = append(allObjects, *root)

	// 3. Build the global node maps
	globalMapByUID := map[types.UID]*Node{}
	globalMapByKey := map[ObjectReferenceKey]*Node{}

	for ix := range allObjects {
		o := &allObjects[ix]
		uid := o.GetUID()
		if uid == "" {
			continue
		}
		gvk := o.GroupVersionKind()
		node := &Node{
			Unstructured: o,
			UID:          uid,
			Kind:         gvk.Kind,
			Namespace:    o.GetNamespace(),
			Name:         o.GetName(),
			Dependencies: map[types.UID]RelationshipSet{},
			Dependents:   map[types.UID]RelationshipSet{},
		}
		if _, ok := globalMapByUID[uid]; ok {
			continue
		}
		globalMapByUID[uid] = node
		globalMapByKey[node.GetObjectReferenceKey()] = node
	}

	// 4. Populate OwnerReference relationships
	for _, node := range globalMapByUID {
		for _, ref := range node.GetOwnerReferences() {
			if owner, ok := globalMapByUID[ref.UID]; ok {
				if ref.Controller != nil && *ref.Controller {
					node.AddDependency(owner.UID, RelationshipControllerRef)
					owner.AddDependent(node.UID, RelationshipControllerRef)
				}
				node.AddDependency(owner.UID, RelationshipOwnerRef)
				owner.AddDependent(node.UID, RelationshipOwnerRef)
			}
		}
	}

	// 5. Populate semantic relationships per resource type
	for _, node := range globalMapByUID {
		rmap := extractRelationships(node, globalMapByUID)
		if rmap == nil {
			continue
		}
		applyRelationships(node, rmap, globalMapByUID, globalMapByKey)
	}

	// 6. BFS traversal from root
	depsIsDependencies := options.Direction == "dependencies"
	rootUID := root.GetUID()

	nodeMap := NodeMap{}
	uidQueue := []types.UID{}
	visited := map[types.UID]struct{}{}

	if rootNode, ok := globalMapByUID[rootUID]; ok {
		nodeMap[rootUID] = rootNode
		rootNode.Depth = 0
		uidQueue = append(uidQueue, rootUID)
	} else {
		return nil, fmt.Errorf("root resource not found in graph")
	}

	// BFS with depth tracking using sentinel
	uidQueue = append(uidQueue, "") // sentinel for depth boundary
	var depth uint

	for len(uidQueue) > 1 {
		uid := uidQueue[0]
		uidQueue = uidQueue[1:]

		if uid == "" {
			depth++
			if options.MaxDepth > 0 && depth >= uint(options.MaxDepth) {
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

		// Allow nodes to keep the smallest depth
		if node.Depth == 0 || depth < node.Depth {
			node.Depth = depth
		}

		deps := node.GetDeps(depsIsDependencies)
		for depUID := range deps {
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

	return &Result{
		NodeMap: nodeMap,
		RootUID: rootUID,
	}, nil
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
