package steve

import (
	"context"
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	k8stesting "k8s.io/client-go/testing"
)

func TestListResourcesForType_PointersAreDistinct(t *testing.T) {
	ctx := context.Background()
	client := NewClient("https://example.com", "token", "", "", false)

	cm1 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "cm1", Namespace: "default"},
	}
	cm2 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "cm2", Namespace: "default"},
	}
	client.dynamicClients["cluster"] = fake.NewSimpleDynamicClient(scheme.Scheme, cm1, cm2)

	ar := APIResourceInfo{
		Name:    "configmaps",
		Kind:    "ConfigMap",
		Group:   "",
		Version: "v1",
		Verbs:   []string{"list"},
	}
	items, err := client.listResourcesForType(ctx, "cluster", ar, "default", 0)
	if err != nil {
		t.Fatalf("listResourcesForType() returned unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	seen := make(map[string]bool)
	for _, it := range items {
		if it.Resource == nil {
			t.Fatalf("item %q has nil Resource pointer", it.Name)
		}
		if it.Resource.GetName() != it.Name {
			t.Fatalf("Resource name %q does not match item name %q", it.Resource.GetName(), it.Name)
		}
		if seen[it.Name] {
			t.Fatalf("duplicate pointer or name %q", it.Name)
		}
		seen[it.Name] = true
	}

	if items[0].Resource == items[1].Resource {
		t.Fatal("Resource pointers should point to distinct objects")
	}
}

func TestGetAllResources_ReturnsContextCancellation(t *testing.T) {
	client := NewClient("https://example.com", "token", "", "", false)

	dynamicClient := fake.NewSimpleDynamicClient(scheme.Scheme)
	client.dynamicClients["cluster"] = dynamicClient

	clientset := k8sfake.NewSimpleClientset()
	clientset.Discovery().(*fakediscovery.FakeDiscovery).Resources = []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "configmaps", Kind: "ConfigMap", Namespaced: true, Verbs: []string{"list"}},
			},
		},
	}
	client.clientsets["cluster"] = clientset

	ctx, cancel := context.WithCancel(context.Background())
	dynamicClient.PrependReactor("list", "configmaps", func(k8stesting.Action) (bool, runtime.Object, error) {
		cancel()
		return true, nil, context.Canceled
	})
	defer cancel()

	_, err := client.GetAllResources(ctx, "cluster", nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("GetAllResources() error = %v, want context.Canceled", err)
	}
}
