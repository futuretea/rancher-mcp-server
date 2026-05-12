package aggregate

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestAggregatePodResources(t *testing.T) {
	pod := unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":      "test-pod",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"name": "container-1",
						"resources": map[string]interface{}{
							"requests": map[string]interface{}{
								"cpu":    "500m",
								"memory": "256Mi",
							},
							"limits": map[string]interface{}{
								"cpu":    "1000m",
								"memory": "512Mi",
							},
						},
					},
					map[string]interface{}{
						"name": "container-2",
						"resources": map[string]interface{}{
							"requests": map[string]interface{}{
								"cpu":    "250m",
								"memory": "128Mi",
							},
							"limits": map[string]interface{}{
								"cpu":    "500m",
								"memory": "256Mi",
							},
						},
					},
				},
			},
		},
	}

	item := &SummaryItem{Group: "default"}
	aggregatePodResources(pod, item)

	if item.PodCount != 1 {
		t.Errorf("expected PodCount 1, got %d", item.PodCount)
	}
	if item.CPUReq != 750 {
		t.Errorf("expected CPUReq 750, got %d", item.CPUReq)
	}
	if item.CPULimit != 1500 {
		t.Errorf("expected CPULimit 1500, got %d", item.CPULimit)
	}
	if item.MemReq != 384*1024*1024 {
		t.Errorf("expected MemReq 384Mi, got %d", item.MemReq)
	}
	if item.MemLimit != 768*1024*1024 {
		t.Errorf("expected MemLimit 768Mi, got %d", item.MemLimit)
	}
}

func TestAggregatePodResources_NoResources(t *testing.T) {
	pod := unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":      "test-pod",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"name": "container-1",
					},
				},
			},
		},
	}

	item := &SummaryItem{Group: "default"}
	aggregatePodResources(pod, item)

	if item.PodCount != 1 {
		t.Errorf("expected PodCount 1, got %d", item.PodCount)
	}
	if item.CPUReq != 0 {
		t.Errorf("expected CPUReq 0, got %d", item.CPUReq)
	}
}

func TestSortSummaryItems_ByCPURequest(t *testing.T) {
	items := []SummaryItem{
		{Group: "ns-c", CPUReq: 100},
		{Group: "ns-a", CPUReq: 500},
		{Group: "ns-b", CPUReq: 300},
	}
	sortSummaryItems(items, "cpu.request")
	if items[0].Group != "ns-a" {
		t.Errorf("expected first item to be ns-a, got %s", items[0].Group)
	}
	if items[1].Group != "ns-b" {
		t.Errorf("expected second item to be ns-b, got %s", items[1].Group)
	}
	if items[2].Group != "ns-c" {
		t.Errorf("expected third item to be ns-c, got %s", items[2].Group)
	}
}

func TestSortSummaryItems_ByPodCount(t *testing.T) {
	items := []SummaryItem{
		{Group: "ns-c", PodCount: 1},
		{Group: "ns-a", PodCount: 10},
		{Group: "ns-b", PodCount: 5},
	}
	sortSummaryItems(items, "pod.count")
	if items[0].Group != "ns-a" {
		t.Errorf("expected first item to be ns-a, got %s", items[0].Group)
	}
}

func TestSortSummaryItems_DefaultByName(t *testing.T) {
	items := []SummaryItem{
		{Group: "ns-c"},
		{Group: "ns-a"},
		{Group: "ns-b"},
	}
	sortSummaryItems(items, "")
	if items[0].Group != "ns-a" {
		t.Errorf("expected first item to be ns-a, got %s", items[0].Group)
	}
}
