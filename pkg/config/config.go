package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// StaticConfig represents the static configuration for the Rancher MCP Server
type StaticConfig struct {
	// Server configuration
	Port int `mapstructure:"port"`

	SSEBaseURL string `mapstructure:"sse_base_url"`

	// Logging configuration
	LogLevel int `mapstructure:"log_level"`

	// Rancher configuration
	RancherServerURL string `mapstructure:"rancher_server_url"`
	RancherToken     string `mapstructure:"rancher_token"`
	RancherAccessKey string `mapstructure:"rancher_access_key"`
	RancherSecretKey string `mapstructure:"rancher_secret_key"`
	RancherTLSInsecure bool `mapstructure:"rancher_tls_insecure"`

	// Security configuration
	ReadOnly           bool `mapstructure:"read_only"`
	DisableDestructive bool `mapstructure:"disable_destructive"`

	// Output configuration
	ListOutput string `mapstructure:"list_output"`

	// Toolset configuration
	Toolsets      []string `mapstructure:"toolsets"`
	EnabledTools  []string `mapstructure:"enabled_tools"`
	DisabledTools []string `mapstructure:"disabled_tools"`
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

// LoadConfig loads configuration from file and environment variables using Viper
// Priority: command-line flags > environment variables > config file > defaults
func LoadConfig(configPath string) (*StaticConfig, error) {
	// Use the global viper instance to access bound command-line flags
	v := viper.GetViper()

	// Set configuration file if provided
	if configPath != "" {
		v.SetConfigFile(configPath)
		v.SetConfigType("yaml")
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// Configure environment variable support
	// Environment variables use RANCHER_MCP_ prefix and replace - with _
	v.SetEnvPrefix("RANCHER_MCP")
	v.AllowEmptyEnv(true)
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))
	v.AutomaticEnv()

	// Unmarshal configuration into struct
	config := &StaticConfig{}
	if err := v.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	
	// Apply defaults for empty values
	applyDefaults(config)

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, err
	}

	return config, nil
}

// applyDefaults applies default values to empty configuration fields
func applyDefaults(config *StaticConfig) {
	if config.ListOutput == "" {
		config.ListOutput = "table"
	}
	if len(config.Toolsets) == 0 {
		config.Toolsets = []string{"config", "core", "rancher"}
	}
}

// HasRancherConfig returns true if Rancher configuration is present
func (c *StaticConfig) HasRancherConfig() bool {
	return c.RancherServerURL != "" && (c.RancherToken != "" || (c.RancherAccessKey != "" && c.RancherSecretKey != ""))
}

// GetPortString returns the port as a string in the format ":port"
func (c *StaticConfig) GetPortString() string {
	if c.Port == 0 {
		return ""
	}
	return fmt.Sprintf(":%d", c.Port)
}
