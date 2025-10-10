package cmd

import (
	"fmt"
	"io"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/futuretea/rancher-mcp-server/pkg/config"
	"github.com/futuretea/rancher-mcp-server/pkg/mcp"
	"github.com/futuretea/rancher-mcp-server/pkg/version"
)

// IOStreams represents standard input, output, and error streams
type IOStreams struct {
	In     io.Reader
	Out    io.Writer
	ErrOut io.Writer
}

// NewMCPServer creates a new cobra command for the Rancher MCP Server
func NewMCPServer(streams IOStreams) *cobra.Command {
	cfg := config.DefaultConfig()

	cmd := &cobra.Command{
		Use:   "rancher-mcp-server",
		Short: "Rancher MCP Server - Model Context Protocol server for Rancher multi-cluster management",
		Long: `Rancher MCP Server is a Model Context Protocol (MCP) server that provides
access to Rancher multi-cluster management capabilities through the MCP protocol.

This server can run in stdio mode for integration with MCP clients or in HTTP mode
for network access.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServer(cfg, streams)
		},
	}

	// Set output streams for the command
	cmd.SetOut(streams.Out)
	cmd.SetErr(streams.ErrOut)

	// Add flags
	cmd.Flags().IntVar(&cfg.Port, "port", cfg.Port, "Port to listen on for HTTP mode (0 for stdio mode)")
	cmd.Flags().IntVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "Log level (0-9)")
	cmd.Flags().StringVar(&cfg.RancherServerURL, "rancher-server-url", cfg.RancherServerURL, "Rancher server URL")
	cmd.Flags().StringVar(&cfg.RancherToken, "rancher-token", cfg.RancherToken, "Rancher bearer token")
	cmd.Flags().StringVar(&cfg.RancherAccessKey, "rancher-access-key", cfg.RancherAccessKey, "Rancher access key")
	cmd.Flags().StringVar(&cfg.RancherSecretKey, "rancher-secret-key", cfg.RancherSecretKey, "Rancher secret key")
	cmd.Flags().BoolVar(&cfg.ReadOnly, "read-only", cfg.ReadOnly, "Run in read-only mode")
	cmd.Flags().BoolVar(&cfg.DisableDestructive, "disable-destructive", cfg.DisableDestructive, "Disable destructive operations")
	cmd.Flags().StringVar(&cfg.ListOutput, "list-output", cfg.ListOutput, "Output format for list operations (table, yaml)")
	cmd.Flags().StringSliceVar(&cfg.Toolsets, "toolsets", cfg.Toolsets, "Comma-separated list of toolsets to enable")
	cmd.Flags().StringSliceVar(&cfg.EnabledTools, "enabled-tools", cfg.EnabledTools, "Comma-separated list of tools to enable")
	cmd.Flags().StringSliceVar(&cfg.DisabledTools, "disabled-tools", cfg.DisabledTools, "Comma-separated list of tools to disable")

	// Add version command
	cmd.AddCommand(newVersionCommand(streams))

	return cmd
}

// runServer runs the MCP server with the given configuration
func runServer(cfg *config.StaticConfig, streams IOStreams) error {
	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("configuration validation failed: %v", err)
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
		// HTTP mode
		fmt.Fprintf(streams.ErrOut, "Starting Rancher MCP Server in HTTP mode on port %d\n", cfg.Port)
		fmt.Fprintf(streams.ErrOut, "Enabled tools: %v\n", server.GetEnabledTools())

		httpServer := &http.Server{
			Addr: fmt.Sprintf(":%d", cfg.Port),
		}

		httpMux := server.ServeHTTP(httpServer)
		return httpMux.Start(fmt.Sprintf(":%d", cfg.Port))
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