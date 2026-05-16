package rancher

import (
	"testing"

	"github.com/futuretea/rancher-mcp-server/pkg/client/norman"
)

func TestParseResourceString(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want string
	}{
		{"empty", "", "-"},
		{"KiB to GB", "1048576Ki", "1"},
		{"MiB to GB", "1024Mi", "1"},
		{"millicores to cores", "1500m", "1.50"},
		{"zero millicores", "0m", "0.00"},
		{"plain string passthrough", "1.5", "1.5"},
		{"invalid Ki", "fooKi", "fooKi"},
		{"invalid Mi", "barMi", "barMi"},
		{"invalid m", "bazm", "bazm"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseResourceString(tt.s)
			if got != tt.want {
				t.Errorf("parseResourceString(%q) = %q, want %q", tt.s, got, tt.want)
			}
		})
	}
}

func TestFormatFloat(t *testing.T) {
	tests := []struct {
		name string
		f    float64
		want string
	}{
		{"integer", 1.0, "1"},
		{"one decimal", 1.5, "1.5"},
		{"two decimals", 1.25, "1.25"},
		{"trailing zero trimmed", 1.50, "1.5"},
		{"very small", 0.1, "0.1"},
		{"zero", 0.0, "0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatFloat(tt.f)
			if got != tt.want {
				t.Errorf("formatFloat(%v) = %q, want %q", tt.f, got, tt.want)
			}
		})
	}
}

func TestGetClusterProvider(t *testing.T) {
	tests := []struct {
		name     string
		driver   string
		provider string
		want     string
	}{
		{"imported rke2", "imported", "rke2", "RKE2"},
		{"imported k3s", "imported", "k3s", "K3S"},
		{"imported unknown", "imported", "other", "Imported"},
		{"native k3s", "k3s", "", "K3S"},
		{"native rke2", "rke2", "", "RKE2"},
		{"RKE", "rancherKubernetesEngine", "", "Rancher Kubernetes Engine"},
		{"AKS variant 1", "azureKubernetesService", "", "Azure Kubernetes Service"},
		{"AKS variant 2", "AKS", "", "Azure Kubernetes Service"},
		{"GKE variant 1", "googleKubernetesEngine", "", "Google Kubernetes Engine"},
		{"GKE variant 2", "GKE", "", "Google Kubernetes Engine"},
		{"EKS", "EKS", "", "Elastic Kubernetes Service"},
		{"unknown", "other", "", "Unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := norman.Cluster{Driver: tt.driver, Provider: tt.provider}
			got := getClusterProvider(c)
			if got != tt.want {
				t.Errorf("getClusterProvider({Driver:%q, Provider:%q}) = %q, want %q",
					tt.driver, tt.provider, got, tt.want)
			}
		})
	}
}

func TestFilterClustersByName(t *testing.T) {
	clusters := []norman.Cluster{
		{Name: "prod-cluster"},
		{Name: "staging-cluster"},
		{Name: "dev-cluster"},
	}

	t.Run("empty filter returns all", func(t *testing.T) {
		result := filterClustersByName(clusters, "")
		if len(result) != 3 {
			t.Fatalf("expected 3 clusters, got %d", len(result))
		}
	})

	t.Run("case insensitive partial match", func(t *testing.T) {
		result := filterClustersByName(clusters, "PROD")
		if len(result) != 1 || result[0].Name != "prod-cluster" {
			t.Fatalf("expected [prod-cluster], got %v", result)
		}
	})

	t.Run("no match", func(t *testing.T) {
		result := filterClustersByName(clusters, "nonexistent")
		if len(result) != 0 {
			t.Fatalf("expected 0 clusters, got %d", len(result))
		}
	})
}
