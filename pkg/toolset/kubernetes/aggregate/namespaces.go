package aggregate

import (
	"context"

	"github.com/futuretea/rancher-mcp-server/pkg/client/steve"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func eachNamespace(single string, many []string) []string {
	if len(many) > 0 {
		return many
	}
	return []string{single}
}

func listKind(ctx context.Context, client steve.ResourceReader, cluster, kind, namespace string, many []string, opts *steve.ListOptions) (*unstructured.UnstructuredList, error) {
	merged := &unstructured.UnstructuredList{}
	for _, name := range eachNamespace(namespace, many) {
		list, err := client.ListResources(ctx, cluster, kind, name, opts)
		if err != nil {
			return nil, err
		}
		merged.Items = append(merged.Items, list.Items...)
	}
	return merged, nil
}

func getEventsInNamespaces(ctx context.Context, client steve.ResourceReader, cluster, namespace, name, kind string, many []string) ([]corev1.Event, error) {
	var events []corev1.Event
	for _, ns := range eachNamespace(namespace, many) {
		batch, err := client.GetEvents(ctx, cluster, ns, name, kind)
		if err != nil {
			return nil, err
		}
		events = append(events, batch...)
	}
	return events, nil
}
