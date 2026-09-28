package capacity

import (
	"context"
	"fmt"

	"github.com/futuretea/rancher-mcp-server/pkg/client/steve"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// processPodsInNamespaces lists pods in each allowlisted namespace. It does not
// list pods or namespaces with an empty namespace.
func (a *Analyzer) processPodsInNamespaces(ctx context.Context, nodeInfoMap map[string]*NodeInfo, p Params) error {
	targets, err := a.allowlistedNamespaceTargets(ctx, p)
	if err != nil {
		return err
	}

	podOpts := &steve.ListOptions{}
	if p.LabelSelector != "" {
		podOpts.LabelSelector = p.LabelSelector
	}
	for _, namespace := range targets {
		pods, err := a.client.ListResources(ctx, p.Cluster, "pod", namespace, podOpts)
		if err != nil {
			return fmt.Errorf("failed to list pods: %w", err)
		}
		for _, pod := range pods.Items {
			if !shouldProcessPod(pod, nodeInfoMap, nil) {
				continue
			}
			nodeName, _, _ := unstructured.NestedString(pod.Object, "spec", "nodeName")
			processSinglePod(pod, nodeInfoMap[nodeName], p.ShowPods, p.ShowContainers)
		}
	}
	return nil
}

func (a *Analyzer) allowlistedNamespaceTargets(ctx context.Context, p Params) ([]string, error) {
	if p.NamespaceLabelSelector == "" {
		return p.Namespaces, nil
	}
	selector := parseLabelSelector(p.NamespaceLabelSelector)
	if len(selector) == 0 {
		return p.Namespaces, nil
	}

	matched := make([]string, 0, len(p.Namespaces))
	for _, name := range p.Namespaces {
		namespace, err := a.client.GetResource(ctx, p.Cluster, "namespace", "", name)
		if apierrors.IsNotFound(err) {
			// A stale allowlist entry has no objects; skip it instead of
			// failing the whole query.
			continue
		}
		if err != nil {
			return nil, err
		}
		if matchLabels(namespace.GetLabels(), selector) {
			matched = append(matched, name)
		}
	}
	return matched, nil
}
