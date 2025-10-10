package api

import (
	"testing"
)

func TestToolAnnotations(t *testing.T) {
	annotations := ToolAnnotations{
		ReadOnlyHint:       boolPtr(true),
		DestructiveHint:    boolPtr(false),
		RequiresRancher:    boolPtr(true),
		RequiresKubernetes: boolPtr(false),
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

func boolPtr(b bool) *bool {
	return &b
}