package kubernetes

import (
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/futuretea/rancher-mcp-server/pkg/toolset"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset/paramutil"
)

// fileTools returns the container file operation tool definitions.
func fileTools() []toolset.ServerTool {
	return []toolset.ServerTool{
		{
			Tool: mcp.Tool{
				Name:        "kubernetes_download_file",
				Description: "Download a file from a container in a pod. Returns the file content as base64-encoded string with metadata. Requires the container to have 'tar' installed. Files are limited by the configured max file size.",
				InputSchema: mcp.ToolInputSchema{
					Type:     "object",
					Required: []string{"cluster", "namespace", "name", "filePath"},
					Properties: map[string]any{
						"cluster": clusterIDProperty,
						"namespace": map[string]any{
							"type":        "string",
							"description": "Namespace name",
						},
						"name": map[string]any{
							"type":        "string",
							"description": "Pod name",
						},
						"container": map[string]any{
							"type":        "string",
							"description": "Container name (optional, defaults to first container)",
							"default":     "",
						},
						"filePath": map[string]any{
							"type":        "string",
							"description": "Absolute path of the file to download from the container",
						},
					},
				},
			},
			Annotations: toolset.ToolAnnotations{
				ReadOnlyHint:       paramutil.BoolPtr(true),
				RequiresKubernetes: paramutil.BoolPtr(true),
			},
			Handler: handleDownloadFile,
		},
	}
}
