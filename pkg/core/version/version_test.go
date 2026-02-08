package version

import (
	"strings"
	"testing"
)

func TestGetVersionInfo(t *testing.T) {
	info := GetVersionInfo()

	// Check that all expected components are present
	expectedComponents := []string{
		BinaryName,
		"Version:",
		"Git commit:",
		"Built:",
		"Go version:",
		"Platform:",
	}

	for _, component := range expectedComponents {
		if !strings.Contains(info, component) {
			t.Errorf("Version info should contain '%s', but got:\n%s", component, info)
		}
	}

	// Check that version is set
	if Version == "" {
		t.Error("Version should not be empty")
	}

	// Check that binary name is set
	if BinaryName == "" {
		t.Error("BinaryName should not be empty")
	}

	// Check that platform contains expected format
	if !strings.Contains(Platform, "/") {
		t.Errorf("Platform should contain '/', but got: %s", Platform)
	}
}

func TestConstants(t *testing.T) {
	if BinaryName != "rancher-mcp-server" {
		t.Errorf("Expected BinaryName to be 'rancher-mcp-server', got: %s", BinaryName)
	}

	if GoVersion == "" {
		t.Error("GoVersion should not be empty")
	}
}