//go:build integration

package integration

import (
	"net/http"
	"strings"
	"testing"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

const (
	clusterListTool    = "cluster_list"
	kubernetesListTool = "kubernetes_list"
)

// TestRancherVersionSupport runs the built server against every requested
// Rancher version and records the supported authentication mode per version:
//
//   - static credentials and rancher_request_token_auth work on all tested versions;
//   - rancher_oauth_token_auth works from 2.14 (OIDC access tokens authenticate
//     to the Rancher API only from 2.14, see rancher/rancher#53016);
//   - on 2.13 the OAuth token is accepted by the server but rejected by Rancher,
//     so the tool call must fail.
func TestRancherVersionSupport(t *testing.T) {
	for _, version := range testVersions() {
		version := version
		t.Run(version, func(t *testing.T) {
			env := startRancher(t, version, freePort(t))
			base := []string{"--rancher-server-url", env.baseURL, "--rancher-tls-insecure"}

			t.Run("static_credentials", func(t *testing.T) {
				serverURL := startMCPServer(t, withArgs(base,
					"--rancher-token", env.apiToken,
					"--toolsets", "rancher")...)
				result, err := callTool(t, serverURL, bearer(env.apiToken), clusterListTool, map[string]any{})
				expectToolSuccess(t, result, err, "static credentials")
			})

			t.Run("request_token_auth", func(t *testing.T) {
				serverURL := startMCPServer(t, withArgs(base,
					"--rancher-request-token-auth",
					"--toolsets", "rancher")...)
				result, err := callTool(t, serverURL, bearer(env.apiToken), clusterListTool, map[string]any{})
				expectToolSuccess(t, result, err, "request token auth")
			})

			t.Run("request_token_r_token_fallback", func(t *testing.T) {
				serverURL := startMCPServer(t, withArgs(base,
					"--rancher-request-token-auth",
					"--toolsets", "rancher")...)
				result, err := callTool(t, serverURL, map[string]string{"R_token": env.apiToken}, clusterListTool, map[string]any{})
				expectToolSuccess(t, result, err, "R_token fallback")
			})

			t.Run("kubernetes_toolset", func(t *testing.T) {
				serverURL := startMCPServer(t, withArgs(base,
					"--rancher-request-token-auth",
					"--toolsets", "rancher,kubernetes")...)
				result, err := callTool(t, serverURL, bearer(env.apiToken), kubernetesListTool, map[string]any{
					"cluster": "local",
					"kind":    "namespace",
					"format":  "json",
				})
				expectToolSuccess(t, result, err, "kubernetes toolset")
			})

			t.Run("kubeconfig_direct", func(t *testing.T) {
				serverURL := startMCPServer(t,
					"--kubeconfig-paths", env.kubeconfig(),
					"--toolsets", "kubernetes")
				result, err := callTool(t, serverURL, map[string]string{}, kubernetesListTool, map[string]any{
					"cluster": "kubeconfig:default",
					"kind":    "namespace",
					"format":  "json",
				})
				expectToolSuccess(t, result, err, "kubeconfig direct access")
			})

			t.Run("write_path", func(t *testing.T) {
				serverURL := startMCPServer(t, withArgs(base,
					"--rancher-request-token-auth",
					"--read-only=false",
					"--toolsets", "kubernetes")...)

				const configMap = `{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"mcp-it-cm","namespace":"default"},"data":{"k":"v"}}`
				getArgs := map[string]any{
					"cluster":   "local",
					"kind":      "configmap",
					"namespace": "default",
					"name":      "mcp-it-cm",
					"format":    "json",
				}

				result, err := callTool(t, serverURL, bearer(env.apiToken), "kubernetes_create", map[string]any{
					"cluster":  "local",
					"resource": configMap,
				})
				expectToolSuccess(t, result, err, "kubernetes_create configmap")

				result, err = callTool(t, serverURL, bearer(env.apiToken), "kubernetes_get", getArgs)
				expectToolSuccess(t, result, err, "kubernetes_get created configmap")

				result, err = callTool(t, serverURL, bearer(env.apiToken), "kubernetes_delete", map[string]any{
					"cluster":   "local",
					"kind":      "configmap",
					"namespace": "default",
					"name":      "mcp-it-cm",
				})
				expectToolSuccess(t, result, err, "kubernetes_delete configmap")

				result, err = callTool(t, serverURL, bearer(env.apiToken), "kubernetes_get", getArgs)
				expectToolFailure(t, result, err, "kubernetes_get deleted configmap")
			})

			t.Run("output_formats", func(t *testing.T) {
				serverURL := startMCPServer(t, withArgs(base,
					"--rancher-request-token-auth",
					"--toolsets", "rancher,kubernetes")...)

				for _, format := range []string{"table", "yaml"} {
					format := format
					t.Run(format, func(t *testing.T) {
						result, err := callTool(t, serverURL, bearer(env.apiToken), clusterListTool, map[string]any{"format": format})
						expectToolSuccess(t, result, err, "cluster_list "+format)
						if strings.TrimSpace(toolResultText(result)) == "" {
							t.Fatalf("cluster_list %s output is empty", format)
						}
					})
				}
			})

			t.Run("tool_error_paths", func(t *testing.T) {
				serverURL := startMCPServer(t, withArgs(base,
					"--rancher-request-token-auth",
					"--toolsets", "rancher,kubernetes")...)

				t.Run("missing_cluster", func(t *testing.T) {
					result, err := callTool(t, serverURL, bearer(env.apiToken), kubernetesListTool, map[string]any{
						"cluster": "does-not-exist",
						"kind":    "namespace",
					})
					expectToolFailure(t, result, err, "kubernetes_list on a missing cluster")
				})

				t.Run("missing_resource", func(t *testing.T) {
					result, err := callTool(t, serverURL, bearer(env.apiToken), "kubernetes_get", map[string]any{
						"cluster": "local",
						"kind":    "namespace",
						"name":    "does-not-exist",
					})
					expectToolFailure(t, result, err, "kubernetes_get on a missing resource")
				})

				t.Run("invalid_kind", func(t *testing.T) {
					result, err := callTool(t, serverURL, bearer(env.apiToken), kubernetesListTool, map[string]any{
						"cluster": "local",
						"kind":    "definitely-not-a-kind",
					})
					expectToolFailure(t, result, err, "kubernetes_list with an invalid kind")
				})
			})

			t.Run("more_rancher_tools", func(t *testing.T) {
				serverURL := startMCPServer(t, withArgs(base,
					"--rancher-request-token-auth",
					"--toolsets", "rancher,kubernetes")...)

				result, err := callTool(t, serverURL, bearer(env.apiToken), "project_list", map[string]any{"cluster": "local"})
				expectToolSuccess(t, result, err, "project_list")

				result, err = callTool(t, serverURL, bearer(env.apiToken), "kubernetes_get", map[string]any{
					"cluster": "local",
					"kind":    "namespace",
					"name":    "default",
					"format":  "json",
				})
				expectToolSuccess(t, result, err, "kubernetes_get on the default namespace")
			})

			scopes := "openid offline_access"
			if supportsConfigurableScopes(version) {
				scopes = "openid offline_access rancher:mcp"
			}
			oidcToken := env.oidcAccessToken(scopes)

			oauthArgs := withArgs(base,
				"--rancher-oauth-token-auth",
				"--rancher-oauth-authorization-server-url", env.baseURL+"/oidc",
				"--rancher-oauth-jwks-url", env.baseURL+"/oidc/.well-known/jwks.json",
				"--rancher-oauth-resource-url", "http://localhost",
				"--toolsets", "rancher")

			if supportsConfigurableScopes(version) {
				t.Run("oauth_passthrough", func(t *testing.T) {
					if scope := accessTokenScope(t, oidcToken); !strings.Contains(scope, "rancher:mcp") {
						t.Fatalf("expected the configured rancher:mcp scope in the issued token, got %s", scope)
					}
					serverURL := startMCPServer(t, oauthArgs...)
					result, err := callTool(t, serverURL, bearer(oidcToken), clusterListTool, map[string]any{})
					expectToolSuccess(t, result, err, "OAuth passthrough")
				})

				t.Run("oauth_rejects_missing_rancher_mcp", func(t *testing.T) {
					weakToken := env.oidcAccessToken("openid offline_access")
					serverURL := startMCPServer(t, oauthArgs...)
					if status := mcpStatus(t, serverURL, bearer(weakToken)); status != http.StatusUnauthorized {
						t.Fatalf("expected 401 for a real token without rancher:mcp, got %d", status)
					}
				})

				t.Run("oauth_rejects_wrong_issuer", func(t *testing.T) {
					wrongIssuerArgs := withArgs(base,
						"--rancher-oauth-token-auth",
						"--rancher-oauth-authorization-server-url", "https://other.example.test/oidc",
						"--rancher-oauth-jwks-url", env.baseURL+"/oidc/.well-known/jwks.json",
						"--rancher-oauth-resource-url", "http://localhost",
						"--toolsets", "rancher")
					serverURL := startMCPServer(t, wrongIssuerArgs...)
					if status := mcpStatus(t, serverURL, bearer(oidcToken)); status != http.StatusUnauthorized {
						t.Fatalf("expected 401 for a token issued by another issuer, got %d", status)
					}
				})
			} else {
				t.Run("oauth_passthrough_unsupported", func(t *testing.T) {
					serverURL := startMCPServer(t, oauthArgs...)
					result, err := callTool(t, serverURL, bearer(oidcToken), clusterListTool, map[string]any{})
					if err == nil && !result.IsError {
						t.Fatalf("%s unexpectedly accepted an OIDC access token as a Rancher API credential", version)
					}
				})
			}
		})
	}
}

func withArgs(base []string, extra ...string) []string {
	return append(append([]string{}, base...), extra...)
}

func bearer(token string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + token}
}

func expectToolSuccess(t *testing.T, result *mcpgo.CallToolResult, err error, label string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: MCP tool call failed: %v", label, err)
	}
	if result.IsError {
		t.Fatalf("%s: expected success, got: %s", label, toolResultText(result))
	}
}

// expectToolFailure accepts either a transport-level error or a tool error
// result. It fails only when the server reports success.
func expectToolFailure(t *testing.T, result *mcpgo.CallToolResult, err error, label string) {
	t.Helper()
	if err != nil {
		return
	}
	if result == nil || !result.IsError {
		t.Fatalf("%s: expected a tool error, got success: %s", label, toolResultText(result))
	}
}

func toolResultText(result *mcpgo.CallToolResult) string {
	if result == nil {
		return "<nil>"
	}
	parts := make([]string, 0, len(result.Content))
	for _, content := range result.Content {
		if text, ok := content.(mcpgo.TextContent); ok {
			parts = append(parts, text.Text)
		}
	}
	return strings.Join(parts, "\n")
}
