package cmd

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/futuretea/rancher-mcp-server/pkg/core/config"
	"github.com/futuretea/rancher-mcp-server/pkg/core/logging"
	"github.com/spf13/viper"
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

	execFlag := cmd.Flags().Lookup("enable-container-exec")
	if execFlag == nil {
		t.Error("Command should have an enable-container-exec flag")
	}
}

func TestValidateRequestTokenAuthMode_StdioRejected(t *testing.T) {
	cfg := &config.StaticConfig{
		Port:                    0,
		RancherRequestTokenAuth: true,
	}
	if err := validateRequestTokenAuthMode(cfg); err == nil {
		t.Fatal("expected stdio mode with request token auth to be rejected")
	}
}

func TestValidateRequestTokenAuthMode_HTTPSAllowed(t *testing.T) {
	cfg := &config.StaticConfig{
		Port:                    8080,
		RancherRequestTokenAuth: true,
	}
	if err := validateRequestTokenAuthMode(cfg); err != nil {
		t.Fatalf("expected HTTP/SSE mode with request token auth to be allowed, got: %v", err)
	}
}

func TestRequestTokenAuthFlagDefined(t *testing.T) {
	streams := IOStreams{
		In:     &bytes.Buffer{},
		Out:    &bytes.Buffer{},
		ErrOut: &bytes.Buffer{},
	}

	cmd := NewMCPServer(streams)
	if cmd.Flags().Lookup("rancher-request-token-auth") == nil {
		t.Error("Command should have a rancher-request-token-auth flag")
	}
}

func TestKubeconfigPathsFlagDefined(t *testing.T) {
	streams := IOStreams{
		In:     &bytes.Buffer{},
		Out:    &bytes.Buffer{},
		ErrOut: &bytes.Buffer{},
	}

	cmd := NewMCPServer(streams)
	if cmd.Flags().Lookup("kubeconfig-paths") == nil {
		t.Fatal("command should have a kubeconfig-paths flag")
	}
}

func TestKubeconfigPathsFlagBindsOrderedValues(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	streams := IOStreams{
		In:     &bytes.Buffer{},
		Out:    &bytes.Buffer{},
		ErrOut: &bytes.Buffer{},
	}
	cmd := NewMCPServer(streams)
	if err := cmd.ParseFlags([]string{"--kubeconfig-paths", "/first.yaml,/second.yaml"}); err != nil {
		t.Fatalf("ParseFlags() error = %v", err)
	}
	if err := bindFlags(cmd); err != nil {
		t.Fatalf("bindFlags() error = %v", err)
	}

	cfg, err := config.LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if want := []string{"/first.yaml", "/second.yaml"}; !reflect.DeepEqual(cfg.KubeconfigPaths, want) {
		t.Fatalf("KubeconfigPaths = %#v, want %#v", cfg.KubeconfigPaths, want)
	}
}

func TestWarnKubeconfigHTTPExposure(t *testing.T) {
	output := &bytes.Buffer{}
	logging.SetStdioMode(false)
	logging.Initialize(5, output)

	warnKubeconfigHTTPExposure(&config.StaticConfig{
		Port:            8080,
		KubeconfigPaths: []string{"/etc/rancher-mcp/clusters.yaml"},
	})
	if !strings.Contains(output.String(), "kubeconfig") || !strings.Contains(output.String(), "unauthenticated") {
		t.Fatalf("warning output = %q, want kubeconfig exposure warning", output.String())
	}
}

func TestStdioRequestTokenAuthRejectedByCommand(t *testing.T) {
	streams := IOStreams{
		In:     &bytes.Buffer{},
		Out:    &bytes.Buffer{},
		ErrOut: &bytes.Buffer{},
	}

	cmd := NewMCPServer(streams)
	cmd.SetArgs([]string{
		"--rancher-request-token-auth",
		"--rancher-server-url", "https://rancher.example.com",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected command to fail in stdio mode with request token auth")
	}
	if !strings.Contains(err.Error(), "rancher_request_token_auth is not supported in stdio mode") {
		t.Fatalf("expected stdio rejection message, got: %v", err)
	}
}

func TestOAuthConfigurationFlagsMatchAudienceFreeContract(t *testing.T) {
	streams := IOStreams{
		In:     &bytes.Buffer{},
		Out:    &bytes.Buffer{},
		ErrOut: &bytes.Buffer{},
	}

	cmd := NewMCPServer(streams)
	for _, name := range []string{
		"rancher-oauth-token-auth",
		"rancher-oauth-authorization-server-url",
		"rancher-oauth-jwks-url",
		"rancher-oauth-resource-url",
	} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("command should have a --%s flag", name)
		}
	}
	if cmd.Flags().Lookup("rancher-oauth-audience") != nil {
		t.Error("command must not expose a --rancher-oauth-audience flag")
	}
}

func TestStdioOAuthRejectedByCommand(t *testing.T) {
	streams := IOStreams{
		In:     &bytes.Buffer{},
		Out:    &bytes.Buffer{},
		ErrOut: &bytes.Buffer{},
	}

	cmd := NewMCPServer(streams)
	cmd.SetArgs([]string{
		"--rancher-server-url", "https://rancher.example.test",
		"--rancher-oauth-token-auth",
		"--rancher-oauth-authorization-server-url", "https://auth.example.test",
		"--rancher-oauth-jwks-url", "https://auth.example.test/jwks",
		"--rancher-oauth-resource-url", "https://mcp.example.test",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected OAuth mode to be rejected in stdio mode")
	}
	if !strings.Contains(err.Error(), "rancher_oauth_token_auth is not supported in stdio mode") {
		t.Fatalf("expected OAuth stdio rejection message, got: %v", err)
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
