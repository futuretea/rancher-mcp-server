package capacity

import (
	"context"
	"testing"
)

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
