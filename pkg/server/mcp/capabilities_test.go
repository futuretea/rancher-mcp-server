package mcp

import (
	"testing"

	"github.com/futuretea/rancher-mcp-server/pkg/core/config"
)

func TestHasRancherCapability_DynamicMode(t *testing.T) {
	cfg := &Configuration{
		StaticConfig: &config.StaticConfig{
			RancherServerURL:        "https://rancher.example.com",
			RancherRequestTokenAuth: true,
		},
	}
	if !cfg.HasRancherCapability() {
		t.Fatal("expected HasRancherCapability to be true in dynamic mode")
	}
	if cfg.HasRancherConfig() {
		t.Fatal("expected HasRancherConfig to remain false in dynamic mode")
	}
}

func TestHasRancherCapability_DynamicModeRequiresURL(t *testing.T) {
	cfg := &Configuration{
		StaticConfig: &config.StaticConfig{
			RancherRequestTokenAuth: true,
		},
	}
	if cfg.HasRancherCapability() {
		t.Fatal("expected HasRancherCapability to be false without server URL")
	}
}

func TestCapabilityStatuses_DynamicMode(t *testing.T) {
	server, err := NewServer(Configuration{
		StaticConfig: &config.StaticConfig{
			RancherServerURL:        "https://rancher.example.com",
			RancherRequestTokenAuth: true,
		},
	})
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	defer server.Close()

	statuses := server.capabilityStatuses()

	rancherStatus, ok := statuses["rancher"]
	if !ok {
		t.Fatal("expected rancher capability")
	}
	if !rancherStatus.Configured || !rancherStatus.Available {
		t.Fatalf("expected rancher capability configured and available in dynamic mode, got %+v", rancherStatus)
	}

	kubernetesStatus, ok := statuses["kubernetes"]
	if !ok {
		t.Fatal("expected kubernetes capability")
	}
	if !kubernetesStatus.Configured || !kubernetesStatus.Available {
		t.Fatalf("expected kubernetes capability configured and available in dynamic mode, got %+v", kubernetesStatus)
	}
}
