package rancher

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/futuretea/rancher-mcp-server/pkg/client/norman"
	"github.com/futuretea/rancher-mcp-server/pkg/client/steve"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset/paramutil"
	"github.com/rancher/norman/types"
	"gopkg.in/yaml.v3"
)

var clusterListHeaders = []string{"id", "name", "source", "state", "provider", "version", "nodes", "cpu", "ram", "pods"}

func TestClusterListHandler_KubeconfigOnlyListsClusterSource(t *testing.T) {
	kubeconfigPath := writeClusterListKubeconfig(t, "direct", "https://direct.example.test")
	steveClient, err := steve.NewClientWithKubeconfigPaths("", "", "", "", false, []string{kubeconfigPath})
	if err != nil {
		t.Fatalf("NewClientWithKubeconfigPaths() error = %v", err)
	}
	client := toolset.NewCombinedClient(nil, steveClient, false)

	assertClusterListOutput(t, client, []map[string]string{{
		"id":       "kubeconfig:direct",
		"name":     "direct",
		"source":   "kubeconfig",
		"state":    "",
		"provider": "",
		"version":  "",
		"nodes":    "",
		"cpu":      "",
		"ram":      "",
		"pods":     "",
	}})
}

func TestProjectListHandler_RejectsKubeconfigClusterReference(t *testing.T) {
	_, err := projectListHandler(context.Background(), nil, map[string]interface{}{
		"cluster": "kubeconfig:direct",
		"format":  "json",
	})
	if err == nil || !strings.Contains(err.Error(), "does not support kubeconfig clusters") {
		t.Fatalf("projectListHandler() error = %v, want kubeconfig applicability error", err)
	}
}

func TestClusterListHandler_MergesRancherAndKubeconfigSources(t *testing.T) {
	rancherServer := startClusterListRancherServer(t)
	normanClient, err := norman.NewClientWithToken(rancherServer.URL, "fixture-token", true)
	if err != nil {
		t.Fatalf("NewClientWithToken() error = %v", err)
	}
	t.Cleanup(normanClient.Close)

	kubeconfigPath := writeClusterListKubeconfig(t, "direct", "https://direct.example.test")
	steveClient, err := steve.NewClientWithKubeconfigPaths("", "", "", "", false, []string{kubeconfigPath})
	if err != nil {
		t.Fatalf("NewClientWithKubeconfigPaths() error = %v", err)
	}
	client := toolset.NewCombinedClient(normanClient, steveClient, false)

	assertClusterListOutput(t, client, []map[string]string{
		expectedRancherClusterRow(),
		{
			"id":       "kubeconfig:direct",
			"name":     "direct",
			"source":   "kubeconfig",
			"state":    "",
			"provider": "",
			"version":  "",
			"nodes":    "",
			"cpu":      "",
			"ram":      "",
			"pods":     "",
		},
	})

	_, err = projectListHandler(context.Background(), client, map[string]interface{}{
		"cluster": "kubeconfig:direct",
		"format":  "json",
	})
	if err == nil || !strings.Contains(err.Error(), "does not support kubeconfig clusters") {
		t.Fatalf("projectListHandler() error = %v, want kubeconfig applicability error", err)
	}
}

func TestClusterListHandler_RancherOnlyPreservesSourceFieldsAcrossFormats(t *testing.T) {
	rancherServer := startClusterListRancherServer(t)
	normanClient, err := norman.NewClientWithToken(rancherServer.URL, "fixture-token", true)
	if err != nil {
		t.Fatalf("NewClientWithToken() error = %v", err)
	}
	t.Cleanup(normanClient.Close)

	assertClusterListOutput(t, toolset.NewCombinedClient(normanClient, nil, false), []map[string]string{expectedRancherClusterRow()})
}

func TestClusterListHandler_RancherOnlyEmptyInventoryUsesNormalEmptyOutput(t *testing.T) {
	rancherServer := startClusterListRancherServerWithClusters(t, []map[string]interface{}{})
	normanClient, err := norman.NewClientWithToken(rancherServer.URL, "fixture-token", true)
	if err != nil {
		t.Fatalf("NewClientWithToken() error = %v", err)
	}
	t.Cleanup(normanClient.Close)

	for _, format := range []string{"json", "yaml", "table"} {
		t.Run(format, func(t *testing.T) {
			output, err := clusterListHandler(context.Background(), toolset.NewCombinedClient(normanClient, nil, false), map[string]interface{}{"format": format})
			if err != nil {
				t.Fatalf("clusterListHandler(%s) error = %v", format, err)
			}

			switch format {
			case "json", "yaml":
				var rows []map[string]string
				if format == "json" {
					err = json.Unmarshal([]byte(output), &rows)
				} else {
					err = yaml.Unmarshal([]byte(output), &rows)
				}
				if err != nil {
					t.Fatalf("decode %s output: %v", format, err)
				}
				if len(rows) != 0 {
					t.Fatalf("%s rows = %#v, want empty", format, rows)
				}
			case "table":
				if output != "" {
					t.Fatalf("table output = %q, want empty", output)
				}
			}
		})
	}
}

func TestClusterListHandler_NoClusterSourcesReturnsConfigurationError(t *testing.T) {
	for _, test := range []struct {
		name   string
		client *toolset.CombinedClient
	}{
		{name: "no clients", client: toolset.NewCombinedClient(nil, nil, false)},
		{name: "Steve without kubeconfig contexts", client: toolset.NewCombinedClient(nil, steve.NewClient("https://rancher.example.test", "fixture-token", "", "", true), false)},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := clusterListHandler(context.Background(), test.client, map[string]interface{}{"format": "json"})
			if !errors.Is(err, paramutil.ErrClusterSourcesNotConfigured) {
				t.Fatalf("clusterListHandler() error = %v, want %v", err, paramutil.ErrClusterSourcesNotConfigured)
			}
		})
	}
}

func expectedRancherClusterRow() map[string]string {
	return map[string]string{
		"id":       "c-rancher",
		"name":     "rancher",
		"source":   "rancher",
		"state":    "active",
		"provider": "Unknown",
		"version":  "",
		"nodes":    "1",
		"cpu":      "-/-",
		"ram":      "-/- GB",
		"pods":     "/",
	}
}

func assertClusterListOutput(t *testing.T, client interface{}, want []map[string]string) {
	t.Helper()
	for _, format := range []string{"json", "yaml", "table"} {
		output, err := clusterListHandler(context.Background(), client, map[string]interface{}{"format": format})
		if err != nil {
			t.Fatalf("clusterListHandler(%s) error = %v", format, err)
		}

		switch format {
		case "json":
			var got []map[string]string
			if err := json.Unmarshal([]byte(output), &got); err != nil {
				t.Fatalf("decode JSON output: %v", err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("JSON rows = %#v, want %#v", got, want)
			}
		case "yaml":
			var got []map[string]string
			if err := yaml.Unmarshal([]byte(output), &got); err != nil {
				t.Fatalf("decode YAML output: %v", err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("YAML rows = %#v, want %#v", got, want)
			}
		case "table":
			lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")
			if len(lines) < 3 {
				t.Fatalf("table output has %d lines, want header, separator, and row: %q", len(lines), output)
			}
			if got := strings.Fields(lines[0]); !reflect.DeepEqual(got, clusterListHeaders) {
				t.Fatalf("table headers = %v, want %v", got, clusterListHeaders)
			}
		}
	}
}

func startClusterListRancherServer(t *testing.T) *httptest.Server {
	return startClusterListRancherServerWithClusters(t, []map[string]interface{}{{
		"id":        "c-rancher",
		"name":      "rancher",
		"state":     "active",
		"nodeCount": 1,
	}})
}

func startClusterListRancherServerWithClusters(t *testing.T, clusters []map[string]interface{}) *httptest.Server {
	t.Helper()

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v3", "/v3/schemas":
			w.Header().Set("X-API-Schemas", server.URL+"/v3/schemas")
			_ = json.NewEncoder(w).Encode(types.SchemaCollection{Data: []types.Schema{{
				ID:                "cluster",
				Type:              "/meta/schemas/schema",
				Links:             map[string]string{"collection": server.URL + "/v3/clusters"},
				Version:           types.APIVersion{Path: "/v3", Version: "v3"},
				CollectionMethods: []string{http.MethodGet},
			}}})
		case "/v3/clusters":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": clusters,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func writeClusterListKubeconfig(t *testing.T, contextName, server string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := "apiVersion: v1\n" +
		"kind: Config\n" +
		"clusters:\n" +
		"- name: target\n" +
		"  cluster:\n" +
		"    server: " + server + "\n" +
		"contexts:\n" +
		"- name: " + contextName + "\n" +
		"  context:\n" +
		"    cluster: target\n" +
		"    user: fixture-user\n" +
		"users:\n" +
		"- name: fixture-user\n" +
		"  user: {}\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write kubeconfig fixture: %v", err)
	}
	return path
}
