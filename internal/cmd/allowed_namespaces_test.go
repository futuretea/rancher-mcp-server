package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/futuretea/rancher-mcp-server/pkg/core/config"
)

func TestAllowedNamespacesConfigurationSources(t *testing.T) {
	for _, test := range []struct {
		name      string
		fileJSON  string
		hiddenEnv string
		env       string
		args      []string
		want      map[string][]string
		wantErr   string
	}{
		{name: "file JSON key cannot override environment", fileJSON: "{}", env: `{"c-env":["app"]}`, want: map[string][]string{"c-env": {"app"}}},
		{name: "environment JSON key cannot override environment", hiddenEnv: "{}", env: `{"c-env":["app"]}`, want: map[string][]string{"c-env": {"app"}}},
		{name: "file JSON key cannot override file", fileJSON: "{}", want: map[string][]string{"c-file": {"from-file"}}},
		{name: "environment JSON key cannot override file", hiddenEnv: "{}", want: map[string][]string{"c-file": {"from-file"}}},
		{name: "invalid file JSON key is ignored", fileJSON: "{", env: `{"c-env":["app"]}`, want: map[string][]string{"c-env": {"app"}}},
		{name: "invalid environment JSON key is ignored", hiddenEnv: "{", env: `{"c-env":["app"]}`, want: map[string][]string{"c-env": {"app"}}},
		{name: "explicit CLI overrides all other sources", fileJSON: "{}", hiddenEnv: "{", env: "{", args: []string{"--allowed-namespaces", `{"kubeconfig:Production":["app"]}`}, want: map[string][]string{"kubeconfig:Production": {"app"}}},
		{name: "explicit empty CLI object clears restrictions", fileJSON: "{", hiddenEnv: "{", env: "{", args: []string{"--allowed-namespaces", "{}"}, want: map[string][]string{}},
		{name: "explicit empty CLI value fails", env: `{"c-env":["app"]}`, args: []string{"--allowed-namespaces", ""}, wantErr: "JSON is empty"},
		{name: "explicit invalid CLI value fails", env: `{"c-env":["app"]}`, args: []string{"--allowed-namespaces", "{"}, wantErr: "invalid JSON"},
	} {
		t.Run(test.name, func(t *testing.T) {
			viper.Reset()
			t.Cleanup(viper.Reset)
			for _, key := range []string{"RANCHER_MCP_ALLOWED_NAMESPACES", "RANCHER_MCP_ALLOWED_NAMESPACES_JSON"} {
				t.Setenv(key, "")
				if err := os.Unsetenv(key); err != nil {
					t.Fatal(err)
				}
			}
			if test.env != "" {
				t.Setenv("RANCHER_MCP_ALLOWED_NAMESPACES", test.env)
			}
			if test.hiddenEnv != "" {
				t.Setenv("RANCHER_MCP_ALLOWED_NAMESPACES_JSON", test.hiddenEnv)
			}
			path := filepath.Join(t.TempDir(), "config.yaml")
			body := "list_output: json\nallowed_namespaces: {c-file: [from-file]}\n"
			if test.fileJSON != "" {
				body += "allowed_namespaces_json: '" + test.fileJSON + "'\n"
			}
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			cmd := NewMCPServer(IOStreams{In: &bytes.Buffer{}, Out: &bytes.Buffer{}, ErrOut: &bytes.Buffer{}})
			var cfg *config.StaticConfig
			cmd.RunE = func(_ *cobra.Command, _ []string) error {
				var err error
				cfg, err = config.LoadConfig(path)
				return err
			}
			cmd.SetArgs(test.args)
			err := cmd.Execute()
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("Execute() error = %v, want %q", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if !reflect.DeepEqual(cfg.AllowedNamespaces, test.want) {
				t.Fatalf("AllowedNamespaces = %#v, want %#v", cfg.AllowedNamespaces, test.want)
			}
		})
	}
}
