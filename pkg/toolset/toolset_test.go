package toolset

import (
	"testing"

	"github.com/futuretea/rancher-mcp-server/pkg/toolset/handler"
)

func TestToolAnnotations(t *testing.T) {
	annotations := ToolAnnotations{
		ReadOnlyHint:       handler.BoolPtr(true),
		DestructiveHint:    handler.BoolPtr(false),
		RequiresRancher:    handler.BoolPtr(true),
		RequiresKubernetes: handler.BoolPtr(false),
	}

	if *annotations.ReadOnlyHint != true {
		t.Error("ReadOnlyHint should be true")
	}

	if *annotations.DestructiveHint != false {
		t.Error("DestructiveHint should be false")
	}

	if *annotations.RequiresRancher != true {
		t.Error("RequiresRancher should be true")
	}

	if *annotations.RequiresKubernetes != false {
		t.Error("RequiresKubernetes should be false")
	}
}
