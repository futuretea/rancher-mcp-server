package mcp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/futuretea/rancher-mcp-server/pkg/core/config"
	"github.com/futuretea/rancher-mcp-server/pkg/core/logging"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset"
	"github.com/mark3labs/mcp-go/mcp"
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

	// Check that we have our expected tools
	expectedTools := []string{"cluster_list", "project_list", "kubernetes_get", "kubernetes_list", "kubernetes_describe", "kubernetes_events"}
	for _, expected := range expectedTools {
		found := false
		for _, actual := range tools {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected tool '%s' not found in registered tools", expected)
		}
	}
}

func TestNewTextResult(t *testing.T) {
	// Test success case
	result := NewTextResult("success message", nil)
	if result.IsError {
		t.Error("Result should not be an error")
	}

	if len(result.Content) != 1 {
		t.Errorf("Expected 1 content item, got %d", len(result.Content))
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("Content should be TextContent")
	}

	if textContent.Text != "success message" {
		t.Errorf("Expected 'success message', got '%s'", textContent.Text)
	}

	// Test error case
	err := fmt.Errorf("test error")
	result = NewTextResult("", err)
	if !result.IsError {
		t.Error("Result should be an error")
	}

	if len(result.Content) != 1 {
		t.Errorf("Expected 1 content item, got %d", len(result.Content))
	}

	textContent, ok = result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("Content should be TextContent")
	}

	if textContent.Text != "test error" {
		t.Errorf("Expected 'test error', got '%s'", textContent.Text)
	}
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

func TestToolArgumentKeys(t *testing.T) {
	got := toolArgumentKeys(map[string]interface{}{
		"content": "base64-secret",
		"cluster": "c1",
		"command": []interface{}{"sh", "-c", "printenv SECRET"},
	})
	want := []string{"cluster", "command", "content"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("toolArgumentKeys() = %#v, want %#v", got, want)
	}
	if strings.Contains(fmt.Sprint(got), "base64-secret") || strings.Contains(fmt.Sprint(got), "SECRET") {
		t.Fatalf("toolArgumentKeys() leaked argument values: %v", got)
	}
}

func TestToolHandlerLogsArgumentKeysOnly(t *testing.T) {
	var logBuf bytes.Buffer
	logging.Initialize(6, &logBuf)
	t.Cleanup(func() {
		logging.Initialize(0, io.Discard)
	})

	s := &Server{}
	handler := s.makeToolHandler(toolset.ServerTool{
		Tool: mcp.Tool{Name: "test_tool"},
		Handler: func(_ context.Context, _ interface{}, _ map[string]interface{}) (string, error) {
			return "ok", nil
		},
	})

	_, err := handler(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]interface{}{
				"cluster": "c1",
				"content": "base64-secret",
				"command": []interface{}{"sh", "-c", "printenv SECRET"},
			},
		},
	})
	if err != nil {
		t.Fatalf("tool handler returned error: %v", err)
	}

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "param keys") {
		t.Fatalf("log output = %q, want param keys log", logOutput)
	}
	for _, wantKey := range []string{"cluster", "command", "content"} {
		if !strings.Contains(logOutput, wantKey) {
			t.Fatalf("log output = %q, want key %q", logOutput, wantKey)
		}
	}
	for _, leakedValue := range []string{"c1", "base64-secret", "sh", "printenv SECRET"} {
		if strings.Contains(logOutput, leakedValue) {
			t.Fatalf("log output leaked argument value %q: %s", leakedValue, logOutput)
		}
	}
}

// TestFileToolFlagsExcluded tests that file tools are excluded when config flags are false.
func TestFileToolFlagsExcluded(t *testing.T) {
	cfg := &config.StaticConfig{
		RancherServerURL:            "https://rancher.example.com",
		RancherAccessKey:            "test-key",
		RancherSecretKey:            "test-secret",
		EnableContainerFileUpload:   false,
		EnableContainerFileDownload: false,
	}
	mcpConfig := Configuration{StaticConfig: cfg}

	server, err := NewServer(mcpConfig)
	if err != nil {
		if server == nil {
			t.Fatal("Server should be created even with fake credentials")
		}
		return
	}

	tools := server.GetEnabledTools()

	// File tools should NOT be in the enabled tools list
	for _, toolName := range tools {
		if toolName == "kubernetes_upload_file" {
			t.Error("kubernetes_upload_file should be excluded when EnableContainerFileUpload is false")
		}
		if toolName == "kubernetes_download_file" {
			t.Error("kubernetes_download_file should be excluded when EnableContainerFileDownload is false")
		}
	}
}

// TestFileToolFlagsIncluded tests that file tools are included when config flags are true.
func TestFileToolFlagsIncluded(t *testing.T) {
	cfg := &config.StaticConfig{
		RancherServerURL:            "https://rancher.example.com",
		RancherAccessKey:            "test-key",
		RancherSecretKey:            "test-secret",
		EnableContainerFileUpload:   true,
		EnableContainerFileDownload: true,
	}
	mcpConfig := Configuration{StaticConfig: cfg}

	server, err := NewServer(mcpConfig)
	if err != nil {
		if server == nil {
			t.Fatal("Server should be created even with fake credentials")
		}
		return
	}

	tools := server.GetEnabledTools()

	// File tools should be in the enabled tools list
	hasUpload := false
	hasDownload := false
	for _, toolName := range tools {
		if toolName == "kubernetes_upload_file" {
			hasUpload = true
		}
		if toolName == "kubernetes_download_file" {
			hasDownload = true
		}
	}

	if !hasUpload {
		t.Error("kubernetes_upload_file should be included when EnableContainerFileUpload is true")
	}
	if !hasDownload {
		t.Error("kubernetes_download_file should be included when EnableContainerFileDownload is true")
	}
}

// TestFileToolsWithDisabledToolsList tests that file tools can be excluded via disabled_tools list.
func TestFileToolsWithDisabledToolsList(t *testing.T) {
	cfg := &config.StaticConfig{
		RancherServerURL:            "https://rancher.example.com",
		RancherAccessKey:            "test-key",
		RancherSecretKey:            "test-secret",
		EnableContainerFileUpload:   true,
		EnableContainerFileDownload: true,
		DisabledTools:               []string{"kubernetes_upload_file"},
	}
	mcpConfig := Configuration{StaticConfig: cfg}

	server, err := NewServer(mcpConfig)
	if err != nil {
		if server == nil {
			t.Fatal("Server should be created even with fake credentials")
		}
		return
	}

	tools := server.GetEnabledTools()

	// Upload should be disabled via disabled_tools, download should still be enabled
	hasUpload := false
	hasDownload := false
	for _, toolName := range tools {
		if toolName == "kubernetes_upload_file" {
			hasUpload = true
		}
		if toolName == "kubernetes_download_file" {
			hasDownload = true
		}
	}

	if hasUpload {
		t.Error("kubernetes_upload_file should be excluded via disabled_tools list")
	}
	if !hasDownload {
		t.Error("kubernetes_download_file should still be enabled")
	}
}

// TestFileToolsWithEnabledToolsList tests that file tools can be included via enabled_tools list.
func TestFileToolsWithEnabledToolsList(t *testing.T) {
	cfg := &config.StaticConfig{
		RancherServerURL:            "https://rancher.example.com",
		RancherAccessKey:            "test-key",
		RancherSecretKey:            "test-secret",
		EnableContainerFileUpload:   true,
		EnableContainerFileDownload: true,
		EnabledTools:                []string{"kubernetes_download_file", "cluster_list"},
	}
	mcpConfig := Configuration{StaticConfig: cfg}

	server, err := NewServer(mcpConfig)
	if err != nil {
		if server == nil {
			t.Fatal("Server should be created even with fake credentials")
		}
		return
	}

	tools := server.GetEnabledTools()

	// Only explicitly enabled tools should be present
	hasUpload := false
	hasDownload := false
	for _, toolName := range tools {
		if toolName == "kubernetes_upload_file" {
			hasUpload = true
		}
		if toolName == "kubernetes_download_file" {
			hasDownload = true
		}
	}

	if hasUpload {
		t.Error("kubernetes_upload_file should be excluded (not in enabled_tools list)")
	}
	if !hasDownload {
		t.Error("kubernetes_download_file should be included (in enabled_tools list)")
	}
}

// TestFileToolsUploadOnlyEnabled tests only upload is enabled.
func TestFileToolsUploadOnlyEnabled(t *testing.T) {
	cfg := &config.StaticConfig{
		RancherServerURL:            "https://rancher.example.com",
		RancherAccessKey:            "test-key",
		RancherSecretKey:            "test-secret",
		EnableContainerFileUpload:   true,
		EnableContainerFileDownload: false,
	}
	mcpConfig := Configuration{StaticConfig: cfg}

	server, err := NewServer(mcpConfig)
	if err != nil {
		if server == nil {
			t.Fatal("Server should be created even with fake credentials")
		}
		return
	}

	tools := server.GetEnabledTools()

	hasUpload := false
	hasDownload := false
	for _, toolName := range tools {
		if toolName == "kubernetes_upload_file" {
			hasUpload = true
		}
		if toolName == "kubernetes_download_file" {
			hasDownload = true
		}
	}

	if !hasUpload {
		t.Error("kubernetes_upload_file should be enabled")
	}
	if hasDownload {
		t.Error("kubernetes_download_file should be disabled")
	}
}

// TestFileToolsDownloadOnlyEnabled tests only download is enabled.
func TestFileToolsDownloadOnlyEnabled(t *testing.T) {
	cfg := &config.StaticConfig{
		RancherServerURL:            "https://rancher.example.com",
		RancherAccessKey:            "test-key",
		RancherSecretKey:            "test-secret",
		EnableContainerFileUpload:   false,
		EnableContainerFileDownload: true,
	}
	mcpConfig := Configuration{StaticConfig: cfg}

	server, err := NewServer(mcpConfig)
	if err != nil {
		if server == nil {
			t.Fatal("Server should be created even with fake credentials")
		}
		return
	}

	tools := server.GetEnabledTools()

	hasUpload := false
	hasDownload := false
	for _, toolName := range tools {
		if toolName == "kubernetes_upload_file" {
			hasUpload = true
		}
		if toolName == "kubernetes_download_file" {
			hasDownload = true
		}
	}

	if hasUpload {
		t.Error("kubernetes_upload_file should be disabled")
	}
	if !hasDownload {
		t.Error("kubernetes_download_file should be enabled")
	}
}

// TestExecToolFlagExcluded tests that the exec tool is excluded when config flag is false.
func TestExecToolFlagExcluded(t *testing.T) {
	cfg := &config.StaticConfig{
		RancherServerURL:    "https://rancher.example.com",
		RancherAccessKey:    "test-key",
		RancherSecretKey:    "test-secret",
		EnableContainerExec: false,
	}
	mcpConfig := Configuration{StaticConfig: cfg}

	server, err := NewServer(mcpConfig)
	if err != nil {
		if server == nil {
			t.Fatal("Server should be created even with fake credentials")
		}
		return
	}

	for _, toolName := range server.GetEnabledTools() {
		if toolName == "kubernetes_exec" {
			t.Error("kubernetes_exec should be excluded when EnableContainerExec is false")
		}
	}
}

// TestExecToolFlagIncluded tests that the exec tool is included when explicitly enabled.
func TestExecToolFlagIncluded(t *testing.T) {
	cfg := &config.StaticConfig{
		RancherServerURL:    "https://rancher.example.com",
		RancherAccessKey:    "test-key",
		RancherSecretKey:    "test-secret",
		EnableContainerExec: true,
		ReadOnly:            false,
	}
	mcpConfig := Configuration{StaticConfig: cfg}

	server, err := NewServer(mcpConfig)
	if err != nil {
		if server == nil {
			t.Fatal("Server should be created even with fake credentials")
		}
		return
	}

	hasExec := false
	for _, toolName := range server.GetEnabledTools() {
		if toolName == "kubernetes_exec" {
			hasExec = true
		}
	}
	if !hasExec {
		t.Error("kubernetes_exec should be included when EnableContainerExec is true and ReadOnly is false")
	}
}

// TestExecToolReadOnlyExcluded tests that read-only mode suppresses the exec tool even when the flag is enabled.
func TestExecToolReadOnlyExcluded(t *testing.T) {
	cfg := &config.StaticConfig{
		RancherServerURL:    "https://rancher.example.com",
		RancherAccessKey:    "test-key",
		RancherSecretKey:    "test-secret",
		EnableContainerExec: true,
		ReadOnly:            true,
	}
	mcpConfig := Configuration{StaticConfig: cfg}

	server, err := NewServer(mcpConfig)
	if err != nil {
		if server == nil {
			t.Fatal("Server should be created even with fake credentials")
		}
		return
	}

	for _, toolName := range server.GetEnabledTools() {
		if toolName == "kubernetes_exec" {
			t.Error("kubernetes_exec should be excluded in read-only mode")
		}
	}
}

func TestValidateUniqueToolNamesRejectsDuplicateNames(t *testing.T) {
	duplicateTool := toolset.ServerTool{
		Tool: mcp.Tool{Name: "duplicate_tool"},
	}

	err := validateUniqueToolNames([]toolset.Toolset{
		staticToolset{name: "first", tools: []toolset.ServerTool{duplicateTool}},
		staticToolset{name: "second", tools: []toolset.ServerTool{duplicateTool}},
	}, nil)
	if err == nil {
		t.Fatal("expected duplicate tool name validation to fail")
	}
	if !strings.Contains(err.Error(), "duplicate tool name") {
		t.Fatalf("unexpected duplicate validation error: %v", err)
	}
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

type staticToolset struct {
	name  string
	tools []toolset.ServerTool
}

func (s staticToolset) GetName() string {
	return s.name
}

func (s staticToolset) GetDescription() string {
	return s.name
}

func (s staticToolset) GetTools(_ interface{}) []toolset.ServerTool {
	return s.tools
}
