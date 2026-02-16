package mcp

import (
	"context"
	"fmt"
	"net/http"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/futuretea/rancher-mcp-server/pkg/client/norman"
	"github.com/futuretea/rancher-mcp-server/pkg/client/steve"
	"github.com/futuretea/rancher-mcp-server/pkg/core/config"
	"github.com/futuretea/rancher-mcp-server/pkg/core/logging"
	"github.com/futuretea/rancher-mcp-server/pkg/core/version"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset/kubernetes"
	rancherToolset "github.com/futuretea/rancher-mcp-server/pkg/toolset/rancher"
)

// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

const authorizationKey contextKey = "Authorization"

// Configuration wraps the static configuration with additional runtime components
type Configuration struct {
	*config.StaticConfig
}

// Server represents the MCP server
type Server struct {
	configuration  *Configuration
	server         *server.MCPServer
	enabledTools   []string
	normanClient   *norman.Client
	steveClient    *steve.Client
	combinedClient *toolset.CombinedClient
}

// NewServer creates a new MCP server with the given configuration
func NewServer(configuration Configuration) (*Server, error) {
	// Note: Logging is initialized in root.go before calling NewServer
	// to properly handle stdio vs HTTP/SSE mode

	var serverOptions []server.ServerOption

	// Configure server capabilities
	serverOptions = append(serverOptions,
		server.WithResourceCapabilities(true, true),
		server.WithPromptCapabilities(true),
		server.WithToolCapabilities(true),
		server.WithLogging(),
	)

	// Initialize Norman client (for Rancher v3 API)
	normanClient, err := norman.NewClient(configuration.StaticConfig)
	if err != nil {
		// Log the error but continue without Norman client
		logging.Warn("Failed to create Norman client: %v", err)
		logging.Warn("Rancher tools will not be available")
	}

	// Initialize Steve client (for Steve API / Kubernetes resources)
	var steveClient *steve.Client
	if configuration.HasRancherConfig() {
		steveClient = steve.NewClient(
			configuration.RancherServerURL,
			configuration.RancherToken,
			configuration.RancherAccessKey,
			configuration.RancherSecretKey,
			configuration.RancherTLSInsecure,
		)
		logging.Info("Steve client initialized for Kubernetes resources")
	}

	s := &Server{
		configuration: &configuration,
		server:        server.NewMCPServer(version.BinaryName, version.Version, serverOptions...),
		normanClient:  normanClient,
		steveClient:   steveClient,
		combinedClient: &toolset.CombinedClient{
			Norman: normanClient,
			Steve:  steveClient,
		},
	}

	// Register tools
	if err := s.registerTools(); err != nil {
		return nil, err
	}

	return s, nil
}

// registerTools registers all available tools based on configuration
func (s *Server) registerTools() error {
	// Initialize toolsets
	availableToolsets := map[string]toolset.Toolset{
		"kubernetes": &kubernetes.Toolset{
			ReadOnly:           s.configuration.ReadOnly,
			DisableDestructive: s.configuration.DisableDestructive,
		},
		"rancher": &rancherToolset.Toolset{},
	}

	// Determine which toolsets to enable
	enabledToolsets := make([]toolset.Toolset, 0)
	if len(s.configuration.Toolsets) > 0 {
		// Use explicitly configured toolsets
		for _, toolsetName := range s.configuration.Toolsets {
			if ts, exists := availableToolsets[toolsetName]; exists {
				enabledToolsets = append(enabledToolsets, ts)
			}
		}
	} else {
		// Use all available toolsets
		for _, ts := range availableToolsets {
			enabledToolsets = append(enabledToolsets, ts)
		}
	}

	// Create combined client for toolsets that need both Norman and Steve clients
	combinedClient := s.combinedClient

	// Register tools from each enabled toolset
	for _, ts := range enabledToolsets {
		tools := ts.GetTools(combinedClient)
		for _, tool := range tools {
			// Check if tool is enabled/disabled by configuration
			if s.shouldEnableTool(tool.Tool.Name) {
				// Create a configured tool handler that uses server configuration
				configuredTool := s.configureTool(tool)
				if err := s.registerTool(configuredTool); err != nil {
					return fmt.Errorf("failed to register tool %s: %w", tool.Tool.Name, err)
				}
			}
		}
	}

	logging.Info("MCP server initialized with %d tools", len(s.enabledTools))
	return nil
}

// shouldEnableTool determines if a tool should be enabled based on configuration
func (s *Server) shouldEnableTool(toolName string) bool {
	// Check if tool is explicitly disabled
	for _, disabledTool := range s.configuration.DisabledTools {
		if disabledTool == toolName {
			return false
		}
	}

	// Check if tool is explicitly enabled
	if len(s.configuration.EnabledTools) > 0 {
		for _, enabledTool := range s.configuration.EnabledTools {
			if enabledTool == toolName {
				return true
			}
		}
		// If enabled tools are specified and this tool is not in the list, disable it
		return false
	}

	// Default: enable the tool
	return true
}

// configureTool creates a configured tool handler that uses server configuration
func (s *Server) configureTool(tool toolset.ServerTool) toolset.ServerTool {
	return toolset.ServerTool{
		Tool:        tool.Tool,
		Annotations: tool.Annotations,
		Handler: func(client interface{}, params map[string]interface{}) (string, error) {
			// Inject default output format if not specified
			if _, hasOutput := params["output"]; !hasOutput && s.configuration.ListOutput != "" {
				params["output"] = s.configuration.ListOutput
			}

			// Inject security parameters
			if s.configuration.ReadOnly {
				params["readOnly"] = true
			}
			if s.configuration.DisableDestructive {
				params["disableDestructive"] = true
			}

			// Inject output filters for resource cleanup
			if len(s.configuration.OutputFilters) > 0 {
				params["outputFilters"] = s.configuration.OutputFilters
			}

			// Admin policy: if show_sensitive_data is disabled, force mask regardless of per-call param
			if !s.configuration.ShowSensitiveData {
				params["showSensitiveData"] = false
			}

			return tool.Handler(client, params)
		},
	}
}

func contextFunc(ctx context.Context, r *http.Request) context.Context {
	if authHeader := r.Header.Get("Authorization"); authHeader != "" {
		return context.WithValue(ctx, authorizationKey, authHeader)
	}
	return ctx
}

// registerTool registers a single tool with the MCP server
func (s *Server) registerTool(tool toolset.ServerTool) error {
	toolHandler := server.ToolHandlerFunc(func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logging.Debug("Tool %s called with params: %v", tool.Tool.Name, request.Params.Arguments)

		// Convert arguments to the format expected by our tool handlers
		params := make(map[string]interface{})
		if arguments, ok := request.Params.Arguments.(map[string]interface{}); ok {
			for key, value := range arguments {
				params[key] = value
			}
		}

		result, err := tool.Handler(s.combinedClient, params)
		return NewTextResult(result, err), nil
	})

	// Use the simpler AddTool method
	s.server.AddTool(tool.Tool, toolHandler)
	s.enabledTools = append(s.enabledTools, tool.Tool.Name)

	logging.Info("Registered tool: %s", tool.Tool.Name)
	return nil
}

// ServeStdio starts the MCP server in stdio mode
func (s *Server) ServeStdio() error {
	logging.Info("Starting MCP server in stdio mode")
	return server.ServeStdio(s.server)
}

// ServeSse starts the MCP server in SSE mode
func (s *Server) ServeSse(baseURL string, httpServer *http.Server) *server.SSEServer {
	logging.Info("Starting MCP server in SSE mode")

	options := make([]server.SSEOption, 0)
	options = append(options, server.WithHTTPServer(httpServer), server.WithSSEContextFunc(contextFunc))

	if baseURL != "" {
		options = append(options, server.WithBaseURL(baseURL))
	}

	return server.NewSSEServer(s.server, options...)
}

// ServeHTTP starts the MCP server in HTTP mode
func (s *Server) ServeHTTP(httpServer *http.Server) *server.StreamableHTTPServer {
	logging.Info("Starting MCP server in HTTP mode")

	options := []server.StreamableHTTPOption{
		server.WithHTTPContextFunc(contextFunc),
		server.WithStreamableHTTPServer(httpServer),
		server.WithStateLess(true),
	}

	return server.NewStreamableHTTPServer(s.server, options...)
}

// GetEnabledTools returns the list of enabled tools
func (s *Server) GetEnabledTools() []string {
	return s.enabledTools
}

// IsHealthy returns true if the server and its clients are properly initialized
func (s *Server) IsHealthy() bool {
	// Check if Norman client is properly configured (if Rancher config is provided)
	if s.configuration.HasRancherConfig() {
		if s.normanClient == nil {
			return false
		}
	}
	return true
}

// Close cleans up the server resources
func (s *Server) Close() {
	logging.Info("Closing MCP server")
	// Nothing to clean up for now
}

// NewTextResult creates a standardized text result for tool responses
func NewTextResult(content string, err error) *mcp.CallToolResult {
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: err.Error(),
				},
			},
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: content,
			},
		},
	}
}
