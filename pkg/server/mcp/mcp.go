// Package mcp manages the MCP server instance, tool registration, and
// configuration-driven tool filtering (read-only, destructive, sensitive data).
package mcp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strings"

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

const (
	authorizationKey contextKey = "Authorization"
	rancherTokenKey  contextKey = "R_token"
)

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
	clientResolver toolset.ClientResolver
	metrics        Metrics
	oauthVerifier  *oauthTokenVerifier
}

// NewServer creates a new MCP server with the given configuration
func NewServer(configuration Configuration) (*Server, error) {
	// Note: Logging is initialized in root.go before calling NewServer
	// to properly handle stdio vs HTTP/SSE mode
	var oauthVerifier *oauthTokenVerifier
	if configuration.RancherOAuthTokenAuth {
		var err error
		oauthVerifier, err = newOAuthTokenVerifier(configuration.StaticConfig)
		if err != nil {
			return nil, fmt.Errorf("initialize OAuth token verifier: %w", err)
		}
	}

	// Configure server capabilities.
	serverOptions := []server.ServerOption{
		server.WithResourceCapabilities(true, true),
		server.WithPromptCapabilities(true),
		server.WithToolCapabilities(true),
		server.WithLogging(),
	}

	s := &Server{
		configuration: &configuration,
		server:        server.NewMCPServer(version.BinaryName, version.Version, serverOptions...),
		metrics:       NewExpvarMetrics(),
		oauthVerifier: oauthVerifier,
	}

	if oauthVerifier != nil {
		logging.Info("auth mode: Rancher OAuth token")
		logging.Info("rancher server URL: %s", configuration.RancherServerURL)

		s.clientResolver = &oauthTokenResolver{
			serverURL:     configuration.RancherServerURL,
			insecure:      configuration.RancherTLSInsecure,
			steveFactory:  steve.NewClientWithToken,
			normanFactory: norman.NewClientWithToken,
			metrics:       s.metrics,
		}
		s.combinedClient = toolset.NewCombinedClient(nil, nil, false)
	} else if configuration.RancherRequestTokenAuth {
		logging.Info("auth mode: per-request token (RancherRequestTokenAuth=true)")
		logging.Info("rancher server URL: %s", configuration.RancherServerURL)

		s.clientResolver = &requestTokenResolver{
			serverURL:     configuration.RancherServerURL,
			insecure:      configuration.RancherTLSInsecure,
			steveFactory:  steve.NewClientWithToken,
			normanFactory: norman.NewClientWithToken,
			metrics:       s.metrics,
		}
		s.combinedClient = toolset.NewCombinedClient(nil, nil, false)
	} else {
		// Initialize Norman client (for Rancher v3 API)
		normanClient, err := norman.NewClient(configuration.StaticConfig)
		if err != nil {
			// Log the error but continue without Norman client
			logging.Warn("Failed to create Norman client: %v", err)
			logging.Warn("Rancher tools will not be available")
		}

		// Initialize Steve client (for Steve API / Kubernetes resources)
		var steveClient *steve.Client
		if configuration.HasRancherConfig() || configuration.HasKubeconfigConfig() {
			var err error
			steveClient, err = steve.NewClientWithKubeconfigPaths(
				configuration.RancherServerURL,
				configuration.RancherToken,
				configuration.RancherAccessKey,
				configuration.RancherSecretKey,
				configuration.RancherTLSInsecure,
				configuration.KubeconfigPaths,
			)
			if err != nil {
				return nil, fmt.Errorf("initialize Steve client: %w", err)
			}
			logging.Info("Steve client initialized for Kubernetes resources")
		}

		s.normanClient = normanClient
		s.steveClient = steveClient
		s.combinedClient = toolset.NewCombinedClient(normanClient, steveClient, false)
		s.clientResolver = &staticResolver{client: s.combinedClient}
	}

	// Register tools
	if err := s.registerTools(); err != nil {
		s.Close()
		return nil, err
	}

	return s, nil
}

// registerTools registers all available tools based on configuration
func (s *Server) registerTools() error {
	available := s.availableToolsets()
	enabled := s.enabledToolsets(available)

	if err := validateUniqueToolNames(enabled, s.combinedClient); err != nil {
		return err
	}

	for _, ts := range enabled {
		if err := s.registerToolset(ts); err != nil {
			return err
		}
	}

	logging.Info("Capability summary: %s", s.capabilitySummary())
	logging.Info("MCP server initialized with %d tools", len(s.enabledTools))
	return nil
}

// availableToolsets returns all toolsets that can be registered.
func (s *Server) availableToolsets() map[string]toolset.Toolset {
	return map[string]toolset.Toolset{
		"kubernetes": &kubernetes.Toolset{
			ReadOnly:           s.configuration.ReadOnly,
			DisableDestructive: s.configuration.DisableDestructive,
		},
		"rancher": &rancherToolset.Toolset{},
	}
}

// enabledToolsets selects the toolsets that should be registered.
// If no toolsets are configured, all available toolsets are used.
func (s *Server) enabledToolsets(available map[string]toolset.Toolset) []toolset.Toolset {
	if len(s.configuration.Toolsets) == 0 {
		enabled := make([]toolset.Toolset, 0, len(available))
		for _, ts := range available {
			enabled = append(enabled, ts)
		}
		return enabled
	}

	enabled := make([]toolset.Toolset, 0, len(s.configuration.Toolsets))
	for _, name := range s.configuration.Toolsets {
		if ts, exists := available[name]; exists {
			enabled = append(enabled, ts)
		}
	}
	return enabled
}

// registerToolset registers all tools from a single toolset.
func (s *Server) registerToolset(ts toolset.Toolset) error {
	for _, rawTool := range ts.GetTools(s.combinedClient) {
		tool := applyDefaultAnnotations(ts.GetName(), rawTool)
		if !s.shouldRegisterTool(tool) {
			continue
		}

		configuredTool := s.configureTool(tool)
		if err := s.registerTool(configuredTool); err != nil {
			return fmt.Errorf("failed to register tool %s: %w", tool.Tool.Name, err)
		}
	}
	return nil
}

// shouldRegisterTool reports whether a tool passes capability, container-operation,
// and enablement checks.
func (s *Server) shouldRegisterTool(tool toolset.ServerTool) bool {
	if allowed, reason := s.capabilityAllowsTool(tool); !allowed {
		logging.Info("Skipping tool %s: %s", tool.Tool.Name, reason)
		return false
	}
	if !s.containerOperationEnabled(tool.Tool.Name) {
		return false
	}
	return s.shouldEnableTool(tool.Tool.Name)
}

// containerOperationEnabled reports whether a container operation tool is enabled
// by configuration. Non-container tools are always enabled.
func (s *Server) containerOperationEnabled(toolName string) bool {
	switch toolName {
	case "kubernetes_upload_file":
		return s.configuration.EnableContainerFileUpload
	case "kubernetes_download_file":
		return s.configuration.EnableContainerFileDownload
	case "kubernetes_exec":
		return s.configuration.EnableContainerExec
	default:
		return true
	}
}

// shouldEnableTool determines if a tool should be enabled based on configuration.
func (s *Server) shouldEnableTool(toolName string) bool {
	// Check if tool is explicitly disabled.
	if slices.Contains(s.configuration.DisabledTools, toolName) {
		return false
	}

	// If an allowlist is configured, only those tools are enabled.
	if len(s.configuration.EnabledTools) > 0 {
		return slices.Contains(s.configuration.EnabledTools, toolName)
	}

	// Default: enable the tool.
	return true
}

// configureTool creates a configured tool handler that uses server configuration
func (s *Server) configureTool(tool toolset.ServerTool) toolset.ServerTool {
	if len(tool.Annotations.ClusterSources) > 0 {
		tool.Tool.Description += fmt.Sprintf(" Applicable cluster sources: %s.", strings.Join(tool.Annotations.ClusterSources, ", "))
	}

	return toolset.ServerTool{
		Tool:        tool.Tool,
		Annotations: tool.Annotations,
		Handler: func(ctx context.Context, client interface{}, params map[string]interface{}) (string, error) {
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

			// Inject maxFileSize for container file operations
			if s.configuration.MaxFileSize != "" {
				params["maxFileSize"] = s.configuration.MaxFileSize
			} else {
				params["maxFileSize"] = kubernetes.DefaultMaxFileSize
			}

			// Admin policy: if show_sensitive_data is disabled, force mask regardless of per-call param
			if !s.configuration.ShowSensitiveData {
				params["showSensitiveData"] = false
			}

			return tool.Handler(ctx, client, params)
		},
	}
}

func contextFunc(ctx context.Context, r *http.Request) context.Context {
	if _, authorizationPresent := r.Header[http.CanonicalHeaderKey("Authorization")]; authorizationPresent {
		ctx = context.WithValue(ctx, authorizationKey, r.Header.Get("Authorization"))
	}
	if rancherToken := r.Header.Get("R_token"); rancherToken != "" {
		ctx = context.WithValue(ctx, rancherTokenKey, rancherToken)
	}
	return ctx
}

// registerTool registers a single tool with the MCP server
func (s *Server) registerTool(tool toolset.ServerTool) error {
	toolHandler := s.makeToolHandler(tool)

	// Use the simpler AddTool method
	s.server.AddTool(tool.Tool, toolHandler)
	s.enabledTools = append(s.enabledTools, tool.Tool.Name)

	logging.Info("Registered tool: %s", tool.Tool.Name)
	return nil
}

func (s *Server) makeToolHandler(tool toolset.ServerTool) server.ToolHandlerFunc {
	return server.ToolHandlerFunc(func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logging.Debug("Tool %s called with param keys: %v", tool.Tool.Name, toolArgumentKeys(request.Params.Arguments))

		resolvedClient, err := s.clientResolver.Resolve(ctx)
		if err != nil {
			return NewTextResult("", err), nil
		}
		if resolvedClient == nil {
			return NewTextResult("", errors.New("resolver returned nil client")), nil
		}
		requestScoped := resolvedClient.IsCloseable()
		if requestScoped && s.metrics != nil {
			s.metrics.IncrementActiveClientCount()
		}
		defer func() {
			resolvedClient.Close()
			if requestScoped && s.metrics != nil {
				s.metrics.DecrementActiveClientCount()
			}
		}()

		// Convert arguments to the format expected by our tool handlers
		params := make(map[string]interface{})
		if arguments, ok := request.Params.Arguments.(map[string]interface{}); ok {
			for key, value := range arguments {
				params[key] = value
			}
		}

		result, err := tool.Handler(ctx, resolvedClient, params)
		return NewTextResult(result, err), nil
	})
}

func toolArgumentKeys(arguments interface{}) []string {
	args, ok := arguments.(map[string]interface{})
	if !ok || len(args) == 0 {
		return nil
	}

	keys := make([]string, 0, len(args))
	for key := range args {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
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

	requestContext := contextFunc
	if s.configuration.RancherOAuthTokenAuth {
		requestContext = func(ctx context.Context, _ *http.Request) context.Context {
			return ctx
		}
	}

	options := []server.StreamableHTTPOption{
		server.WithHTTPContextFunc(requestContext),
		server.WithStreamableHTTPServer(httpServer),
		server.WithStateLess(true),
	}

	return server.NewStreamableHTTPServer(s.server, options...)
}

// ServeOAuthHTTP starts the Streamable HTTP handler protected by the configured OAuth verifier.
func (s *Server) ServeOAuthHTTP(httpServer *http.Server) http.Handler {
	streamableHTTPServer := s.ServeHTTP(httpServer)
	if s.oauthVerifier == nil {
		return streamableHTTPServer
	}
	return s.oauthVerifier.middleware(s.configuration.RancherOAuthResourceURL, streamableHTTPServer)
}

// OAuthProtectedResourceMetadataHandler returns the configured OAuth discovery handler.
func (s *Server) OAuthProtectedResourceMetadataHandler() http.Handler {
	return oauthProtectedResourceMetadataHandler(s.configuration.StaticConfig)
}

// GetEnabledTools returns the list of enabled tools
func (s *Server) GetEnabledTools() []string {
	return s.enabledTools
}

// IsHealthy returns true if the server and its clients are properly initialized
func (s *Server) IsHealthy() bool {
	return s != nil && s.server != nil
}

// Close cleans up the server resources
func (s *Server) Close() {
	logging.Info("Closing MCP server")
	if s != nil && s.oauthVerifier != nil {
		s.oauthVerifier.Close()
	}
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
