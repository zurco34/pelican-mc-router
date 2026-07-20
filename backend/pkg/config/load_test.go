package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	workingDirectory := changeWorkingDirectory(t, t.TempDir())
	defer workingDirectory()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf(
			"Server.Host = %q, want %q",
			cfg.Server.Host,
			"0.0.0.0",
		)
	}

	if cfg.Server.Port != 8080 {
		t.Errorf(
			"Server.Port = %d, want %d",
			cfg.Server.Port,
			8080,
		)
	}

	if cfg.Pelican.Timeout != 15*time.Second {
		t.Errorf(
			"Pelican.Timeout = %v, want %v",
			cfg.Pelican.Timeout,
			15*time.Second,
		)
	}

	if cfg.Discovery.Interval != 30*time.Second {
		t.Errorf(
			"Discovery.Interval = %v, want %v",
			cfg.Discovery.Interval,
			30*time.Second,
		)
	}
}

func TestLoadFromFile(t *testing.T) {
	directory := t.TempDir()

	configContent := []byte(`
server:
  host: "127.0.0.1"
  port: 9090

pelican:
  url: "https://panel.example.com"
  api_key: "test-key"
  timeout: 20s

discovery:
  interval: 45s

router:
  backend: "infrared"
  domain: "mc.example.com"

infrared:
  config_path: "/tmp/infrared.json"
  reload_signal: "SIGUSR1"

logging:
  level: "debug"
  format: "console"
`)

	configPath := filepath.Join(directory, "config.yaml")

	if err := os.WriteFile(configPath, configContent, 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	restore := changeWorkingDirectory(t, directory)
	defer restore()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf(
			"Server.Host = %q, want %q",
			cfg.Server.Host,
			"127.0.0.1",
		)
	}

	if cfg.Server.Port != 9090 {
		t.Errorf(
			"Server.Port = %d, want %d",
			cfg.Server.Port,
			9090,
		)
	}

	if cfg.Pelican.URL != "https://panel.example.com" {
		t.Errorf(
			"Pelican.URL = %q, want expected URL",
			cfg.Pelican.URL,
		)
	}

	if cfg.Pelican.Timeout != 20*time.Second {
		t.Errorf(
			"Pelican.Timeout = %v, want %v",
			cfg.Pelican.Timeout,
			20*time.Second,
		)
	}

	if cfg.Discovery.Interval != 45*time.Second {
		t.Errorf(
			"Discovery.Interval = %v, want %v",
			cfg.Discovery.Interval,
			45*time.Second,
		)
	}

	if cfg.Router.Domain != "mc.example.com" {
		t.Errorf(
			"Router.Domain = %q, want %q",
			cfg.Router.Domain,
			"mc.example.com",
		)
	}
}

func changeWorkingDirectory(
	t *testing.T,
	directory string,
) func() {
	t.Helper()

	original, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}

	if err := os.Chdir(directory); err != nil {
		t.Fatalf("change working directory: %v", err)
	}

	return func() {
		if err := os.Chdir(original); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	}
}
