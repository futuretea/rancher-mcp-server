package steve

import (
	"context"
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// InspectPodResult contains the results of inspecting a pod.
type InspectPodResult struct {
	Pod     *unstructured.Unstructured `json:"pod"`
	Parent  *unstructured.Unstructured `json:"parent,omitempty"`
	Metrics *unstructured.Unstructured `json:"metrics,omitempty"`
	Logs    map[string]string          `json:"logs"`
}

// ToJSON converts the InspectPodResult to a JSON string.
func (r *InspectPodResult) ToJSON() (string, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// InspectPod retrieves comprehensive information about a pod including its parent, metrics, and logs.
func (c *Client) InspectPod(ctx context.Context, clusterID, namespace, podName string) (*InspectPodResult, error) {
	pod, err := c.GetResource(ctx, clusterID, "pod", namespace, podName)
	if err != nil {
		return nil, fmt.Errorf("failed to get pod: %w", err)
	}

	result := &InspectPodResult{
		Pod:    pod,
		Parent: c.findPodParent(ctx, clusterID, namespace, pod),
		Logs:   make(map[string]string),
	}

	// Get pod metrics (ignore error as metrics-server might not be installed)
	result.Metrics, _ = c.GetResource(ctx, clusterID, "pod.metrics.k8s.io", namespace, podName)

	// Get container logs
	tailLines := int64(50)
	if logs, err := c.GetAllContainerLogs(ctx, clusterID, namespace, podName, &PodLogOptions{TailLines: &tailLines}); err == nil {
		result.Logs = logs
	}

	return result, nil
}

// findPodParent finds the parent workload (Deployment/StatefulSet/DaemonSet/Job) of a pod.
func (c *Client) findPodParent(ctx context.Context, clusterID, namespace string, pod *unstructured.Unstructured) *unstructured.Unstructured {
	ownerRefs, found, _ := unstructured.NestedSlice(pod.Object, "metadata", "ownerReferences")
	if !found || len(ownerRefs) == 0 {
		return nil
	}

	for _, ref := range ownerRefs {
		ownerRef, ok := ref.(map[string]interface{})
		if !ok {
			continue
		}
		kind, _ := ownerRef["kind"].(string)
		name, _ := ownerRef["name"].(string)

		switch kind {
		case "ReplicaSet":
			// ReplicaSet is usually owned by a Deployment
			if parent := c.findReplicaSetParent(ctx, clusterID, namespace, name); parent != nil {
				return parent
			}
		case "StatefulSet", "DaemonSet", "Job":
			if parent, err := c.GetResource(ctx, clusterID, kind, namespace, name); err == nil {
				return parent
			}
		}
	}
	return nil
}

// findReplicaSetParent finds the parent workload of a ReplicaSet.
func (c *Client) findReplicaSetParent(ctx context.Context, clusterID, namespace, rsName string) *unstructured.Unstructured {
	rs, err := c.GetResource(ctx, clusterID, "replicaset", namespace, rsName)
	if err != nil {
		return nil
	}

	ownerRefs, found, _ := unstructured.NestedSlice(rs.Object, "metadata", "ownerReferences")
	if !found {
		return nil
	}

	for _, ref := range ownerRefs {
		ownerRef, ok := ref.(map[string]interface{})
		if !ok {
			continue
		}
		kind, _ := ownerRef["kind"].(string)
		name, _ := ownerRef["name"].(string)

		if kind == "Deployment" {
			if parent, err := c.GetResource(ctx, clusterID, kind, namespace, name); err == nil {
				return parent
			}
		}
	}
	return nil
}
