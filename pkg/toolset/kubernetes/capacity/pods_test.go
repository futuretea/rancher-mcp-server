package capacity

import (
	"testing"
)

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

func TestProcessSinglePod_NilNode(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("processSinglePod panicked with nil nodeInfo: %v", r)
		}
	}()

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
					},
				},
			},
		},
	}
	u := makeUnstructured("Pod", "orphan-pod", "default", content)
	processSinglePod(u, nil, false, false)
}
