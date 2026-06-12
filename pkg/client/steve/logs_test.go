package steve

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	dynfake "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
)

func TestGetAllContainerLogs_MissingContainersReturnsError(t *testing.T) {
	client := NewClient("https://example.com", "token", "", "", false)

	pod := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":      "pod",
				"namespace": "default",
			},
			"spec": map[string]interface{}{},
		},
	}
	dynClient := dynfake.NewSimpleDynamicClient(scheme.Scheme, pod)
	client.dynamicClients["cluster"] = dynClient
	client.clientsets["cluster"] = k8sfake.NewSimpleClientset()

	_, err := client.GetAllContainerLogs(context.Background(), "cluster", "default", "pod", nil)
	if err == nil {
		t.Fatal("expected error when pod spec has no containers")
	}
	if !strings.Contains(err.Error(), "containers not found") {
		t.Fatalf("expected 'containers not found' error, got: %v", err)
	}
}

func TestGetAllContainerLogs_HappyPath(t *testing.T) {
	client := NewClient("https://example.com", "token", "", "", false)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "app"},
				{Name: "sidecar"},
			},
		},
	}
	dynClient := dynfake.NewSimpleDynamicClient(scheme.Scheme, pod)
	client.dynamicClients["cluster"] = dynClient
	client.clientsets["cluster"] = k8sfake.NewSimpleClientset()

	logs, err := client.GetAllContainerLogs(context.Background(), "cluster", "default", "pod", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("expected logs for 2 containers, got %d", len(logs))
	}
	for _, container := range []string{"app", "sidecar"} {
		got, ok := logs[container]
		if !ok {
			t.Fatalf("missing logs for container %q", container)
		}
		if got != "fake logs" {
			t.Fatalf("unexpected logs for container %q: %q", container, got)
		}
	}
}

func TestGetAllContainerLogs_PropagatesOptions(t *testing.T) {
	client := NewClient("https://example.com", "token", "", "", false)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "app"},
				{Name: "sidecar"},
			},
		},
	}
	dynClient := dynfake.NewSimpleDynamicClient(scheme.Scheme, pod)
	client.dynamicClients["cluster"] = dynClient
	client.clientsets["cluster"] = k8sfake.NewSimpleClientset()

	since := int64(120)
	logs, err := client.GetAllContainerLogs(context.Background(), "cluster", "default", "pod", &PodLogOptions{
		TailLines:    &since,
		SinceSeconds: &since,
		Previous:     true,
		Timestamps:   true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("expected logs for 2 containers, got %d", len(logs))
	}
}
