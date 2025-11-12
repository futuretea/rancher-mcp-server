package cmd

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/futuretea/rancher-mcp-server/pkg/config"
	internalhttp "github.com/futuretea/rancher-mcp-server/pkg/http"
	"github.com/futuretea/rancher-mcp-server/pkg/mcp"
	"github.com/futuretea/rancher-mcp-server/pkg/version"
)

// IOStreams represents standard input, output, and error streams
type IOStreams struct {
	In     io.Reader
	Out    io.Writer
	ErrOut io.Writer
}

// bindFlags binds command-line flags to viper configuration keys
func bindFlags(cmd *cobra.Command) {
	// Map of viper config key to flag name
	flagBindings := map[string]string{
		// Server configuration
		"port":         "port",
		"sse_base_url": "sse-base-url",
		"log_level":    "log-level",
		// Rancher configuration
		"rancher_server_url":    "rancher-server-url",
		"rancher_token":         "rancher-token",
		"rancher_access_key":    "rancher-access-key",
		"rancher_secret_key":    "rancher-secret-key",
		"rancher_tls_insecure":  "rancher-tls-insecure",
		// Security configuration
		"read_only":           "read-only",
		"disable_destructive": "disable-destructive",
		// Output configuration
		"list_output": "list-output",
		// Toolset configuration
		"toolsets":       "toolsets",
		"enabled_tools":  "enabled-tools",
		"disabled_tools": "disabled-tools",
	}

	for key, flag := range flagBindings {
		viper.BindPFlag(key, cmd.Flags().Lookup(flag))
	}
}

// NewMCPServer creates a new cobra command for the Rancher MCP Server
func NewMCPServer(streams IOStreams) *cobra.Command {
	var cfgFile string

	cmd := &cobra.Command{
		Use:   "rancher-mcp-server",
		Short: "Rancher MCP Server - Model Context Protocol server for Rancher multi-cluster management",
		Long: `Rancher MCP Server is a Model Context Protocol (MCP) server that provides
access to Rancher multi-cluster management capabilities through the MCP protocol.

This server can run in stdio mode for integration with MCP clients or in HTTP mode
for network access.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			bindFlags(cmd)
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServer(cfgFile, streams)
		},
	}

	// Set output streams for the command
	cmd.SetOut(streams.Out)
	cmd.SetErr(streams.ErrOut)

	// Add configuration file flag
	cmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file path (supports YAML)")

	// Server configuration flags
	cmd.Flags().Int("port", 0, "Port to listen on for HTTP/SSE mode (0 for stdio mode)")
	cmd.Flags().String("sse-base-url", "", "SSE public base URL to use when sending the endpoint message (e.g. https://example.com)")
	cmd.Flags().Int("log-level", 0, "Log level (0-9)")

	// Rancher configuration flags
	cmd.Flags().String("rancher-server-url", "", "Rancher server URL")
	cmd.Flags().String("rancher-token", "", "Rancher bearer token")
	cmd.Flags().String("rancher-access-key", "", "Rancher access key")
	cmd.Flags().String("rancher-secret-key", "", "Rancher secret key")
	cmd.Flags().Bool("rancher-tls-insecure", false, "Rancher server tls insecure")

	// Security configuration flags
	cmd.Flags().Bool("read-only", false, "Run in read-only mode")
	cmd.Flags().Bool("disable-destructive", false, "Disable destructive operations")

	// Output configuration flags
	cmd.Flags().String("list-output", "table", "Output format for list operations (table, yaml, json)")

	// Toolset configuration flags
	cmd.Flags().StringSlice("toolsets", []string{"config", "core", "rancher"}, "Comma-separated list of toolsets to enable")
	cmd.Flags().StringSlice("enabled-tools", []string{}, "Comma-separated list of tools to enable")
	cmd.Flags().StringSlice("disabled-tools", []string{}, "Comma-separated list of tools to disable")

	// Add version command
	cmd.AddCommand(newVersionCommand(streams))

	return cmd
}

// runServer runs the MCP server with the given configuration
func runServer(cfgFile string, streams IOStreams) error {
	// Load configuration from file, environment variables, and command-line flags
	cfg, err := config.LoadConfig(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create MCP server configuration
	mcpConfig := mcp.Configuration{
		StaticConfig: cfg,
	}

	// Create MCP server
	server, err := mcp.NewServer(mcpConfig)
	if err != nil {
		return fmt.Errorf("failed to create MCP server: %v", err)
	}
	defer server.Close()

	// Start server based on port configuration
	if cfg.Port == 0 {
		// Stdio mode
		fmt.Fprintf(streams.ErrOut, "Starting Rancher MCP Server in stdio mode\n")
		fmt.Fprintf(streams.ErrOut, "Enabled tools: %v\n", server.GetEnabledTools())
		return server.ServeStdio()
	} else {
		// HTTP/SSE mode
		fmt.Fprintf(streams.ErrOut, "Starting Rancher MCP Server in HTTP/SSE mode on port %d\n", cfg.Port)
		fmt.Fprintf(streams.ErrOut, "Enabled tools: %v\n", server.GetEnabledTools())
		if cfg.SSEBaseURL != "" {
			fmt.Fprintf(streams.ErrOut, "SSE Base URL: %s\n", cfg.SSEBaseURL)
		}

		ctx := context.Background()
		return internalhttp.Serve(ctx, server, cfg)
	}
}

// newVersionCommand creates the version command
func newVersionCommand(streams IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(streams.Out, "%s\n", version.GetVersionInfo())
		},
	}

	// Set output streams for the command
	cmd.SetOut(streams.Out)
	cmd.SetErr(streams.ErrOut)

	return cmd
}