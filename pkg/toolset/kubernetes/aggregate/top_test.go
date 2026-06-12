package aggregate

import (
	"context"
	"testing"

	"github.com/futuretea/rancher-mcp-server/pkg/client/steve/fake"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestSortTopItems_ByCPURequest(t *testing.T) {
	items := []TopItem{
		{Name: "pod-c", CPUReq: 100},
		{Name: "pod-a", CPUReq: 500},
		{Name: "pod-b", CPUReq: 300},
	}
	sortTopItems(items, "cpu.request")
	if items[0].Name != "pod-a" {
		t.Errorf("expected first item to be pod-a, got %s", items[0].Name)
	}
	if items[1].Name != "pod-b" {
		t.Errorf("expected second item to be pod-b, got %s", items[1].Name)
	}
	if items[2].Name != "pod-c" {
		t.Errorf("expected third item to be pod-c, got %s", items[2].Name)
	}
}

func TestSortTopItems_ByMemoryUtil(t *testing.T) {
	items := []TopItem{
		{Name: "pod-c", MemUtil: 100},
		{Name: "pod-a", MemUtil: 500},
		{Name: "pod-b", MemUtil: 300},
	}
	sortTopItems(items, "mem.util")
	if items[0].Name != "pod-a" {
		t.Errorf("expected first item to be pod-a, got %s", items[0].Name)
	}
}

func TestSortTopItems_ByMemoryUtilAlias(t *testing.T) {
	items := []TopItem{
		{Name: "pod-c", MemUtil: 100},
		{Name: "pod-a", MemUtil: 500},
		{Name: "pod-b", MemUtil: 300},
	}
	sortTopItems(items, "memory.util")
	if items[0].Name != "pod-a" {
		t.Errorf("expected first item to be pod-a for memory.util alias, got %s", items[0].Name)
	}
	if items[1].Name != "pod-b" {
		t.Errorf("expected second item to be pod-b, got %s", items[1].Name)
	}
	if items[2].Name != "pod-c" {
		t.Errorf("expected third item to be pod-c, got %s", items[2].Name)
	}
}

func TestSortTopItems_ByRestartCount(t *testing.T) {
	items := []TopItem{
		{Name: "pod-c", Restarts: 1},
		{Name: "pod-a", Restarts: 10},
		{Name: "pod-b", Restarts: 5},
	}
	sortTopItems(items, "restart.count")
	if items[0].Name != "pod-a" {
		t.Errorf("expected first item to be pod-a, got %s", items[0].Name)
	}
}

func TestSortTopItems_DefaultName(t *testing.T) {
	items := []TopItem{
		{Name: "pod-c"},
		{Name: "pod-a"},
		{Name: "pod-b"},
	}
	sortTopItems(items, "")
	if items[0].Name != "pod-a" {
		t.Errorf("expected first item to be pod-a, got %s", items[0].Name)
	}
}

func TestSortTopItems_ByCPULimit(t *testing.T) {
	items := []TopItem{
		{Name: "pod-a", CPULimit: 100},
		{Name: "pod-b", CPULimit: 500},
	}
	sortTopItems(items, "cpu.limit")
	if items[0].Name != "pod-b" {
		t.Errorf("expected pod-b first by cpu.limit, got %s", items[0].Name)
	}
}

func TestSortTopItems_ByMemoryLimit(t *testing.T) {
	items := []TopItem{
		{Name: "pod-a", MemLimit: 1024},
		{Name: "pod-b", MemLimit: 4096},
	}
	sortTopItems(items, "memory.limit")
	if items[0].Name != "pod-b" {
		t.Errorf("expected pod-b first by memory.limit, got %s", items[0].Name)
	}
}

func TestSortTopItems_ByCPUUtilPct(t *testing.T) {
	items := []TopItem{
		{Name: "pod-a", CPUReq: 1000, CPUUtil: 200}, // 20%
		{Name: "pod-b", CPUReq: 1000, CPUUtil: 800}, // 80%
	}
	sortTopItems(items, "cpu.util.percentage")
	if items[0].Name != "pod-b" {
		t.Errorf("expected pod-b first by cpu.util.percentage (80%% > 20%%), got %s", items[0].Name)
	}
}

func TestSortTopItems_ByName(t *testing.T) {
	items := []TopItem{
		{Name: "pod-c"},
		{Name: "pod-a"},
		{Name: "pod-b"},
	}
	sortTopItems(items, "name")
	if items[0].Name != "pod-a" {
		t.Errorf("expected pod-a first by name, got %s", items[0].Name)
	}
}

func TestSortTopItems_PodCount(t *testing.T) {
	// pod.count falls through to name sort
	items := []TopItem{
		{Name: "pod-b"},
		{Name: "pod-a"},
	}
	sortTopItems(items, "pod.count")
	if items[0].Name != "pod-a" {
		t.Errorf("expected pod-a first (name fallback), got %s", items[0].Name)
	}
}

func TestExtractPodTopItem(t *testing.T) {
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
				},
			},
			"status": map[string]interface{}{
				"containerStatuses": []interface{}{
					map[string]interface{}{
						"name":         "container-1",
						"restartCount": float64(3),
					},
				},
			},
		},
	}

	item := extractPodTopItem(pod, nil)
	if item.Name != "test-pod" {
		t.Errorf("expected name test-pod, got %s", item.Name)
	}
	if item.Namespace != "default" {
		t.Errorf("expected namespace default, got %s", item.Namespace)
	}
	if item.CPUReq != 500 {
		t.Errorf("expected CPUReq 500, got %d", item.CPUReq)
	}
	if item.CPULimit != 1000 {
		t.Errorf("expected CPULimit 1000, got %d", item.CPULimit)
	}
	if item.MemReq != 256*1024*1024 {
		t.Errorf("expected MemReq 256Mi, got %d", item.MemReq)
	}
	if item.MemLimit != 512*1024*1024 {
		t.Errorf("expected MemLimit 512Mi, got %d", item.MemLimit)
	}
	if item.Restarts != 3 {
		t.Errorf("expected Restarts 3, got %d", item.Restarts)
	}
}

func TestExtractPodTopItem_WithMetrics(t *testing.T) {
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
						},
					},
				},
			},
		},
	}

	metricsMap := map[string]*podMetrics{
		"default/test-pod": {cpuUtil: 800, memUtil: 400 * 1024 * 1024},
	}

	item := extractPodTopItem(pod, metricsMap)
	if item.CPUUtil != 800 {
		t.Errorf("expected CPUUtil 800, got %d", item.CPUUtil)
	}
	if item.MemUtil != 400*1024*1024 {
		t.Errorf("expected MemUtil 400Mi, got %d", item.MemUtil)
	}
}

func TestExtractNodeTopItem(t *testing.T) {
	node := unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name": "node-1",
			},
			"status": map[string]interface{}{
				"capacity": map[string]interface{}{
					"cpu":    "4",
					"memory": "16Gi",
				},
				"allocatable": map[string]interface{}{
					"cpu":    "3800m",
					"memory": "15Gi",
				},
			},
		},
	}

	item := extractNodeTopItem(node, nil)
	if item.Name != "node-1" {
		t.Errorf("expected name node-1, got %s", item.Name)
	}
	if item.CPUReq != 4000 {
		t.Errorf("expected CPUReq (capacity) 4000, got %d", item.CPUReq)
	}
	if item.CPULimit != 3800 {
		t.Errorf("expected CPULimit (allocatable) 3800, got %d", item.CPULimit)
	}
}

func TestTopAnalyzerBuildResult_TruncatesToRequestedLimit(t *testing.T) {
	items := make([]TopItem, 60)
	for i := range items {
		items[i] = TopItem{Name: "pod"}
	}

	result, err := (&TopAnalyzer{}).buildResult(items, TopParams{Limit: 50}, "")
	if err != nil {
		t.Fatalf("buildResult() error = %v", err)
	}
	if result.Total != 60 {
		t.Fatalf("Total = %d, want 60", result.Total)
	}
	if len(result.Items) != 50 {
		t.Fatalf("len(Items) = %d, want 50", len(result.Items))
	}
	if !result.Truncated {
		t.Fatal("Truncated = false, want true")
	}
}

func TestTopAnalyzerBuildResult_ClampsLimitToMaxItems(t *testing.T) {
	items := make([]TopItem, MaxItems+25)
	for i := range items {
		items[i] = TopItem{Name: "pod"}
	}

	result, err := (&TopAnalyzer{}).buildResult(items, TopParams{Limit: MaxItems + 100}, "")
	if err != nil {
		t.Fatalf("buildResult() error = %v", err)
	}
	if len(result.Items) != MaxItems {
		t.Fatalf("len(Items) = %d, want %d", len(result.Items), MaxItems)
	}
	if !result.Truncated {
		t.Fatal("Truncated = false, want true")
	}
}

func TestCalcPercentage(t *testing.T) {
	if got := calcPercentage(50, 100); got != 50.0 {
		t.Errorf("calcPercentage(50, 100) = %f, want 50.0", got)
	}
	if got := calcPercentage(0, 0); got != 0.0 {
		t.Errorf("calcPercentage(0, 0) = %f, want 0.0", got)
	}
}

func TestResourceQuantityToMilli(t *testing.T) {
	if got := resourceQuantityToMilli("500m"); got != 500 {
		t.Errorf("resourceQuantityToMilli('500m') = %d, want 500", got)
	}
	if got := resourceQuantityToMilli("2"); got != 2000 {
		t.Errorf("resourceQuantityToMilli('2') = %d, want 2000", got)
	}
	if got := resourceQuantityToMilli(""); got != 0 {
		t.Errorf("resourceQuantityToMilli('') = %d, want 0", got)
	}
}

func TestResourceQuantityToBytes(t *testing.T) {
	if got := resourceQuantityToBytes("1Gi"); got != 1024*1024*1024 {
		t.Errorf("resourceQuantityToBytes('1Gi') = %d, want %d", got, 1024*1024*1024)
	}
	if got := resourceQuantityToBytes("512Mi"); got != 512*1024*1024 {
		t.Errorf("resourceQuantityToBytes('512Mi') = %d, want %d", got, 512*1024*1024)
	}
	if got := resourceQuantityToBytes(""); got != 0 {
		t.Errorf("resourceQuantityToBytes('') = %d, want 0", got)
	}
}

func TestResourceQuantityToMilli_Invalid(t *testing.T) {
	if got := resourceQuantityToMilli("invalid"); got != 0 {
		t.Errorf("resourceQuantityToMilli('invalid') = %d, want 0", got)
	}
}

func TestResourceQuantityToMilli_Negative(t *testing.T) {
	if got := resourceQuantityToMilli("-1"); got != -1000 {
		t.Errorf("resourceQuantityToMilli('-1') = %d, want -1000", got)
	}
}

func TestResourceQuantityToBytes_Invalid(t *testing.T) {
	if got := resourceQuantityToBytes("not-a-size"); got != 0 {
		t.Errorf("resourceQuantityToBytes('not-a-size') = %d, want 0", got)
	}
}

func TestExtractRestartCount(t *testing.T) {
	tests := []struct {
		input map[string]interface{}
		want  int32
	}{
		{map[string]interface{}{"restartCount": int64(5)}, 5},
		{map[string]interface{}{"restartCount": int32(5)}, 5},
		{map[string]interface{}{"restartCount": int(5)}, 5},
		{map[string]interface{}{"restartCount": float64(5)}, 5},
		{map[string]interface{}{"restartCount": float32(5)}, 5},
		{map[string]interface{}{"restartCount": "unexpected"}, 0},
		{map[string]interface{}{}, 0},
	}
	for _, tt := range tests {
		got := extractRestartCount(tt.input)
		if got != tt.want {
			t.Errorf("extractRestartCount(%v) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestNeedsMetrics(t *testing.T) {
	metricSorts := []string{
		"cpu.util",
		"mem.util", "memory.util",
		"cpu.util.percentage",
		"mem.util.percentage", "memory.util.percentage",
	}
	for _, s := range metricSorts {
		if !needsMetrics(s) {
			t.Errorf("needsMetrics(%q) = false, want true", s)
		}
	}
	nonMetricSorts := []string{"", "cpu.request", "mem.request", "memory.request", "name", "restart.count"}
	for _, s := range nonMetricSorts {
		if needsMetrics(s) {
			t.Errorf("needsMetrics(%q) = true, want false", s)
		}
	}
}

func TestClampLimit(t *testing.T) {
	tests := []struct {
		input int
		want  int
	}{
		{-10, DefaultLimit},
		{0, DefaultLimit},
		{1, 1},
		{DefaultLimit, DefaultLimit},
		{MaxItems, MaxItems},
		{MaxItems + 1, MaxItems},
		{1000, MaxItems},
	}
	for _, tt := range tests {
		got := ClampLimit(tt.input)
		if got != tt.want {
			t.Errorf("ClampLimit(%d) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestTopAnalyzer_Analyze_Pods(t *testing.T) {
	c := fake.NewClient()
	addTopPodResource(c, "pod-a", "default", "500m", "256Mi")
	addTopPodResource(c, "pod-b", "default", "1", "512Mi")
	addTopPodResource(c, "pod-c", "kube-system", "100m", "128Mi")

	a := NewTopAnalyzer(c)
	result, err := a.Analyze(context.Background(), TopParams{
		Cluster:   "test-cluster",
		Kind:      "pod",
		Namespace: "default",
		SortBy:    "cpu.request",
	})
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if result.Total != 2 {
		t.Fatalf("expected 2 pods in default ns, got %d", result.Total)
	}
	if result.Items[0].Name != "pod-b" {
		t.Errorf("expected pod-b (1 cpu) first by cpu.request, got %s", result.Items[0].Name)
	}
	if result.Items[0].CPUReq != 1000 {
		t.Errorf("expected CPUReq 1000 for pod-b, got %d", result.Items[0].CPUReq)
	}
}

func TestTopAnalyzer_Analyze_Nodes(t *testing.T) {
	c := fake.NewClient()
	addTopNodeResource(c, "node-a", "4", "16Gi", "3800m", "15Gi")
	addTopNodeResource(c, "node-b", "8", "32Gi", "7800m", "31Gi")

	a := NewTopAnalyzer(c)
	result, err := a.Analyze(context.Background(), TopParams{
		Cluster: "test-cluster",
		Kind:    "node",
		SortBy:  "cpu.request",
	})
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if result.Total != 2 {
		t.Fatalf("expected 2 nodes, got %d", result.Total)
	}
	if result.Items[0].Name != "node-b" {
		t.Errorf("expected node-b (8 cpu) first, got %s", result.Items[0].Name)
	}
}

func TestTopAnalyzer_Analyze_DefaultKind(t *testing.T) {
	c := fake.NewClient()
	addTopPodResource(c, "pod-1", "default", "500m", "256Mi")

	a := NewTopAnalyzer(c)
	result, err := a.Analyze(context.Background(), TopParams{
		Cluster: "test-cluster",
	})
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	if result.Total != 1 {
		t.Fatalf("expected 1 pod (default kind), got %d", result.Total)
	}
}

func TestTopAnalyzer_Analyze_UnsupportedKind(t *testing.T) {
	c := fake.NewClient()
	a := NewTopAnalyzer(c)
	_, err := a.Analyze(context.Background(), TopParams{
		Cluster: "test-cluster",
		Kind:    "service",
	})
	if err == nil {
		t.Fatal("expected error for unsupported kind")
	}
}

func TestTopAnalyzer_Analyze_LabelSelector(t *testing.T) {
	c := fake.NewClient()
	addTopPodResourceWithLabels(c, "pod-a", "default", "500m", "256Mi", map[string]string{"app": "nginx"})
	addTopPodResourceWithLabels(c, "pod-b", "default", "250m", "128Mi", map[string]string{"app": "redis"})

	a := NewTopAnalyzer(c)
	result, err := a.Analyze(context.Background(), TopParams{
		Cluster:       "test-cluster",
		Kind:          "pod",
		LabelSelector: "app=nginx",
	})
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	if result.Total != 1 {
		t.Fatalf("expected 1 pod with app=nginx, got %d", result.Total)
	}
	if result.Items[0].Name != "pod-a" {
		t.Errorf("expected pod-a, got %s", result.Items[0].Name)
	}
}

func addTopPodResource(c *fake.Client, name, namespace, cpu, memory string) {
	addTopPodResourceWithLabels(c, name, namespace, cpu, memory, nil)
}

func addTopPodResourceWithLabels(c *fake.Client, name, namespace, cpu, memory string, labels map[string]string) {
	u := &unstructured.Unstructured{}
	u.SetUnstructuredContent(map[string]interface{}{
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name": "main",
					"resources": map[string]interface{}{
						"requests": map[string]interface{}{
							"cpu":    cpu,
							"memory": memory,
						},
					},
				},
			},
		},
	})
	u.SetKind("Pod")
	u.SetName(name)
	u.SetNamespace(namespace)
	u.SetLabels(labels)
	c.AddResource(u)
}

func addTopNodeResource(c *fake.Client, name, capacityCPU, capacityMem, allocCPU, allocMem string) {
	u := &unstructured.Unstructured{}
	u.SetUnstructuredContent(map[string]interface{}{
		"status": map[string]interface{}{
			"capacity": map[string]interface{}{
				"cpu":    capacityCPU,
				"memory": capacityMem,
			},
			"allocatable": map[string]interface{}{
				"cpu":    allocCPU,
				"memory": allocMem,
			},
		},
	})
	u.SetKind("Node")
	u.SetName(name)
	c.AddResource(u)
}
