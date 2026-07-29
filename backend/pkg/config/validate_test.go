package config

import (
	"errors"
	"testing"
	"time"
)

func TestConfigValidateInfrastructure(t *testing.T) {
	cfg := Config{
		Server: ServerConfig{
			Host:              "0.0.0.0",
			Port:              8080,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       time.Minute,
		},
		Database: DatabaseConfig{
			Path: "./data/pelican-mc-router.db",
		},
		Discovery: DiscoveryConfig{
			Interval: 30 * time.Second,
		},
		Retry: RetryConfig{Attempts: 3, InitialBackoff: time.Millisecond, MaxBackoff: time.Second},
		Infrared: InfraredConfig{
			ProxiesPath:      "/etc/infrared/proxies",
			ReloadMarkerPath: "/etc/infrared/control/infrared.reload",
		},
		Router: RouterConfig{
			Backend: "infrared",
		},
	}

	if err := cfg.ValidateInfrastructure(); err != nil {
		t.Fatalf("ValidateInfrastructure() error = %v", err)
	}
}
func TestConfigValidateInfrastructureDoesNotRequireSetupSettings(
	t *testing.T,
) {
	cfg := Config{
		Server: ServerConfig{
			Host:              "0.0.0.0",
			Port:              8080,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       time.Minute,
		},
		Database: DatabaseConfig{
			Path: "./data/pelican-mc-router.db",
		},
		Discovery: DiscoveryConfig{
			Interval: 30 * time.Second,
		},
		Retry: RetryConfig{Attempts: 3, InitialBackoff: time.Millisecond, MaxBackoff: time.Second},
		Infrared: InfraredConfig{
			ProxiesPath:      "/etc/infrared/proxies",
			ReloadMarkerPath: "/etc/infrared/control/infrared.reload",
		},
		Router: RouterConfig{
			Backend: "infrared",
		},
	}

	cfg.Pelican.URL = ""
	cfg.Pelican.APIKey = ""
	cfg.Router.Domain = ""

	if err := cfg.ValidateInfrastructure(); err != nil {
		t.Fatalf(
			"ValidateInfrastructure() error = %v",
			err,
		)
	}
}

func TestConfigValidateInfrastructureDashboardAuth(t *testing.T) {
	tests := []struct {
		name    string
		auth    DashboardAuthConfig
		wantErr error
	}{
		{name: "disabled"},
		{name: "enabled", auth: DashboardAuthConfig{Enabled: true, IssuerURL: "https://issuer.example", Audience: "pelican-mc-router", RoleClaim: "roles", RequiredRole: "viewer", DiscoveryTimeout: time.Second}},
		{name: "missing issuer", auth: DashboardAuthConfig{Enabled: true, Audience: "pelican-mc-router", RoleClaim: "roles", RequiredRole: "viewer", DiscoveryTimeout: time.Second}, wantErr: ErrMissingDashboardAuthIssuerURL},
		{name: "non HTTPS issuer", auth: DashboardAuthConfig{Enabled: true, IssuerURL: "http://issuer.example", Audience: "pelican-mc-router", RoleClaim: "roles", RequiredRole: "viewer", DiscoveryTimeout: time.Second}, wantErr: ErrInvalidDashboardAuthIssuerURL},
		{name: "missing audience", auth: DashboardAuthConfig{Enabled: true, IssuerURL: "https://issuer.example", RoleClaim: "roles", RequiredRole: "viewer", DiscoveryTimeout: time.Second}, wantErr: ErrMissingDashboardAuthAudience},
		{name: "missing role", auth: DashboardAuthConfig{Enabled: true, IssuerURL: "https://issuer.example", Audience: "pelican-mc-router", RoleClaim: "roles", DiscoveryTimeout: time.Second}, wantErr: ErrMissingDashboardAuthRequiredRole},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.DashboardAuth = test.auth
			err := cfg.ValidateInfrastructure()
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("ValidateInfrastructure() error = %v, want %v", err, test.wantErr)
			}
		})
	}
}

func TestConfigValidateInfrastructureRejectsMissingInfraredProxiesPath(
	t *testing.T,
) {
	cfg := validConfig()
	cfg.Infrared.ProxiesPath = "   "

	err := cfg.ValidateInfrastructure()
	if !errors.Is(err, ErrMissingInfraredProxiesPath) {
		t.Fatalf(
			"ValidateInfrastructure() error = %v, want error %v",
			err,
			ErrMissingInfraredProxiesPath,
		)
	}
}

func TestConfigValidateInfrastructureRejectsMissingInfraredReloadMarkerPath(
	t *testing.T,
) {
	cfg := validConfig()
	cfg.Infrared.ReloadMarkerPath = "   "

	err := cfg.ValidateInfrastructure()
	if !errors.Is(err, ErrMissingInfraredReloadMarkerPath) {
		t.Fatalf(
			"ValidateInfrastructure() error = %v, want error %v",
			err,
			ErrMissingInfraredReloadMarkerPath,
		)
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr error
	}{
		{
			name: "valid configuration",
			cfg:  validConfig(),
		},
		{
			name: "invalid server port",
			cfg: func() Config {
				cfg := validConfig()
				cfg.Server.Port = 0

				return cfg
			}(),
			wantErr: ErrInvalidServerPort,
		},
		{
			name: "invalid server read header timeout",
			cfg: func() Config {
				cfg := validConfig()
				cfg.Server.ReadHeaderTimeout = 0
				return cfg
			}(),
			wantErr: ErrInvalidServerReadHeaderTimeout,
		},
		{
			name: "invalid server read timeout",
			cfg: func() Config {
				cfg := validConfig()
				cfg.Server.ReadTimeout = 0
				return cfg
			}(),
			wantErr: ErrInvalidServerReadTimeout,
		},
		{
			name: "invalid server write timeout",
			cfg: func() Config {
				cfg := validConfig()
				cfg.Server.WriteTimeout = 0
				return cfg
			}(),
			wantErr: ErrInvalidServerWriteTimeout,
		},
		{
			name: "invalid server idle timeout",
			cfg: func() Config {
				cfg := validConfig()
				cfg.Server.IdleTimeout = 0
				return cfg
			}(),
			wantErr: ErrInvalidServerIdleTimeout,
		},

		{
			name: "missing database path",
			cfg: func() Config {
				cfg := validConfig()
				cfg.Database.Path = ""

				return cfg
			}(),
			wantErr: ErrMissingDatabasePath,
		},

		{
			name: "missing Pelican URL",
			cfg: func() Config {
				cfg := validConfig()
				cfg.Pelican.URL = ""

				return cfg
			}(),
			wantErr: ErrMissingPelicanURL,
		},
		{
			name: "invalid Pelican URL",
			cfg: func() Config {
				cfg := validConfig()
				cfg.Pelican.URL = "ftp://panel.example.com"

				return cfg
			}(),
			wantErr: ErrInvalidPelicanURL,
		},
		{
			name: "missing Pelican API key",
			cfg: func() Config {
				cfg := validConfig()
				cfg.Pelican.APIKey = ""

				return cfg
			}(),
			wantErr: ErrMissingPelicanAPIKey,
		},
		{
			name: "invalid Pelican timeout",
			cfg: func() Config {
				cfg := validConfig()
				cfg.Pelican.Timeout = 0

				return cfg
			}(),
			wantErr: ErrInvalidPelicanTimeout,
		},
		{
			name: "invalid discovery interval",
			cfg: func() Config {
				cfg := validConfig()
				cfg.Discovery.Interval = 0

				return cfg
			}(),
			wantErr: ErrInvalidDiscoveryInterval,
		},
		{
			name: "missing router domain",
			cfg: func() Config {
				cfg := validConfig()
				cfg.Router.Domain = ""

				return cfg
			}(),
			wantErr: ErrMissingRouterDomain,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.cfg.Validate()

			if test.wantErr == nil {
				if err != nil {
					t.Fatalf("Validate() error = %v", err)
				}

				return
			}

			if !errors.Is(err, test.wantErr) {
				t.Fatalf(
					"Validate() error = %v, want error %v",
					err,
					test.wantErr,
				)
			}
		})
	}
}

func TestConfigValidateReturnsMultipleErrors(t *testing.T) {
	cfg := validConfig()
	cfg.Server.Port = 0
	cfg.Pelican.APIKey = ""
	cfg.Router.Domain = ""

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want validation errors")
	}

	expectedErrors := []error{
		ErrInvalidServerPort,
		ErrMissingPelicanAPIKey,
		ErrMissingRouterDomain,
	}

	for _, expectedErr := range expectedErrors {
		if !errors.Is(err, expectedErr) {
			t.Errorf(
				"Validate() error = %v, want error %v",
				err,
				expectedErr,
			)
		}
	}
}

func validConfig() Config {
	return Config{
		Server: ServerConfig{
			Host:              "0.0.0.0",
			Port:              8080,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       time.Minute,
		},
		Database: DatabaseConfig{
			Path: "./data/pelican-mc-router.db",
		},
		Pelican: PelicanConfig{
			URL:     "https://panel.example.com",
			APIKey:  "test-key",
			Timeout: 15 * time.Second,
		},
		Discovery: DiscoveryConfig{
			Interval: 30 * time.Second,
		},
		Retry: RetryConfig{Attempts: 3, InitialBackoff: time.Millisecond, MaxBackoff: time.Second},
		Router: RouterConfig{
			Backend: "infrared",
			Domain:  "mc.example.com",
		},
		MCRouter: MCRouterConfig{
			APIURL: "http://mc-router:8080",
		},
		Infrared: InfraredConfig{
			ProxiesPath:      "/etc/infrared/proxies",
			ReloadMarkerPath: "/etc/infrared/control/infrared.reload",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}
}

func TestConfigValidateInfrastructureValidatesSelectedRouterBackend(
	t *testing.T,
) {
	tests := []struct {
		name    string
		update  func(*Config)
		wantErr error
	}{
		{
			name: "mc-router does not require Infrared settings",
			update: func(cfg *Config) {
				cfg.Router.Backend = "mc-router"
				cfg.Infrared = InfraredConfig{}
			},
		},
		{
			name: "mc-router requires API URL",
			update: func(cfg *Config) {
				cfg.Router.Backend = "mc-router"
				cfg.MCRouter.APIURL = "   "
			},
			wantErr: ErrMissingMCRouterAPIURL,
		},
		{
			name: "mc-router rejects invalid API URL",
			update: func(cfg *Config) {
				cfg.Router.Backend = "mc-router"
				cfg.MCRouter.APIURL = "ftp://mc-router:8080"
			},
			wantErr: ErrInvalidMCRouterAPIURL,
		},
		{
			name: "Infrared does not require mc-router API URL",
			update: func(cfg *Config) {
				cfg.Router.Backend = "infrared"
				cfg.MCRouter.APIURL = ""
			},
		},
		{
			name: "unsupported router backend",
			update: func(cfg *Config) {
				cfg.Router.Backend = "unknown"
			},
			wantErr: ErrUnsupportedRouterBackend,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := validConfig()
			test.update(&cfg)

			err := cfg.ValidateInfrastructure()

			if test.wantErr == nil {
				if err != nil {
					t.Fatalf(
						"ValidateInfrastructure() error = %v",
						err,
					)
				}

				return
			}

			if !errors.Is(err, test.wantErr) {
				t.Fatalf(
					"ValidateInfrastructure() error = %v, want %v",
					err,
					test.wantErr,
				)
			}
		})
	}
}
