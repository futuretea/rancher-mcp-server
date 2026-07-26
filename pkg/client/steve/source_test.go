package steve

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"k8s.io/client-go/rest"
)

func TestNewClientWithKubeconfigPaths_UsesConfiguredContextDirectly(t *testing.T) {
	path := writeKubeconfig(t, "direct", "https://direct.example.test", "")

	client, err := NewClientWithKubeconfigPaths("", "", "", "", false, []string{path})
	if err != nil {
		t.Fatalf("NewClientWithKubeconfigPaths() error = %v", err)
	}

	config, err := client.createRestConfig("kubeconfig:direct")
	if err != nil {
		t.Fatalf("createRestConfig() error = %v", err)
	}
	if config.Host != "https://direct.example.test" {
		t.Fatalf("Host = %q, want direct kubeconfig server", config.Host)
	}
	if strings.Contains(config.Host, "/k8s/clusters/") {
		t.Fatalf("Host = %q must not be a Rancher Steve URL", config.Host)
	}
}

func TestNewClientWithKubeconfigPaths_PreservesExecConfiguration(t *testing.T) {
	path := writeKubeconfig(t, "exec-context", "https://direct.example.test", `
    exec:
      apiVersion: client.authentication.k8s.io/v1
      command: fixture-command
      interactiveMode: Never`)

	client, err := NewClientWithKubeconfigPaths("", "", "", "", false, []string{path})
	if err != nil {
		t.Fatalf("NewClientWithKubeconfigPaths() error = %v", err)
	}

	config, err := client.createRestConfig("kubeconfig:exec-context")
	if err != nil {
		t.Fatalf("createRestConfig() error = %v", err)
	}
	if config.ExecProvider == nil {
		t.Fatal("ExecProvider must be preserved from the kubeconfig")
	}
	if config.Transport != nil || config.Insecure || !reflect.DeepEqual(config.TLSClientConfig, rest.TLSClientConfig{}) {
		t.Fatal("kubeconfig rest.Config must not be post-processed")
	}
}

func TestNewClientWithKubeconfigPaths_PreservesKubeconfigTLSConfiguration(t *testing.T) {
	t.Run("certificate material and server name", func(t *testing.T) {
		path := writeKubeconfigWithCluster(t, "tls-context", "https://direct.example.test", `
    certificate-authority-data: Y2EtZGF0YQ==
    tls-server-name: direct.internal`, `
    client-certificate-data: Y2xpZW50LWNlcnQ=
    client-key-data: Y2xpZW50LWtleQ==
    exec:
      apiVersion: client.authentication.k8s.io/v1
      command: fixture-command
      interactiveMode: Never`)

		client, err := NewClientWithKubeconfigPaths("", "", "", "", false, []string{path})
		if err != nil {
			t.Fatalf("NewClientWithKubeconfigPaths() error = %v", err)
		}

		config, err := client.createRestConfig("kubeconfig:tls-context")
		if err != nil {
			t.Fatalf("createRestConfig() error = %v", err)
		}
		if config.ExecProvider == nil {
			t.Fatal("ExecProvider must be preserved alongside TLS configuration")
		}
		if !bytes.Equal(config.CAData, []byte("ca-data")) {
			t.Fatalf("CAData = %q, want kubeconfig data", config.CAData)
		}
		if !bytes.Equal(config.CertData, []byte("client-cert")) {
			t.Fatalf("CertData = %q, want kubeconfig data", config.CertData)
		}
		if !bytes.Equal(config.KeyData, []byte("client-key")) {
			t.Fatalf("KeyData = %q, want kubeconfig data", config.KeyData)
		}
		if config.ServerName != "direct.internal" {
			t.Fatalf("ServerName = %q, want kubeconfig value", config.ServerName)
		}
	})

	t.Run("insecure setting", func(t *testing.T) {
		path := writeKubeconfigWithCluster(t, "insecure-context", "https://direct.example.test", `
    insecure-skip-tls-verify: true`, "")

		client, err := NewClientWithKubeconfigPaths("", "", "", "", false, []string{path})
		if err != nil {
			t.Fatalf("NewClientWithKubeconfigPaths() error = %v", err)
		}

		config, err := client.createRestConfig("kubeconfig:insecure-context")
		if err != nil {
			t.Fatalf("createRestConfig() error = %v", err)
		}
		if !config.Insecure {
			t.Fatal("Insecure must be preserved from the kubeconfig")
		}
	})
}

func TestNewClientWithKubeconfigPaths_RoutesBareReferenceToRancherWhenKubeconfigAlsoConfigured(t *testing.T) {
	path := writeKubeconfig(t, "direct", "https://direct.example.test", "")
	client, err := NewClientWithKubeconfigPaths("https://rancher.example.test", "fixture-token", "", "", false, []string{path})
	if err != nil {
		t.Fatalf("NewClientWithKubeconfigPaths() error = %v", err)
	}

	config, err := client.createRestConfig("c-rancher")
	if err != nil {
		t.Fatalf("createRestConfig() error = %v", err)
	}
	if config.Host != "https://rancher.example.test/k8s/clusters/c-rancher" {
		t.Fatalf("Host = %q, want Rancher Steve URL", config.Host)
	}
}

func TestCreateRestConfig_RejectsInvalidClusterReferences(t *testing.T) {
	path := writeKubeconfig(t, "direct", "https://direct.example.test", "")
	client, err := NewClientWithKubeconfigPaths("", "", "", "", false, []string{path})
	if err != nil {
		t.Fatalf("NewClientWithKubeconfigPaths() error = %v", err)
	}

	for _, reference := range []string{"", "c-rancher", "kubeconfig:", "kubeconfig:missing", "kubeconfig:../escape", "unknown:cluster"} {
		t.Run(reference, func(t *testing.T) {
			_, err := client.createRestConfig(reference)
			var referenceErr *ClusterReferenceError
			if !errors.As(err, &referenceErr) {
				t.Fatalf("createRestConfig(%q) error = %v, want ClusterReferenceError", reference, err)
			}
		})
	}
}

func TestCreateRestConfig_RoutingMatrix(t *testing.T) {
	path := writeKubeconfig(t, "direct", "https://direct.example.test", "")
	mixed, err := NewClientWithKubeconfigPaths("https://rancher.example.test", "fixture-token", "", "", false, []string{path})
	if err != nil {
		t.Fatalf("NewClientWithKubeconfigPaths(mixed) error = %v", err)
	}
	rancherOnly, err := NewClientWithKubeconfigPaths("https://rancher.example.test", "fixture-token", "", "", false, nil)
	if err != nil {
		t.Fatalf("NewClientWithKubeconfigPaths(rancher-only) error = %v", err)
	}
	kubeconfigOnly, err := NewClientWithKubeconfigPaths("", "", "", "", false, []string{path})
	if err != nil {
		t.Fatalf("NewClientWithKubeconfigPaths(kubeconfig-only) error = %v", err)
	}

	for _, test := range []struct {
		name      string
		client    *Client
		reference string
		wantHost  string
	}{
		{name: "prefixed configured", client: mixed, reference: "kubeconfig:direct", wantHost: "https://direct.example.test"},
		{name: "bare Rancher", client: mixed, reference: "c-rancher", wantHost: "https://rancher.example.test/k8s/clusters/c-rancher"},
		{name: "prefixed unconfigured", client: rancherOnly, reference: "kubeconfig:direct"},
		{name: "bare kubeconfig-only", client: kubeconfigOnly, reference: "c-rancher"},
		{name: "malformed prefix", client: mixed, reference: "unknown:cluster"},
		{name: "traversal", client: mixed, reference: "kubeconfig:../escape"},
	} {
		t.Run(test.name, func(t *testing.T) {
			config, err := test.client.createRestConfig(test.reference)
			if test.wantHost != "" {
				if err != nil {
					t.Fatalf("createRestConfig(%q) error = %v", test.reference, err)
				}
				if config.Host != test.wantHost {
					t.Fatalf("createRestConfig(%q) host = %q, want %q", test.reference, config.Host, test.wantHost)
				}
				return
			}

			var referenceErr *ClusterReferenceError
			if !errors.As(err, &referenceErr) {
				t.Fatalf("createRestConfig(%q) error = %v, want ClusterReferenceError", test.reference, err)
			}
		})
	}
}

func TestNewClientWithKubeconfigPaths_FirstPathWins(t *testing.T) {
	directory := t.TempDir()
	first := filepath.Join(directory, "first.yaml")
	second := filepath.Join(directory, "second.yaml")
	writeKubeconfigAt(t, first, "shared", "https://first.example.test", "")
	writeKubeconfigAt(t, second, "shared", "https://second.example.test", "")

	client, err := NewClientWithKubeconfigPaths("", "", "", "", false, []string{first, second})
	if err != nil {
		t.Fatalf("NewClientWithKubeconfigPaths() error = %v", err)
	}
	config, err := client.createRestConfig("kubeconfig:shared")
	if err != nil {
		t.Fatalf("createRestConfig() error = %v", err)
	}
	if config.Host != "https://first.example.test" {
		t.Fatalf("Host = %q, want first configured path", config.Host)
	}
}

func writeKubeconfig(t *testing.T, contextName, server, userConfig string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	writeKubeconfigAt(t, path, contextName, server, userConfig)
	return path
}

func writeKubeconfigAt(t *testing.T, path, contextName, server, userConfig string) {
	writeKubeconfigAtWithCluster(t, path, contextName, server, "", userConfig)
}

func writeKubeconfigWithCluster(t *testing.T, contextName, server, clusterConfig, userConfig string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	writeKubeconfigAtWithCluster(t, path, contextName, server, clusterConfig, userConfig)
	return path
}

func writeKubeconfigAtWithCluster(t *testing.T, path, contextName, server, clusterConfig, userConfig string) {
	t.Helper()
	content := "apiVersion: v1\n" +
		"kind: Config\n" +
		"clusters:\n" +
		"- name: target\n" +
		"  cluster:\n" +
		"    server: " + server + "\n" +
		clusterConfig + "\n" +
		"contexts:\n" +
		"- name: " + contextName + "\n" +
		"  context:\n" +
		"    cluster: target\n" +
		"    user: fixture-user\n" +
		"current-context: " + contextName + "\n" +
		"users:\n" +
		"- name: fixture-user\n" +
		"  user:" + userConfig + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write kubeconfig fixture: %v", err)
	}
}
