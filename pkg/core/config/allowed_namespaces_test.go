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

func TestLoadConfig_AllowedNamespacesInvalidFileOverrides(t *testing.T) {
	for _, tt := range []struct {
		name     string
		source   string
		override string
		want     map[string][]string
		wantErr  string
	}{
		{name: "file alone rejects scalar namespace list", wantErr: "cannot unmarshal !!str"},
		{name: "environment replaces invalid file", source: "environment", override: `{"c-env":["app"]}`, want: map[string][]string{"c-env": {"app"}}},
		{name: "CLI replaces invalid file and environment", source: "CLI", override: `{"c-cli":["app"]}`, want: map[string][]string{"c-cli": {"app"}}},
		{name: "empty environment object clears invalid file", source: "environment", override: `{}`, want: map[string][]string{}},
		{name: "empty CLI object clears invalid file", source: "CLI", override: `{}`, want: map[string][]string{}},
		{name: "invalid environment JSON fails", source: "environment", override: `{`, wantErr: "invalid JSON"},
		{name: "invalid CLI JSON fails", source: "CLI", override: `{`, wantErr: "invalid JSON"},
		{name: "invalid environment namespace fails", source: "environment", override: `{"c-env":[" "]}`, wantErr: "whitespace-only"},
		{name: "invalid CLI namespace fails", source: "CLI", override: `{"c-cli":[" "]}`, wantErr: "whitespace-only"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			t.Cleanup(viper.Reset)
			t.Setenv("RANCHER_MCP_ALLOWED_NAMESPACES", "")
			if err := os.Unsetenv("RANCHER_MCP_ALLOWED_NAMESPACES"); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "config.yaml")
			body := "list_output: json\nallowed_namespaces: {c-file: old-namespace}\n"
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			switch tt.source {
			case "environment":
				t.Setenv("RANCHER_MCP_ALLOWED_NAMESPACES", tt.override)
			case "CLI":
				t.Setenv("RANCHER_MCP_ALLOWED_NAMESPACES", `{`)
				cmd := &cobra.Command{Use: "test"}
				flags := cmd.Flags()
				flags.String("allowed-namespaces", "", "")
				if err := flags.Set("allowed-namespaces", tt.override); err != nil {
					t.Fatal(err)
				}
				if err := viper.BindPFlag("allowed_namespaces_json", flags.Lookup("allowed-namespaces")); err != nil {
					t.Fatal(err)
				}
			}

			cfg, err := LoadConfig(path)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("LoadConfig() error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("LoadConfig() error = %v", err)
			}
			if !reflect.DeepEqual(cfg.AllowedNamespaces, tt.want) {
				t.Fatalf("AllowedNamespaces = %#v, want %#v", cfg.AllowedNamespaces, tt.want)
			}
		})
	}
}

func TestLoadConfig_AllowedNamespacesKeyCase(t *testing.T) {
	for _, key := range []string{"allowed_namespaces", "ALLOWED_NAMESPACES", "Allowed_Namespaces"} {
		for _, source := range []string{"file", "environment", "CLI"} {
			t.Run(key+"/"+source, func(t *testing.T) {
				viper.Reset()
				t.Cleanup(viper.Reset)
				path := filepath.Join(t.TempDir(), "config.yaml")
				body := "list_output: json\n" + key + ":\n  'kubeconfig:Production': [app]\n  'kubeconfig:production': [default]\n"
				if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
					t.Fatal(err)
				}
				want := map[string][]string{"kubeconfig:Production": {"app"}, "kubeconfig:production": {"default"}}
				if source != "file" {
					t.Setenv("RANCHER_MCP_ALLOWED_NAMESPACES", `{"c-env":["env"]}`)
					want = map[string][]string{"c-env": {"env"}}
				}
				if source == "CLI" {
					cmd := &cobra.Command{Use: "test"}
					flags := cmd.Flags()
					flags.String("allowed-namespaces", "", "")
					if err := flags.Set("allowed-namespaces", `{"c-cli":["cli"]}`); err != nil {
						t.Fatal(err)
					}
					if err := viper.BindPFlag("allowed_namespaces_json", flags.Lookup("allowed-namespaces")); err != nil {
						t.Fatal(err)
					}
					want = map[string][]string{"c-cli": {"cli"}}
				}
				cfg, err := LoadConfig(path)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(cfg.AllowedNamespaces, want) {
					t.Fatalf("AllowedNamespaces = %#v, want %#v", cfg.AllowedNamespaces, want)
				}
			})
		}
	}
}
