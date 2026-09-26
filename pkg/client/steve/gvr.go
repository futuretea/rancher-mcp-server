// Package steve provides a Kubernetes dynamic client for accessing clusters via Rancher's Steve API.
package steve

import (
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// K8sKindsToGVRs maps lowercase Kubernetes resource kind names to their corresponding
// GroupVersionResource (GVR) identifiers. This mapping is used for dynamic client operations
// to resolve resource types across different API groups and versions.
var K8sKindsToGVRs = map[string]schema.GroupVersionResource{
	// --- CORE Kubernetes Resources (Group: "") ---
	"pod":                   {Group: "", Version: "v1", Resource: "pods"},
	"service":               {Group: "", Version: "v1", Resource: "services"},
	"svc":                   {Group: "", Version: "v1", Resource: "services"},
	"configmap":             {Group: "", Version: "v1", Resource: "configmaps"},
	"cm":                    {Group: "", Version: "v1", Resource: "configmaps"},
	"secret":                {Group: "", Version: "v1", Resource: "secrets"},
	"event":                 {Group: "", Version: "v1", Resource: "events"},
	"ev":                    {Group: "", Version: "v1", Resource: "events"},
	"namespace":             {Group: "", Version: "v1", Resource: "namespaces"},
	"ns":                    {Group: "", Version: "v1", Resource: "namespaces"},
	"node":                  {Group: "", Version: "v1", Resource: "nodes"},
	"no":                    {Group: "", Version: "v1", Resource: "nodes"},
	"serviceaccount":        {Group: "", Version: "v1", Resource: "serviceaccounts"},
	"sa":                    {Group: "", Version: "v1", Resource: "serviceaccounts"},
	"persistentvolume":      {Group: "", Version: "v1", Resource: "persistentvolumes"},
	"pv":                    {Group: "", Version: "v1", Resource: "persistentvolumes"},
	"persistentvolumeclaim": {Group: "", Version: "v1", Resource: "persistentvolumeclaims"},
	"pvc":                   {Group: "", Version: "v1", Resource: "persistentvolumeclaims"},
	"resourcequota":         {Group: "", Version: "v1", Resource: "resourcequotas"},
	"quota":                 {Group: "", Version: "v1", Resource: "resourcequotas"},
	"limitrange":            {Group: "", Version: "v1", Resource: "limitranges"},
	"limits":                {Group: "", Version: "v1", Resource: "limitranges"},
	"endpoints":             {Group: "", Version: "v1", Resource: "endpoints"},
	"ep":                    {Group: "", Version: "v1", Resource: "endpoints"},

	// --- Apps Resources (Group: "apps") ---
	"deployment":  {Group: "apps", Version: "v1", Resource: "deployments"},
	"deploy":      {Group: "apps", Version: "v1", Resource: "deployments"},
	"statefulset": {Group: "apps", Version: "v1", Resource: "statefulsets"},
	"sts":         {Group: "apps", Version: "v1", Resource: "statefulsets"},
	"daemonset":   {Group: "apps", Version: "v1", Resource: "daemonsets"},
	"ds":          {Group: "apps", Version: "v1", Resource: "daemonsets"},
	"replicaset":  {Group: "apps", Version: "v1", Resource: "replicasets"},
	"rs":          {Group: "apps", Version: "v1", Resource: "replicasets"},

	// --- Batch Resources (Group: "batch") ---
	"job":     {Group: "batch", Version: "v1", Resource: "jobs"},
	"cronjob": {Group: "batch", Version: "v1", Resource: "cronjobs"},
	"cj":      {Group: "batch", Version: "v1", Resource: "cronjobs"},

	// --- Networking Resources (Group: "networking.k8s.io") ---
	"ingress":       {Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
	"ing":           {Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
	"networkpolicy": {Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"},
	"netpol":        {Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"},
	"ingressclass":  {Group: "networking.k8s.io", Version: "v1", Resource: "ingressclasses"},

	// --- Autoscaling Resources (Group: "autoscaling") ---
	"horizontalpodautoscaler": {Group: "autoscaling", Version: "v2", Resource: "horizontalpodautoscalers"},
	"hpa":                     {Group: "autoscaling", Version: "v2", Resource: "horizontalpodautoscalers"},
	"vpa":                     {Group: "autoscaling.k8s.io", Version: "v1", Resource: "verticalpodautoscalers"},

	// --- RBAC Resources (Group: "rbac.authorization.k8s.io") ---
	"role":               {Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "roles"},
	"rolebinding":        {Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "rolebindings"},
	"clusterrole":        {Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"},
	"clusterrolebinding": {Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterrolebindings"},

	// --- Storage Resources (Group: "storage.k8s.io") ---
	"storageclass":     {Group: "storage.k8s.io", Version: "v1", Resource: "storageclasses"},
	"sc":               {Group: "storage.k8s.io", Version: "v1", Resource: "storageclasses"},
	"volumeattachment": {Group: "storage.k8s.io", Version: "v1", Resource: "volumeattachments"},

	// --- Custom Resource Definitions (Group: "apiextensions.k8s.io") ---
	"crd":                       {Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"},
	"customresourcedefinition":  {Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"},
	"customresourcedefinitions": {Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"},

	// --- Discovery/Endpoint Resources (Group: "discovery.k8s.io") ---
	"endpointslice":  {Group: "discovery.k8s.io", Version: "v1", Resource: "endpointslices"},
	"endpointslices": {Group: "discovery.k8s.io", Version: "v1", Resource: "endpointslices"},

	// --- Policy Resources (Group: "policy") ---
	"poddisruptionbudget": {Group: "policy", Version: "v1", Resource: "poddisruptionbudgets"},
	"pdb":                 {Group: "policy", Version: "v1", Resource: "poddisruptionbudgets"},

	// --- METRICS Resources (Group: "metrics.k8s.io") ---
	"node.metrics.k8s.io": {Group: "metrics.k8s.io", Version: "v1beta1", Resource: "nodes"},
	"pod.metrics.k8s.io":  {Group: "metrics.k8s.io", Version: "v1beta1", Resource: "pods"},
	"nodemetrics":         {Group: "metrics.k8s.io", Version: "v1beta1", Resource: "nodes"},
	"podmetrics":          {Group: "metrics.k8s.io", Version: "v1beta1", Resource: "pods"},

	// --- RANCHER CORE Resources (Group: "management.cattle.io") ---
	"cluster":                    {Group: "management.cattle.io", Version: "v3", Resource: "clusters"},
	"project":                    {Group: "management.cattle.io", Version: "v3", Resource: "projects"},
	"user":                       {Group: "management.cattle.io", Version: "v3", Resource: "users"},
	"roletemplate":               {Group: "management.cattle.io", Version: "v3", Resource: "roletemplates"},
	"globalrole":                 {Group: "management.cattle.io", Version: "v3", Resource: "globalroles"},
	"globalrolebinding":          {Group: "management.cattle.io", Version: "v3", Resource: "globalrolebindings"},
	"clusterroletemplatebinding": {Group: "management.cattle.io", Version: "v3", Resource: "clusterroletemplatebindings"},
	"projectroletemplatebinding": {Group: "management.cattle.io", Version: "v3", Resource: "projectroletemplatebindings"},
	"nodetemplate":               {Group: "management.cattle.io", Version: "v3", Resource: "nodetemplates"},
	"nodedriver":                 {Group: "management.cattle.io", Version: "v3", Resource: "nodedrivers"},
	"setting":                    {Group: "management.cattle.io", Version: "v3", Resource: "settings"},

	// --- RANCHER FLEET Resources (Group: "fleet.cattle.io") ---
	"bundle":           {Group: "fleet.cattle.io", Version: "v1alpha1", Resource: "bundles"},
	"gitrepo":          {Group: "fleet.cattle.io", Version: "v1alpha1", Resource: "gitrepos"},
	"bundledeployment": {Group: "fleet.cattle.io", Version: "v1alpha1", Resource: "bundledeployments"},
	"clustergroup":     {Group: "fleet.cattle.io", Version: "v1alpha1", Resource: "clustergroups"},
	"fleetcluster":     {Group: "fleet.cattle.io", Version: "v1alpha1", Resource: "clusters"},

	// --- Cert-Manager Resources (Group: "cert-manager.io") ---
	"certificate":   {Group: "cert-manager.io", Version: "v1", Resource: "certificates"},
	"issuer":        {Group: "cert-manager.io", Version: "v1", Resource: "issuers"},
	"clusterissuer": {Group: "cert-manager.io", Version: "v1", Resource: "clusterissuers"},
}

// GetGVR returns the GroupVersionResource for a given kind.
// Returns an empty GVR and false if the kind is not found.
func GetGVR(kind string) (schema.GroupVersionResource, bool) {
	gvr, ok := K8sKindsToGVRs[kind]
	return gvr, ok
}

// builtinGVRScopes records the namespaced bit of built-in Kubernetes GVRs.
// Static entries outside this set (Rancher, fleet, and cert-manager kinds)
// have their scope discovered from the cluster instead. New built-in entries
// added to K8sKindsToGVRs must also be recorded here.
var builtinGVRScopes = map[schema.GroupVersionResource]bool{
	{Group: "", Version: "v1", Resource: "pods"}:                                          true,
	{Group: "", Version: "v1", Resource: "services"}:                                      true,
	{Group: "", Version: "v1", Resource: "configmaps"}:                                    true,
	{Group: "", Version: "v1", Resource: "secrets"}:                                       true,
	{Group: "", Version: "v1", Resource: "events"}:                                        true,
	{Group: "", Version: "v1", Resource: "namespaces"}:                                    false,
	{Group: "", Version: "v1", Resource: "nodes"}:                                         false,
	{Group: "", Version: "v1", Resource: "serviceaccounts"}:                               true,
	{Group: "", Version: "v1", Resource: "persistentvolumes"}:                             false,
	{Group: "", Version: "v1", Resource: "persistentvolumeclaims"}:                        true,
	{Group: "", Version: "v1", Resource: "resourcequotas"}:                                true,
	{Group: "", Version: "v1", Resource: "limitranges"}:                                   true,
	{Group: "", Version: "v1", Resource: "endpoints"}:                                     true,
	{Group: "apps", Version: "v1", Resource: "deployments"}:                               true,
	{Group: "apps", Version: "v1", Resource: "statefulsets"}:                              true,
	{Group: "apps", Version: "v1", Resource: "daemonsets"}:                                true,
	{Group: "apps", Version: "v1", Resource: "replicasets"}:                               true,
	{Group: "batch", Version: "v1", Resource: "jobs"}:                                     true,
	{Group: "batch", Version: "v1", Resource: "cronjobs"}:                                 true,
	{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"}:                    true,
	{Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"}:              true,
	{Group: "networking.k8s.io", Version: "v1", Resource: "ingressclasses"}:               false,
	{Group: "autoscaling", Version: "v2", Resource: "horizontalpodautoscalers"}:           true,
	{Group: "autoscaling.k8s.io", Version: "v1", Resource: "verticalpodautoscalers"}:      true,
	{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "roles"}:                true,
	{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "rolebindings"}:         true,
	{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"}:         false,
	{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterrolebindings"}:  false,
	{Group: "storage.k8s.io", Version: "v1", Resource: "storageclasses"}:                  false,
	{Group: "storage.k8s.io", Version: "v1", Resource: "volumeattachments"}:               false,
	{Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"}: false,
	{Group: "discovery.k8s.io", Version: "v1", Resource: "endpointslices"}:                true,
	{Group: "policy", Version: "v1", Resource: "poddisruptionbudgets"}:                    true,
	{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "nodes"}:                      false,
	{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "pods"}:                       true,
}

// ResolveStaticScope resolves kind to its GVR and namespaced bit without
// contacting the cluster. It mirrors the static branches of resolveGVR and
// returns ok=false when the scope must be discovered instead: unknown kinds,
// apiVersion-qualified references whose group does not match the static table
// entry, and static entries outside the built-in Kubernetes set (Rancher,
// fleet, and cert-manager kinds) whose scope is not hard-coded.
func ResolveStaticScope(kind string) (gvr schema.GroupVersionResource, namespaced, ok bool) {
	original := strings.TrimSpace(kind)
	if original == "" {
		return schema.GroupVersionResource{}, false, false
	}
	if apiVersion, apiKind, parsed := parseAPIVersionKind(original); parsed {
		name := strings.ToLower(apiKind)
		if gvr, hit := GetGVR(name); hit && gvrMatchesAPIVersion(gvr, apiVersion) {
			namespaced, ok = builtinGVRScope(gvr)
			return gvr, namespaced, ok
		}
		// Plural resource names match only a built-in GVR of the same
		// group-version; a different group must fall through to discovery so
		// a conflicting custom resource cannot inherit the built-in scope.
		if gvr, namespaced, hit := findBuiltinGVRByResource(name, apiVersion); hit {
			return gvr, namespaced, true
		}
		return schema.GroupVersionResource{}, false, false
	}
	name := strings.ToLower(original)
	if gvr, hit := GetGVR(name); hit {
		namespaced, ok = builtinGVRScope(gvr)
		return gvr, namespaced, ok
	}
	if gvr, namespaced, hit := findBuiltinGVRByResource(name, ""); hit {
		return gvr, namespaced, true
	}
	return schema.GroupVersionResource{}, false, false
}

func builtinGVRScope(gvr schema.GroupVersionResource) (namespaced, known bool) {
	namespaced, known = builtinGVRScopes[gvr]
	return namespaced, known
}

// findBuiltinGVRByResource matches a plural resource name against the
// built-in scope table. With an empty apiVersion the first match wins; a
// non-empty apiVersion must match the GVR's group-version.
func findBuiltinGVRByResource(resource, apiVersion string) (schema.GroupVersionResource, bool, bool) {
	for gvr, namespaced := range builtinGVRScopes {
		if gvr.Resource != resource {
			continue
		}
		if apiVersion != "" && !gvrMatchesAPIVersion(gvr, apiVersion) {
			continue
		}
		return gvr, namespaced, true
	}
	return schema.GroupVersionResource{}, false, false
}
