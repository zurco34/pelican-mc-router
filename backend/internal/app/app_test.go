package app

import (
	"testing"

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
