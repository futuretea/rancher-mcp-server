package mcp

import (
	"testing"

	"github.com/futuretea/rancher-mcp-server/pkg/core/config"
)

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
