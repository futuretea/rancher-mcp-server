package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func TestLoadConfig_AllowedNamespacesDuplicateKeys(t *testing.T) {
	for _, test := range []struct {
		name string
		body string
	}{
		{name: "empty then restricted", body: "allowed_namespaces: {}\nALLOWED_NAMESPACES: {c-file: [app]}\n"},
		{name: "restricted then empty", body: "Allowed_Namespaces: {c-file: [app]}\nallowed_namespaces: {}\n"},
		{name: "same values", body: "ALLOWED_NAMESPACES: {c-file: [app]}\nAllowed_Namespaces: {c-file: [app]}\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := writeDuplicateAllowedNamespacesConfig(t, test.body)
			_, err := LoadConfig(path)
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), "duplicate") || !strings.Contains(strings.ToLower(err.Error()), "allowed_namespaces") {
				t.Fatalf("LoadConfig() error = %v, want a descriptive duplicate allowed_namespaces error", err)
			}
		})
	}
}

func TestLoadConfig_AllowedNamespacesDuplicateKeysOverrides(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
		raw    string
		want   map[string][]string
	}{
		{name: "environment replaces file", source: "environment", raw: `{"c-env":["env"]}`, want: map[string][]string{"c-env": {"env"}}},
		{name: "CLI replaces file and environment", source: "CLI", raw: `{"c-cli":["cli"]}`, want: map[string][]string{"c-cli": {"cli"}}},
		{name: "empty environment replaces file", source: "environment", raw: `{}`, want: map[string][]string{}},
		{name: "empty CLI replaces file and environment", source: "CLI", raw: `{}`, want: map[string][]string{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := writeDuplicateAllowedNamespacesConfig(t, "allowed_namespaces: {}\nALLOWED_NAMESPACES: {c-file: [app]}\n")
			t.Setenv("RANCHER_MCP_ALLOWED_NAMESPACES", test.raw)
			if test.source == "CLI" {
				t.Setenv("RANCHER_MCP_ALLOWED_NAMESPACES", `{`)
				cmd := &cobra.Command{Use: "test"}
				cmd.Flags().String("allowed-namespaces", "", "")
				if err := cmd.Flags().Set("allowed-namespaces", test.raw); err != nil {
					t.Fatal(err)
				}
				if err := viper.BindPFlag("allowed_namespaces_json", cmd.Flags().Lookup("allowed-namespaces")); err != nil {
					t.Fatal(err)
				}
			}
			cfg, err := LoadConfig(path)
			if err != nil {
				t.Fatalf("LoadConfig() error = %v, want successful %s override", err, test.source)
			}
			if !reflect.DeepEqual(cfg.AllowedNamespaces, test.want) {
				t.Fatalf("AllowedNamespaces = %#v, want %#v", cfg.AllowedNamespaces, test.want)
			}
		})
	}
}

func writeDuplicateAllowedNamespacesConfig(t *testing.T, body string) string {
	t.Helper()
	viper.Reset()
	t.Cleanup(viper.Reset)
	t.Setenv("RANCHER_MCP_ALLOWED_NAMESPACES", "")
	if err := os.Unsetenv("RANCHER_MCP_ALLOWED_NAMESPACES"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("list_output: json\n"+body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
