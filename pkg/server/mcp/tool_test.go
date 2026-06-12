package mcp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/futuretea/rancher-mcp-server/pkg/core/logging"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset"
	"github.com/mark3labs/mcp-go/mcp"
)

func TestToolArgumentKeys(t *testing.T) {
	got := toolArgumentKeys(map[string]interface{}{
		"content": "base64-secret",
		"cluster": "c1",
		"command": []interface{}{"sh", "-c", "printenv SECRET"},
	})
	want := []string{"cluster", "command", "content"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("toolArgumentKeys() = %#v, want %#v", got, want)
	}
	if strings.Contains(fmt.Sprint(got), "base64-secret") || strings.Contains(fmt.Sprint(got), "SECRET") {
		t.Fatalf("toolArgumentKeys() leaked argument values: %v", got)
	}
}

func TestToolHandlerLogsArgumentKeysOnly(t *testing.T) {
	var logBuf bytes.Buffer
	logging.Initialize(6, &logBuf)
	t.Cleanup(func() {
		logging.Initialize(0, io.Discard)
	})

	s := &Server{}
	handler := s.makeToolHandler(toolset.ServerTool{
		Tool: mcp.Tool{Name: "test_tool"},
		Handler: func(_ context.Context, _ interface{}, _ map[string]interface{}) (string, error) {
			return "ok", nil
		},
	})

	_, err := handler(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]interface{}{
				"cluster": "c1",
				"content": "base64-secret",
				"command": []interface{}{"sh", "-c", "printenv SECRET"},
			},
		},
	})
	if err != nil {
		t.Fatalf("tool handler returned error: %v", err)
	}

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "param keys") {
		t.Fatalf("log output = %q, want param keys log", logOutput)
	}
	for _, wantKey := range []string{"cluster", "command", "content"} {
		if !strings.Contains(logOutput, wantKey) {
			t.Fatalf("log output = %q, want key %q", logOutput, wantKey)
		}
	}
	for _, leakedValue := range []string{"c1", "base64-secret", "sh", "printenv SECRET"} {
		if strings.Contains(logOutput, leakedValue) {
			t.Fatalf("log output leaked argument value %q: %s", leakedValue, logOutput)
		}
	}
}

func TestValidateUniqueToolNamesRejectsDuplicateNames(t *testing.T) {
	duplicateTool := toolset.ServerTool{
		Tool: mcp.Tool{Name: "duplicate_tool"},
	}

	err := validateUniqueToolNames([]toolset.Toolset{
		staticToolset{name: "first", tools: []toolset.ServerTool{duplicateTool}},
		staticToolset{name: "second", tools: []toolset.ServerTool{duplicateTool}},
	}, nil)
	if err == nil {
		t.Fatal("expected duplicate tool name validation to fail")
	}
	if !strings.Contains(err.Error(), "duplicate tool name") {
		t.Fatalf("unexpected duplicate validation error: %v", err)
	}
}

type staticToolset struct {
	name  string
	tools []toolset.ServerTool
}

func (s staticToolset) GetName() string {
	return s.name
}

func (s staticToolset) GetDescription() string {
	return s.name
}

func (s staticToolset) GetTools(_ interface{}) []toolset.ServerTool {
	return s.tools
}
