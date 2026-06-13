package config

import "testing"

func TestValidate_Port(t *testing.T) {
	t.Run("valid port", func(t *testing.T) {
		c := &StaticConfig{Port: 8080, ListOutput: "json"}
		if err := c.Validate(); err != nil {
			t.Fatalf("expected valid, got: %v", err)
		}
	})
	t.Run("negative port", func(t *testing.T) {
		c := &StaticConfig{Port: -1}
		if err := c.Validate(); err == nil {
			t.Fatal("expected error for negative port")
		}
	})
	t.Run("port too high", func(t *testing.T) {
		c := &StaticConfig{Port: 99999}
		if err := c.Validate(); err == nil {
			t.Fatal("expected error for port > 65535")
		}
	})
}

func TestValidate_LogLevel(t *testing.T) {
	t.Run("valid level", func(t *testing.T) {
		c := &StaticConfig{Port: 8080, LogLevel: 3, ListOutput: "json"}
		if err := c.Validate(); err != nil {
			t.Fatalf("expected valid, got: %v", err)
		}
	})
	t.Run("negative log level", func(t *testing.T) {
		c := &StaticConfig{Port: 8080, LogLevel: -1, ListOutput: "json"}
		if err := c.Validate(); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("log level too high", func(t *testing.T) {
		c := &StaticConfig{Port: 8080, LogLevel: 10, ListOutput: "json"}
		if err := c.Validate(); err == nil {
			t.Fatal("expected error for log_level > 9")
		}
	})
}

func TestValidate_ListOutput(t *testing.T) {
	t.Run("empty is invalid", func(t *testing.T) {
		c := &StaticConfig{Port: 8080, ListOutput: ""}
		if err := c.Validate(); err == nil {
			t.Fatal("expected error for empty list_output")
		}
	})
	t.Run("valid table", func(t *testing.T) {
		c := &StaticConfig{Port: 8080, ListOutput: "table"}
		if err := c.Validate(); err != nil {
			t.Fatalf("expected valid, got: %v", err)
		}
	})
	t.Run("valid json uppercase", func(t *testing.T) {
		c := &StaticConfig{Port: 8080, ListOutput: "JSON"}
		if err := c.Validate(); err != nil {
			t.Fatalf("expected valid (case insensitive), got: %v", err)
		}
	})
	t.Run("invalid output", func(t *testing.T) {
		c := &StaticConfig{Port: 8080, ListOutput: "xml"}
		if err := c.Validate(); err == nil {
			t.Fatal("expected error for invalid list_output")
		}
	})
}

func TestValidate_RancherAuth(t *testing.T) {
	t.Run("no rancher config is valid", func(t *testing.T) {
		c := &StaticConfig{Port: 8080, ListOutput: "json"}
		if err := c.Validate(); err != nil {
			t.Fatalf("empty Rancher config should be valid, got: %v", err)
		}
	})

	t.Run("invalid URL scheme", func(t *testing.T) {
		c := &StaticConfig{Port: 8080, ListOutput: "json", RancherServerURL: "ftp://rancher"}
		if err := c.Validate(); err == nil {
			t.Fatal("expected error for invalid URL scheme")
		}
	})

	t.Run("URL with token is valid", func(t *testing.T) {
		c := &StaticConfig{Port: 8080, ListOutput: "json", RancherServerURL: "https://rancher.example.com", RancherToken: "token-123"}
		if err := c.Validate(); err != nil {
			t.Fatalf("expected valid, got: %v", err)
		}
	})

	t.Run("URL with access/secret key is valid", func(t *testing.T) {
		c := &StaticConfig{Port: 8080, ListOutput: "json", RancherServerURL: "https://rancher.example.com", RancherAccessKey: "ak", RancherSecretKey: "sk"}
		if err := c.Validate(); err != nil {
			t.Fatalf("expected valid, got: %v", err)
		}
	})

	t.Run("URL without auth fails", func(t *testing.T) {
		c := &StaticConfig{Port: 8080, ListOutput: "json", RancherServerURL: "https://rancher.example.com"}
		if err := c.Validate(); err == nil {
			t.Fatal("expected error for missing auth")
		}
	})

	t.Run("both auth methods fails", func(t *testing.T) {
		c := &StaticConfig{Port: 8080, ListOutput: "json", RancherServerURL: "https://rancher.example.com", RancherToken: "t", RancherAccessKey: "ak", RancherSecretKey: "sk"}
		if err := c.Validate(); err == nil {
			t.Fatal("expected error for conflicting auth methods")
		}
	})

	t.Run("access key without secret fails", func(t *testing.T) {
		c := &StaticConfig{Port: 8080, ListOutput: "json", RancherServerURL: "https://rancher.example.com", RancherAccessKey: "ak"}
		if err := c.Validate(); err == nil {
			t.Fatal("expected error for incomplete key auth")
		}
	})
}

func TestHasRancherConfig(t *testing.T) {
	t.Run("empty config", func(t *testing.T) {
		c := &StaticConfig{}
		if c.HasRancherConfig() {
			t.Fatal("expected false for empty config")
		}
	})
	t.Run("token auth", func(t *testing.T) {
		c := &StaticConfig{RancherServerURL: "https://r", RancherToken: "t"}
		if !c.HasRancherConfig() {
			t.Fatal("expected true for token auth")
		}
	})
	t.Run("missing URL", func(t *testing.T) {
		c := &StaticConfig{RancherToken: "t"}
		if c.HasRancherConfig() {
			t.Fatal("expected false without server URL")
		}
	})
}

func TestGetPortString(t *testing.T) {
	if s := (&StaticConfig{Port: 0}).GetPortString(); s != "" {
		t.Errorf("expected empty for port 0, got %q", s)
	}
	if s := (&StaticConfig{Port: 8080, ListOutput: "json"}).GetPortString(); s != ":8080" {
		t.Errorf("expected ':8080', got %q", s)
	}
}

func TestValidate_DynamicModeOnlyServerURL(t *testing.T) {
	c := &StaticConfig{
		Port:                    8080,
		ListOutput:              "json",
		RancherServerURL:        "https://rancher.example.com",
		RancherRequestTokenAuth: true,
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("expected dynamic mode with only server URL to be valid, got: %v", err)
	}
}

func TestValidate_DynamicModeRejectsEmptyServerURL(t *testing.T) {
	c := &StaticConfig{
		Port:                    8080,
		ListOutput:              "json",
		RancherRequestTokenAuth: true,
	}
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for dynamic mode without rancher_server_url")
	}
}

func TestValidate_DynamicModeRejectsStaticCredentials(t *testing.T) {
	cases := []struct {
		name string
		cfg  *StaticConfig
	}{
		{
			name: "token",
			cfg: &StaticConfig{
				Port:                    8080,
				ListOutput:              "json",
				RancherServerURL:        "https://rancher.example.com",
				RancherRequestTokenAuth: true,
				RancherToken:            "token-123",
			},
		},
		{
			name: "access key",
			cfg: &StaticConfig{
				Port:                    8080,
				ListOutput:              "json",
				RancherServerURL:        "https://rancher.example.com",
				RancherRequestTokenAuth: true,
				RancherAccessKey:        "ak",
			},
		},
		{
			name: "secret key",
			cfg: &StaticConfig{
				Port:                    8080,
				ListOutput:              "json",
				RancherServerURL:        "https://rancher.example.com",
				RancherRequestTokenAuth: true,
				RancherSecretKey:        "sk",
			},
		},
		{
			name: "access and secret key",
			cfg: &StaticConfig{
				Port:                    8080,
				ListOutput:              "json",
				RancherServerURL:        "https://rancher.example.com",
				RancherRequestTokenAuth: true,
				RancherAccessKey:        "ak",
				RancherSecretKey:        "sk",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.cfg.Validate(); err == nil {
				t.Fatal("expected error when combining dynamic mode with static credentials")
			}
		})
	}
}

func TestHasRancherConfig_DynamicModeFalse(t *testing.T) {
	c := &StaticConfig{
		RancherServerURL:        "https://rancher.example.com",
		RancherRequestTokenAuth: true,
	}
	if c.HasRancherConfig() {
		t.Fatal("expected HasRancherConfig to remain false in dynamic mode")
	}
}
