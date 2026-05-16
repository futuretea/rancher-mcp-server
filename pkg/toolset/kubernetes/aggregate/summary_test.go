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
	if item.CPULimit != 0 {
		t.Errorf("expected CPULimit 0, got %d", item.CPULimit)
	}
	if item.MemReq != 0 {
		t.Errorf("expected MemReq 0, got %d", item.MemReq)
	}
	if item.MemLimit != 0 {
		t.Errorf("expected MemLimit 0, got %d", item.MemLimit)
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

func TestSummarizePods_GroupByNamespace(t *testing.T) {
	pods := []unstructured.Unstructured{
		summaryTestPod("pod-a", "prod", map[string]string{"app": "api"}, "500m", "256Mi"),
		summaryTestPod("pod-b", "prod", map[string]string{"app": "worker"}, "250m", "128Mi"),
		summaryTestPod("pod-c", "staging", map[string]string{"app": "api"}, "100m", "64Mi"),
	}

	result, err := summarizePods(pods, SummaryParams{GroupBy: "namespace", SortBy: "name", Limit: 50})
	if err != nil {
		t.Fatalf("summarizePods() error = %v", err)
	}
	if result.Total != 2 {
		t.Fatalf("Total = %d, want 2", result.Total)
	}
	if result.Items[0].Group != "prod" || result.Items[0].PodCount != 2 || result.Items[0].CPUReq != 750 {
		t.Fatalf("prod group = %+v, want podCount=2 cpuReq=750", result.Items[0])
	}
	if result.Items[1].Group != "staging" || result.Items[1].PodCount != 1 || result.Items[1].CPUReq != 100 {
		t.Fatalf("staging group = %+v, want podCount=1 cpuReq=100", result.Items[1])
	}
}

func TestSummarizePods_GroupByLabel(t *testing.T) {
	pods := []unstructured.Unstructured{
		summaryTestPod("pod-a", "prod", map[string]string{"app": "api"}, "500m", "256Mi"),
		summaryTestPod("pod-b", "prod", map[string]string{"app": "worker"}, "250m", "128Mi"),
		summaryTestPod("pod-c", "prod", map[string]string{}, "100m", "64Mi"),
	}

	result, err := summarizePods(pods, SummaryParams{GroupBy: "label", GroupByKey: "app", SortBy: "name", Limit: 50})
	if err != nil {
		t.Fatalf("summarizePods() error = %v", err)
	}
	if result.Total != 3 {
		t.Fatalf("Total = %d, want 3", result.Total)
	}
	if result.Items[0].Group != "<none>" {
		t.Fatalf("first group = %q, want <none>", result.Items[0].Group)
	}
	if result.Items[1].Group != "api" || result.Items[1].CPUReq != 500 {
		t.Fatalf("api group = %+v, want cpuReq=500", result.Items[1])
	}
	if result.Items[2].Group != "worker" || result.Items[2].CPUReq != 250 {
		t.Fatalf("worker group = %+v, want cpuReq=250", result.Items[2])
	}
}

func TestSummarizePods_GroupByLabelRequiresKey(t *testing.T) {
	pods := []unstructured.Unstructured{
		summaryTestPod("pod-a", "prod", map[string]string{"app": "api"}, "500m", "256Mi"),
	}

	_, err := summarizePods(pods, SummaryParams{GroupBy: "label"})
	if err == nil {
		t.Fatal("summarizePods() error = nil, want groupByKey error")
	}
}

func summaryTestPod(name, namespace string, labels map[string]string, cpu, memory string) unstructured.Unstructured {
	labelValues := make(map[string]interface{}, len(labels))
	for key, value := range labels {
		labelValues[key] = value
	}

	return unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
				"labels":    labelValues,
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"name": "container-1",
						"resources": map[string]interface{}{
							"requests": map[string]interface{}{
								"cpu":    cpu,
								"memory": memory,
							},
						},
					},
				},
			},
		},
	}
}
