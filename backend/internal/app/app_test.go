package app

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/zurco34/pelican-mc-router/internal/settings"
	"github.com/zurco34/pelican-mc-router/pkg/config"
)

type fakeRuntimeSettingsStore struct {
	setupComplete bool
	setupErr      error
	settings      settings.Settings
	loadErr       error
	loadCalled    bool
}

func (f *fakeRuntimeSettingsStore) IsSetupComplete() (bool, error) {
	return f.setupComplete, f.setupErr
}

func (f *fakeRuntimeSettingsStore) Load() (settings.Settings, error) {
	f.loadCalled = true

	return f.settings, f.loadErr
}

func TestBuildRuntimeServicesSetupIncomplete(t *testing.T) {
	store := &fakeRuntimeSettingsStore{
		setupComplete: false,
	}

	discoveryService, routingService, err := buildRuntimeServices(
		store,
		5*time.Second,
	)
	if err != nil {
		t.Fatalf("buildRuntimeServices() error = %v", err)
	}

	if discoveryService != nil {
		t.Fatal("discovery service is not nil")
	}

	if routingService != nil {
		t.Fatal("routing service is not nil")
	}

	if store.loadCalled {
		t.Fatal("Load() was called while setup was incomplete")
	}
}

func TestBuildRuntimeServicesReturnsSetupStatusError(t *testing.T) {
	store := &fakeRuntimeSettingsStore{
		setupErr: errors.New("database unavailable"),
	}

	_, _, err := buildRuntimeServices(
		store,
		5*time.Second,
	)
	if err == nil {
		t.Fatal("buildRuntimeServices() error = nil, want an error")
	}

	if !strings.Contains(err.Error(), "determine setup status") {
		t.Fatalf(
			"error = %q, want setup status context",
			err,
		)
	}
}

func TestBuildRuntimeServicesReturnsLoadError(t *testing.T) {
	store := &fakeRuntimeSettingsStore{
		setupComplete: true,
		loadErr:       errors.New("settings unavailable"),
	}

	_, _, err := buildRuntimeServices(
		store,
		5*time.Second,
	)
	if err == nil {
		t.Fatal("buildRuntimeServices() error = nil, want an error")
	}

	if !store.loadCalled {
		t.Fatal("Load() was not called")
	}

	if !strings.Contains(err.Error(), "load runtime settings") {
		t.Fatalf(
			"error = %q, want runtime settings context",
			err,
		)
	}
}

func TestServerAddress(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.ServerConfig
		want string
	}{
		{
			name: "IPv4 wildcard",
			cfg: config.ServerConfig{
				Host: "0.0.0.0",
				Port: 8080,
			},
			want: "0.0.0.0:8080",
		},
		{
			name: "hostname",
			cfg: config.ServerConfig{
				Host: "localhost",
				Port: 9090,
			},
			want: "localhost:9090",
		},
		{
			name: "IPv6 wildcard",
			cfg: config.ServerConfig{
				Host: "::",
				Port: 8080,
			},
			want: "[::]:8080",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := serverAddress(test.cfg)

			if got != test.want {
				t.Errorf(
					"serverAddress() = %q, want %q",
					got,
					test.want,
				)
			}
		})
	}
}
