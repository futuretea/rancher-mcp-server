package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"expvar"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/futuretea/rancher-mcp-server/pkg/client/norman"
	"github.com/futuretea/rancher-mcp-server/pkg/client/steve"
	"github.com/futuretea/rancher-mcp-server/pkg/core/config"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset"
	mcpclient "github.com/mark3labs/mcp-go/client"
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

func TestNewServer_KubeconfigOnlyRegistersApplicableTools(t *testing.T) {
	kubeconfigPath := writeServerKubeconfig(t, "direct", "https://direct.example.test")
	server, err := NewServer(Configuration{StaticConfig: &config.StaticConfig{
		KubeconfigPaths: []string{kubeconfigPath},
		Toolsets:        []string{"kubernetes", "rancher"},
		ListOutput:      "json",
	}})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	defer server.Close()

	tools := server.GetEnabledTools()
	assertToolsPresent(t, tools, "cluster_list", "kubernetes_get", "kubernetes_list")
	assertToolsAbsent(t, tools, "project_list")

	kubernetesStatus := server.GetHealthStatus().Capabilities["kubernetes"]
	if !kubernetesStatus.Configured || !kubernetesStatus.Available {
		t.Fatalf("kubernetes status = %+v, want configured and available", kubernetesStatus)
	}
	if strings.Contains(kubernetesStatus.Reason, "rancher configuration missing") {
		t.Fatalf("kubernetes status has Rancher-only reason: %+v", kubernetesStatus)
	}
}

func TestNewServer_KubeconfigOnlyDispatchesRegisteredKubernetesTool(t *testing.T) {
	var requestPath string
	apiServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/namespaces/default/pods" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"apiVersion":"v1","kind":"PodList","items":[{"apiVersion":"v1","kind":"Pod","metadata":{"name":"direct-pod","namespace":"default"}}]}`))
	}))
	defer apiServer.Close()

	server, err := NewServer(Configuration{StaticConfig: &config.StaticConfig{
		KubeconfigPaths: []string{writeServerKubeconfigWithInsecureTLS(t, "direct", apiServer.URL)},
		Toolsets:        []string{"kubernetes", "rancher"},
		ListOutput:      "json",
	}})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	defer server.Close()

	client := newServerInProcessClient(t, server)
	result, err := client.CallTool(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{
		Name: "kubernetes_list",
		Arguments: map[string]interface{}{
			"cluster":   "kubeconfig:direct",
			"kind":      "pod",
			"namespace": "default",
			"format":    "json",
		},
	}})
	if err != nil {
		t.Fatalf("CallTool(kubernetes_list) error = %v", err)
	}
	if result.IsError {
		t.Fatalf("CallTool(kubernetes_list) error result = %#v", result.Content)
	}
	if len(result.Content) != 1 {
		t.Fatalf("CallTool(kubernetes_list) content count = %d, want 1", len(result.Content))
	}
	text, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("CallTool(kubernetes_list) content = %T, want mcp.TextContent", result.Content[0])
	}
	if !strings.Contains(text.Text, "direct-pod") {
		t.Fatalf("CallTool(kubernetes_list) output = %q, want direct kubeconfig pod", text.Text)
	}
	if requestPath != "/api/v1/namespaces/default/pods" {
		t.Fatalf("Kubernetes API path = %q, want direct kubeconfig API path", requestPath)
	}
	if strings.Contains(requestPath, "/k8s/clusters/") {
		t.Fatalf("Kubernetes API path = %q must not use Rancher Steve routing", requestPath)
	}
}

func TestNewServer_KubeconfigOnlyPublishesSourceDescriptions(t *testing.T) {
	server, err := NewServer(Configuration{StaticConfig: &config.StaticConfig{
		KubeconfigPaths: []string{writeServerKubeconfig(t, "direct", "https://direct.example.test")},
		Toolsets:        []string{"kubernetes", "rancher"},
		ListOutput:      "json",
	}})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	defer server.Close()

	client := newServerInProcessClient(t, server)
	result, err := client.ListTools(context.Background(), mcp.ListToolsRequest{})
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}
	descriptions := make(map[string]string, len(result.Tools))
	for _, tool := range result.Tools {
		descriptions[tool.Name] = tool.Description
	}

	if got, want := descriptions["cluster_list"], "List all available clusters Applicable cluster sources: rancher, kubeconfig."; got != want {
		t.Fatalf("cluster_list description = %q, want %q", got, want)
	}
	if got, want := descriptions["kubernetes_list"], "List Kubernetes resources by kind and optional namespace. Supports label selectors for filtering. Applicable cluster sources: rancher, kubeconfig."; got != want {
		t.Fatalf("kubernetes_list description = %q, want %q", got, want)
	}
}

func TestNewServer_MixedSourcesRegistersRancherAndKubeconfigTools(t *testing.T) {
	rancherServer := startRancherSchemaServer(t)
	kubeconfigPath := writeServerKubeconfig(t, "direct", "https://direct.example.test")
	server, err := NewServer(Configuration{StaticConfig: &config.StaticConfig{
		RancherServerURL:   rancherServer.URL,
		RancherToken:       "fixture-token",
		RancherTLSInsecure: true,
		KubeconfigPaths:    []string{kubeconfigPath},
		Toolsets:           []string{"kubernetes", "rancher"},
		ListOutput:         "json",
	}})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	defer server.Close()

	assertToolsPresent(t, server.GetEnabledTools(), "cluster_list", "project_list", "kubernetes_get", "kubernetes_list")
}

func TestNewServer_RejectsInvalidKubeconfigPaths(t *testing.T) {
	directory := t.TempDir()
	malformedPath := filepath.Join(directory, "malformed.yaml")
	if err := os.WriteFile(malformedPath, []byte("contexts: ["), 0o600); err != nil {
		t.Fatalf("write malformed fixture: %v", err)
	}
	emptyPath := filepath.Join(directory, "empty.yaml")
	if err := os.WriteFile(emptyPath, []byte("apiVersion: v1\nkind: Config\n"), 0o600); err != nil {
		t.Fatalf("write empty fixture: %v", err)
	}

	for _, path := range []string{filepath.Join(directory, "missing.yaml"), malformedPath, emptyPath} {
		t.Run(filepath.Base(path), func(t *testing.T) {
			_, err := NewServer(Configuration{StaticConfig: &config.StaticConfig{
				KubeconfigPaths: []string{path},
				ListOutput:      "json",
			}})
			if err == nil || !strings.Contains(err.Error(), "kubeconfig") {
				t.Fatalf("NewServer() error = %v, want readable kubeconfig startup error", err)
			}
		})
	}
}

func writeServerKubeconfig(t *testing.T, contextName, server string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := "apiVersion: v1\n" +
		"kind: Config\n" +
		"clusters:\n" +
		"- name: target\n" +
		"  cluster:\n" +
		"    server: " + server + "\n" +
		"contexts:\n" +
		"- name: " + contextName + "\n" +
		"  context:\n" +
		"    cluster: target\n" +
		"    user: fixture-user\n" +
		"users:\n" +
		"- name: fixture-user\n" +
		"  user: {}\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write kubeconfig fixture: %v", err)
	}
	return path
}

func writeServerKubeconfigWithInsecureTLS(t *testing.T, contextName, server string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := "apiVersion: v1\n" +
		"kind: Config\n" +
		"clusters:\n" +
		"- name: target\n" +
		"  cluster:\n" +
		"    server: " + server + "\n" +
		"    insecure-skip-tls-verify: true\n" +
		"contexts:\n" +
		"- name: " + contextName + "\n" +
		"  context:\n" +
		"    cluster: target\n" +
		"    user: fixture-user\n" +
		"users:\n" +
		"- name: fixture-user\n" +
		"  user: {}\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write kubeconfig fixture: %v", err)
	}
	return path
}

func newServerInProcessClient(t *testing.T, server *Server) *mcpclient.Client {
	t.Helper()
	client, err := mcpclient.NewInProcessClient(server.server)
	if err != nil {
		t.Fatalf("NewInProcessClient() error = %v", err)
	}
	t.Cleanup(func() {
		_ = client.Close()
	})
	if err := client.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	_, err = client.Initialize(context.Background(), mcp.InitializeRequest{Params: mcp.InitializeParams{
		ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
		ClientInfo:      mcp.Implementation{Name: "mcp-server-test", Version: "1.0.0"},
	}})
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	return client
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

func TestContextFunc_PreservesPresentEmptyAuthorization(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "")
	req.Header.Set("R_token", "fallback-token")

	_, err := bearerTokenFromContext(contextFunc(context.Background(), req))
	if err == nil || !strings.Contains(err.Error(), "malformed Authorization header") {
		t.Fatalf("expected malformed Authorization error for a present empty header, got %v", err)
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
