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

	if cfg.Server.ReadHeaderTimeout != 5*time.Second {
		t.Errorf("Server.ReadHeaderTimeout = %v, want 5s", cfg.Server.ReadHeaderTimeout)
	}
	if cfg.Server.ReadTimeout != 15*time.Second {
		t.Errorf("Server.ReadTimeout = %v, want 15s", cfg.Server.ReadTimeout)
	}
	if cfg.Server.WriteTimeout != 30*time.Second {
		t.Errorf("Server.WriteTimeout = %v, want 30s", cfg.Server.WriteTimeout)
	}
	if cfg.Server.IdleTimeout != time.Minute {
		t.Errorf("Server.IdleTimeout = %v, want 1m", cfg.Server.IdleTimeout)
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

	if cfg.Discovery.WildcardBackendHost != "" {
		t.Errorf(
			"Discovery.WildcardBackendHost = %q, want empty string",
			cfg.Discovery.WildcardBackendHost,
		)
	}
	if cfg.DashboardAuth.Enabled {
		t.Error("DashboardAuth.Enabled = true, want false")
	}
	if cfg.DashboardAuth.RoleClaim != "roles" {
		t.Errorf("DashboardAuth.RoleClaim = %q, want roles", cfg.DashboardAuth.RoleClaim)
	}
	if cfg.DashboardAuth.RequiredRole != "viewer" {
		t.Errorf("DashboardAuth.RequiredRole = %q, want viewer", cfg.DashboardAuth.RequiredRole)
	}
	if cfg.DashboardAuth.OperatorRole != "operator" {
		t.Errorf("DashboardAuth.OperatorRole = %q, want operator", cfg.DashboardAuth.OperatorRole)
	}
	if cfg.DashboardAuth.DiscoveryTimeout != 5*time.Second {
		t.Errorf("DashboardAuth.DiscoveryTimeout = %v, want 5s", cfg.DashboardAuth.DiscoveryTimeout)
	}
	if cfg.Secrets.Directory != "/run/secrets/pelican-mc-router" {
		t.Errorf("Secrets.Directory = %q, want default", cfg.Secrets.Directory)
	}
	if cfg.Secrets.BootstrapTokenName != "bootstrap-token" {
		t.Errorf("Secrets.BootstrapTokenName = %q, want default", cfg.Secrets.BootstrapTokenName)
	}

	if cfg.Database.Path != "./data/pelican-mc-router.db" {
		t.Errorf(
			"Database.Path = %q, want %q",
			cfg.Database.Path,
			"./data/pelican-mc-router.db",
		)
	}

	if cfg.Router.Backend != "mc-router" {
		t.Errorf(
			"Router.Backend = %q, want %q",
			cfg.Router.Backend,
			"mc-router",
		)
	}

	if cfg.MCRouter.APIURL != "http://mc-router:8080" {
		t.Errorf(
			"MCRouter.APIURL = %q, want %q",
			cfg.MCRouter.APIURL,
			"http://mc-router:8080",
		)
	}

	if cfg.Infrared.ProxiesPath != "/etc/infrared/proxies" {
		t.Errorf(
			"Infrared.ProxiesPath = %q, want %q",
			cfg.Infrared.ProxiesPath,
			"/etc/infrared/proxies",
		)
	}

	if cfg.Infrared.ReloadMarkerPath != "/etc/infrared/control/infrared.reload" {
		t.Errorf(
			"Infrared.ReloadMarkerPath = %q, want %q",
			cfg.Infrared.ReloadMarkerPath,
			"/etc/infrared/control/infrared.reload",
		)
	}
}

func TestLoadFromFile(t *testing.T) {

	t.Setenv("PELICAN_MC_ROUTER_PELICAN_URL", "")
	t.Setenv("PELICAN_MC_ROUTER_ROUTER_DOMAIN", "")

	directory := t.TempDir()

	configContent := []byte(`
server:
  host: "127.0.0.1"
  port: 9090
  read_header_timeout: 6s
  read_timeout: 16s
  write_timeout: 31s
  idle_timeout: 61s

pelican:
  url: "https://panel.example.com"
  api_key: "test-key"
  timeout: 20s

discovery:
  interval: 45s
  wildcard_backend_host: "172.50.0.1"

router:
  backend: "infrared"
  domain: "mc.example.com"

mcrouter:
  api_url: "http://mc-router:8080"

infrared:
  proxies_path: "/tmp/infrared/proxies"
  reload_marker_path: "/tmp/infrared/control/infrared.reload"

logging:
  level: "debug"
  format: "console"

database:
  path: "/tmp/router.db"
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
	if cfg.Server.ReadHeaderTimeout != 6*time.Second {
		t.Errorf("Server.ReadHeaderTimeout = %v, want 6s", cfg.Server.ReadHeaderTimeout)
	}
	if cfg.Server.ReadTimeout != 16*time.Second {
		t.Errorf("Server.ReadTimeout = %v, want 16s", cfg.Server.ReadTimeout)
	}
	if cfg.Server.WriteTimeout != 31*time.Second {
		t.Errorf("Server.WriteTimeout = %v, want 31s", cfg.Server.WriteTimeout)
	}
	if cfg.Server.IdleTimeout != 61*time.Second {
		t.Errorf("Server.IdleTimeout = %v, want 61s", cfg.Server.IdleTimeout)
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

	if cfg.Discovery.WildcardBackendHost != "172.50.0.1" {
		t.Errorf(
			"Discovery.WildcardBackendHost = %q, want %q",
			cfg.Discovery.WildcardBackendHost,
			"172.50.0.1",
		)
	}

	if cfg.Router.Domain != "mc.example.com" {
		t.Errorf(
			"Router.Domain = %q, want %q",
			cfg.Router.Domain,
			"mc.example.com",
		)
	}

	if cfg.MCRouter.APIURL != "http://mc-router:8080" {
		t.Errorf(
			"MCRouter.APIURL = %q, want %q",
			cfg.MCRouter.APIURL,
			"http://mc-router:8080",
		)
	}

	if cfg.Infrared.ProxiesPath != "/tmp/infrared/proxies" {
		t.Errorf(
			"Infrared.ProxiesPath = %q, want %q",
			cfg.Infrared.ProxiesPath,
			"/tmp/infrared/proxies",
		)
	}

	if cfg.Infrared.ReloadMarkerPath != "/tmp/infrared/control/infrared.reload" {
		t.Errorf(
			"Infrared.ReloadMarkerPath = %q, want %q",
			cfg.Infrared.ReloadMarkerPath,
			"/tmp/infrared/control/infrared.reload",
		)
	}

	if cfg.Database.Path != "/tmp/router.db" {
		t.Errorf(
			"Database.Path = %q, want %q",
			cfg.Database.Path,
			"/tmp/router.db",
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

func TestLoadWildcardBackendHostFromEnvironment(t *testing.T) {
	workingDirectory := changeWorkingDirectory(t, t.TempDir())
	defer workingDirectory()

	t.Setenv(
		"PELICAN_MC_ROUTER_DISCOVERY_WILDCARD_BACKEND_HOST",
		"172.50.0.1",
	)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Discovery.WildcardBackendHost != "172.50.0.1" {
		t.Errorf(
			"Discovery.WildcardBackendHost = %q, want %q",
			cfg.Discovery.WildcardBackendHost,
			"172.50.0.1",
		)
	}
}
