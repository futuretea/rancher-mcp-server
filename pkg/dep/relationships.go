package dep

import (
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unstructuredv1 "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

// getNestedString extracts a nested string field from unstructured content.
func getNestedString(obj map[string]interface{}, fields ...string) string {
	val, found, err := unstructuredv1.NestedString(obj, fields...)
	if !found || err != nil {
		return ""
	}
	return val
}

// getPodRelationships extracts relationships for a Pod.
// Covers: Node, ServiceAccount, volumes (ConfigMap/Secret/PVC), env (ConfigMap/Secret), imagePullSecrets.
func getPodRelationships(n *Node) *RelationshipMap {
	var pod corev1.Pod
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(n.UnstructuredContent(), &pod); err != nil {
		return nil
	}

	ns := pod.Namespace
	result := NewRelationshipMap()

	// Pod -> Node
	if nodeName := pod.Spec.NodeName; nodeName != "" {
		ref := ObjectReference{Kind: "Node", Name: nodeName}
		result.AddDependencyByKey(ref.Key(), RelationshipPodNode)
	}

	// Pod -> ServiceAccount
	if sa := pod.Spec.ServiceAccountName; sa != "" {
		ref := ObjectReference{Kind: "ServiceAccount", Name: sa, Namespace: ns}
		result.AddDependencyByKey(ref.Key(), RelationshipPodServiceAccount)
	}

	// Pod -> ImagePullSecrets
	for _, ips := range pod.Spec.ImagePullSecrets {
		ref := ObjectReference{Kind: "Secret", Name: ips.Name, Namespace: ns}
		result.AddDependencyByKey(ref.Key(), RelationshipPodImagePullSecret)
	}

	// Pod -> Volumes (ConfigMap, Secret, PVC)
	for _, v := range pod.Spec.Volumes {
		vs := v.VolumeSource
		switch {
		case vs.ConfigMap != nil:
			ref := ObjectReference{Kind: "ConfigMap", Name: vs.ConfigMap.Name, Namespace: ns}
			result.AddDependencyByKey(ref.Key(), RelationshipPodVolume)
		case vs.Secret != nil:
			ref := ObjectReference{Kind: "Secret", Name: vs.Secret.SecretName, Namespace: ns}
			result.AddDependencyByKey(ref.Key(), RelationshipPodVolume)
		case vs.PersistentVolumeClaim != nil:
			ref := ObjectReference{Kind: "PersistentVolumeClaim", Name: vs.PersistentVolumeClaim.ClaimName, Namespace: ns}
			result.AddDependencyByKey(ref.Key(), RelationshipPodVolume)
		case vs.Projected != nil:
			for _, src := range vs.Projected.Sources {
				switch {
				case src.ConfigMap != nil:
					ref := ObjectReference{Kind: "ConfigMap", Name: src.ConfigMap.Name, Namespace: ns}
					result.AddDependencyByKey(ref.Key(), RelationshipPodVolume)
				case src.Secret != nil:
					ref := ObjectReference{Kind: "Secret", Name: src.Secret.Name, Namespace: ns}
					result.AddDependencyByKey(ref.Key(), RelationshipPodVolume)
				}
			}
		}
	}

	// Pod -> Container Environment (ConfigMap/Secret refs)
	var cList []corev1.Container
	cList = append(cList, pod.Spec.InitContainers...)
	cList = append(cList, pod.Spec.Containers...)
	for _, c := range cList {
		for _, env := range c.EnvFrom {
			switch {
			case env.ConfigMapRef != nil:
				ref := ObjectReference{Kind: "ConfigMap", Name: env.ConfigMapRef.Name, Namespace: ns}
				result.AddDependencyByKey(ref.Key(), RelationshipPodContainerEnv)
			case env.SecretRef != nil:
				ref := ObjectReference{Kind: "Secret", Name: env.SecretRef.Name, Namespace: ns}
				result.AddDependencyByKey(ref.Key(), RelationshipPodContainerEnv)
			}
		}
		for _, env := range c.Env {
			if env.ValueFrom == nil {
				continue
			}
			switch {
			case env.ValueFrom.ConfigMapKeyRef != nil:
				ref := ObjectReference{Kind: "ConfigMap", Name: env.ValueFrom.ConfigMapKeyRef.Name, Namespace: ns}
				result.AddDependencyByKey(ref.Key(), RelationshipPodContainerEnv)
			case env.ValueFrom.SecretKeyRef != nil:
				ref := ObjectReference{Kind: "Secret", Name: env.ValueFrom.SecretKeyRef.Name, Namespace: ns}
				result.AddDependencyByKey(ref.Key(), RelationshipPodContainerEnv)
			}
		}
	}

	return result
}

// getServiceRelationships extracts relationships for a Service.
// Covers: Pod (via label selector).
func getServiceRelationships(n *Node, globalMapByUID map[types.UID]*Node) *RelationshipMap {
	var svc corev1.Service
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(n.UnstructuredContent(), &svc); err != nil {
		return nil
	}

	if len(svc.Spec.Selector) == 0 {
		return nil
	}

	ns := svc.Namespace
	result := NewRelationshipMap()

	selector, err := labels.ValidatedSelectorFromSet(labels.Set(svc.Spec.Selector))
	if err != nil {
		return nil
	}

	// Find matching pods
	for _, node := range globalMapByUID {
		if node.Kind == "Pod" && node.Namespace == ns {
			if selector.Matches(labels.Set(node.GetLabels())) {
				result.AddDependencyByKey(node.GetObjectReferenceKey(), RelationshipService)
			}
		}
	}

	return result
}

// getIngressRelationships extracts relationships for an Ingress.
// Covers: IngressClass, Service (backend), Secret (TLS).
func getIngressRelationships(n *Node) *RelationshipMap {
	var ing networkingv1.Ingress
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(n.UnstructuredContent(), &ing); err != nil {
		return nil
	}

	ns := ing.Namespace
	result := NewRelationshipMap()

	// Ingress -> IngressClass
	if ingc := ing.Spec.IngressClassName; ingc != nil && *ingc != "" {
		ref := ObjectReference{Group: "networking.k8s.io", Kind: "IngressClass", Name: *ingc}
		result.AddDependencyByKey(ref.Key(), RelationshipIngressClass)
	}

	// Ingress -> Services (backends)
	var backends []networkingv1.IngressBackend
	if ing.Spec.DefaultBackend != nil {
		backends = append(backends, *ing.Spec.DefaultBackend)
	}
	for _, rule := range ing.Spec.Rules {
		if rule.HTTP != nil {
			for _, path := range rule.HTTP.Paths {
				backends = append(backends, path.Backend)
			}
		}
	}
	for _, b := range backends {
		if b.Service != nil {
			ref := ObjectReference{Kind: "Service", Name: b.Service.Name, Namespace: ns}
			result.AddDependencyByKey(ref.Key(), RelationshipIngressService)
		}
	}

	// Ingress -> TLS Secrets
	for _, tls := range ing.Spec.TLS {
		if tls.SecretName != "" {
			ref := ObjectReference{Kind: "Secret", Name: tls.SecretName, Namespace: ns}
			result.AddDependencyByKey(ref.Key(), RelationshipIngressTLSSecret)
		}
	}

	return result
}

// getIngressClassRelationships extracts relationships for an IngressClass.
// Covers: Parameters reference.
func getIngressClassRelationships(n *Node) *RelationshipMap {
	var ingc networkingv1.IngressClass
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(n.UnstructuredContent(), &ingc); err != nil {
		return nil
	}

	result := NewRelationshipMap()

	if p := ingc.Spec.Parameters; p != nil {
		group := ""
		if p.APIGroup != nil {
			group = *p.APIGroup
		}
		ns := ""
		if p.Namespace != nil {
			ns = *p.Namespace
		}
		ref := ObjectReference{Group: group, Kind: p.Kind, Namespace: ns, Name: p.Name}
		result.AddDependencyByKey(ref.Key(), RelationshipIngressClassParameters)
	}

	return result
}

// getPVRelationships extracts relationships for a PersistentVolume.
// Covers: PVC (claim ref), StorageClass.
func getPVRelationships(n *Node) *RelationshipMap {
	var pv corev1.PersistentVolume
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(n.UnstructuredContent(), &pv); err != nil {
		return nil
	}

	result := NewRelationshipMap()

	// PV -> PVC
	if pvcRef := pv.Spec.ClaimRef; pvcRef != nil {
		ref := ObjectReference{Kind: "PersistentVolumeClaim", Name: pvcRef.Name, Namespace: pvcRef.Namespace}
		result.AddDependencyByKey(ref.Key(), RelationshipPersistentVolumeClaim)
	}

	// PV -> StorageClass
	if sc := pv.Spec.StorageClassName; sc != "" {
		ref := ObjectReference{Group: "storage.k8s.io", Kind: "StorageClass", Name: sc}
		result.AddDependencyByKey(ref.Key(), RelationshipPersistentVolumeStorageClass)
	}

	return result
}

// getPVCRelationships extracts relationships for a PersistentVolumeClaim.
// Covers: PV (volume name).
func getPVCRelationships(n *Node) *RelationshipMap {
	var pvc corev1.PersistentVolumeClaim
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(n.UnstructuredContent(), &pvc); err != nil {
		return nil
	}

	result := NewRelationshipMap()

	if pvName := pvc.Spec.VolumeName; pvName != "" {
		ref := ObjectReference{Kind: "PersistentVolume", Name: pvName}
		result.AddDependencyByKey(ref.Key(), RelationshipPersistentVolumeClaim)
	}

	return result
}

// getRoleBindingRelationships extracts relationships for a RoleBinding.
// Covers: Role/ClusterRole (roleRef), ServiceAccount (subjects).
func getRoleBindingRelationships(n *Node) *RelationshipMap {
	var rb rbacv1.RoleBinding
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(n.UnstructuredContent(), &rb); err != nil {
		return nil
	}

	ns := rb.Namespace
	result := NewRelationshipMap()

	// RoleBinding -> Role/ClusterRole
	r := rb.RoleRef
	if r.APIGroup == rbacv1.GroupName {
		switch r.Kind {
		case "ClusterRole":
			ref := ObjectReference{Group: rbacv1.GroupName, Kind: "ClusterRole", Name: r.Name}
			result.AddDependencyByKey(ref.Key(), RelationshipRoleBindingRole)
		case "Role":
			ref := ObjectReference{Group: rbacv1.GroupName, Kind: "Role", Namespace: ns, Name: r.Name}
			result.AddDependencyByKey(ref.Key(), RelationshipRoleBindingRole)
		}
	}

	// RoleBinding -> Subjects (ServiceAccounts)
	for _, s := range rb.Subjects {
		if s.Kind == rbacv1.ServiceAccountKind && s.APIGroup == corev1.GroupName && s.Namespace != "" {
			ref := ObjectReference{Kind: "ServiceAccount", Namespace: s.Namespace, Name: s.Name}
			result.AddDependentByKey(ref.Key(), RelationshipRoleBindingSubject)
		}
	}

	return result
}

// getClusterRoleBindingRelationships extracts relationships for a ClusterRoleBinding.
// Covers: ClusterRole (roleRef), ServiceAccount (subjects).
func getClusterRoleBindingRelationships(n *Node) *RelationshipMap {
	var crb rbacv1.ClusterRoleBinding
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(n.UnstructuredContent(), &crb); err != nil {
		return nil
	}

	result := NewRelationshipMap()

	// ClusterRoleBinding -> ClusterRole
	r := crb.RoleRef
	if r.APIGroup == rbacv1.GroupName && r.Kind == "ClusterRole" {
		ref := ObjectReference{Group: rbacv1.GroupName, Kind: "ClusterRole", Name: r.Name}
		result.AddDependencyByKey(ref.Key(), RelationshipClusterRoleBindingRole)
	}

	// ClusterRoleBinding -> Subjects (ServiceAccounts)
	for _, s := range crb.Subjects {
		if s.Kind == rbacv1.ServiceAccountKind && s.APIGroup == corev1.GroupName && s.Namespace != "" {
			ref := ObjectReference{Kind: "ServiceAccount", Namespace: s.Namespace, Name: s.Name}
			result.AddDependentByKey(ref.Key(), RelationshipClusterRoleBindingSubject)
		}
	}

	return result
}

// getPDBRelationships extracts relationships for a PodDisruptionBudget.
// Covers: Pod (via label selector).
func getPDBRelationships(n *Node, globalMapByUID map[types.UID]*Node) *RelationshipMap {
	var pdb policyv1.PodDisruptionBudget
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(n.UnstructuredContent(), &pdb); err != nil {
		return nil
	}

	if pdb.Spec.Selector == nil {
		return nil
	}

	ns := pdb.Namespace
	result := NewRelationshipMap()

	selector, err := metav1.LabelSelectorAsSelector(pdb.Spec.Selector)
	if err != nil {
		return nil
	}

	for _, node := range globalMapByUID {
		if node.Kind == "Pod" && node.Namespace == ns {
			if selector.Matches(labels.Set(node.GetLabels())) {
				result.AddDependencyByKey(node.GetObjectReferenceKey(), RelationshipPodDisruptionBudget)
			}
		}
	}

	return result
}

// getEventRelationships extracts relationships for an Event.
// Covers: involved object (via UID).
func getEventRelationships(n *Node) *RelationshipMap {
	result := NewRelationshipMap()

	content := n.UnstructuredContent()
	gvk := n.GroupVersionKind()

	switch {
	case gvk.Group == "" && gvk.Kind == "Event":
		// core/v1 Event
		regUID := types.UID(getNestedString(content, "involvedObject", "uid"))
		if regUID != "" {
			result.AddDependencyByUID(regUID, RelationshipEventRegarding)
		}
	case gvk.Group == "events.k8s.io" && gvk.Kind == "Event":
		// events.k8s.io/v1 Event
		regUID := types.UID(getNestedString(content, "regarding", "uid"))
		if regUID != "" {
			result.AddDependencyByUID(regUID, RelationshipEventRegarding)
		}
	}

	return result
}

// extractRelationships determines the appropriate relationship extractor for a node
// and returns the resulting RelationshipMap.
func extractRelationships(node *Node, globalMapByUID map[types.UID]*Node) *RelationshipMap {
	gvk := node.GroupVersionKind()
	group := gvk.Group
	kind := gvk.Kind

	switch {
	case group == "" && kind == "Pod":
		return getPodRelationships(node)
	case group == "" && kind == "Service":
		return getServiceRelationships(node, globalMapByUID)
	case group == "networking.k8s.io" && kind == "Ingress":
		return getIngressRelationships(node)
	case group == "networking.k8s.io" && kind == "IngressClass":
		return getIngressClassRelationships(node)
	case group == "" && kind == "PersistentVolume":
		return getPVRelationships(node)
	case group == "" && kind == "PersistentVolumeClaim":
		return getPVCRelationships(node)
	case group == "rbac.authorization.k8s.io" && kind == "RoleBinding":
		return getRoleBindingRelationships(node)
	case group == "rbac.authorization.k8s.io" && kind == "ClusterRoleBinding":
		return getClusterRoleBindingRelationships(node)
	case group == "policy" && kind == "PodDisruptionBudget":
		return getPDBRelationships(node, globalMapByUID)
	case (group == "" || group == "events.k8s.io") && kind == "Event":
		return getEventRelationships(node)
	default:
		return nil
	}
}
