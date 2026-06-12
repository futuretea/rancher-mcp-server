package steve

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
)

func newUnstructured(apiVersion, kind, namespace, name string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetAPIVersion(apiVersion)
	u.SetKind(kind)
	u.SetNamespace(namespace)
	u.SetName(name)
	return u
}

func TestFindReplicaSetParent_OnlyDeploymentIsParent(t *testing.T) {
	ctx := context.Background()
	client := NewClient("https://example.com", "token", "", "", false)

	deployment := newUnstructured("apps/v1", "Deployment", "default", "my-deploy")
	replicaSet := newUnstructured("apps/v1", "ReplicaSet", "default", "my-rs")
	replicaSet.SetOwnerReferences([]metav1.OwnerReference{
		{APIVersion: "apps/v1", Kind: "Deployment", Name: "my-deploy"},
		{APIVersion: "apps/v1", Kind: "StatefulSet", Name: "my-sts"},
	})
	client.dynamicClients["cluster"] = fake.NewSimpleDynamicClient(scheme.Scheme, deployment, replicaSet)

	parent := client.findReplicaSetParent(ctx, "cluster", "default", "my-rs")
	if parent == nil {
		t.Fatal("expected Deployment parent")
	}
	if parent.GetName() != "my-deploy" || parent.GetKind() != "Deployment" {
		t.Fatalf("expected Deployment/my-deploy, got %s/%s", parent.GetKind(), parent.GetName())
	}

	// Verify StatefulSet owner is not treated as a ReplicaSet parent.
	statefulSet := newUnstructured("apps/v1", "StatefulSet", "default", "my-sts")
	replicaSetOwnedBySts := newUnstructured("apps/v1", "ReplicaSet", "default", "rs-sts")
	replicaSetOwnedBySts.SetOwnerReferences([]metav1.OwnerReference{
		{APIVersion: "apps/v1", Kind: "StatefulSet", Name: "my-sts"},
	})
	client.dynamicClients["cluster"] = fake.NewSimpleDynamicClient(scheme.Scheme, statefulSet, replicaSetOwnedBySts)

	parent = client.findReplicaSetParent(ctx, "cluster", "default", "rs-sts")
	if parent != nil {
		t.Fatalf("expected nil parent for StatefulSet owner, got %s/%s", parent.GetKind(), parent.GetName())
	}
}

func TestFindPodParent_ReplicaSetOwnedByDeployment(t *testing.T) {
	ctx := context.Background()
	client := NewClient("https://example.com", "token", "", "", false)

	deployment := newUnstructured("apps/v1", "Deployment", "default", "my-deploy")
	replicaSet := newUnstructured("apps/v1", "ReplicaSet", "default", "my-rs")
	replicaSet.SetOwnerReferences([]metav1.OwnerReference{
		{APIVersion: "apps/v1", Kind: "Deployment", Name: "my-deploy"},
	})
	pod := newUnstructured("v1", "Pod", "default", "my-pod")
	pod.SetOwnerReferences([]metav1.OwnerReference{
		{APIVersion: "apps/v1", Kind: "ReplicaSet", Name: "my-rs"},
	})
	client.dynamicClients["cluster"] = fake.NewSimpleDynamicClient(scheme.Scheme, deployment, replicaSet, pod)

	parent := client.findPodParent(ctx, "cluster", "default", pod)
	if parent == nil {
		t.Fatal("expected parent workload")
	}
	if parent.GetKind() != "Deployment" || parent.GetName() != "my-deploy" {
		t.Fatalf("expected Deployment/my-deploy, got %s/%s", parent.GetKind(), parent.GetName())
	}
}
