package steve

import (
	"bytes"
	"context"
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// PodLogOptions contains options for fetching pod logs.
type PodLogOptions struct {
	Container    string
	TailLines    *int64
	SinceSeconds *int64
	Timestamps   bool
	Previous     bool
}

// GetPodLogs retrieves logs from a specific pod and container.
func (c *Client) GetPodLogs(ctx context.Context, clusterID, namespace, podName string, opts *PodLogOptions) (string, error) {
	clientset, err := c.getClientset(clusterID)
	if err != nil {
		return "", fmt.Errorf("failed to create clientset: %w", err)
	}

	podLogOpts := &corev1.PodLogOptions{}
	if opts != nil {
		if opts.Container != "" {
			podLogOpts.Container = opts.Container
		}
		if opts.TailLines != nil {
			podLogOpts.TailLines = opts.TailLines
		}
		if opts.SinceSeconds != nil {
			podLogOpts.SinceSeconds = opts.SinceSeconds
		}
		podLogOpts.Timestamps = opts.Timestamps
		podLogOpts.Previous = opts.Previous
	}

	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, podLogOpts)
	stream, err := req.Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to open log stream: %w", err)
	}
	defer func() { _ = stream.Close() }()

	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, stream); err != nil {
		return "", fmt.Errorf("failed to read log stream: %w", err)
	}

	return buf.String(), nil
}

// GetAllContainerLogs retrieves logs from all containers in a pod.
func (c *Client) GetAllContainerLogs(ctx context.Context, clusterID, namespace, podName string, opts *PodLogOptions) (map[string]string, error) {
	pod, err := c.GetResource(ctx, clusterID, "pod", namespace, podName)
	if err != nil {
		return nil, fmt.Errorf("failed to get pod: %w", err)
	}

	containers, found, err := unstructured.NestedSlice(pod.Object, "spec", "containers")
	if err != nil {
		return nil, fmt.Errorf("failed to get containers from pod spec: %w", err)
	}
	if !found {
		return nil, fmt.Errorf("containers not found in pod spec")
	}

	tailLines := int64(100)
	timestamps := false
	var sinceSeconds *int64
	previous := false
	if opts != nil {
		if opts.TailLines != nil {
			tailLines = *opts.TailLines
		}
		timestamps = opts.Timestamps
		sinceSeconds = opts.SinceSeconds
		previous = opts.Previous
	}

	logs := make(map[string]string)
	for _, ctr := range containers {
		container, ok := ctr.(map[string]interface{})
		if !ok {
			continue
		}
		name, ok := container["name"].(string)
		if !ok {
			continue
		}

		logOpts := &PodLogOptions{
			Container:    name,
			TailLines:    &tailLines,
			Timestamps:   timestamps,
			SinceSeconds: sinceSeconds,
			Previous:     previous,
		}
		containerLogs, err := c.GetPodLogs(ctx, clusterID, namespace, podName, logOpts)
		if err != nil {
			logs[name] = fmt.Sprintf("Error getting logs: %v", err)
		} else {
			logs[name] = containerLogs
		}
	}

	return logs, nil
}

// MultiPodLogResult contains the log result for a single pod.
type MultiPodLogResult struct {
	Pod       string            `json:"pod"`
	Namespace string            `json:"namespace,omitempty"`
	Logs      map[string]string `json:"logs,omitempty"`
	Error     string            `json:"error,omitempty"`
}

// GetMultiPodLogs retrieves logs from multiple pods using label selector and merges them.
// Returns logs organized by pod name, with each pod's logs organized by container name.
func (c *Client) GetMultiPodLogs(ctx context.Context, clusterID, namespace string, labelSelector string, opts *PodLogOptions) ([]MultiPodLogResult, error) {
	listOpts := &ListOptions{
		LabelSelector: labelSelector,
	}
	podList, err := c.ListResources(ctx, clusterID, "pod", namespace, listOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	if len(podList.Items) == 0 {
		return []MultiPodLogResult{}, nil
	}

	results := make([]MultiPodLogResult, 0, len(podList.Items))

	for _, pod := range podList.Items {
		podName := pod.GetName()
		podNamespace := pod.GetNamespace()

		result := MultiPodLogResult{
			Pod:       podName,
			Namespace: podNamespace,
			Logs:      make(map[string]string),
		}

		containerLogs, err := c.GetAllContainerLogs(ctx, clusterID, podNamespace, podName, opts)
		if err != nil {
			result.Error = err.Error()
		} else {
			result.Logs = containerLogs
		}

		results = append(results, result)
	}

	return results, nil
}
