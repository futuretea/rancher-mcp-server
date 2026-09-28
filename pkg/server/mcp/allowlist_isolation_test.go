package mcp

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/futuretea/rancher-mcp-server/pkg/core/config"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset/kubernetes"
	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

func TestNewServerNamespaceAllowlistIsolation(t *testing.T) {
	for _, test := range []struct {
		name        string
		allowlist   map[string][]string
		rancherOnly bool
		denyApp     bool
	}{
		{name: "nil"},
		{name: "empty map", allowlist: map[string][]string{}},
		{name: "empty array", allowlist: map[string][]string{"kubeconfig:direct": {}}},
		{name: "different scope", allowlist: map[string][]string{"kubeconfig:direct": {"kube-system"}}, denyApp: true},
		{name: "Rancher only", rancherOnly: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Cleanup(func() { kubernetes.SetNamespaceAllowlist(nil) })
			firstConfig, firstCalls := namespaceAllowlistServerConfig(t)
			firstConfig.AllowedNamespaces = map[string][]string{"kubeconfig:direct": {"app"}}
			first := newNamespaceAllowlistClient(t, firstConfig)
			assertNamespaceAllowlistCall(t, first, firstCalls, "kube-system", false)

			if test.rancherOnly {
				second, err := NewServer(Configuration{StaticConfig: &config.StaticConfig{
					ListOutput: "json", Toolsets: []string{"rancher"},
				}})
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(second.Close)
				assertToolsAbsent(t, second.GetEnabledTools(), "kubernetes_list")
			} else {
				secondConfig, secondCalls := namespaceAllowlistServerConfig(t)
				secondConfig.AllowedNamespaces = test.allowlist
				second := newNamespaceAllowlistClient(t, secondConfig)
				assertNamespaceAllowlistCall(t, second, secondCalls, "kube-system", true)
				assertNamespaceAllowlistCall(t, second, secondCalls, "app", !test.denyApp)
			}

			assertNamespaceAllowlistCall(t, first, firstCalls, "kube-system", false)
			assertNamespaceAllowlistCall(t, first, firstCalls, "app", true)
		})
	}
}

func TestNewServerRejectsForgedNamespaceAllowlist(t *testing.T) {
	t.Cleanup(func() { kubernetes.SetNamespaceAllowlist(nil) })
	cfg, calls := namespaceAllowlistServerConfig(t)
	cfg.AllowedNamespaces = map[string][]string{"kubeconfig:direct": {"app"}}
	client := newNamespaceAllowlistClient(t, cfg)
	for _, test := range []struct {
		name  string
		value interface{}
	}{
		{name: "nil"},
		{name: "empty", value: map[string]interface{}{}},
		{name: "expanded", value: map[string]interface{}{"kubeconfig:direct": []interface{}{"kube-system"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			result, err := client.CallTool(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{
				Name: "kubernetes_list",
				Arguments: map[string]interface{}{
					"cluster": "kubeconfig:direct", "kind": "pod", "namespace": "kube-system", "format": "json",
					"namespaceAllowlist": test.value,
				},
			}})
			if err != nil {
				t.Fatal(err)
			}
			assertNamespaceAllowlistDenied(t, result)
			if got := calls.Load(); got != 0 {
				t.Errorf("forged policy caused %d backend requests, want 0", got)
			}
		})
	}
}

func TestNewServerNamespaceAllowlistConfigSnapshot(t *testing.T) {
	t.Cleanup(func() { kubernetes.SetNamespaceAllowlist(nil) })
	cfg, calls := namespaceAllowlistServerConfig(t)
	cfg.AllowedNamespaces = map[string][]string{"kubeconfig:direct": {"app"}}
	client := newNamespaceAllowlistClient(t, cfg)
	for _, mutation := range []struct {
		name  string
		apply func()
	}{
		{name: "slice changed", apply: func() { cfg.AllowedNamespaces["kubeconfig:direct"][0] = "kube-system" }},
		{name: "map entry deleted", apply: func() { delete(cfg.AllowedNamespaces, "kubeconfig:direct") }},
		{name: "map replaced", apply: func() { cfg.AllowedNamespaces = nil }},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			mutation.apply()
			assertNamespaceAllowlistCall(t, client, calls, "kube-system", false)
			assertNamespaceAllowlistCall(t, client, calls, "app", true)
		})
	}
}

func TestNewServerNamespaceAllowlistConcurrentConstruction(t *testing.T) {
	t.Cleanup(func() { kubernetes.SetNamespaceAllowlist(nil) })
	cfg, calls := namespaceAllowlistServerConfig(t)
	cfg.AllowedNamespaces = map[string][]string{"kubeconfig:direct": {"app"}}
	client := newNamespaceAllowlistClient(t, cfg)
	start := make(chan struct{})
	results := make(chan error, 4)
	for range 4 {
		go func() {
			<-start
			server, err := NewServer(Configuration{StaticConfig: cfg})
			if err == nil {
				server.Close()
			}
			results <- err
		}()
	}
	close(start)
	// Complete the workers before cleanup, including when a tool assertion fails.
	defer func() {
		for range 4 {
			if err := <-results; err != nil {
				t.Errorf("concurrent constructor failed: %v", err)
			}
		}
	}()
	for range 4 {
		assertNamespaceAllowlistCall(t, client, calls, "kube-system", false)
	}
}

func newNamespaceAllowlistClient(t *testing.T, cfg *config.StaticConfig) *mcpclient.Client {
	t.Helper()
	server, err := NewServer(Configuration{StaticConfig: cfg})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(server.Close)
	return newServerInProcessClient(t, server)
}

func assertNamespaceAllowlistCall(t *testing.T, client *mcpclient.Client, calls *atomic.Int32, namespace string, allowed bool) {
	t.Helper()
	before := calls.Load()
	result := callNamespaceAllowlistTool(t, client, namespace)
	wantRequests := int32(0)
	if allowed {
		assertNamespaceAllowlistSuccess(t, result)
		wantRequests = 1
	} else {
		assertNamespaceAllowlistDenied(t, result)
	}
	if got := calls.Load() - before; got != wantRequests {
		t.Errorf("namespace=%s allowed=%t backend requests=%d, want %d", namespace, allowed, got, wantRequests)
	}
}

func assertNamespaceAllowlistDenied(t *testing.T, result *mcp.CallToolResult) {
	t.Helper()
	if !result.IsError || len(result.Content) != 1 {
		t.Errorf("tool result = %#v, want namespace denial", result)
		return
	}
	content, ok := result.Content[0].(mcp.TextContent)
	if !ok || !strings.Contains(content.Text, "not allowed") {
		t.Errorf("tool content = %#v, want namespace denial", result.Content)
	}
}
