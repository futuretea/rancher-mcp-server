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

// matchPodsInNamespace returns Pod nodes in namespace whose labels match selector.
func matchPodsInNamespace(globalMapByUID map[types.UID]*Node, namespace string, selector labels.Selector) []*Node {
	var pods []*Node
	for _, node := range globalMapByUID {
		if node.Kind == "Pod" && node.Namespace == namespace && selector.Matches(labels.Set(node.GetLabels())) {
			pods = append(pods, node)
		}
	}
	return pods
}

// getPodRelationships extracts relationships for a Pod.
// Covers: Node, ServiceAccount, volumes (ConfigMap/Secret/PVC), env (ConfigMap/Secret), imagePullSecrets.
func getPodRelationships(n *Node) *RelationshipMap {
	var pod corev1.Pod
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(n.UnstructuredContent(), &pod); err != nil {
		return nil
	}

	result := NewRelationshipMap()
	addPodNodeRelationship(&pod, result)
	addPodServiceAccountRelationship(&pod, result)
	addPodImagePullSecretRelationships(&pod, result)
	addPodVolumeRelationships(&pod, result)
	addPodContainerEnvRelationships(&pod, result)
	return result
}

// addPodNodeRelationship records the Pod -> Node relationship.
func addPodNodeRelationship(pod *corev1.Pod, result *RelationshipMap) {
	if pod.Spec.NodeName == "" {
		return
	}
	ref := ObjectReference{Kind: "Node", Name: pod.Spec.NodeName}
	result.AddDependencyByKey(ref.Key(), RelationshipPodNode)
}

// addPodServiceAccountRelationship records the Pod -> ServiceAccount relationship.
func addPodServiceAccountRelationship(pod *corev1.Pod, result *RelationshipMap) {
	if pod.Spec.ServiceAccountName == "" {
		return
	}
	ref := ObjectReference{Kind: "ServiceAccount", Name: pod.Spec.ServiceAccountName, Namespace: pod.Namespace}
	result.AddDependencyByKey(ref.Key(), RelationshipPodServiceAccount)
}

// addPodImagePullSecretRelationships records Pod -> Secret relationships for image pull secrets.
func addPodImagePullSecretRelationships(pod *corev1.Pod, result *RelationshipMap) {
	ns := pod.Namespace
	for _, ips := range pod.Spec.ImagePullSecrets {
		ref := ObjectReference{Kind: "Secret", Name: ips.Name, Namespace: ns}
		result.AddDependencyByKey(ref.Key(), RelationshipPodImagePullSecret)
	}
}

// addPodVolumeRelationships records Pod -> Volume relationships (ConfigMap, Secret, PVC, Projected).
func addPodVolumeRelationships(pod *corev1.Pod, result *RelationshipMap) {
	ns := pod.Namespace
	for _, v := range pod.Spec.Volumes {
		addVolumeSourceRelationship(v.VolumeSource, ns, result)
	}
}

// addVolumeSourceRelationship records relationships for a single volume source.
func addVolumeSourceRelationship(vs corev1.VolumeSource, ns string, result *RelationshipMap) {
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
			addProjectedVolumeSourceRelationship(src, ns, result)
		}
	}
}

// addProjectedVolumeSourceRelationship records relationships for a projected volume source.
func addProjectedVolumeSourceRelationship(src corev1.VolumeProjection, ns string, result *RelationshipMap) {
	switch {
	case src.ConfigMap != nil:
		ref := ObjectReference{Kind: "ConfigMap", Name: src.ConfigMap.Name, Namespace: ns}
		result.AddDependencyByKey(ref.Key(), RelationshipPodVolume)
	case src.Secret != nil:
		ref := ObjectReference{Kind: "Secret", Name: src.Secret.Name, Namespace: ns}
		result.AddDependencyByKey(ref.Key(), RelationshipPodVolume)
	}
}

// addPodContainerEnvRelationships records Container -> ConfigMap/Secret relationships from env.
func addPodContainerEnvRelationships(pod *corev1.Pod, result *RelationshipMap) {
	ns := pod.Namespace
	containers := make([]corev1.Container, 0, len(pod.Spec.InitContainers)+len(pod.Spec.Containers))
	containers = append(containers, pod.Spec.InitContainers...)
	containers = append(containers, pod.Spec.Containers...)

	for _, c := range containers {
		for _, env := range c.EnvFrom {
			addEnvFromRelationship(env, ns, result)
		}
		for _, env := range c.Env {
			addEnvValueRelationship(env, ns, result)
		}
	}
}

// addEnvFromRelationship records relationships from an EnvFromSource.
func addEnvFromRelationship(env corev1.EnvFromSource, ns string, result *RelationshipMap) {
	switch {
	case env.ConfigMapRef != nil:
		ref := ObjectReference{Kind: "ConfigMap", Name: env.ConfigMapRef.Name, Namespace: ns}
		result.AddDependencyByKey(ref.Key(), RelationshipPodContainerEnv)
	case env.SecretRef != nil:
		ref := ObjectReference{Kind: "Secret", Name: env.SecretRef.Name, Namespace: ns}
		result.AddDependencyByKey(ref.Key(), RelationshipPodContainerEnv)
	}
}

// addEnvValueRelationship records relationships from an EnvVar valueFrom.
func addEnvValueRelationship(env corev1.EnvVar, ns string, result *RelationshipMap) {
	if env.ValueFrom == nil {
		return
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

	for _, pod := range matchPodsInNamespace(globalMapByUID, ns, selector) {
		result.AddDependencyByKey(pod.GetObjectReferenceKey(), RelationshipService)
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
	// A missing subject namespace defaults to the RoleBinding's own namespace.
	for _, s := range rb.Subjects {
		if s.Kind == rbacv1.ServiceAccountKind && s.APIGroup == corev1.GroupName {
			saNS := s.Namespace
			if saNS == "" {
				saNS = ns
			}
			ref := ObjectReference{Kind: "ServiceAccount", Namespace: saNS, Name: s.Name}
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

	for _, pod := range matchPodsInNamespace(globalMapByUID, ns, selector) {
		result.AddDependencyByKey(pod.GetObjectReferenceKey(), RelationshipPodDisruptionBudget)
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

// extractorKey identifies the GroupVersionKind used to dispatch relationship extractors.
type extractorKey struct {
	group string
	kind  string
}

// relationshipExtractors maps a GVK to its extractor. Extractors that need the
// global UID map receive it; others ignore it.
var relationshipExtractors = map[extractorKey]func(*Node, map[types.UID]*Node) *RelationshipMap{
	{"", "Pod"}:                                         func(n *Node, _ map[types.UID]*Node) *RelationshipMap { return getPodRelationships(n) },
	{"", "Service"}:                                     func(n *Node, byUID map[types.UID]*Node) *RelationshipMap { return getServiceRelationships(n, byUID) },
	{"networking.k8s.io", "Ingress"}:                    func(n *Node, _ map[types.UID]*Node) *RelationshipMap { return getIngressRelationships(n) },
	{"networking.k8s.io", "IngressClass"}:               func(n *Node, _ map[types.UID]*Node) *RelationshipMap { return getIngressClassRelationships(n) },
	{"", "PersistentVolume"}:                            func(n *Node, _ map[types.UID]*Node) *RelationshipMap { return getPVRelationships(n) },
	{"", "PersistentVolumeClaim"}:                       func(n *Node, _ map[types.UID]*Node) *RelationshipMap { return getPVCRelationships(n) },
	{"rbac.authorization.k8s.io", "RoleBinding"}:        func(n *Node, _ map[types.UID]*Node) *RelationshipMap { return getRoleBindingRelationships(n) },
	{"rbac.authorization.k8s.io", "ClusterRoleBinding"}: func(n *Node, _ map[types.UID]*Node) *RelationshipMap { return getClusterRoleBindingRelationships(n) },
	{"policy", "PodDisruptionBudget"}:                   func(n *Node, byUID map[types.UID]*Node) *RelationshipMap { return getPDBRelationships(n, byUID) },
	{"", "Event"}:                                       func(n *Node, _ map[types.UID]*Node) *RelationshipMap { return getEventRelationships(n) },
	{"events.k8s.io", "Event"}:                          func(n *Node, _ map[types.UID]*Node) *RelationshipMap { return getEventRelationships(n) },
}

// extractRelationships determines the appropriate relationship extractor for a node
// and returns the resulting RelationshipMap.
func extractRelationships(node *Node, globalMapByUID map[types.UID]*Node) *RelationshipMap {
	gvk := node.GroupVersionKind()
	extractor, ok := relationshipExtractors[extractorKey{group: gvk.Group, kind: gvk.Kind}]
	if !ok {
		return nil
	}
	return extractor(node, globalMapByUID)
}
