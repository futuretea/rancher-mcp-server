package mcp

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/futuretea/rancher-mcp-server/pkg/core/config"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset/kubernetes"
	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

func TestNewServerAppliesNamespaceAllowlist(t *testing.T) {
	t.Cleanup(func() { kubernetes.SetNamespaceAllowlist(nil) })
	cfg, calls := namespaceAllowlistServerConfig(t)
	cfg.AllowedNamespaces = map[string][]string{"kubeconfig:direct": {"app"}}
	server, err := NewServer(Configuration{StaticConfig: cfg})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(server.Close)
	client := newServerInProcessClient(t, server)

	t.Run("denied before backend", func(t *testing.T) {
		result := callNamespaceAllowlistTool(t, client, "kube-system")
		if !result.IsError {
			t.Errorf("out-of-list namespace succeeded: %#v", result.Content)
		} else if len(result.Content) != 1 {
			t.Errorf("error content count = %d, want 1", len(result.Content))
		} else if content, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(content.Text, "not allowed") {
			t.Errorf("tool result = %#v, want namespace denial", result.Content)
		}
		if got := calls.Load(); got != 0 {
			t.Errorf("backend requests = %d, want 0 for denied namespace", got)
		}
	})
	t.Run("allowed namespace succeeds", func(t *testing.T) {
		before := calls.Load()
		result := callNamespaceAllowlistTool(t, client, "app")
		assertNamespaceAllowlistSuccess(t, result)
		if got := calls.Load() - before; got != 1 {
			t.Errorf("backend requests = %d, want 1 for allowed namespace", got)
		}
	})
}

func TestNewServerClearsNamespaceAllowlist(t *testing.T) {
	t.Cleanup(func() { kubernetes.SetNamespaceAllowlist(nil) })
	for _, test := range []struct {
		name      string
		allowlist map[string][]string
	}{
		{name: "nil"},
		{name: "empty map", allowlist: map[string][]string{}},
		{name: "empty array", allowlist: map[string][]string{"kubeconfig:direct": {}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg, calls := namespaceAllowlistServerConfig(t)
			cfg.AllowedNamespaces = map[string][]string{"kubeconfig:direct": {"app"}}
			restricted, err := NewServer(Configuration{StaticConfig: cfg})
			if err != nil {
				t.Fatal(err)
			}
			restricted.Close()

			cfg.AllowedNamespaces = test.allowlist
			server, err := NewServer(Configuration{StaticConfig: cfg})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(server.Close)
			result := callNamespaceAllowlistTool(t, newServerInProcessClient(t, server), "kube-system")
			assertNamespaceAllowlistSuccess(t, result)
			if got := calls.Load(); got != 1 {
				t.Errorf("backend requests = %d, want 1 for unrestricted namespace", got)
			}
		})
	}
}

func namespaceAllowlistServerConfig(t *testing.T) (*config.StaticConfig, *atomic.Int32) {
	t.Helper()
	calls := &atomic.Int32{}
	apiServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		namespace := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/namespaces/"), "/pods")
		if r.Method != http.MethodGet || (namespace != "app" && namespace != "kube-system") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"apiVersion":"v1","kind":"PodList","items":[{"apiVersion":"v1","kind":"Pod","metadata":{"name":"visible-pod","namespace":%q}}]}`, namespace)
	}))
	t.Cleanup(apiServer.Close)
	return &config.StaticConfig{
		KubeconfigPaths: []string{writeServerKubeconfigWithInsecureTLS(t, "direct", apiServer.URL)},
		Toolsets:        []string{"kubernetes"},
		ListOutput:      "json",
	}, calls
}

func callNamespaceAllowlistTool(t *testing.T, client *mcpclient.Client, namespace string) *mcp.CallToolResult {
	t.Helper()
	result, err := client.CallTool(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{
		Name: "kubernetes_list",
		Arguments: map[string]interface{}{
			"cluster": "kubeconfig:direct", "kind": "pod", "namespace": namespace, "format": "json",
		},
	}})
	if err != nil {
		t.Fatalf("CallTool(kubernetes_list): %v", err)
	}
	return result
}

func assertNamespaceAllowlistSuccess(t *testing.T, result *mcp.CallToolResult) {
	t.Helper()
	if result.IsError || len(result.Content) != 1 {
		t.Fatalf("tool result = %#v, want successful pod list", result)
	}
	content, ok := result.Content[0].(mcp.TextContent)
	if !ok || !strings.Contains(content.Text, "visible-pod") {
		t.Fatalf("tool content = %#v, want visible-pod", result.Content)
	}
}
