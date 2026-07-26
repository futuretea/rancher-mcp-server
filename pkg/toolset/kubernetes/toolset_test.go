package kubernetes

import (
	"strings"
	"testing"
)

func TestClusterParameterDescription_DocumentsKubeconfigReference(t *testing.T) {
	description, _ := clusterIDProperty["description"].(string)
	if !strings.Contains(description, "kubeconfig:<context>") {
		t.Fatalf("cluster parameter description = %q, want kubeconfig reference syntax", description)
	}
}
