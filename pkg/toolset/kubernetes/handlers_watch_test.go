package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/futuretea/rancher-mcp-server/pkg/client/steve"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestWatchDiffWithReader_ReportsDeletedAndRecreatedResources(t *testing.T) {
	reader := &sequenceResourceReader{
		lists: []*unstructured.UnstructuredList{
			{
				Items: []unstructured.Unstructured{
					newWatchTestObject("apps/v1", "Deployment", "default", "demo", 1),
				},
			},
			{},
			{
				Items: []unstructured.Unstructured{
					newWatchTestObject("apps/v1", "Deployment", "default", "demo", 2),
				},
			},
		},
	}

	output, err := watchDiffWithReader(context.Background(), reader, &watchRequest{
		cluster:        "c1",
		kind:           "deployment",
		namespace:      "default",
		interval:       0,
		iterations:     3,
		maxItems:       MaxWatchItems,
		maxOutputBytes: MaxWatchOutputBytes,
	})
	if err != nil {
		t.Fatalf("watchDiffWithReader() returned unexpected error: %v", err)
	}
	if !strings.Contains(output, "deletions=1") {
		t.Fatalf("expected deletion count in output, got %q", output)
	}
	if !strings.Contains(output, "Deleted Resource") {
		t.Fatalf("expected explicit deleted resource marker, got %q", output)
	}
	if !strings.Contains(output, "+ New Resource") {
		t.Fatalf("expected recreated resource to be treated as new, got %q", output)
	}
}

func TestWatchDiffWithReader_RejectsOversizedIteration(t *testing.T) {
	items := make([]unstructured.Unstructured, 0, MaxWatchItems+1)
	for i := 0; i < MaxWatchItems+1; i++ {
		items = append(items, newWatchTestObject("v1", "Pod", "default", fmt.Sprintf("pod-%d", i), int64(i)))
	}

	reader := &sequenceResourceReader{
		lists: []*unstructured.UnstructuredList{
			{Items: items},
		},
	}

	_, err := watchDiffWithReader(context.Background(), reader, &watchRequest{
		cluster:        "c1",
		kind:           "pod",
		namespace:      "default",
		interval:       0,
		iterations:     1,
		maxItems:       MaxWatchItems,
		maxOutputBytes: MaxWatchOutputBytes,
	})
	if err == nil || !strings.Contains(err.Error(), "per-iteration limit") {
		t.Fatalf("expected oversized iteration error, got %v", err)
	}
}

func TestWatchDiffWithReader_RejectsLargeOutput(t *testing.T) {
	reader := &sequenceResourceReader{
		lists: []*unstructured.UnstructuredList{
			{
				Items: []unstructured.Unstructured{
					newWatchTestObject("apps/v1", "Deployment", "default", "demo", 1),
				},
			},
		},
	}

	_, err := watchDiffWithReader(context.Background(), reader, &watchRequest{
		cluster:        "c1",
		kind:           "deployment",
		namespace:      "default",
		interval:       0,
		iterations:     1,
		maxItems:       MaxWatchItems,
		maxOutputBytes: 32,
	})
	if err == nil || !strings.Contains(err.Error(), "output limit") {
		t.Fatalf("expected output limit error, got %v", err)
	}
}

func TestWaitForNextIteration_RespectsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := waitForNextIteration(ctx, time.Second)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("waitForNextIteration() error = %v, want %v", err, context.Canceled)
	}
}

type sequenceResourceReader struct {
	lists []*unstructured.UnstructuredList
	index int
}

func (r *sequenceResourceReader) GetResource(context.Context, string, string, string, string) (*unstructured.Unstructured, error) {
	return nil, errors.New("not implemented")
}

func (r *sequenceResourceReader) ListResources(context.Context, string, string, string, *steve.ListOptions) (*unstructured.UnstructuredList, error) {
	if r.index >= len(r.lists) {
		return &unstructured.UnstructuredList{}, nil
	}
	current := r.lists[r.index]
	r.index++
	return current.DeepCopy(), nil
}

func (r *sequenceResourceReader) GetEvents(context.Context, string, string, string, string) ([]corev1.Event, error) {
	return nil, nil
}

func newWatchTestObject(apiVersion, kind, namespace, name string, replicas int64) unstructured.Unstructured {
	return unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": apiVersion,
			"kind":       kind,
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"replicas": replicas,
			},
		},
	}
}
