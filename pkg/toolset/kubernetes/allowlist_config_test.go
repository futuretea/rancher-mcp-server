package kubernetes

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/futuretea/rancher-mcp-server/pkg/core/config"
	"github.com/spf13/viper"
)

func TestAllowlistConfigKeyCase(t *testing.T) {
	for _, key := range []string{"allowed_namespaces", "ALLOWED_NAMESPACES", "Allowed_Namespaces"} {
		t.Run(key, func(t *testing.T) {
			viper.Reset()
			t.Cleanup(viper.Reset)
			resetNamespaceAllowlist()
			t.Cleanup(resetNamespaceAllowlist)
			path := filepath.Join(t.TempDir(), "config.yaml")
			body := "list_output: json\n" + key + ":\n  'kubeconfig:Production': [app]\n"
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			cfg, err := config.LoadConfig(path)
			if err != nil {
				t.Fatal(err)
			}
			SetNamespaceAllowlist(cfg.AllowedNamespaces)
			reader := newRecordingReader()
			reader.inner.AddResource(podObject("app", "visible"))
			reader.inner.AddResource(podObject("kube-system", "hidden"))
			out, err := listHandler(context.Background(), reader, map[string]interface{}{
				"cluster": "kubeconfig:Production", "kind": "pod", "format": "json",
			})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out, "visible") || strings.Contains(out, "hidden") {
				t.Errorf("list result = %s, want only app/visible", out)
			}
			reader.assertNoEmptyNamespacedList(t)
			calls := len(reader.calls)
			_, err = listHandler(context.Background(), reader, map[string]interface{}{
				"cluster": "kubeconfig:Production", "kind": "pod", "namespace": "kube-system", "format": "json",
			})
			if err == nil || !strings.Contains(err.Error(), "not allowed") || len(reader.calls) != calls {
				t.Fatalf("forbidden namespace: error=%v calls=%v, want rejection before backend access", err, reader.calls[calls:])
			}
		})
	}
}
