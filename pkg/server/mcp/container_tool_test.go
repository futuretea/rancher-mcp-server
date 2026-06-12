package mcp

import (
	"testing"

	"github.com/futuretea/rancher-mcp-server/pkg/core/config"
)

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
