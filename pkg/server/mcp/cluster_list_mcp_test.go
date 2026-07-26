package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/futuretea/rancher-mcp-server/pkg/core/config"
	protocol "github.com/mark3labs/mcp-go/mcp"
)

func TestNewServer_KubeconfigOnlyDispatchesClusterList(t *testing.T) {
	server, err := NewServer(Configuration{StaticConfig: &config.StaticConfig{
		KubeconfigPaths: []string{writeServerKubeconfig(t, "direct", "https://direct.example.test")},
		Toolsets:        []string{"kubernetes", "rancher"},
		ListOutput:      "json",
	}})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	defer server.Close()

	client := newServerInProcessClient(t, server)
	result, err := client.CallTool(context.Background(), protocol.CallToolRequest{Params: protocol.CallToolParams{
		Name:      "cluster_list",
		Arguments: map[string]interface{}{"format": "json"},
	}})
	if err != nil {
		t.Fatalf("CallTool(cluster_list) error = %v", err)
	}
	if result.IsError {
		t.Fatalf("CallTool(cluster_list) error result = %#v", result.Content)
	}
	if len(result.Content) != 1 {
		t.Fatalf("CallTool(cluster_list) content count = %d, want 1", len(result.Content))
	}
	text, ok := result.Content[0].(protocol.TextContent)
	if !ok {
		t.Fatalf("CallTool(cluster_list) content = %T, want mcp.TextContent", result.Content[0])
	}

	var rows []map[string]string
	if err := json.Unmarshal([]byte(text.Text), &rows); err != nil {
		t.Fatalf("decode cluster_list output: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("cluster_list rows = %#v, want one direct context", rows)
	}
	row := rows[0]
	if row["id"] != "kubeconfig:direct" || row["name"] != "direct" || row["source"] != "kubeconfig" {
		t.Fatalf("cluster_list row identity = %#v, want kubeconfig direct context", row)
	}
	for _, field := range []string{"state", "provider", "version", "nodes", "cpu", "ram", "pods"} {
		if row[field] != "" {
			t.Fatalf("cluster_list row[%q] = %q, want empty Rancher-derived field", field, row[field])
		}
	}
}
