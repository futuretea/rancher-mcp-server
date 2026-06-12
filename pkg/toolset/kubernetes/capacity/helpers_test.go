package capacity

import (
	"github.com/futuretea/rancher-mcp-server/pkg/client/steve/fake"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func makeUnstructured(_, name, namespace string, content map[string]interface{}) unstructured.Unstructured {
	u := unstructured.Unstructured{}
	u.SetUnstructuredContent(content)
	u.SetName(name)
	u.SetNamespace(namespace)
	return u
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

func makeFakeClient() *fake.Client {
	c := fake.NewClient()
	addFakeNodes(c)
	addFakePods(c)
	return c
}

func addFakeNodes(c *fake.Client) {
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
}

func addFakePods(c *fake.Client) {
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
}
