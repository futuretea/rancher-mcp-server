package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.Port != 0 {
		t.Errorf("Expected Port to be 0, got %d", config.Port)
	}

	if config.LogLevel != 0 {
		t.Errorf("Expected LogLevel to be 0, got %d", config.LogLevel)
	}

	if config.ListOutput != "table" {
		t.Errorf("Expected ListOutput to be 'table', got '%s'", config.ListOutput)
	}

	if len(config.Toolsets) != 3 {
		t.Errorf("Expected 3 default toolsets, got %d", len(config.Toolsets))
	}

	expectedToolsets := []string{"config", "core", "rancher"}
	for i, toolset := range expectedToolsets {
		if config.Toolsets[i] != toolset {
			t.Errorf("Expected toolsets[%d] to be '%s', got '%s'", i, toolset, config.Toolsets[i])
		}
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *StaticConfig
		wantErr bool
	}{
		{
			name:    "valid default config",
			config:  DefaultConfig(),
			wantErr: false,
		},
		{
			name: "valid port",
			config: &StaticConfig{
				Port:       8080,
				ListOutput: "table",
			},
			wantErr: false,
		},
		{
			name: "invalid port negative",
			config: &StaticConfig{
				Port: -1,
			},
			wantErr: true,
		},
		{
			name: "invalid port too high",
			config: &StaticConfig{
				Port: 65536,
			},
			wantErr: true,
		},
		{
			name: "valid log level",
			config: &StaticConfig{
				LogLevel:   5,
				ListOutput: "table",
			},
			wantErr: false,
		},
		{
			name: "invalid log level negative",
			config: &StaticConfig{
				LogLevel: -1,
			},
			wantErr: true,
		},
		{
			name: "invalid log level too high",
			config: &StaticConfig{
				LogLevel: 10,
			},
			wantErr: true,
		},
		{
			name: "valid list output table",
			config: &StaticConfig{
				ListOutput: "table",
			},
			wantErr: false,
		},
		{
			name: "valid list output yaml",
			config: &StaticConfig{
				ListOutput: "yaml",
			},
			wantErr: false,
		},
		{
			name: "invalid list output",
			config: &StaticConfig{
				ListOutput: "invalid",
			},
			wantErr: true, // Should error for invalid list output values
		},
		{
			name: "valid rancher config with token",
			config: &StaticConfig{
				RancherServerURL: "https://rancher.example.com",
				RancherToken:     "token123",
				ListOutput:       "table",
			},
			wantErr: false,
		},
		{
			name: "valid rancher config with access key",
			config: &StaticConfig{
				RancherServerURL: "https://rancher.example.com",
				RancherAccessKey: "access123",
				RancherSecretKey: "secret123",
				ListOutput:       "table",
			},
			wantErr: false,
		},
		{
			name: "invalid rancher URL",
			config: &StaticConfig{
				RancherServerURL: "rancher.example.com", // missing protocol
				RancherToken:     "token123",
			},
			wantErr: true,
		},
		{
			name: "rancher URL without auth",
			config: &StaticConfig{
				RancherServerURL: "https://rancher.example.com",
			},
			wantErr: true,
		},
		{
			name: "rancher with both auth methods",
			config: &StaticConfig{
				RancherServerURL: "https://rancher.example.com",
				RancherToken:     "token123",
				RancherAccessKey: "access123",
				RancherSecretKey: "secret123",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
port: 8080
log_level: 2
list_output: yaml
rancher_server_url: https://rancher.example.com
rancher_token: test-token
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Test loading config
	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if config.Port != 8080 {
		t.Errorf("Expected Port to be 8080, got %d", config.Port)
	}

	if config.LogLevel != 2 {
		t.Errorf("Expected LogLevel to be 2, got %d", config.LogLevel)
	}

	if config.ListOutput != "yaml" {
		t.Errorf("Expected ListOutput to be 'yaml', got '%s'", config.ListOutput)
	}

	if config.RancherServerURL != "https://rancher.example.com" {
		t.Errorf("Expected RancherServerURL to be 'https://rancher.example.com', got '%s'", config.RancherServerURL)
	}

	if config.RancherToken != "test-token" {
		t.Errorf("Expected RancherToken to be 'test-token', got '%s'", config.RancherToken)
	}

	// Test loading non-existent config
	_, err = LoadConfig("/non/existent/config.yaml")
	if err == nil {
		t.Error("Expected error for non-existent config file")
	}

	// Test loading invalid config
	invalidConfigPath := filepath.Join(tmpDir, "invalid.yaml")
	os.WriteFile(invalidConfigPath, []byte("invalid: yaml: content: ["), 0644)

	_, err = LoadConfig(invalidConfigPath)
	if err == nil {
		t.Error("Expected error for invalid config file")
	}
}

func TestHasRancherConfig(t *testing.T) {
	tests := []struct {
		name   string
		config *StaticConfig
		expect bool
	}{
		{
			name:   "no rancher config",
			config: &StaticConfig{},
			expect: false,
		},
		{
			name: "rancher config with token",
			config: &StaticConfig{
				RancherServerURL: "https://rancher.example.com",
				RancherToken:     "token123",
			},
			expect: true,
		},
		{
			name: "rancher config with access key",
			config: &StaticConfig{
				RancherServerURL: "https://rancher.example.com",
				RancherAccessKey: "access123",
				RancherSecretKey: "secret123",
			},
			expect: true,
		},
		{
			name: "rancher URL without auth",
			config: &StaticConfig{
				RancherServerURL: "https://rancher.example.com",
			},
			expect: false,
		},
		{
			name: "auth without URL",
			config: &StaticConfig{
				RancherToken: "token123",
			},
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.HasRancherConfig()
			if result != tt.expect {
				t.Errorf("HasRancherConfig() = %v, want %v", result, tt.expect)
			}
		})
	}
}

func TestGetPortString(t *testing.T) {
	tests := []struct {
		name   string
		config *StaticConfig
		expect string
	}{
		{
			name:   "stdio mode (port 0)",
			config: &StaticConfig{Port: 0},
			expect: "",
		},
		{
			name:   "http mode port 8080",
			config: &StaticConfig{Port: 8080},
			expect: ":8080",
		},
		{
			name:   "http mode port 3000",
			config: &StaticConfig{Port: 3000},
			expect: ":3000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.GetPortString()
			if result != tt.expect {
				t.Errorf("GetPortString() = %v, want %v", result, tt.expect)
			}
		})
	}
}

