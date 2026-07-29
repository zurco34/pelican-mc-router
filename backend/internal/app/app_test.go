package app

import (
	"net/http"
	"testing"
	"time"

	"github.com/zurco34/pelican-mc-router/pkg/config"
)

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

func TestNewHTTPServerConfiguresTimeouts(t *testing.T) {
	handler := http.NewServeMux()
	cfg := config.ServerConfig{
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       time.Minute,
	}

	server := newHTTPServer("127.0.0.1:8080", handler, cfg)

	if server.Addr != "127.0.0.1:8080" {
		t.Errorf("Addr = %q, want 127.0.0.1:8080", server.Addr)
	}
	if server.Handler != handler {
		t.Error("Handler was not preserved")
	}
	if server.ReadHeaderTimeout != cfg.ReadHeaderTimeout {
		t.Errorf("ReadHeaderTimeout = %v, want %v", server.ReadHeaderTimeout, cfg.ReadHeaderTimeout)
	}
	if server.ReadTimeout != cfg.ReadTimeout {
		t.Errorf("ReadTimeout = %v, want %v", server.ReadTimeout, cfg.ReadTimeout)
	}
	if server.WriteTimeout != cfg.WriteTimeout {
		t.Errorf("WriteTimeout = %v, want %v", server.WriteTimeout, cfg.WriteTimeout)
	}
	if server.IdleTimeout != cfg.IdleTimeout {
		t.Errorf("IdleTimeout = %v, want %v", server.IdleTimeout, cfg.IdleTimeout)
	}
}
