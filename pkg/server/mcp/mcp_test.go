package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"expvar"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/futuretea/rancher-mcp-server/pkg/client/norman"
	"github.com/futuretea/rancher-mcp-server/pkg/client/steve"
	"github.com/futuretea/rancher-mcp-server/pkg/core/config"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/rancher/norman/types"
)

func startRancherSchemaServer(t *testing.T) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		schemas := types.SchemaCollection{
			Data: []types.Schema{
				{
					ID:      "cluster",
					Type:    "/meta/schemas/schema",
					Links:   map[string]string{},
					Version: types.APIVersion{Path: "/v3", Version: "v3"},
				},
			},
		}

		w.Header().Set("X-API-Schemas", "http://"+r.Host+"/v3/schemas")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(schemas)
	}))
	t.Cleanup(server.Close)
	return server
}

func TestNewServer(t *testing.T) {
	cfg := &config.StaticConfig{
		RancherServerURL: "https://rancher.example.com",
		RancherAccessKey: "test-key",
		RancherSecretKey: "test-secret",
	}
	mcpConfig := Configuration{StaticConfig: cfg}

	server, err := NewServer(mcpConfig)
	if err != nil {
		// Server creation may fail due to fake credentials, but client should still be created
		if server == nil {
			t.Fatal("Server should be created even with fake credentials")
		}
		return
	}

	if server == nil {
		t.Fatal("Server should not be nil")
	}

	// Check that tools are registered
	tools := server.GetEnabledTools()
	if len(tools) < 1 {
		t.Errorf("Expected at least 1 tool, got %d", len(tools))
	}

	// Kubernetes tools are registered when Rancher config exists. Rancher-specific
	// tools are hidden when the Norman client cannot be initialized.
	assertToolsPresent(t, tools, "kubernetes_get", "kubernetes_list", "kubernetes_describe", "kubernetes_events")
	assertToolsAbsent(t, tools, "cluster_list", "project_list")
}

func assertToolsPresent(t *testing.T, tools []string, expectedTools ...string) {
	t.Helper()

	toolNames := makeToolNameSet(tools)
	for _, expected := range expectedTools {
		if !toolNames[expected] {
			t.Errorf("Expected tool '%s' not found in registered tools", expected)
		}
	}
}

func assertToolsAbsent(t *testing.T, tools []string, unexpectedTools ...string) {
	t.Helper()

	toolNames := makeToolNameSet(tools)
	for _, unexpected := range unexpectedTools {
		if toolNames[unexpected] {
			t.Errorf("Unexpected tool '%s' found in registered tools", unexpected)
		}
	}
}

func makeToolNameSet(tools []string) map[string]bool {
	toolNames := make(map[string]bool, len(tools))
	for _, tool := range tools {
		toolNames[tool] = true
	}
	return toolNames
}

func TestServerMethods(t *testing.T) {
	cfg := &config.StaticConfig{
		RancherServerURL: "https://rancher.example.com",
		RancherAccessKey: "test-key",
		RancherSecretKey: "test-secret",
	}
	mcpConfig := Configuration{StaticConfig: cfg}

	server, err := NewServer(mcpConfig)
	if err != nil {
		// Server creation may fail due to fake credentials, but client should still be created
		if server == nil {
			t.Fatal("Server should be created even with fake credentials")
		}
		return
	}

	// Test GetEnabledTools
	tools := server.GetEnabledTools()
	if len(tools) == 0 {
		t.Error("GetEnabledTools should return at least one tool")
	}

	// Test Close (should not panic)
	defer server.Close()

	// Note: We can't easily test ServeStdio, ServeSse, ServeHTTP without
	// actually starting servers, but we can verify they exist and have the right signatures
}

func TestNewServerHidesCapabilityDependentToolsWithoutRancherConfig(t *testing.T) {
	server, err := NewServer(Configuration{StaticConfig: &config.StaticConfig{}})
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	defer server.Close()

	tools := server.GetEnabledTools()

	for _, hiddenTool := range []string{"cluster_list", "project_list", "kubernetes_get", "kubernetes_resource_diff"} {
		for _, actual := range tools {
			if actual == hiddenTool {
				t.Fatalf("expected tool %q to be hidden without runtime capability, enabled tools: %v", hiddenTool, tools)
			}
		}
	}

	foundLocalDiff := false
	for _, actual := range tools {
		if actual == "kubernetes_diff" {
			foundLocalDiff = true
			break
		}
	}
	if !foundLocalDiff {
		t.Fatalf("expected local kubernetes_diff to stay enabled without runtime capability, enabled tools: %v", tools)
	}
}

func TestGetHealthStatusWithoutRancherConfig(t *testing.T) {
	server, err := NewServer(Configuration{StaticConfig: &config.StaticConfig{}})
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	defer server.Close()

	if !server.IsHealthy() {
		t.Fatal("expected server to be healthy when process initialized")
	}

	status := server.GetHealthStatus()
	if status.Status != "ok" {
		t.Fatalf("expected health status ok, got %s", status.Status)
	}

	rancherStatus, ok := status.Capabilities["rancher"]
	if !ok {
		t.Fatal("expected rancher capability in health status")
	}
	if rancherStatus.Configured || rancherStatus.Available {
		t.Fatalf("expected rancher capability to be unconfigured and unavailable, got %+v", rancherStatus)
	}

	kubernetesStatus, ok := status.Capabilities["kubernetes"]
	if !ok {
		t.Fatal("expected kubernetes capability in health status")
	}
	if kubernetesStatus.Configured || kubernetesStatus.Available {
		t.Fatalf("expected kubernetes capability to be unconfigured and unavailable, got %+v", kubernetesStatus)
	}
}

func TestMakeToolHandler_ResolvesAndClosesClient(t *testing.T) {
	server := startRancherSchemaServer(t)

	normanClient, err := norman.NewClientWithToken(server.URL, "token", true)
	if err != nil {
		t.Fatalf("failed to create Norman client: %v", err)
	}
	steveClient := steve.NewClientWithToken(server.URL, "token", true)

	// Use closeable=false to match production static-mode clients, which must
	// remain usable across multiple tool calls.
	staticClient := toolset.NewCombinedClient(normanClient, steveClient, false)

	s := &Server{
		configuration:  &Configuration{},
		clientResolver: &staticResolver{client: staticClient},
	}

	handlerCalled := false
	tool := toolset.ServerTool{
		Tool: mcp.Tool{Name: "test_tool"},
		Handler: func(_ context.Context, _ interface{}, _ map[string]interface{}) (string, error) {
			handlerCalled = true
			return "ok", nil
		},
	}

	mcpHandler := s.makeToolHandler(tool)
	_, err = mcpHandler(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if !handlerCalled {
		t.Fatal("expected tool handler to be called")
	}
	if !normanClient.IsUsable() {
		t.Fatal("expected static Norman client to remain usable after handler")
	}
}

// stubResolver is a test resolver that returns a fixed CombinedClient.
type stubResolver struct {
	client *toolset.CombinedClient
}

func (r *stubResolver) Resolve(_ context.Context) (*toolset.CombinedClient, error) {
	return r.client, nil
}

func TestMakeToolHandler_RequestScopedClientClosesAndDecrementsActiveCount(t *testing.T) {
	server := startRancherSchemaServer(t)

	normanClient, err := norman.NewClientWithToken(server.URL, "token", true)
	if err != nil {
		t.Fatalf("failed to create Norman client: %v", err)
	}
	steveClient := steve.NewClientWithToken(server.URL, "token", true)

	closeableClient := toolset.NewCombinedClient(normanClient, steveClient, true)
	metrics := NewExpvarMetrics()
	metrics.(*expvarMetrics).activeClientCount.Set(0)

	s := &Server{
		configuration:  &Configuration{},
		clientResolver: &stubResolver{client: closeableClient},
		metrics:        metrics,
	}

	tool := toolset.ServerTool{
		Tool:    mcp.Tool{Name: "test_tool"},
		Handler: func(_ context.Context, _ interface{}, _ map[string]interface{}) (string, error) { return "", nil },
	}

	mcpHandler := s.makeToolHandler(tool)
	_, err = mcpHandler(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if normanClient.IsUsable() {
		t.Fatal("expected Norman client to be closed after handler")
	}
	if active := expvar.Get("active_client_count").String(); active != "0" {
		t.Fatalf("expected active_client_count to return to 0, got %s", active)
	}
}

func TestMakeToolHandler_RequestScopedClientClosesOnHandlerError(t *testing.T) {
	server := startRancherSchemaServer(t)

	normanClient, err := norman.NewClientWithToken(server.URL, "token", true)
	if err != nil {
		t.Fatalf("failed to create Norman client: %v", err)
	}
	steveClient := steve.NewClientWithToken(server.URL, "token", true)

	closeableClient := toolset.NewCombinedClient(normanClient, steveClient, true)
	metrics := NewExpvarMetrics()
	metrics.(*expvarMetrics).activeClientCount.Set(0)

	s := &Server{
		configuration:  &Configuration{},
		clientResolver: &stubResolver{client: closeableClient},
		metrics:        metrics,
	}

	tool := toolset.ServerTool{
		Tool: mcp.Tool{Name: "test_tool"},
		Handler: func(_ context.Context, _ interface{}, _ map[string]interface{}) (string, error) {
			return "", errors.New("handler error")
		},
	}

	mcpHandler := s.makeToolHandler(tool)
	result, err := mcpHandler(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("handler returned transport error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected tool result to be an error")
	}

	if normanClient.IsUsable() {
		t.Fatal("expected Norman client to be closed after handler error")
	}
	if active := expvar.Get("active_client_count").String(); active != "0" {
		t.Fatalf("expected active_client_count to return to 0 after handler error, got %s", active)
	}
}

func TestContextFunc_PlacesAuthorizationInContext(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer context-token")

	ctx := contextFunc(context.Background(), req)
	token, err := bearerTokenFromContext(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "context-token" {
		t.Fatalf("expected context-token, got %q", token)
	}
}

func TestContextFunc_NoAuthorizationLeavesContextUnchanged(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	base := context.WithValue(context.Background(), authorizationKey, "existing-value")

	ctx := contextFunc(base, req)
	if ctx != base {
		t.Fatal("expected context to be unchanged when Authorization header is absent")
	}
}
