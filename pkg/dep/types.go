// Package dep provides lightweight Kubernetes resource dependency/dependent
// graph resolution, inspired by kube-lineage.
package dep

import (
	"fmt"
	"sort"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

// Relationship represents a relationship type between two Kubernetes objects.
type Relationship string

// RelationshipSet contains a set of relationships.
type RelationshipSet map[Relationship]struct{}

// List returns the contents as a sorted string slice.
func (s RelationshipSet) List() []string {
	res := make([]string, 0, len(s))
	for key := range s {
		res = append(res, string(key))
	}
	sort.Strings(res)
	return res
}

// Relationship constants for supported relationship types.
const (
	// Owner-Dependent relationships.
	RelationshipControllerRef Relationship = "ControllerReference"
	RelationshipOwnerRef      Relationship = "OwnerReference"

	// Pod relationships.
	RelationshipPodNode            Relationship = "PodNode"
	RelationshipPodServiceAccount  Relationship = "PodServiceAccount"
	RelationshipPodVolume          Relationship = "PodVolume"
	RelationshipPodContainerEnv    Relationship = "PodContainerEnvironment"
	RelationshipPodImagePullSecret Relationship = "PodImagePullSecret"

	// Service relationships.
	RelationshipService Relationship = "Service"

	// Ingress & IngressClass relationships.
	RelationshipIngressClass           Relationship = "IngressClass"
	RelationshipIngressClassParameters Relationship = "IngressClassParameters"
	RelationshipIngressService         Relationship = "IngressService"
	RelationshipIngressTLSSecret       Relationship = "IngressTLSSecret"

	// PersistentVolume & PersistentVolumeClaim relationships.
	RelationshipPersistentVolumeClaim        Relationship = "PersistentVolumeClaim"
	RelationshipPersistentVolumeStorageClass Relationship = "PersistentVolumeStorageClass"

	// RBAC relationships.
	RelationshipRoleBindingSubject        Relationship = "RoleBindingSubject"
	RelationshipRoleBindingRole           Relationship = "RoleBindingRole"
	RelationshipClusterRoleBindingSubject Relationship = "ClusterRoleBindingSubject"
	RelationshipClusterRoleBindingRole    Relationship = "ClusterRoleBindingRole"

	// PodDisruptionBudget relationships.
	RelationshipPodDisruptionBudget Relationship = "PodDisruptionBudget"

	// Event relationships.
	RelationshipEventRegarding Relationship = "EventRegarding"
)

// ObjectReferenceKey is a compact string representation of an ObjectReference.
type ObjectReferenceKey string

// ObjectReference is a reference to a Kubernetes object.
type ObjectReference struct {
	Group     string
	Kind      string
	Namespace string
	Name      string
}

// Key converts the ObjectReference into an ObjectReferenceKey.
func (o *ObjectReference) Key() ObjectReferenceKey {
	return ObjectReferenceKey(fmt.Sprintf("%s\\%s\\%s\\%s", o.Group, o.Kind, o.Namespace, o.Name))
}

// Node represents a Kubernetes object in a relationship tree.
type Node struct {
	*unstructured.Unstructured
	UID          types.UID
	Kind         string
	Namespace    string
	Name         string
	Dependencies map[types.UID]RelationshipSet
	Dependents   map[types.UID]RelationshipSet
	Depth        uint
}

// AddDependency adds a dependency relationship to this node.
func (n *Node) AddDependency(uid types.UID, r Relationship) {
	if _, ok := n.Dependencies[uid]; !ok {
		n.Dependencies[uid] = RelationshipSet{}
	}
	n.Dependencies[uid][r] = struct{}{}
}

// AddDependent adds a dependent relationship to this node.
func (n *Node) AddDependent(uid types.UID, r Relationship) {
	if _, ok := n.Dependents[uid]; !ok {
		n.Dependents[uid] = RelationshipSet{}
	}
	n.Dependents[uid][r] = struct{}{}
}

// GetDeps returns either Dependencies or Dependents based on direction.
func (n *Node) GetDeps(depsIsDependencies bool) map[types.UID]RelationshipSet {
	if depsIsDependencies {
		return n.Dependencies
	}
	return n.Dependents
}

// GetObjectReferenceKey returns the ObjectReferenceKey for this node.
func (n *Node) GetObjectReferenceKey() ObjectReferenceKey {
	gvk := n.GroupVersionKind()
	ref := ObjectReference{
		Group:     gvk.Group,
		Kind:      gvk.Kind,
		Name:      n.Name,
		Namespace: n.Namespace,
	}
	return ref.Key()
}

// NodeMap contains a relationship tree stored as a map of nodes.
type NodeMap map[types.UID]*Node

// NodeList contains a list of nodes for sorting.
type NodeList []*Node

func (n NodeList) Len() int { return len(n) }
func (n NodeList) Less(i, j int) bool {
	a, b := n[i], n[j]
	if a.Namespace != b.Namespace {
		return a.Namespace < b.Namespace
	}
	if a.Kind != b.Kind {
		return a.Kind < b.Kind
	}
	return a.Name < b.Name
}
func (n NodeList) Swap(i, j int) { n[i], n[j] = n[j], n[i] }

// RelationshipMap contains relationships a Kubernetes object has with others.
type RelationshipMap struct {
	DependenciesByRef map[ObjectReferenceKey]RelationshipSet
	DependenciesByUID map[types.UID]RelationshipSet
	DependentsByRef   map[ObjectReferenceKey]RelationshipSet
	DependentsByUID   map[types.UID]RelationshipSet
}

// NewRelationshipMap creates a new empty RelationshipMap.
func NewRelationshipMap() *RelationshipMap {
	return &RelationshipMap{
		DependenciesByRef: map[ObjectReferenceKey]RelationshipSet{},
		DependenciesByUID: map[types.UID]RelationshipSet{},
		DependentsByRef:   map[ObjectReferenceKey]RelationshipSet{},
		DependentsByUID:   map[types.UID]RelationshipSet{},
	}
}

// AddDependencyByKey adds a dependency by object reference key.
func (m *RelationshipMap) AddDependencyByKey(k ObjectReferenceKey, r Relationship) {
	if _, ok := m.DependenciesByRef[k]; !ok {
		m.DependenciesByRef[k] = RelationshipSet{}
	}
	m.DependenciesByRef[k][r] = struct{}{}
}

// AddDependencyByUID adds a dependency by UID.
func (m *RelationshipMap) AddDependencyByUID(uid types.UID, r Relationship) {
	if _, ok := m.DependenciesByUID[uid]; !ok {
		m.DependenciesByUID[uid] = RelationshipSet{}
	}
	m.DependenciesByUID[uid][r] = struct{}{}
}

// AddDependentByKey adds a dependent by object reference key.
func (m *RelationshipMap) AddDependentByKey(k ObjectReferenceKey, r Relationship) {
	if _, ok := m.DependentsByRef[k]; !ok {
		m.DependentsByRef[k] = RelationshipSet{}
	}
	m.DependentsByRef[k][r] = struct{}{}
}
