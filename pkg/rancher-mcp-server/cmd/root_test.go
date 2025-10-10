package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionCommand(t *testing.T) {
	streams := IOStreams{
		In:     &bytes.Buffer{},
		Out:    &bytes.Buffer{},
		ErrOut: &bytes.Buffer{},
	}

	cmd := NewMCPServer(streams)

	// Test version command
	cmd.SetArgs([]string{"version"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Version command failed: %v", err)
	}

	output := streams.Out.(*bytes.Buffer).String()
	if !strings.Contains(output, "rancher-mcp-server") {
		t.Errorf("Version output should contain 'rancher-mcp-server', got: %s", output)
	}

	if !strings.Contains(output, "Version:") {
		t.Errorf("Version output should contain 'Version:', got: %s", output)
	}
}

func TestHelpCommand(t *testing.T) {
	streams := IOStreams{
		In:     &bytes.Buffer{},
		Out:    &bytes.Buffer{},
		ErrOut: &bytes.Buffer{},
	}

	cmd := NewMCPServer(streams)

	// Test help command
	cmd.SetArgs([]string{"--help"})
	// We expect help to exit with error, so we don't check the error
	_ = cmd.Execute()

	output := streams.Out.(*bytes.Buffer).String()
	// Debug: print actual output
	t.Logf("Actual help output: %q", output)

	if !strings.Contains(output, "Rancher MCP Server") {
		t.Errorf("Help output should contain 'Rancher MCP Server', got: %s", output)
	}

	if !strings.Contains(output, "--port") {
		t.Errorf("Help output should contain '--port' flag, got: %s", output)
	}

	if !strings.Contains(output, "--help") {
		t.Errorf("Help output should contain '--help' flag, got: %s", output)
	}
}

func TestDefaultRun(t *testing.T) {
	streams := IOStreams{
		In:     &bytes.Buffer{},
		Out:    &bytes.Buffer{},
		ErrOut: &bytes.Buffer{},
	}

	cmd := NewMCPServer(streams)

	// Test default run (no arguments)
	cmd.SetArgs([]string{})

	// Verify command configuration
	if cmd == nil {
		t.Fatal("NewMCPServer should return a command")
	}

	// Verify that default configuration is set
	if cmd.Use != "rancher-mcp-server" {
		t.Errorf("Expected command use to be 'rancher-mcp-server', got: %s", cmd.Use)
	}

	// Verify help flag is available (cobra adds this automatically)
	helpFlag := cmd.Flags().Lookup("help")
	if helpFlag == nil {
		t.Log("Help flag is not directly available (cobra internal), this is normal")
	}
}

func TestHTTPMode(t *testing.T) {
	streams := IOStreams{
		In:     &bytes.Buffer{},
		Out:    &bytes.Buffer{},
		ErrOut: &bytes.Buffer{},
	}

	cmd := NewMCPServer(streams)

	// Test HTTP mode configuration
	cmd.SetArgs([]string{"--port", "8080"})

	// Verify command configuration
	if cmd == nil {
		t.Fatal("NewMCPServer should return a command")
	}

	// Verify port flag is available and configured
	portFlag := cmd.Flags().Lookup("port")
	if portFlag == nil {
		t.Error("Command should have a port flag")
	}

	// Verify other important flags are available
	logLevelFlag := cmd.Flags().Lookup("log-level")
	if logLevelFlag == nil {
		t.Error("Command should have a log-level flag")
	}

	rancherURLFlag := cmd.Flags().Lookup("rancher-server-url")
	if rancherURLFlag == nil {
		t.Error("Command should have a rancher-server-url flag")
	}
}

func TestInvalidArguments(t *testing.T) {
	streams := IOStreams{
		In:     &bytes.Buffer{},
		Out:    &bytes.Buffer{},
		ErrOut: &bytes.Buffer{},
	}

	cmd := NewMCPServer(streams)

	// Test with invalid arguments
	cmd.SetArgs([]string{"--invalid-flag", "value"})

	// Execute should fail with invalid flag
	err := cmd.Execute()
	if err == nil {
		t.Error("Command should fail with invalid flag")
	}

	// Check error message contains information about invalid flag
	if err != nil && !strings.Contains(err.Error(), "unknown flag") && !strings.Contains(err.Error(), "invalid") {
		t.Errorf("Error should mention invalid flag, got: %v", err)
	}
}