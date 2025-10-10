package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// StaticConfig represents the static configuration for the Rancher MCP Server
type StaticConfig struct {
	// Server configuration
	Port int `yaml:"port"`

	// Logging configuration
	LogLevel int `yaml:"log_level"`

	// Rancher configuration
	RancherServerURL string `yaml:"rancher_server_url"`
	RancherToken     string `yaml:"rancher_token"`
	RancherAccessKey string `yaml:"rancher_access_key"`
	RancherSecretKey string `yaml:"rancher_secret_key"`

	// Security configuration
	ReadOnly           bool `yaml:"read_only"`
	DisableDestructive bool `yaml:"disable_destructive"`

	// Output configuration
	ListOutput string `yaml:"list_output"`

	// Toolset configuration
	Toolsets      []string `yaml:"toolsets"`
	EnabledTools  []string `yaml:"enabled_tools"`
	DisabledTools []string `yaml:"disabled_tools"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *StaticConfig {
	return &StaticConfig{
		Port:               0, // 0 means stdio mode
		LogLevel:           0,
		ListOutput:         "table",
		Toolsets:           []string{"config", "core", "rancher"},
		ReadOnly:           false,
		DisableDestructive: false,
	}
}

// Validate validates the configuration
func (c *StaticConfig) Validate() error {
	// Validate port
	if c.Port < 0 || c.Port > 65535 {
		return fmt.Errorf("port must be between 0 and 65535, got %d", c.Port)
	}

	// Validate log level
	if c.LogLevel < 0 || c.LogLevel > 9 {
		return fmt.Errorf("log_level must be between 0 and 9, got %d", c.LogLevel)
	}

	// Validate list output
	validOutputs := map[string]bool{
		"table": true,
		"yaml":  true,
		"json":  true,
	}
	if !validOutputs[strings.ToLower(c.ListOutput)] {
		return fmt.Errorf("list_output must be one of: table, yaml, json, got %s", c.ListOutput)
	}

	// Validate Rancher configuration
	if c.RancherServerURL != "" {
		if !strings.HasPrefix(c.RancherServerURL, "http://") && !strings.HasPrefix(c.RancherServerURL, "https://") {
			return fmt.Errorf("rancher_server_url must start with http:// or https://, got %s", c.RancherServerURL)
		}

		// Check authentication methods
		hasTokenAuth := c.RancherToken != ""
		hasKeyAuth := c.RancherAccessKey != "" && c.RancherSecretKey != ""

		if !hasTokenAuth && !hasKeyAuth {
			return fmt.Errorf("rancher authentication required: either rancher_token or both rancher_access_key and rancher_secret_key must be provided")
		}

		if hasTokenAuth && hasKeyAuth {
			return fmt.Errorf("cannot use both rancher_token and rancher_access_key/rancher_secret_key authentication methods")
		}
	}

	return nil
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(configPath string) (*StaticConfig, error) {
	config := DefaultConfig()

	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file %s: %v", configPath, err)
		}

		if err := yaml.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse config file %s: %v", configPath, err)
		}
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}

	return config, nil
}

// HasRancherConfig returns true if Rancher configuration is present
func (c *StaticConfig) HasRancherConfig() bool {
	return c.RancherServerURL != "" && (c.RancherToken != "" || (c.RancherAccessKey != "" && c.RancherSecretKey != ""))
}
