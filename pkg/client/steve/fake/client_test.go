package fake

import (
	"context"
	"testing"

	"github.com/futuretea/rancher-mcp-server/pkg/client/steve"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func makeResource(kind, name, namespace string, labels map[string]string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetUnstructuredContent(map[string]interface{}{})
	u.SetKind(kind)
	u.SetName(name)
	u.SetNamespace(namespace)
	u.SetLabels(labels)
	return u
}

func TestClient_ImplementsResourceReader(t *testing.T) {
	// Compile-time check already done above, but verify at runtime
	var c interface{} = NewClient()
	_, ok := c.(steve.ResourceReader)
	if !ok {
		t.Fatal("fake.Client must implement steve.ResourceReader")
	}
}

func TestClient_GetResource_Found(t *testing.T) {
	c := NewClient()
	c.AddResource(makeResource("Node", "node-1", "", nil))

	obj, err := c.GetResource(context.Background(), "cluster-1", "node", "", "node-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obj.GetName() != "node-1" {
		t.Errorf("expected node-1, got %s", obj.GetName())
	}
}

func TestClient_GetResource_NotFound(t *testing.T) {
	c := NewClient()
	_, err := c.GetResource(context.Background(), "cluster-1", "node", "", "no-such-node")
	if err == nil {
		t.Fatal("expected error for missing resource")
	}
}

func TestClient_GetResource_WrongKind(t *testing.T) {
	c := NewClient()
	c.AddResource(makeResource("Pod", "my-pod", "default", nil))

	_, err := c.GetResource(context.Background(), "cluster-1", "node", "", "my-pod")
	if err == nil {
		t.Fatal("expected error when querying wrong kind")
	}
}

func TestClient_GetResource_NamespaceFilter(t *testing.T) {
	c := NewClient()
	c.AddResource(makeResource("Pod", "pod-a", "ns-a", nil))
	c.AddResource(makeResource("Pod", "pod-b", "ns-b", nil))

	obj, err := c.GetResource(context.Background(), "cluster-1", "pod", "ns-a", "pod-a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obj.GetName() != "pod-a" {
		t.Errorf("expected pod-a, got %s", obj.GetName())
	}

	// Same name, wrong namespace
	_, err = c.GetResource(context.Background(), "cluster-1", "pod", "ns-b", "pod-a")
	if err == nil {
		t.Fatal("expected error when name matches but namespace differs")
	}
}

func TestClient_ListResources_AllInKind(t *testing.T) {
	c := NewClient()
	c.AddResource(makeResource("Pod", "pod-1", "default", nil))
	c.AddResource(makeResource("Pod", "pod-2", "default", nil))
	c.AddResource(makeResource("Node", "node-1", "", nil))

	list, err := c.ListResources(context.Background(), "cluster-1", "pod", "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list.Items) != 2 {
		t.Fatalf("expected 2 pods, got %d", len(list.Items))
	}
}

func TestClient_ListResources_NamespaceFilter(t *testing.T) {
	c := NewClient()
	c.AddResource(makeResource("Pod", "pod-a", "ns-a", nil))
	c.AddResource(makeResource("Pod", "pod-b", "ns-b", nil))

	list, err := c.ListResources(context.Background(), "cluster-1", "pod", "ns-a", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("expected 1 pod in ns-a, got %d", len(list.Items))
	}
	if list.Items[0].GetName() != "pod-a" {
		t.Errorf("expected pod-a, got %s", list.Items[0].GetName())
	}
}

func TestClient_ListResources_LabelSelector(t *testing.T) {
	c := NewClient()
	c.AddResource(makeResource("Pod", "pod-a", "default", map[string]string{"app": "nginx", "env": "prod"}))
	c.AddResource(makeResource("Pod", "pod-b", "default", map[string]string{"app": "redis", "env": "prod"}))
	c.AddResource(makeResource("Pod", "pod-c", "default", nil))

	list, err := c.ListResources(context.Background(), "cluster-1", "pod", "", &steve.ListOptions{LabelSelector: "app=nginx"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("expected 1 pod with app=nginx, got %d", len(list.Items))
	}
	if list.Items[0].GetName() != "pod-a" {
		t.Errorf("expected pod-a, got %s", list.Items[0].GetName())
	}
}

func TestClient_ListResources_MultipleLabelSelectors(t *testing.T) {
	c := NewClient()
	c.AddResource(makeResource("Pod", "p1", "default", map[string]string{"app": "nginx", "env": "prod"}))
	c.AddResource(makeResource("Pod", "p2", "default", map[string]string{"app": "nginx", "env": "dev"}))

	list, err := c.ListResources(context.Background(), "cluster-1", "pod", "", &steve.ListOptions{LabelSelector: "app=nginx,env=prod"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("expected 1 pod matching both labels, got %d", len(list.Items))
	}
}

func TestClient_ListResources_EmptyKind(t *testing.T) {
	c := NewClient()
	list, err := c.ListResources(context.Background(), "cluster-1", "nonexistent", "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list.Items) != 0 {
		t.Fatalf("expected empty list for unknown kind, got %d", len(list.Items))
	}
}

func TestClient_ListResources_InvalidLabelSelector(t *testing.T) {
	c := NewClient()
	_, err := c.ListResources(context.Background(), "cluster-1", "pod", "", &steve.ListOptions{LabelSelector: "!!invalid!!"})
	if err == nil {
		t.Fatal("expected error for invalid label selector")
	}
}

func TestClient_GetEvents_All(t *testing.T) {
	c := NewClient()
	c.AddEvent(corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Namespace: "default"},
		Reason:         "Failed",
		InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "pod-1", Namespace: "default"},
	})
	c.AddEvent(corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Namespace: "kube-system"},
		Reason:         "OOMKill",
		InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "pod-2", Namespace: "kube-system"},
	})

	events, err := c.GetEvents(context.Background(), "cluster-1", "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
}

func TestClient_GetEvents_NamespaceFilter(t *testing.T) {
	c := NewClient()
	c.AddEvent(corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Namespace: "default"},
		Reason:         "Failed",
		InvolvedObject: corev1.ObjectReference{Kind: "Pod"},
	})
	c.AddEvent(corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Namespace: "kube-system"},
		Reason:         "OOMKill",
		InvolvedObject: corev1.ObjectReference{Kind: "Pod"},
	})

	events, err := c.GetEvents(context.Background(), "cluster-1", "default", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event in default ns, got %d", len(events))
	}
}

func TestClient_GetEvents_KindFilter(t *testing.T) {
	c := NewClient()
	c.AddEvent(corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Namespace: "default"},
		InvolvedObject: corev1.ObjectReference{Kind: "Pod"},
	})
	c.AddEvent(corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Namespace: "default"},
		InvolvedObject: corev1.ObjectReference{Kind: "Node"},
	})

	events, err := c.GetEvents(context.Background(), "cluster-1", "", "", "Node")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 Node event, got %d", len(events))
	}
}

func TestClient_KindCaseInsensitive(t *testing.T) {
	c := NewClient()
	c.AddResource(makeResource("Node", "node-1", "", nil))

	// Query with lowercase
	_, err := c.GetResource(context.Background(), "cluster-1", "node", "", "node-1")
	if err != nil {
		t.Fatalf("lowercase kind query failed: %v", err)
	}

	// Query with mixed case
	_, err = c.GetResource(context.Background(), "cluster-1", "Node", "", "node-1")
	if err != nil {
		t.Fatalf("mixed-case kind query failed: %v", err)
	}
}
