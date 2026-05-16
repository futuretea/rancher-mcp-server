package kubernetes

import (
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/futuretea/rancher-mcp-server/pkg/toolset"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset/paramutil"
)

// appendWriteTools appends write-operation tools (create, patch, exec, upload, delete)
// to the tools slice, respecting ReadOnly and DisableDestructive flags.
func (t *Toolset) appendWriteTools(tools []toolset.ServerTool) []toolset.ServerTool {
	if !t.ReadOnly {
		tools = append(tools,
			toolset.ServerTool{
				Tool: mcp.Tool{
					Name:        "kubernetes_create",
					Description: "Create a Kubernetes resource from a JSON manifest.",
					InputSchema: mcp.ToolInputSchema{
						Type:     "object",
						Required: []string{"cluster", "resource"},
						Properties: map[string]any{
							"cluster": map[string]any{
								"type":        "string",
								"description": "Cluster ID (use cluster_list tool to get available cluster IDs)",
							},
							"resource": map[string]any{
								"type":        "string",
								"description": "Resource manifest as JSON string (must include apiVersion, kind, metadata, and spec)",
							},
						},
					},
				},
				Annotations: toolset.ToolAnnotations{
					ReadOnlyHint: paramutil.BoolPtr(false),
				},
				Handler: createHandler,
			},
			toolset.ServerTool{
				Tool: mcp.Tool{
					Name:        "kubernetes_patch",
					Description: "Patch a Kubernetes resource using JSON Patch (RFC 6902).",
					InputSchema: mcp.ToolInputSchema{
						Type:     "object",
						Required: []string{"cluster", "kind", "name", "patch"},
						Properties: map[string]any{
							"cluster": map[string]any{
								"type":        "string",
								"description": "Cluster ID (use cluster_list tool to get available cluster IDs)",
							},
							"kind": map[string]any{
								"type":        "string",
								"description": "Resource kind (e.g., deployment, service, App). For CRDs, pass the manifest kind and optionally apiVersion.",
							},
							"apiVersion": apiVersionProperty,
							"namespace": map[string]any{
								"type":        "string",
								"description": "Namespace name (optional for cluster-scoped resources)",
								"default":     "",
							},
							"name": map[string]any{
								"type":        "string",
								"description": "Resource name",
							},
							"patch": map[string]any{
								"type":        "string",
								"description": "JSON Patch array as string, e.g., '[{\"op\":\"replace\",\"path\":\"/spec/replicas\",\"value\":3}]'",
							},
						},
					},
				},
				Annotations: toolset.ToolAnnotations{
					ReadOnlyHint: paramutil.BoolPtr(false),
				},
				Handler: patchHandler,
			},
			toolset.ServerTool{
				Tool: mcp.Tool{
					Name:        "kubernetes_exec",
					Description: "Execute a non-interactive command in a pod container. Disabled by default and blocked in read-only mode. The command must be an argv-style string array; stdin and TTY are not supported.",
					InputSchema: mcp.ToolInputSchema{
						Type:     "object",
						Required: []string{"cluster", "namespace", "name", "command"},
						Properties: map[string]any{
							"cluster": map[string]any{
								"type":        "string",
								"description": "Cluster ID (use cluster_list tool to get available cluster IDs)",
							},
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
							"command": map[string]any{
								"type":        "array",
								"description": "Command and arguments as an argv-style array, e.g. [\"printenv\", \"HOSTNAME\"]",
								"items": map[string]any{
									"type": "string",
								},
								"minItems": 1,
							},
						},
					},
				},
				Annotations: toolset.ToolAnnotations{
					ReadOnlyHint:       paramutil.BoolPtr(false),
					DestructiveHint:    paramutil.BoolPtr(true),
					RequiresKubernetes: paramutil.BoolPtr(true),
				},
				Handler: handleExec,
			},
			toolset.ServerTool{
				Tool: mcp.Tool{
					Name:        "kubernetes_upload_file",
					Description: "Upload a file to a container in a pod. Accepts base64-encoded file content. Requires the container to have 'tar' installed. Files are limited by the configured max file size.",
					InputSchema: mcp.ToolInputSchema{
						Type:     "object",
						Required: []string{"cluster", "namespace", "name", "filePath", "content"},
						Properties: map[string]any{
							"cluster": map[string]any{
								"type":        "string",
								"description": "Cluster ID (use cluster_list tool to get available cluster IDs)",
							},
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
								"description": "Absolute destination path for the file in the container",
							},
							"content": map[string]any{
								"type":        "string",
								"description": "Base64-encoded file content to upload",
							},
						},
					},
				},
				Annotations: toolset.ToolAnnotations{
					ReadOnlyHint:       paramutil.BoolPtr(false),
					DestructiveHint:    paramutil.BoolPtr(false),
					RequiresKubernetes: paramutil.BoolPtr(true),
				},
				Handler: handleUploadFile,
			},
		)

		if !t.DisableDestructive {
			tools = append(tools, toolset.ServerTool{
				Tool: mcp.Tool{
					Name:        "kubernetes_delete",
					Description: "Delete a Kubernetes resource.",
					InputSchema: mcp.ToolInputSchema{
						Type:     "object",
						Required: []string{"cluster", "kind", "name"},
						Properties: map[string]any{
							"cluster": map[string]any{
								"type":        "string",
								"description": "Cluster ID (use cluster_list tool to get available cluster IDs)",
							},
							"kind": map[string]any{
								"type":        "string",
								"description": "Resource kind (e.g., deployment, service, App). For CRDs, pass the manifest kind and optionally apiVersion.",
							},
							"apiVersion": apiVersionProperty,
							"namespace": map[string]any{
								"type":        "string",
								"description": "Namespace name (optional for cluster-scoped resources)",
								"default":     "",
							},
							"name": map[string]any{
								"type":        "string",
								"description": "Resource name",
							},
						},
					},
				},
				Annotations: toolset.ToolAnnotations{
					ReadOnlyHint:    paramutil.BoolPtr(false),
					DestructiveHint: paramutil.BoolPtr(true),
				},
				Handler: deleteHandler,
			})
		}
	}

	return tools
}
