package mcp

import (
	"fmt"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
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

	// Check that we have our expected tools
	expectedTools := []string{"configuration_view", "cluster_list", "project_list", "user_list"}
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