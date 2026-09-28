//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	allowlistApp    = "mcp-allowlist-app"
	allowlistOther  = "mcp-allowlist-other"
	allowlistMarker = "mcp-allowlist-marker"
)

func testNamespaceAllowlist(t *testing.T, env *rancherEnv) {
	for _, namespace := range []string{allowlistApp, allowlistOther} {
		env.kubectl("create", "namespace", namespace)
		env.kubectl("create", "configmap", allowlistMarker, "-n", namespace, "--from-literal=value=original")
	}
	rancherArgs := []string{
		"--rancher-server-url", env.baseURL, "--rancher-tls-insecure",
		"--rancher-request-token-auth", "--toolsets", "kubernetes", "--read-only=false",
	}
	for _, connection := range []struct {
		name    string
		cluster string
		args    []string
		headers map[string]string
	}{
		{name: "rancher", cluster: "local", args: rancherArgs, headers: bearer(env.apiToken)},
		{name: "kubeconfig", cluster: "kubeconfig:default", args: []string{
			"--kubeconfig-paths", env.kubeconfig(), "--toolsets", "kubernetes", "--read-only=false",
		}},
	} {
		t.Run(connection.name, func(t *testing.T) {
			// The nonexistent name exercises stale allowlist entries against a real API.
			allowlist := fmt.Sprintf(`{%q:[%q,"mcp-allowlist-missing"]}`, connection.cluster, allowlistApp)
			serverURL := startMCPServer(t, withArgs(connection.args, "--allowed-namespaces", allowlist)...)
			t.Run("reads", func(t *testing.T) {
				testNamespaceAllowlistReads(t, env, serverURL, connection.cluster, connection.headers)
			})
			t.Run("get_all", func(t *testing.T) {
				testNamespaceAllowlistGetAll(t, serverURL, connection.cluster, connection.headers)
			})
			t.Run("allowed_writes", func(t *testing.T) {
				testAllowedNamespaceWrites(t, env, serverURL, connection.cluster, connection.headers)
			})
			t.Run("denied_writes", func(t *testing.T) {
				testDeniedNamespaceWrites(t, env, serverURL, connection.cluster, connection.headers)
			})
		})
	}
	t.Run("configuration", func(t *testing.T) {
		testNamespaceAllowlistConfiguration(t, rancherArgs, bearer(env.apiToken))
	})
}

func testNamespaceAllowlistReads(t *testing.T, env *rancherEnv, serverURL, cluster string, headers map[string]string) {
	for _, namespace := range []string{"", allowlistApp} {
		t.Run("configmaps/"+namespace, func(t *testing.T) {
			result, err := callTool(t, serverURL, headers, kubernetesListTool, map[string]any{
				"cluster": cluster, "kind": "configmap", "namespace": namespace, "name": allowlistMarker, "format": "json",
			})
			assertAllowlistResources(t, result, err, []string{allowlistApp + "/" + allowlistMarker})
		})
	}
	t.Run("namespace_objects", func(t *testing.T) {
		result, err := callTool(t, serverURL, headers, kubernetesListTool, map[string]any{
			"cluster": cluster, "kind": "namespace", "format": "json",
		})
		assertAllowlistResources(t, result, err, []string{"/" + allowlistApp})
	})
	t.Run("cluster_scoped_nodes", func(t *testing.T) {
		var want []string
		for _, node := range strings.Fields(env.kubectl("get", "nodes", "-o", "jsonpath={.items[*].metadata.name}")) {
			want = append(want, "/"+node)
		}
		if len(want) == 0 {
			t.Fatal("test cluster has no nodes")
		}
		result, err := callTool(t, serverURL, headers, kubernetesListTool, map[string]any{
			"cluster": cluster, "kind": "node", "format": "json",
		})
		assertAllowlistResources(t, result, err, want)
	})
	for _, tool := range []string{kubernetesListTool, "kubernetes_get"} {
		t.Run("denied_namespace/"+tool, func(t *testing.T) {
			result, err := callTool(t, serverURL, headers, tool, map[string]any{
				"cluster": cluster, "kind": "configmap", "namespace": allowlistOther, "name": allowlistMarker, "format": "json",
			})
			assertNamespaceDenied(t, result, err)
		})
	}
	t.Run("denied_namespace_object", func(t *testing.T) {
		result, err := callTool(t, serverURL, headers, "kubernetes_get", map[string]any{
			"cluster": cluster, "kind": "namespace", "name": allowlistOther, "format": "json",
		})
		assertNamespaceDenied(t, result, err)
	})
	t.Run("forged_policy", func(t *testing.T) {
		result, err := callTool(t, serverURL, headers, kubernetesListTool, map[string]any{
			"cluster": cluster, "kind": "configmap", "namespace": allowlistOther, "format": "json",
			"namespaceAllowlist": map[string][]string{cluster: {allowlistOther}},
		})
		assertNamespaceDenied(t, result, err)
	})
}

func testNamespaceAllowlistGetAll(t *testing.T, serverURL, cluster string, headers map[string]string) {
	result, err := callTool(t, serverURL, headers, "kubernetes_get_all", map[string]any{
		"cluster": cluster, "scope": "namespaced", "name": allowlistMarker, "format": "json",
	})
	expectToolSuccess(t, result, err, "get-all with an omitted namespace")
	var items []struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
		Kind      string `json:"kind"`
	}
	if err := json.Unmarshal([]byte(toolResultText(result)), &items); err != nil {
		t.Fatalf("decode get-all resources: %v", err)
	}
	if len(items) != 1 || items[0].Name != allowlistMarker || items[0].Namespace != allowlistApp || items[0].Kind != "ConfigMap" {
		t.Fatalf("get-all resources = %+v, want only the allowed ConfigMap", items)
	}
}

func testAllowedNamespaceWrites(t *testing.T, env *rancherEnv, serverURL, cluster string, headers map[string]string) {
	const name = "mcp-allowlist-write"
	resource := fmt.Sprintf(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":%q,"namespace":%q},"data":{"value":"created"}}`, name, allowlistApp)
	result, err := callTool(t, serverURL, headers, "kubernetes_create", map[string]any{"cluster": cluster, "resource": resource})
	expectToolSuccess(t, result, err, "create in allowed namespace")
	if got := env.kubectl("get", "configmap", name, "-n", allowlistApp, "-o", "jsonpath={.data.value}"); got != "created" {
		t.Fatalf("created value = %q, want created", got)
	}
	result, err = callTool(t, serverURL, headers, "kubernetes_get", map[string]any{
		"cluster": cluster, "kind": "configmap", "namespace": allowlistApp, "name": name, "format": "json",
	})
	expectToolSuccess(t, result, err, "get in allowed namespace")
	var created unstructured.Unstructured
	if err := json.Unmarshal([]byte(toolResultText(result)), &created); err != nil {
		t.Fatalf("decode created configmap: %v", err)
	}
	if created.GetName() != name || created.GetNamespace() != allowlistApp {
		t.Fatalf("get returned %s/%s, want %s/%s", created.GetNamespace(), created.GetName(), allowlistApp, name)
	}
	result, err = callTool(t, serverURL, headers, "kubernetes_patch", map[string]any{
		"cluster": cluster, "kind": "configmap", "namespace": allowlistApp, "name": name,
		"patch": `[{"op":"replace","path":"/data/value","value":"updated"}]`,
	})
	expectToolSuccess(t, result, err, "patch in allowed namespace")
	if got := env.kubectl("get", "configmap", name, "-n", allowlistApp, "-o", "jsonpath={.data.value}"); got != "updated" {
		t.Fatalf("patched value = %q, want updated", got)
	}
	result, err = callTool(t, serverURL, headers, "kubernetes_delete", map[string]any{
		"cluster": cluster, "kind": "configmap", "namespace": allowlistApp, "name": name,
	})
	expectToolSuccess(t, result, err, "delete in allowed namespace")
	if got := env.kubectl("get", "configmap", name, "-n", allowlistApp, "--ignore-not-found", "-o", "name"); got != "" {
		t.Fatalf("deleted configmap still exists: %s", got)
	}
}

func testDeniedNamespaceWrites(t *testing.T, env *rancherEnv, serverURL, cluster string, headers map[string]string) {
	const name = "mcp-allowlist-denied-write"
	resource := fmt.Sprintf(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":%q,"namespace":%q}}`, name, allowlistOther)
	for _, test := range []struct {
		tool string
		args map[string]any
	}{
		{tool: "kubernetes_create", args: map[string]any{"cluster": cluster, "resource": resource}},
		{tool: "kubernetes_patch", args: map[string]any{
			"cluster": cluster, "kind": "configmap", "namespace": allowlistOther, "name": allowlistMarker,
			"patch": `[{"op":"replace","path":"/data/value","value":"forbidden"}]`,
		}},
		{tool: "kubernetes_delete", args: map[string]any{
			"cluster": cluster, "kind": "configmap", "namespace": allowlistOther, "name": allowlistMarker,
		}},
	} {
		t.Run(test.tool, func(t *testing.T) {
			result, err := callTool(t, serverURL, headers, test.tool, test.args)
			assertNamespaceDenied(t, result, err)
		})
	}
	if got := env.kubectl("get", "configmap", allowlistMarker, "-n", allowlistOther, "-o", "jsonpath={.data.value}"); got != "original" {
		t.Fatalf("denied namespace was modified: value = %q", got)
	}
	if got := env.kubectl("get", "configmap", name, "-n", allowlistOther, "--ignore-not-found", "-o", "name"); got != "" {
		t.Fatalf("denied create persisted a configmap: %s", got)
	}
}

func testNamespaceAllowlistConfiguration(t *testing.T, base []string, headers map[string]string) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	body := fmt.Sprintf("allowed_namespaces:\n  local: [%s]\n", allowlistOther)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	app := fmt.Sprintf(`{"local":[%q]}`, allowlistApp)
	for _, test := range []struct {
		name string
		env  string
		flag string
		want []string
	}{
		{name: "file", want: []string{allowlistOther + "/" + allowlistMarker}},
		{name: "environment_replaces_file", env: app, want: []string{allowlistApp + "/" + allowlistMarker}},
		{name: "CLI_replaces_invalid_environment", env: "{", flag: app, want: []string{allowlistApp + "/" + allowlistMarker}},
		{name: "empty_object_clears_restrictions", env: app, flag: "{}", want: []string{allowlistApp + "/" + allowlistMarker, allowlistOther + "/" + allowlistMarker}},
		{name: "empty_array_is_unrestricted", flag: `{"local":[]}`, want: []string{allowlistApp + "/" + allowlistMarker, allowlistOther + "/" + allowlistMarker}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("RANCHER_MCP_ALLOWED_NAMESPACES", test.env)
			if test.env == "" {
				if err := os.Unsetenv("RANCHER_MCP_ALLOWED_NAMESPACES"); err != nil {
					t.Fatal(err)
				}
			}
			args := withArgs(base, "--config", path)
			if test.flag != "" {
				args = withArgs(args, "--allowed-namespaces", test.flag)
			}
			serverURL := startMCPServer(t, args...)
			result, err := callTool(t, serverURL, headers, kubernetesListTool, map[string]any{
				"cluster": "local", "kind": "configmap", "name": allowlistMarker, "format": "json",
			})
			assertAllowlistResources(t, result, err, test.want)
		})
	}
}

func assertAllowlistResources(t *testing.T, result *mcpgo.CallToolResult, err error, want []string) {
	t.Helper()
	expectToolSuccess(t, result, err, "namespace allowlist query")
	var items []unstructured.Unstructured
	if err := json.Unmarshal([]byte(toolResultText(result)), &items); err != nil {
		t.Fatalf("decode resource list: %v", err)
	}
	got := make([]string, 0, len(items))
	for _, item := range items {
		got = append(got, item.GetNamespace()+"/"+item.GetName())
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("resources = %v, want %v", got, want)
	}
}

func assertNamespaceDenied(t *testing.T, result *mcpgo.CallToolResult, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("expected a namespace denial, got an MCP transport error: %v", err)
	}
	if !result.IsError || !strings.Contains(toolResultText(result), "namespace \""+allowlistOther+"\" is not allowed") {
		t.Fatalf("expected a denied-namespace tool error, got: %s", toolResultText(result))
	}
}
