package capacity

import (
	"testing"
)

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

func testSortNodesFixture() []NodeInfo {
	return []NodeInfo{
		{Name: "node-b", CPU: Resource{Requested: 500, Limited: 2000, Utilized: 1000, Allocatable: 4000}, Memory: Resource{Requested: 2048, Limited: 4096, Utilized: 1024, Allocatable: 8192}, PodCount: PodCountInfo{Requested: 10}},
		{Name: "node-a", CPU: Resource{Requested: 1000, Limited: 3000, Utilized: 500, Allocatable: 4000}, Memory: Resource{Requested: 1024, Limited: 2048, Utilized: 2048, Allocatable: 8192}, PodCount: PodCountInfo{Requested: 5}},
		{Name: "node-c", CPU: Resource{Requested: 250, Limited: 1000, Utilized: 2000, Allocatable: 4000}, Memory: Resource{Requested: 4096, Limited: 8192, Utilized: 512, Allocatable: 8192}, PodCount: PodCountInfo{Requested: 20}},
	}
}

func TestSortNodes_Basic(t *testing.T) {
	nodes := testSortNodesFixture()

	t.Run("sort by cpu.request descending", func(t *testing.T) {
		local := make([]NodeInfo, len(nodes))
		copy(local, nodes)
		SortNodes(local, "cpu.request")
		if local[0].Name != "node-a" || local[2].Name != "node-c" {
			t.Errorf("expected [node-a node-b node-c], got [%s %s %s]",
				local[0].Name, local[1].Name, local[2].Name)
		}
	})

	t.Run("sort by pod.count descending", func(t *testing.T) {
		local := make([]NodeInfo, len(nodes))
		copy(local, nodes)
		SortNodes(local, "pod.count")
		if local[0].Name != "node-c" {
			t.Errorf("expected node-c (20 pods) first, got %s", local[0].Name)
		}
	})

	t.Run("sort by name ascending", func(t *testing.T) {
		local := make([]NodeInfo, len(nodes))
		copy(local, nodes)
		SortNodes(local, "name")
		if local[0].Name != "node-a" || local[1].Name != "node-b" || local[2].Name != "node-c" {
			t.Errorf("expected alphabetical order, got [%s %s %s]",
				local[0].Name, local[1].Name, local[2].Name)
		}
	})

	t.Run("default sort by name", func(t *testing.T) {
		local := make([]NodeInfo, len(nodes))
		copy(local, nodes)
		SortNodes(local, "")
		if local[0].Name != "node-a" {
			t.Errorf("expected default sort (name), got %s first", local[0].Name)
		}
	})
}

func TestSortNodes_ResourceFields(t *testing.T) {
	nodes := testSortNodesFixture()

	t.Run("sort by memory.request descending", func(t *testing.T) {
		local := make([]NodeInfo, len(nodes))
		copy(local, nodes)
		SortNodes(local, "memory.request")
		if local[0].Name != "node-c" {
			t.Errorf("expected node-c first, got %s", local[0].Name)
		}
	})

	t.Run("sort by cpu.limit descending", func(t *testing.T) {
		local := make([]NodeInfo, len(nodes))
		copy(local, nodes)
		SortNodes(local, "cpu.limit")
		if local[0].Name != "node-a" {
			t.Errorf("expected node-a first, got %s", local[0].Name)
		}
	})

	t.Run("sort by memory.limit descending", func(t *testing.T) {
		local := make([]NodeInfo, len(nodes))
		copy(local, nodes)
		SortNodes(local, "mem.limit")
		if local[0].Name != "node-c" {
			t.Errorf("expected node-c first, got %s", local[0].Name)
		}
	})
}

func TestSortNodes_UtilizationAndPercentage(t *testing.T) {
	nodes := testSortNodesFixture()

	t.Run("sort by cpu.util descending", func(t *testing.T) {
		local := make([]NodeInfo, len(nodes))
		copy(local, nodes)
		SortNodes(local, "cpu.util")
		if local[0].Name != "node-c" {
			t.Errorf("expected node-c first, got %s", local[0].Name)
		}
	})

	t.Run("sort by memory.util descending", func(t *testing.T) {
		local := make([]NodeInfo, len(nodes))
		copy(local, nodes)
		SortNodes(local, "mem.util")
		if local[0].Name != "node-a" {
			t.Errorf("expected node-a first, got %s", local[0].Name)
		}
	})

	t.Run("sort by cpu.util.percentage descending", func(t *testing.T) {
		local := make([]NodeInfo, len(nodes))
		copy(local, nodes)
		SortNodes(local, "cpu.util.percentage")
		if local[0].Name != "node-c" {
			t.Errorf("expected node-c first, got %s", local[0].Name)
		}
	})

	t.Run("sort by memory.request.percentage descending", func(t *testing.T) {
		local := make([]NodeInfo, len(nodes))
		copy(local, nodes)
		SortNodes(local, "memory.request.percentage")
		if local[0].Name != "node-c" {
			t.Errorf("expected node-c first, got %s", local[0].Name)
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
