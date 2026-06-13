package capacity

import (
	"testing"

	"github.com/futuretea/rancher-mcp-server/pkg/toolset/kubernetes/internal/formatutil"
	corev1 "k8s.io/api/core/v1"
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
		got := formatutil.ResourceQuantityToMilli(tt.q)
		if got != tt.want {
			t.Errorf("formatutil.ResourceQuantityToMilli(%q) = %d, want %d", tt.q, got, tt.want)
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
		got := formatutil.ResourceQuantityToBytes(tt.q)
		if got != tt.want {
			t.Errorf("formatutil.ResourceQuantityToBytes(%q) = %d, want %d", tt.q, got, tt.want)
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

func TestMatchesNodeSelector_Nil(t *testing.T) {
	u := makeUnstructured("Node", "node-1", "", map[string]interface{}{})
	if !matchesNodeSelector(u, nil) {
		t.Fatal("nil selector should match all")
	}
}

func TestResourceQuantityToMilli_Invalid(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("resourceQuantityToMilli panicked on invalid input: %v", r)
		}
	}()

	if got := formatutil.ResourceQuantityToMilli("not-a-quantity"); got != 0 {
		t.Errorf("expected 0 for invalid quantity, got %d", got)
	}
}

func TestResourceQuantityToBytes_Invalid(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("resourceQuantityToBytes panicked on invalid input: %v", r)
		}
	}()

	if got := formatutil.ResourceQuantityToBytes("bad-bytes"); got != 0 {
		t.Errorf("expected 0 for invalid quantity, got %d", got)
	}
}

func TestExtractNodeInfo_InvalidPods(t *testing.T) {
	content := map[string]interface{}{
		"status": map[string]interface{}{
			"capacity": map[string]interface{}{
				"cpu":    "4",
				"memory": "16Gi",
				"pods":   "not-a-number",
			},
			"allocatable": map[string]interface{}{
				"cpu":    "3900m",
				"memory": "15Gi",
				"pods":   "also-bad",
			},
		},
	}
	u := makeUnstructured("Node", "node-bad-pods", "", content)

	info := extractNodeInfo(u)
	if info == nil {
		t.Fatal("expected non-nil NodeInfo")
	}
	if info.CPU.Capacity != 4000 {
		t.Errorf("expected CPU capacity 4000m, got %d", info.CPU.Capacity)
	}
	if info.Memory.Capacity != 17179869184 {
		t.Errorf("expected Memory capacity 16Gi, got %d", info.Memory.Capacity)
	}
	if info.PodCount.Capacity != 0 {
		t.Errorf("expected PodCount.Capacity 0 for invalid value, got %d", info.PodCount.Capacity)
	}
	if info.PodCount.Allocatable != 0 {
		t.Errorf("expected PodCount.Allocatable 0 for invalid value, got %d", info.PodCount.Allocatable)
	}
}
