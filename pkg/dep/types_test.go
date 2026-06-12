package dep

import (
	"sort"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func TestRelationshipSet_List(t *testing.T) {
	t.Run("empty set", func(t *testing.T) {
		s := RelationshipSet{}
		result := s.List()
		if len(result) != 0 {
			t.Fatalf("expected empty list, got %v", result)
		}
	})

	t.Run("sorted output", func(t *testing.T) {
		s := RelationshipSet{
			RelationshipService:        {},
			RelationshipPodNode:        {},
			RelationshipEventRegarding: {},
		}
		result := s.List()
		if len(result) != 3 {
			t.Fatalf("expected 3 items, got %d: %v", len(result), result)
		}
		// Must be sorted alphabetically
		if result[0] != string(RelationshipEventRegarding) ||
			result[1] != string(RelationshipPodNode) ||
			result[2] != string(RelationshipService) {
			t.Fatalf("expected sorted output, got %v", result)
		}
	})
}

func TestObjectReference_Key(t *testing.T) {
	tests := []struct {
		name string
		ref  ObjectReference
		want ObjectReferenceKey
	}{
		{
			name: "full reference",
			ref:  ObjectReference{Group: "apps", Kind: "Deployment", Namespace: "default", Name: "nginx"},
			want: "apps\\Deployment\\default\\nginx",
		},
		{
			name: "no group",
			ref:  ObjectReference{Group: "", Kind: "Pod", Namespace: "kube-system", Name: "coredns-abc"},
			want: "\\Pod\\kube-system\\coredns-abc",
		},
		{
			name: "no namespace",
			ref:  ObjectReference{Group: "", Kind: "Node", Namespace: "", Name: "node-1"},
			want: "\\Node\\\\node-1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ref.Key()
			if got != tt.want {
				t.Fatalf("Key() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNode_AddDependency(t *testing.T) {
	node := &Node{
		Dependencies: map[types.UID]RelationshipSet{},
	}
	uid := types.UID("abc-123")

	node.AddDependency(uid, RelationshipPodNode)
	if deps, ok := node.Dependencies[uid]; !ok {
		t.Fatal("expected dependency to exist")
	} else if _, ok := deps[RelationshipPodNode]; !ok {
		t.Fatal("expected PodNode relationship")
	}

	// Add second relationship to same UID
	node.AddDependency(uid, RelationshipService)
	if len(node.Dependencies[uid]) != 2 {
		t.Fatalf("expected 2 relationships, got %d", len(node.Dependencies[uid]))
	}
}

func TestNode_AddDependency_NilSafe(t *testing.T) {
	var node *Node
	node.AddDependency(types.UID("abc-123"), RelationshipPodNode) // must not panic
}

func TestNode_AddDependent(t *testing.T) {
	node := &Node{
		Dependents: map[types.UID]RelationshipSet{},
	}
	uid := types.UID("xyz-789")

	node.AddDependent(uid, RelationshipPodNode)
	if deps, ok := node.Dependents[uid]; !ok {
		t.Fatal("expected dependent to exist")
	} else if _, ok := deps[RelationshipPodNode]; !ok {
		t.Fatal("expected PodNode relationship")
	}
}

func TestNode_AddDependent_NilSafe(t *testing.T) {
	var node *Node
	node.AddDependent(types.UID("xyz-789"), RelationshipPodNode) // must not panic
}

func TestNode_GetDeps(t *testing.T) {
	node := &Node{
		Dependencies: map[types.UID]RelationshipSet{
			"dep-1": {RelationshipPodNode: {}},
		},
		Dependents: map[types.UID]RelationshipSet{
			"dep-2": {RelationshipService: {}},
		},
	}

	t.Run("dependencies direction", func(t *testing.T) {
		deps := node.GetDeps(true)
		if _, ok := deps["dep-1"]; !ok {
			t.Fatal("expected dep-1 in dependencies")
		}
		if _, ok := deps["dep-2"]; ok {
			t.Fatal("expected dep-2 NOT in dependencies")
		}
	})

	t.Run("dependents direction", func(t *testing.T) {
		deps := node.GetDeps(false)
		if _, ok := deps["dep-2"]; !ok {
			t.Fatal("expected dep-2 in dependents")
		}
		if _, ok := deps["dep-1"]; ok {
			t.Fatal("expected dep-1 NOT in dependents")
		}
	})
}

func TestNode_GetObjectReferenceKey(t *testing.T) {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    "Deployment",
	})
	u.SetName("nginx")
	u.SetNamespace("default")

	node := &Node{Unstructured: u, Name: "nginx", Namespace: "default"}
	key := node.GetObjectReferenceKey()
	want := ObjectReferenceKey("apps\\Deployment\\default\\nginx")
	if key != want {
		t.Fatalf("GetObjectReferenceKey() = %q, want %q", key, want)
	}
}

func TestNodeList_Sort(t *testing.T) {
	nodes := NodeList{
		{Namespace: "z-ns", Kind: "Pod", Name: "a"},
		{Namespace: "a-ns", Kind: "Pod", Name: "z"},
		{Namespace: "a-ns", Kind: "Deployment", Name: "b"},
		{Namespace: "a-ns", Kind: "Pod", Name: "a"},
	}
	sort.Sort(nodes)

	// Expected order: ns first, then kind, then name
	expected := []string{
		"a-ns/Deployment/b",
		"a-ns/Pod/a",
		"a-ns/Pod/z",
		"z-ns/Pod/a",
	}
	for i, n := range nodes {
		got := n.Namespace + "/" + n.Kind + "/" + n.Name
		if got != expected[i] {
			t.Fatalf("position %d: got %q, want %q", i, got, expected[i])
		}
	}
}

func TestNewRelationshipMap(t *testing.T) {
	rm := NewRelationshipMap()
	if rm == nil {
		t.Fatal("expected non-nil RelationshipMap")
	}
	if rm.DependenciesByRef == nil {
		t.Fatal("DependenciesByRef not initialized")
	}
	if rm.DependenciesByUID == nil {
		t.Fatal("DependenciesByUID not initialized")
	}
	if rm.DependentsByRef == nil {
		t.Fatal("DependentsByRef not initialized")
	}
	if rm.DependentsByUID == nil {
		t.Fatal("DependentsByUID not initialized")
	}
}

func TestRelationshipMap_AddDependencyByKey(t *testing.T) {
	rm := NewRelationshipMap()
	key := ObjectReferenceKey("apps\\Deployment\\default\\nginx")

	rm.AddDependencyByKey(key, RelationshipControllerRef)
	if deps, ok := rm.DependenciesByRef[key]; !ok {
		t.Fatal("expected dependency by key to exist")
	} else if _, ok := deps[RelationshipControllerRef]; !ok {
		t.Fatal("expected ControllerRef relationship")
	}

	// Add second relationship
	rm.AddDependencyByKey(key, RelationshipOwnerRef)
	if len(rm.DependenciesByRef[key]) != 2 {
		t.Fatalf("expected 2 relationships, got %d", len(rm.DependenciesByRef[key]))
	}
}

func TestRelationshipMap_AddDependencyByUID(t *testing.T) {
	rm := NewRelationshipMap()
	uid := types.UID("uid-456")

	rm.AddDependencyByUID(uid, RelationshipPodVolume)
	if deps, ok := rm.DependenciesByUID[uid]; !ok {
		t.Fatal("expected dependency by UID to exist")
	} else if _, ok := deps[RelationshipPodVolume]; !ok {
		t.Fatal("expected PodVolume relationship")
	}
}

func TestRelationshipMap_AddDependentByKey(t *testing.T) {
	rm := NewRelationshipMap()
	key := ObjectReferenceKey("\\ServiceAccount\\default\\sa-name")

	rm.AddDependentByKey(key, RelationshipRoleBindingSubject)
	if deps, ok := rm.DependentsByRef[key]; !ok {
		t.Fatal("expected dependent by key to exist")
	} else if _, ok := deps[RelationshipRoleBindingSubject]; !ok {
		t.Fatal("expected RoleBindingSubject relationship")
	}
}
