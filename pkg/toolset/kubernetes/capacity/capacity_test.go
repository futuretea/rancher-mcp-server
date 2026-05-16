package capacity

import (
	"context"
	"testing"

	"github.com/futuretea/rancher-mcp-server/pkg/client/steve/fake"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestParseLabelSelector(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		result := parseLabelSelector("")
		if len(result) != 0 {
			t.Fatalf("expected empty map, got %v", result)
		}
	})

	t.Run("single equals", func(t *testing.T) {
		result := parseLabelSelector("app=nginx")
		if result["app"] != "nginx" {
			t.Fatalf("expected app=nginx, got %v", result)
		}
	})

	t.Run("double equals", func(t *testing.T) {
		result := parseLabelSelector("app==nginx")
		if result["app"] != "nginx" {
			t.Fatalf("expected app=nginx, got %v", result)
		}
	})

	t.Run("comma separated", func(t *testing.T) {
		result := parseLabelSelector("app=nginx,env=prod")
		if result["app"] != "nginx" || result["env"] != "prod" {
			t.Fatalf("expected app=nginx env=prod, got %v", result)
		}
	})

	t.Run("space separated", func(t *testing.T) {
		result := parseLabelSelector("app=nginx env=prod")
		if result["app"] != "nginx" || result["env"] != "prod" {
			t.Fatalf("expected app=nginx env=prod, got %v", result)
		}
	})

	t.Run("whitespace in values", func(t *testing.T) {
		result := parseLabelSelector("app=nginx,env=prod")
		if result["app"] != "nginx" || result["env"] != "prod" {
			t.Fatalf("expected app=nginx env=prod, got %v", result)
		}
	})
}

func TestMatchLabels(t *testing.T) {
	labels := map[string]string{"app": "nginx", "env": "prod"}

	t.Run("all match", func(t *testing.T) {
		if !matchLabels(labels, map[string]string{"app": "nginx"}) {
			t.Fatal("expected match")
		}
	})

	t.Run("partial mismatch", func(t *testing.T) {
		if matchLabels(labels, map[string]string{"app": "nginx", "env": "dev"}) {
			t.Fatal("expected no match on env=dev")
		}
	})

	t.Run("empty selector always matches", func(t *testing.T) {
		if !matchLabels(labels, map[string]string{}) {
			t.Fatal("empty selector should always match")
		}
	})

	t.Run("missing key", func(t *testing.T) {
		if matchLabels(labels, map[string]string{"tier": "frontend"}) {
			t.Fatal("expected no match on missing key")
		}
	})
}

func TestResourceQuantityToMilli(t *testing.T) {
	tests := []struct {
		q    string
		want int64
	}{
		{"", 0},
		{"1", 1000},
		{"500m", 500},
		{"2", 2000},
		{"0.5", 500},
	}
	for _, tt := range tests {
		got := resourceQuantityToMilli(tt.q)
		if got != tt.want {
			t.Errorf("resourceQuantityToMilli(%q) = %d, want %d", tt.q, got, tt.want)
		}
	}
}

func TestResourceQuantityToBytes(t *testing.T) {
	tests := []struct {
		q    string
		want int64
	}{
		{"", 0},
		{"1Ki", 1024},
		{"1Mi", 1048576},
		{"1Gi", 1073741824},
	}
	for _, tt := range tests {
		got := resourceQuantityToBytes(tt.q)
		if got != tt.want {
			t.Errorf("resourceQuantityToBytes(%q) = %d, want %d", tt.q, got, tt.want)
		}
	}
}

func TestParseTaintPart(t *testing.T) {
	t.Run("key=value:effect", func(t *testing.T) {
		k, v, e := parseTaintPart("node-role=master:NoSchedule")
		if k != "node-role" || v != "master" || e != "NoSchedule" {
			t.Errorf("got (%q, %q, %q), want (node-role, master, NoSchedule)", k, v, e)
		}
	})

	t.Run("key=value", func(t *testing.T) {
		k, v, e := parseTaintPart("key=val")
		if k != "key" || v != "val" || e != "" {
			t.Errorf("got (%q, %q, %q), want (key, val, \"\")", k, v, e)
		}
	})

	t.Run("key:effect", func(t *testing.T) {
		k, v, e := parseTaintPart("key:NoExecute")
		if k != "key" || v != "" || e != "NoExecute" {
			t.Errorf("got (%q, %q, %q), want (key, \"\", NoExecute)", k, v, e)
		}
	})

	t.Run("key only", func(t *testing.T) {
		k, v, e := parseTaintPart("mykey")
		if k != "mykey" || v != "" || e != "" {
			t.Errorf("got (%q, %q, %q), want (mykey, \"\", \"\")", k, v, e)
		}
	})
}

func TestTaintMatches(t *testing.T) {
	taints := []corev1.Taint{
		{Key: "node-role", Value: "master", Effect: corev1.TaintEffectNoSchedule},
		{Key: "dedicated", Value: "app", Effect: corev1.TaintEffectNoExecute},
	}

	t.Run("exact match", func(t *testing.T) {
		if !taintMatches(taints, "node-role", "master", "NoSchedule") {
			t.Fatal("expected match")
		}
	})

	t.Run("key only match", func(t *testing.T) {
		if !taintMatches(taints, "node-role", "", "") {
			t.Fatal("expected key-only match")
		}
	})

	t.Run("wrong value", func(t *testing.T) {
		if taintMatches(taints, "node-role", "worker", "") {
			t.Fatal("expected no match on wrong value")
		}
	})

	t.Run("wrong effect", func(t *testing.T) {
		if taintMatches(taints, "dedicated", "app", "NoSchedule") {
			t.Fatal("expected no match on wrong effect")
		}
	})

	t.Run("missing key", func(t *testing.T) {
		if taintMatches(taints, "nonexistent", "", "") {
			t.Fatal("expected no match on missing key")
		}
	})
}

func TestMatchTaints(t *testing.T) {
	taints := []corev1.Taint{
		{Key: "node-role", Value: "master", Effect: corev1.TaintEffectNoSchedule},
	}

	t.Run("exact selector matches", func(t *testing.T) {
		if !matchTaints(taints, "node-role=master:NoSchedule") {
			t.Fatal("expected match")
		}
	})

	t.Run("exclude matching taint", func(t *testing.T) {
		if matchTaints(taints, "node-role-") {
			t.Fatal("expected no match when excluding present taint")
		}
	})

	t.Run("exclude non-matching taint", func(t *testing.T) {
		if !matchTaints(taints, "nonexistent-") {
			t.Fatal("expected match when excluding absent taint")
		}
	})

	t.Run("empty selector always matches", func(t *testing.T) {
		if !matchTaints(taints, "") {
			t.Fatal("empty selector should always match")
		}
	})

	t.Run("require absent key matches taintless node", func(t *testing.T) {
		if !matchTaints(nil, "key-") {
			t.Fatal("expected match: node without taint should pass exclusion")
		}
	})
}

func makeUnstructured(_, name, namespace string, content map[string]interface{}) unstructured.Unstructured {
	u := unstructured.Unstructured{}
	u.SetUnstructuredContent(content)
	u.SetName(name)
	u.SetNamespace(namespace)
	return u
}

func TestExtractTaints(t *testing.T) {
	t.Run("no taints", func(t *testing.T) {
		u := makeUnstructured("Node", "node-1", "", map[string]interface{}{})
		taints := extractTaints(u)
		if len(taints) != 0 {
			t.Fatalf("expected 0 taints, got %d", len(taints))
		}
	})

	t.Run("single taint", func(t *testing.T) {
		content := map[string]interface{}{
			"spec": map[string]interface{}{
				"taints": []interface{}{
					map[string]interface{}{
						"key":    "node-role",
						"value":  "master",
						"effect": "NoSchedule",
					},
				},
			},
		}
		u := makeUnstructured("Node", "node-1", "", content)
		taints := extractTaints(u)
		if len(taints) != 1 {
			t.Fatalf("expected 1 taint, got %d", len(taints))
		}
		if taints[0].Key != "node-role" || taints[0].Value != "master" || taints[0].Effect != corev1.TaintEffectNoSchedule {
			t.Errorf("got taint %+v", taints[0])
		}
	})
}

func TestExtractNodeInfo(t *testing.T) {
	content := map[string]interface{}{
		"status": map[string]interface{}{
			"capacity": map[string]interface{}{
				"cpu":    "4",
				"memory": "16Gi",
				"pods":   "110",
			},
			"allocatable": map[string]interface{}{
				"cpu":    "3900m",
				"memory": "15Gi",
				"pods":   "110",
			},
		},
	}
	u := makeUnstructured("Node", "node-1", "", content)

	info := extractNodeInfo(u)
	if info == nil {
		t.Fatal("expected non-nil NodeInfo")
	}
	if info.Name != "node-1" {
		t.Errorf("expected name 'node-1', got %q", info.Name)
	}
	if info.CPU.Capacity != 4000 {
		t.Errorf("expected CPU capacity 4000m, got %d", info.CPU.Capacity)
	}
	if info.Memory.Capacity != 17179869184 {
		t.Errorf("expected Memory capacity 16Gi, got %d", info.Memory.Capacity)
	}
	if info.PodCount.Capacity != 110 {
		t.Errorf("expected Pod capacity 110, got %d", info.PodCount.Capacity)
	}
	if info.CPU.Allocatable != 3900 {
		t.Errorf("expected CPU allocatable 3900m, got %d", info.CPU.Allocatable)
	}
}

func TestAggregateNodeToCluster(t *testing.T) {
	cluster := &NodeInfo{Name: "*"}
	node := &NodeInfo{
		Name:     "node-1",
		CPU:      Resource{Capacity: 4000, Allocatable: 3900, Requested: 2500, Limited: 5000, Utilized: 1800},
		Memory:   Resource{Capacity: 17179869184, Allocatable: 16106127360, Requested: 8589934592, Limited: 12884901888, Utilized: 6442450944},
		PodCount: PodCountInfo{Capacity: 110, Allocatable: 110, Requested: 15},
	}

	aggregateNodeToCluster(cluster, node)

	if cluster.CPU.Capacity != 4000 {
		t.Errorf("expected cluster CPU capacity 4000, got %d", cluster.CPU.Capacity)
	}
	if cluster.CPU.Requested != 2500 {
		t.Errorf("expected cluster CPU requested 2500, got %d", cluster.CPU.Requested)
	}
	if cluster.Memory.Capacity != 17179869184 {
		t.Errorf("expected cluster Memory capacity %d, got %d", 17179869184, cluster.Memory.Capacity)
	}
	if cluster.PodCount.Requested != 15 {
		t.Errorf("expected cluster PodCount requested 15, got %d", cluster.PodCount.Requested)
	}
}

func TestShouldProcessPod(t *testing.T) {
	nodeInfoMap := map[string]*NodeInfo{
		"node-1": {Name: "node-1"},
	}

	t.Run("running pod on tracked node", func(t *testing.T) {
		content := map[string]interface{}{
			"status": map[string]interface{}{"phase": "Running"},
			"spec":   map[string]interface{}{"nodeName": "node-1"},
		}
		u := makeUnstructured("Pod", "my-pod", "default", content)
		if !shouldProcessPod(u, nodeInfoMap, nil) {
			t.Fatal("running pod on tracked node should be processed")
		}
	})

	t.Run("succeeded pod is skipped", func(t *testing.T) {
		content := map[string]interface{}{
			"status": map[string]interface{}{"phase": "Succeeded"},
			"spec":   map[string]interface{}{"nodeName": "node-1"},
		}
		u := makeUnstructured("Pod", "done-pod", "default", content)
		if shouldProcessPod(u, nodeInfoMap, nil) {
			t.Fatal("Succeeded pod should be skipped")
		}
	})

	t.Run("failed pod is skipped", func(t *testing.T) {
		content := map[string]interface{}{
			"status": map[string]interface{}{"phase": "Failed"},
			"spec":   map[string]interface{}{"nodeName": "node-1"},
		}
		u := makeUnstructured("Pod", "failed-pod", "default", content)
		if shouldProcessPod(u, nodeInfoMap, nil) {
			t.Fatal("Failed pod should be skipped")
		}
	})

	t.Run("unassigned pod is skipped", func(t *testing.T) {
		content := map[string]interface{}{
			"status": map[string]interface{}{"phase": "Pending"},
			"spec":   map[string]interface{}{"nodeName": ""},
		}
		u := makeUnstructured("Pod", "pending-pod", "default", content)
		if shouldProcessPod(u, nodeInfoMap, nil) {
			t.Fatal("unassigned pod should be skipped")
		}
	})

	t.Run("pod on filtered-out node is skipped", func(t *testing.T) {
		content := map[string]interface{}{
			"status": map[string]interface{}{"phase": "Running"},
			"spec":   map[string]interface{}{"nodeName": "node-2"},
		}
		u := makeUnstructured("Pod", "orphan-pod", "default", content)
		if shouldProcessPod(u, nodeInfoMap, nil) {
			t.Fatal("pod on filtered-out node should be skipped")
		}
	})
}

func TestProcessSinglePod(t *testing.T) {
	content := map[string]interface{}{
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name": "app",
					"resources": map[string]interface{}{
						"requests": map[string]interface{}{
							"cpu":    "500m",
							"memory": "256Mi",
						},
						"limits": map[string]interface{}{
							"cpu":    "1",
							"memory": "512Mi",
						},
					},
				},
			},
		},
	}
	u := makeUnstructured("Pod", "test-pod", "default", content)

	nodeInfo := &NodeInfo{Name: "node-1"}
	processSinglePod(u, nodeInfo, false, false)

	if nodeInfo.PodCount.Requested != 1 {
		t.Errorf("expected PodCount.Requested=1, got %d", nodeInfo.PodCount.Requested)
	}
	if nodeInfo.CPU.Requested != 500 {
		t.Errorf("expected CPU.Requested=500m, got %d", nodeInfo.CPU.Requested)
	}
	if nodeInfo.Memory.Requested != 268435456 {
		t.Errorf("expected Memory.Requested=256Mi, got %d", nodeInfo.Memory.Requested)
	}
	if nodeInfo.CPU.Limited != 1000 {
		t.Errorf("expected CPU.Limited=1000m, got %d", nodeInfo.CPU.Limited)
	}
	if nodeInfo.Memory.Limited != 536870912 {
		t.Errorf("expected Memory.Limited=512Mi, got %d", nodeInfo.Memory.Limited)
	}
}

func TestMatchesNodeSelector(t *testing.T) {
	u := makeUnstructured("Node", "node-1", "", map[string]interface{}{})
	u.SetLabels(map[string]string{"env": "prod", "zone": "a"})

	t.Run("empty selector matches all", func(t *testing.T) {
		if !matchesNodeSelector(u, map[string]string{}) {
			t.Fatal("empty selector should match")
		}
	})

	t.Run("matching label", func(t *testing.T) {
		if !matchesNodeSelector(u, map[string]string{"env": "prod"}) {
			t.Fatal("expected match")
		}
	})

	t.Run("non-matching label", func(t *testing.T) {
		if matchesNodeSelector(u, map[string]string{"env": "dev"}) {
			t.Fatal("expected no match")
		}
	})
}

func TestCalcPercentage(t *testing.T) {
	if got := calcPercentage(50, 100); got != 50.0 {
		t.Errorf("expected 50%%, got %f", got)
	}
	if got := calcPercentage(0, 100); got != 0.0 {
		t.Errorf("expected 0%%, got %f", got)
	}
	if got := calcPercentage(50, 0); got != 0.0 {
		t.Errorf("expected 0%% for zero total, got %f", got)
	}
	if got := calcPercentage(25, 100); got != 25.0 {
		t.Errorf("expected 25%%, got %f", got)
	}
}

func TestSortNodes(t *testing.T) {
	nodes := []NodeInfo{
		{Name: "node-b", CPU: Resource{Requested: 500}, Memory: Resource{Requested: 2048}, PodCount: PodCountInfo{Requested: 10}},
		{Name: "node-a", CPU: Resource{Requested: 1000}, Memory: Resource{Requested: 1024}, PodCount: PodCountInfo{Requested: 5}},
		{Name: "node-c", CPU: Resource{Requested: 250}, Memory: Resource{Requested: 4096}, PodCount: PodCountInfo{Requested: 20}},
	}

	t.Run("sort by cpu.request descending", func(t *testing.T) {
		SortNodes(nodes, "cpu.request")
		if nodes[0].Name != "node-a" || nodes[2].Name != "node-c" {
			t.Errorf("expected [node-a node-b node-c], got [%s %s %s]",
				nodes[0].Name, nodes[1].Name, nodes[2].Name)
		}
	})

	t.Run("sort by pod.count descending", func(t *testing.T) {
		SortNodes(nodes, "pod.count")
		if nodes[0].Name != "node-c" {
			t.Errorf("expected node-c (20 pods) first, got %s", nodes[0].Name)
		}
	})

	t.Run("sort by name ascending", func(t *testing.T) {
		SortNodes(nodes, "name")
		if nodes[0].Name != "node-a" || nodes[1].Name != "node-b" || nodes[2].Name != "node-c" {
			t.Errorf("expected alphabetical order, got [%s %s %s]",
				nodes[0].Name, nodes[1].Name, nodes[2].Name)
		}
	})

	t.Run("default sort by name", func(t *testing.T) {
		SortNodes(nodes, "")
		if nodes[0].Name != "node-a" {
			t.Errorf("expected default sort (name), got %s first", nodes[0].Name)
		}
	})
}

func TestFormatCPU(t *testing.T) {
	if got := formatCPU(500, false); got != "0.50c" {
		t.Errorf("expected '0.50c', got %q", got)
	}
	if got := formatCPU(500, true); got != "500m" {
		t.Errorf("expected '500m' for small raw, got %q", got)
	}
	if got := formatCPU(2000, true); got != "2.00c" {
		t.Errorf("expected '2.00c' for large raw, got %q", got)
	}
}

func TestFormatMemory(t *testing.T) {
	if got := formatMemory(1024*1024*1024, false); got != "1.00Gi" {
		t.Errorf("expected '1.00Gi', got %q", got)
	}
	if got := formatMemory(512*1024*1024, true); got != "512Mi" {
		t.Errorf("expected '512Mi', got %q", got)
	}
	if got := formatMemory(128*1024, true); got != "128Ki" {
		t.Errorf("expected '128Ki', got %q", got)
	}
	if got := formatMemory(256, true); got != "256" {
		t.Errorf("expected '256', got %q", got)
	}
}

func TestFormatLabels(t *testing.T) {
	if got := formatLabels(map[string]string{}); got != "" {
		t.Errorf("expected empty for nil labels, got %q", got)
	}
	if got := formatLabels(map[string]string{"app": "nginx"}); got != "app=nginx" {
		t.Errorf("expected 'app=nginx', got %q", got)
	}
	if got := formatLabels(map[string]string{"key": ""}); got != "key" {
		t.Errorf("expected 'key' for empty value, got %q", got)
	}
}

func TestToAnySlice(t *testing.T) {
	got := toAnySlice([]string{"a", "b"})
	if len(got) != 2 || got[0].(string) != "a" || got[1].(string) != "b" {
		t.Errorf("expected [a b], got %v", got)
	}
	if got := toAnySlice(nil); len(got) != 0 {
		t.Errorf("expected empty for nil, got %v", got)
	}
}

func TestFormatResult(t *testing.T) {
	result := &Result{
		Nodes:   []NodeInfo{},
		Cluster: NodeInfo{Name: "*"},
	}

	t.Run("json format", func(t *testing.T) {
		got, err := FormatResult(result, "json", false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got == "" {
			t.Fatal("expected non-empty JSON")
		}
	})

	t.Run("yaml format", func(t *testing.T) {
		got, err := FormatResult(result, "yaml", false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got == "" {
			t.Fatal("expected non-empty YAML")
		}
	})

	t.Run("table format default", func(t *testing.T) {
		got, err := FormatResult(result, "table", false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got == "" {
			t.Fatal("expected non-empty table")
		}
	})

	t.Run("unknown format defaults to table", func(t *testing.T) {
		got, err := FormatResult(result, "", false)
		if err != nil {
			t.Fatalf("unexpected error for empty format: %v", err)
		}
		if got == "" {
			t.Fatal("expected non-empty table for default")
		}
	})
}

func TestMatchesNodeSelector_Nil(t *testing.T) {
	u := makeUnstructured("Node", "node-1", "", map[string]interface{}{})
	if !matchesNodeSelector(u, nil) {
		t.Fatal("nil selector should match all")
	}
}

func TestAnalyze_Integration(t *testing.T) {
	c := makeFakeClient()

	a := NewAnalyzer(c)
	result, err := a.Analyze(context.Background(), Params{
		Cluster: "test-cluster",
	})
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if len(result.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(result.Nodes))
	}

	// Verify node names
	nodeNames := map[string]bool{}
	for _, n := range result.Nodes {
		nodeNames[n.Name] = true
	}
	if !nodeNames["node-1"] || !nodeNames["node-2"] {
		t.Errorf("expected nodes node-1 and node-2, got %v", nodeNames)
	}

	// Verify cluster-level aggregation
	if result.Cluster.PodCount.Requested != 3 {
		t.Errorf("expected 3 pods requested, got %d", result.Cluster.PodCount.Requested)
	}
	// 500m + 1000m + 250m = 1750m CPU requested
	if result.Cluster.CPU.Requested != 1750 {
		t.Errorf("expected 1750m CPU requested, got %d", result.Cluster.CPU.Requested)
	}
	// 256Mi + 512Mi + 128Mi = 896Mi memory requested
	expectedMem := int64(256+512+128) * 1024 * 1024
	if result.Cluster.Memory.Requested != expectedMem {
		t.Errorf("expected %d memory requested, got %d", expectedMem, result.Cluster.Memory.Requested)
	}
}

func TestAnalyze_Integration_ShowPods(t *testing.T) {
	c := makeFakeClient()

	a := NewAnalyzer(c)
	result, err := a.Analyze(context.Background(), Params{
		Cluster:  "test-cluster",
		ShowPods: true,
	})
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// node-1 should have 2 pods, node-2 should have 1
	for _, n := range result.Nodes {
		switch n.Name {
		case "node-1":
			if len(n.Pods) != 2 {
				t.Errorf("node-1: expected 2 pods, got %d", len(n.Pods))
			}
		case "node-2":
			if len(n.Pods) != 1 {
				t.Errorf("node-2: expected 1 pod, got %d", len(n.Pods))
			}
		}
	}
}

func TestAnalyze_Integration_NodeLabelSelector(t *testing.T) {
	c := makeFakeClient()

	a := NewAnalyzer(c)
	result, err := a.Analyze(context.Background(), Params{
		Cluster:           "test-cluster",
		NodeLabelSelector: "env=prod",
	})
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if len(result.Nodes) != 1 {
		t.Fatalf("expected 1 node matching env=prod, got %d", len(result.Nodes))
	}
	if result.Nodes[0].Name != "node-1" {
		t.Errorf("expected node-1 (env=prod), got %s", result.Nodes[0].Name)
	}
}

func TestAnalyze_Integration_PodLabelSelector(t *testing.T) {
	c := makeFakeClient()

	a := NewAnalyzer(c)
	result, err := a.Analyze(context.Background(), Params{
		Cluster:       "test-cluster",
		LabelSelector: "app=nginx",
	})
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Only pod-a (app=nginx, on node-1) and pod-c (app=nginx, on node-2) should count
	if result.Cluster.PodCount.Requested != 2 {
		t.Errorf("expected 2 pods (app=nginx), got %d", result.Cluster.PodCount.Requested)
	}
	// pod-a has 256Mi, pod-c has 128Mi = 384Mi
	expectedMem := int64(256+128) * 1024 * 1024
	if result.Cluster.Memory.Requested != expectedMem {
		t.Errorf("expected %d memory requested, got %d", expectedMem, result.Cluster.Memory.Requested)
	}
}

func makeFakeClient() *fake.Client {
	c := fake.NewClient()

	c.AddResource(makeUnstructuredPtr("Node", "node-1", "", map[string]interface{}{
		"status": map[string]interface{}{
			"capacity": map[string]interface{}{
				"cpu":    "4",
				"memory": "16Gi",
				"pods":   "110",
			},
			"allocatable": map[string]interface{}{
				"cpu":    "3900m",
				"memory": "15Gi",
				"pods":   "110",
			},
		},
	}, map[string]string{"env": "prod"}))

	c.AddResource(makeUnstructuredPtr("Node", "node-2", "", map[string]interface{}{
		"status": map[string]interface{}{
			"capacity": map[string]interface{}{
				"cpu":    "2",
				"memory": "8Gi",
				"pods":   "110",
			},
			"allocatable": map[string]interface{}{
				"cpu":    "1900m",
				"memory": "7Gi",
				"pods":   "110",
			},
		},
	}, map[string]string{"env": "staging"}))

	// pod-a: Running on node-1, app=nginx, 500m/256Mi requests, 1/512Mi limits
	c.AddResource(makeUnstructuredPtr("Pod", "pod-a", "default", map[string]interface{}{
		"status": map[string]interface{}{
			"phase": "Running",
		},
		"spec": map[string]interface{}{
			"nodeName": "node-1",
			"containers": []interface{}{
				map[string]interface{}{
					"name": "app",
					"resources": map[string]interface{}{
						"requests": map[string]interface{}{
							"cpu":    "500m",
							"memory": "256Mi",
						},
						"limits": map[string]interface{}{
							"cpu":    "1",
							"memory": "512Mi",
						},
					},
				},
			},
		},
	}, map[string]string{"app": "nginx"}))

	// pod-b: Running on node-1, app=redis, 1000m/512Mi requests
	c.AddResource(makeUnstructuredPtr("Pod", "pod-b", "default", map[string]interface{}{
		"status": map[string]interface{}{
			"phase": "Running",
		},
		"spec": map[string]interface{}{
			"nodeName": "node-1",
			"containers": []interface{}{
				map[string]interface{}{
					"name": "cache",
					"resources": map[string]interface{}{
						"requests": map[string]interface{}{
							"cpu":    "1",
							"memory": "512Mi",
						},
					},
				},
			},
		},
	}, map[string]string{"app": "redis"}))

	// pod-c: Running on node-2, app=nginx, 250m/128Mi requests
	c.AddResource(makeUnstructuredPtr("Pod", "pod-c", "default", map[string]interface{}{
		"status": map[string]interface{}{
			"phase": "Running",
		},
		"spec": map[string]interface{}{
			"nodeName": "node-2",
			"containers": []interface{}{
				map[string]interface{}{
					"name": "web",
					"resources": map[string]interface{}{
						"requests": map[string]interface{}{
							"cpu":    "250m",
							"memory": "128Mi",
						},
					},
				},
			},
		},
	}, map[string]string{"app": "nginx"}))

	return c
}

func makeUnstructuredPtr(kind, name, namespace string, content map[string]interface{}, labels map[string]string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetUnstructuredContent(content)
	u.SetKind(kind)
	u.SetName(name)
	u.SetNamespace(namespace)
	u.SetLabels(labels)
	return u
}
