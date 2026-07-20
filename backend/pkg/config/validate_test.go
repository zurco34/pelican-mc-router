package config

import (
	"errors"
	"testing"
	"time"
)

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
			Host: "0.0.0.0",
			Port: 8080,
		},
		Pelican: PelicanConfig{
			URL:     "https://panel.example.com",
			APIKey:  "test-key",
			Timeout: 15 * time.Second,
		},
		Discovery: DiscoveryConfig{
			Interval: 30 * time.Second,
		},
		Router: RouterConfig{
			Backend: "infrared",
			Domain:  "mc.example.com",
		},
		Infrared: InfraredConfig{
			ConfigPath:   "/etc/infrared/config.json",
			ReloadSignal: "SIGHUP",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}
}
