package toolset

import (
	"github.com/mark3labs/mcp-go/mcp"
)

// Toolset defines the interface for a set of MCP tools.
type Toolset interface {
	// GetName returns the name of the toolset.
	GetName() string

	// GetDescription returns the description of the toolset.
	GetDescription() string

	// GetTools returns the tools provided by this toolset.
	GetTools(client interface{}) []ServerTool
}

// ToolAnnotations provides additional metadata for tools.
type ToolAnnotations struct {
	// ReadOnlyHint indicates if the tool is read-only.
	ReadOnlyHint *bool

	// DestructiveHint indicates if the tool performs destructive operations.
	DestructiveHint *bool

	// RequiresRancher indicates if the tool requires Rancher configuration.
	RequiresRancher *bool

	// RequiresKubernetes indicates if the tool requires Kubernetes configuration.
	RequiresKubernetes *bool
}

// ServerTool represents an MCP tool with its metadata and handler.
// This is a wrapper around mcp.Tool that includes additional server-specific information.
type ServerTool struct {
	// Tool is the MCP tool definition.
	Tool mcp.Tool

	// Annotations provides additional metadata about the tool.
	Annotations ToolAnnotations

	// Handler is the function that handles tool calls.
	Handler ToolHandler
}

// ToolHandler is the function signature for handling tool calls.
type ToolHandler func(client interface{}, params map[string]interface{}) (string, error)
