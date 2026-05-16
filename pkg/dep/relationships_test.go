package dep

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func TestGetNestedString(t *testing.T) {
	obj := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name": "test-pod",
		},
		"spec": map[string]interface{}{
			"nodeName": "node-1",
		},
	}

	t.Run("found", func(t *testing.T) {
		got := getNestedString(obj, "spec", "nodeName")
		if got != "node-1" {
			t.Fatalf("expected 'node-1', got %q", got)
		}
	})

	t.Run("not found", func(t *testing.T) {
		got := getNestedString(obj, "status", "phase")
		if got != "" {
			t.Fatalf("expected empty, got %q", got)
		}
	})

	t.Run("not a string", func(t *testing.T) {
		obj := map[string]interface{}{"count": 42}
		got := getNestedString(obj, "count")
		if got != "" {
			t.Fatalf("expected empty for non-string, got %q", got)
		}
	})
}

func makeFakeNode(kind, namespace, name string, content map[string]interface{}) *Node {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    kind,
	})
	u.SetName(name)
	u.SetNamespace(namespace)
	u.SetUnstructuredContent(content)
	return &Node{
		Unstructured: u,
		UID:          types.UID(namespace + "/" + kind + "/" + name),
		Kind:         kind,
		Namespace:    namespace,
		Name:         name,
		Dependencies: map[types.UID]RelationshipSet{},
		Dependents:   map[types.UID]RelationshipSet{},
	}
}

func TestGetPodRelationships(t *testing.T) {
	content := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      "test-pod",
			"namespace": "default",
		},
		"spec": map[string]interface{}{
			"nodeName":           "node-1",
			"serviceAccountName": "default-sa",
			"imagePullSecrets": []interface{}{
				map[string]interface{}{"name": "regcred"},
			},
			"volumes": []interface{}{
				map[string]interface{}{
					"name": "config-vol",
					"configMap": map[string]interface{}{
						"name": "app-config",
					},
				},
			},
			"containers": []interface{}{
				map[string]interface{}{
					"name": "app",
					"env": []interface{}{
						map[string]interface{}{
							"name": "DB_PASSWORD",
							"valueFrom": map[string]interface{}{
								"secretKeyRef": map[string]interface{}{
									"name": "db-secret",
									"key":  "password",
								},
							},
						},
					},
				},
			},
		},
	}

	node := makeFakeNode("Pod", "default", "test-pod", content)
	result := getPodRelationships(node)
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Check node dependency
	nodeKey := ObjectReferenceKey("\\Node\\\\node-1")
	if _, ok := result.DependenciesByRef[nodeKey]; !ok {
		t.Errorf("expected dependency on Node")
	}

	// Check ServiceAccount dependency
	saKey := ObjectReferenceKey("\\ServiceAccount\\default\\default-sa")
	if _, ok := result.DependenciesByRef[saKey]; !ok {
		t.Errorf("expected dependency on ServiceAccount")
	}

	// Check image pull secret
	secretKey := ObjectReferenceKey("\\Secret\\default\\regcred")
	if _, ok := result.DependenciesByRef[secretKey]; !ok {
		t.Errorf("expected dependency on imagePullSecret regcred")
	}

	// Check ConfigMap volume
	cmKey := ObjectReferenceKey("\\ConfigMap\\default\\app-config")
	if _, ok := result.DependenciesByRef[cmKey]; !ok {
		t.Errorf("expected dependency on ConfigMap app-config")
	}

	// Check env secret
	envSecretKey := ObjectReferenceKey("\\Secret\\default\\db-secret")
	if _, ok := result.DependenciesByRef[envSecretKey]; !ok {
		t.Errorf("expected dependency on Secret db-secret from env")
	}
}

func TestGetServiceRelationships(t *testing.T) {
	svcContent := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      "my-svc",
			"namespace": "default",
		},
		"spec": map[string]interface{}{
			"selector": map[string]interface{}{
				"app": "nginx",
			},
		},
	}
	svcNode := makeFakeNode("Service", "default", "my-svc", svcContent)

	podContent := map[string]interface{}{}
	podNode := makeFakeNode("Pod", "default", "nginx-pod", podContent)
	podNode.SetLabels(map[string]string{"app": "nginx"})

	globalMap := map[types.UID]*Node{
		podNode.UID: podNode,
	}

	result := getServiceRelationships(svcNode, globalMap)
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	podKey := podNode.GetObjectReferenceKey()
	if _, ok := result.DependenciesByRef[podKey]; !ok {
		t.Errorf("expected Service -> Pod dependency, got deps: %v", result.DependenciesByRef)
	}
}

func TestGetServiceRelationships_EmptySelector(t *testing.T) {
	svcContent := map[string]interface{}{
		"spec": map[string]interface{}{},
	}
	svcNode := makeFakeNode("Service", "default", "empty-svc", svcContent)
	result := getServiceRelationships(svcNode, nil)
	if result != nil {
		t.Fatal("expected nil result for empty selector")
	}
}

func TestGetIngressRelationships(t *testing.T) {
	content := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      "my-ingress",
			"namespace": "default",
		},
		"spec": map[string]interface{}{
			"ingressClassName": "nginx",
			"rules": []interface{}{
				map[string]interface{}{
					"http": map[string]interface{}{
						"paths": []interface{}{
							map[string]interface{}{
								"backend": map[string]interface{}{
									"service": map[string]interface{}{
										"name": "api-svc",
									},
								},
							},
						},
					},
				},
			},
			"tls": []interface{}{
				map[string]interface{}{
					"secretName": "tls-secret",
				},
			},
		},
	}

	node := makeFakeNode("Ingress", "default", "my-ingress", content)
	// Override GVK for networking.k8s.io
	node.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "networking.k8s.io",
		Version: "v1",
		Kind:    "Ingress",
	})

	result := getIngressRelationships(node)
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	icKey := ObjectReferenceKey("networking.k8s.io\\IngressClass\\\\nginx")
	if _, ok := result.DependenciesByRef[icKey]; !ok {
		t.Errorf("expected IngressClass dependency, got: %v", result.DependenciesByRef)
	}

	svcKey := ObjectReferenceKey("\\Service\\default\\api-svc")
	if _, ok := result.DependenciesByRef[svcKey]; !ok {
		t.Errorf("expected Service dependency, got: %v", result.DependenciesByRef)
	}

	tlsKey := ObjectReferenceKey("\\Secret\\default\\tls-secret")
	if _, ok := result.DependenciesByRef[tlsKey]; !ok {
		t.Errorf("expected TLS Secret dependency, got: %v", result.DependenciesByRef)
	}
}

func TestGetIngressClassRelationships(t *testing.T) {
	content := map[string]interface{}{
		"spec": map[string]interface{}{
			"parameters": map[string]interface{}{
				"apiGroup":  "apps",
				"kind":      "Deployment",
				"namespace": "default",
				"name":      "nginx-controller",
			},
		},
	}
	node := makeFakeNode("IngressClass", "", "nginx", content)

	result := getIngressClassRelationships(node)
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	paramKey := ObjectReferenceKey("apps\\Deployment\\default\\nginx-controller")
	if _, ok := result.DependenciesByRef[paramKey]; !ok {
		t.Errorf("expected Parameters dependency, got: %v", result.DependenciesByRef)
	}
}

func TestGetPVRelationships(t *testing.T) {
	content := map[string]interface{}{
		"spec": map[string]interface{}{
			"claimRef": map[string]interface{}{
				"namespace": "default",
				"name":      "my-pvc",
			},
			"storageClassName": "fast",
		},
	}
	node := makeFakeNode("PersistentVolume", "", "pv-1", content)

	result := getPVRelationships(node)
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	pvcKey := ObjectReferenceKey("\\PersistentVolumeClaim\\default\\my-pvc")
	if _, ok := result.DependenciesByRef[pvcKey]; !ok {
		t.Errorf("expected PVC dependency, got: %v", result.DependenciesByRef)
	}

	scKey := ObjectReferenceKey("storage.k8s.io\\StorageClass\\\\fast")
	if _, ok := result.DependenciesByRef[scKey]; !ok {
		t.Errorf("expected StorageClass dependency, got: %v", result.DependenciesByRef)
	}
}

func TestGetPVCRelationships(t *testing.T) {
	content := map[string]interface{}{
		"spec": map[string]interface{}{
			"volumeName": "pv-abc",
		},
	}
	node := makeFakeNode("PersistentVolumeClaim", "default", "my-pvc", content)

	result := getPVCRelationships(node)
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	pvKey := ObjectReferenceKey("\\PersistentVolume\\\\pv-abc")
	if _, ok := result.DependenciesByRef[pvKey]; !ok {
		t.Errorf("expected PV dependency, got: %v", result.DependenciesByRef)
	}
}

func TestGetRoleBindingRelationships(t *testing.T) {
	content := map[string]interface{}{
		"metadata": map[string]interface{}{
			"namespace": "default",
		},
		"roleRef": map[string]interface{}{
			"apiGroup": "rbac.authorization.k8s.io",
			"kind":     "ClusterRole",
			"name":     "view",
		},
		"subjects": []interface{}{
			map[string]interface{}{
				"kind":      "ServiceAccount",
				"apiGroup":  "",
				"namespace": "default",
				"name":      "my-sa",
			},
		},
	}
	node := makeFakeNode("RoleBinding", "default", "rb-view", content)
	node.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "rbac.authorization.k8s.io",
		Version: "v1",
		Kind:    "RoleBinding",
	})

	result := getRoleBindingRelationships(node)
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	crKey := ObjectReferenceKey("rbac.authorization.k8s.io\\ClusterRole\\\\view")
	if _, ok := result.DependenciesByRef[crKey]; !ok {
		t.Errorf("expected ClusterRole dependency, got: %v", result.DependenciesByRef)
	}

	saKey := ObjectReferenceKey("\\ServiceAccount\\default\\my-sa")
	if _, ok := result.DependentsByRef[saKey]; !ok {
		t.Errorf("expected ServiceAccount dependent, got: %v", result.DependentsByRef)
	}
}

func TestGetClusterRoleBindingRelationships(t *testing.T) {
	content := map[string]interface{}{
		"roleRef": map[string]interface{}{
			"apiGroup": "rbac.authorization.k8s.io",
			"kind":     "ClusterRole",
			"name":     "admin",
		},
		"subjects": []interface{}{
			map[string]interface{}{
				"kind":      "ServiceAccount",
				"apiGroup":  "",
				"namespace": "kube-system",
				"name":      "admin-sa",
			},
		},
	}
	node := makeFakeNode("ClusterRoleBinding", "", "crb-admin", content)
	node.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "rbac.authorization.k8s.io",
		Version: "v1",
		Kind:    "ClusterRoleBinding",
	})

	result := getClusterRoleBindingRelationships(node)
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	crKey := ObjectReferenceKey("rbac.authorization.k8s.io\\ClusterRole\\\\admin")
	if _, ok := result.DependenciesByRef[crKey]; !ok {
		t.Errorf("expected ClusterRole dependency, got: %v", result.DependenciesByRef)
	}

	saKey := ObjectReferenceKey("\\ServiceAccount\\kube-system\\admin-sa")
	if _, ok := result.DependentsByRef[saKey]; !ok {
		t.Errorf("expected ServiceAccount dependent, got: %v", result.DependentsByRef)
	}
}

func TestGetPDBRelationships(t *testing.T) {
	content := map[string]interface{}{
		"metadata": map[string]interface{}{
			"namespace": "default",
		},
		"spec": map[string]interface{}{
			"selector": map[string]interface{}{
				"matchLabels": map[string]interface{}{
					"app": "nginx",
				},
			},
		},
	}
	node := makeFakeNode("PodDisruptionBudget", "default", "nginx-pdb", content)
	node.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "policy",
		Version: "v1",
		Kind:    "PodDisruptionBudget",
	})

	podNode := makeFakeNode("Pod", "default", "nginx-pod", map[string]interface{}{})
	podNode.SetLabels(map[string]string{"app": "nginx"})

	globalMap := map[types.UID]*Node{podNode.UID: podNode}

	result := getPDBRelationships(node, globalMap)
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	podKey := podNode.GetObjectReferenceKey()
	if _, ok := result.DependenciesByRef[podKey]; !ok {
		t.Errorf("expected PDB -> Pod dependency, got: %v", result.DependenciesByRef)
	}
}

func TestGetEventRelationships(t *testing.T) {
	t.Run("core event", func(t *testing.T) {
		content := map[string]interface{}{
			"involvedObject": map[string]interface{}{
				"uid": "pod-uid-123",
			},
		}
		node := makeFakeNode("Event", "default", "evt-1", content)
		// core/v1 Event: group="" kind="Event"
		node.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "",
			Version: "v1",
			Kind:    "Event",
		})

		result := getEventRelationships(node)
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if _, ok := result.DependenciesByUID[types.UID("pod-uid-123")]; !ok {
			t.Errorf("expected EventRegarding dependency, got: %v", result.DependenciesByUID)
		}
	})

	t.Run("events.k8s.io event", func(t *testing.T) {
		content := map[string]interface{}{
			"regarding": map[string]interface{}{
				"uid": "deploy-uid-456",
			},
		}
		node := makeFakeNode("Event", "default", "evt-2", content)
		node.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "events.k8s.io",
			Version: "v1",
			Kind:    "Event",
		})

		result := getEventRelationships(node)
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if _, ok := result.DependenciesByUID[types.UID("deploy-uid-456")]; !ok {
			t.Errorf("expected EventRegarding dependency, got: %v", result.DependenciesByUID)
		}
	})
}

func TestExtractRelationships(t *testing.T) {
	t.Run("unknown kind returns nil", func(t *testing.T) {
		node := makeFakeNode("UnknownKind", "default", "test", map[string]interface{}{})
		node.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "example.com",
			Version: "v1",
			Kind:    "UnknownKind",
		})
		result := extractRelationships(node, nil)
		if result != nil {
			t.Fatal("expected nil for unknown kind")
		}
	})

	t.Run("pod dispatches to getPodRelationships", func(t *testing.T) {
		content := map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":      "test-pod",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"nodeName":           "node-1",
				"serviceAccountName": "default-sa",
				"containers": []interface{}{
					map[string]interface{}{
						"name":  "app",
						"image": "nginx",
					},
				},
			},
		}
		node := makeFakeNode("Pod", "default", "test-pod", content)
		result := extractRelationships(node, nil)
		if result == nil {
			t.Fatal("expected non-nil result for Pod")
		}
		nodeKey := ObjectReferenceKey("\\Node\\\\node-1")
		if _, ok := result.DependenciesByRef[nodeKey]; !ok {
			t.Errorf("expected Pod->Node dependency via dispatch")
		}
	})
}
